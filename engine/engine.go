// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"encoding/json"
	"maps"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/drone/runner-go/pipeline/runtime"

	"github.com/sirupsen/logrus"
	"github.com/alessio/shellescape"
)

const BOOT_MAX_DELAY time.Duration = 3 * time.Minute

func getMakeDirectoriesCommand(files []*File) string {
	var command []string
	for _, file := range files {
		if command != nil {
			command = append(command, "&&")
		}
		command = append(command, "mkdir -p")
		command = append(command, shellescape.Quote(path.Dir(file.Path)))
	}
	return strings.Join(command, " ")
}

func getStepCommand(command string, args []string, envs map[string]string, workingDir string) string {
	commandWithArgs := strings.Join(
		[]string{
			shellescape.Quote(command),
			shellescape.QuoteCommand(args),
		},
		" ",
	)

	var envCommand []string
	envCommand = append(envCommand, "env")
	for name, value := range envs {
		envCommand = append(envCommand, shellescape.Quote(
			fmt.Sprintf("%s=%s", name, value),
		))
	}
	envCommand = append(envCommand, commandWithArgs)

	return (
		"mkdir -p " +
		shellescape.Quote(workingDir) +
		" && " +
		"cd " +
		shellescape.Quote(workingDir) +
		" && " +
		strings.Join(envCommand, " "))
}

// Opts configures the Engine.
type Opts struct {
	ImageDir string
	TempDir  string
}

// Machine configuration, loaded from JSON
type MachineConfig struct {
	Username        string `json:"username,omitempty"`
	BaseImage       string `json:"base_image,omitempty"`
	BaseImageFormat string `json:"base_image_format,omitempty"`
}

func loadMachineConfig(filename string) (MachineConfig, error) {
	var result MachineConfig
	data, err := os.ReadFile(filename)
	if err != nil {
		if err.(*os.PathError) != nil {
			return result, err
		}
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return result, err
	}

	if result.Username == "" {
		result.Username = "root"
	}

	if result.BaseImage == "" {
		result.BaseImage = filename[:len(filename)-10] + ".img"
	}

	if result.BaseImageFormat == "" {
		if strings.HasSuffix(result.BaseImage, ".qcow2") {
			result.BaseImageFormat = "qcow2"
		} else {
			result.BaseImageFormat = "raw"
		}
	}

	return result, nil
}

// Engine implements a pipeline engine.
type Engine struct {
	ImageDir            string
	TempDir             string
	MachineConfig       MachineConfig
	Image               string
	SshPort             int
	QemuProcess         *os.Process
	QemuProcessExitChan chan error
}

// New returns a new engine.
func New(opts Opts) (*Engine, error) {
	tempDir := opts.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	return &Engine{
		ImageDir: opts.ImageDir,
		TempDir: tempDir,
	}, nil
}

func (e *Engine) ssh(ctx context.Context, command string) error {
	logrus.WithFields(logrus.Fields{
		"command": command,
	}).Debug("running SSH command")
	return exec.CommandContext(
		ctx,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=2",
		"-i", "id_rsa",
		"-p", strconv.Itoa(e.SshPort),
		fmt.Sprintf("%s@localhost", e.MachineConfig.Username),
		command,
	).Run()
}

func (e *Engine) sshOutput(ctx context.Context, command string, output io.Writer) error {
	logrus.WithFields(logrus.Fields{
		"command": command,
	}).Debug("running SSH command")
	cmd := exec.CommandContext(
		ctx,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=2",
		"-i", "id_rsa",
		"-p", strconv.Itoa(e.SshPort),
		fmt.Sprintf("%s@localhost", e.MachineConfig.Username),
		command,
	)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

func writeTemp(dir string, pattern string, data []byte) (string, error) {
	file, err := os.CreateTemp(dir, pattern)
	defer file.Close()
	if err != nil {
		return "", err
	}
	file.Write(data)
	return file.Name(), nil
}

func (e *Engine) scpUpload(ctx context.Context, data []byte, to string) error {
	tempFile, err := writeTemp(e.TempDir, "drone-qemu-upload-*", data)
	if err != nil {
		return fmt.Errorf("couldn't create temporary file to upload: %w", err)
	}
	defer os.Remove(tempFile)

	cmd := exec.CommandContext(
		ctx,
		"scp",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-i", "id_rsa",
		"-P", strconv.Itoa(e.SshPort),
		tempFile,
		fmt.Sprintf("%s@localhost:%s", e.MachineConfig.Username, to),
	)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *Engine) uploadFiles(ctx context.Context, files []*File) error {
	// Make directories for uploaded files
	makeDirectoryCommand := getMakeDirectoriesCommand(files)
	if err := e.ssh(ctx, makeDirectoryCommand); err != nil {
		return fmt.Errorf("failed to create directories for uploaded files: %w", err)
	}

	// Upload files
	for _, file := range files {
		if file.IsDir {
			continue
		}

		// Upload
		err := e.scpUpload(ctx, file.Data, file.Path)
		if err != nil {
			return fmt.Errorf("sftp failed: %w", err)
		}
	}

	return nil
}

// Setup the pipeline environment.
func (e *Engine) Setup(ctx context.Context, specv runtime.Spec) error {
	spec := specv.(*Spec)

	// Load configuration
	var err error
	e.MachineConfig, err = loadMachineConfig(path.Join(e.ImageDir, spec.Settings.Image + ".qemu.json"))
	if err != nil {
		return fmt.Errorf("error loading machine config JSON: %w", err)
	}

	// Pick random port
	e.SshPort = rand.Intn(65536 - 1025) + 1025

	// Pick random image name
	e.Image = path.Join(e.TempDir, fmt.Sprintf("drone-qemu-%d.qcow2", rand.Int()))

	// Create the temporary image
	logrus.WithFields(logrus.Fields{
		"image": e.Image,
	}).Info("creating image")
	err = exec.CommandContext(
		ctx,
		"qemu-img", "create",
		"-f", "qcow2",
		"-b", e.MachineConfig.BaseImage,
		"-F", e.MachineConfig.BaseImageFormat,
		e.Image,
	).Run()
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w", err)
	}

	// Start Qemu
	logrus.Info("starting qemu-system-x86_64")
	cmd := exec.CommandContext(
		ctx,
		path.Join(e.ImageDir, spec.Settings.Image + ".qemu.sh"),
	)
	cmd.Env = append(cmd.Env, "QEMU_IMAGE=" + e.Image)
	cmd.Env = append(cmd.Env, "QEMU_SSH_PORT=" + strconv.Itoa(e.SshPort))
	//cmd.Stdout = os.Stdout // DEBUG
	//cmd.Stderr = os.Stderr // DEBUG
	err = cmd.Start()
	if err != nil {
		os.Remove(e.Image)
		e.Image = ""
		return fmt.Errorf("qemu process failed to start: %w", err)
	}
	e.QemuProcess = cmd.Process
	e.QemuProcessExitChan = make(chan error)
	go func() {
		e.QemuProcessExitChan <- cmd.Wait()
	}()

	// Try to connect via SSH until it succeeds
	bootChannel := make(chan bool)
	start := time.Now()
	go func() {
		for time.Since(start) <= BOOT_MAX_DELAY {
			time.Sleep(5 * time.Second)

			if err := e.ssh(ctx, "true"); err == nil {
				bootChannel <- true
				return
			}
			logrus.Infof("connection failing: %v", err)
		}
		bootChannel <- false
	}()

	select {
	case err = <-e.QemuProcessExitChan:
		return fmt.Errorf("qemu process died: %w", err)
	case booted := <- bootChannel:
		if booted {
			logrus.WithFields(logrus.Fields{
				"duration": time.Since(start),
			}).Info("machine has started")
		} else {
			return errors.New("machine did not come online")
		}
	}

	// Upload files
	err = e.uploadFiles(ctx, spec.Files)
	if err != nil {
		return err
	}

	return nil
}

// Destroy the pipeline environment.
func (e *Engine) Destroy(ctx context.Context, specv runtime.Spec) error {
	// Stop the Qemu process
	if e.QemuProcess != nil {
		e.QemuProcess.Signal(syscall.SIGINT)
		e.QemuProcess.Wait()
	}

	// Delete the temporary image
	if e.Image != "" {
		os.Remove(e.Image)
	}

	return nil
}

// Run runs the pipeline step.
func (e *Engine) Run(ctx context.Context, specv runtime.Spec, stepv runtime.Step, output io.Writer) (*runtime.State, error) {
	// spec := specv.(*Spec)
	step := stepv.(*Step)

	// Upload files
	err := e.uploadFiles(ctx, step.Files)
	if err != nil {
		return nil, err
	}

	// Add secrets to env
	envs := step.Envs
	if len(step.Secrets) > 0 {
		envs = make(map[string]string)
		maps.Copy(envs, step.Envs)
		for _, secret := range step.Secrets {
			envs[secret.Env] = string(secret.Data)
		}
	}

	// Build full command
	fullCommand := getStepCommand(
		step.Command,
		step.Args,
		envs,
		step.WorkingDir,
	)
	logrus.WithFields(logrus.Fields{
		"command": fullCommand,
	}).Debug("running command")

	// SSH and run command
	err = e.sshOutput(ctx, fullCommand, output)

	exitCode := 0
	if err != nil {
		exit_err := err.(*exec.ExitError)
		exitCode = exit_err.ExitCode()
	}

	return &runtime.State{
		ExitCode: exitCode,
		Exited:   true,
	}, nil
}

// Ping pings the underlying runtime to verify connectivity.
func (e *Engine) Ping(ctx context.Context) error {
	return nil
}

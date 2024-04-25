// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package engine

import (
	"context"
	"errors"
	"io"
	"fmt"
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

// Engine implements a pipeline engine.
type Engine struct {
	ImageDir    string
	TempDir     string
	username    string
	Image       string
	SshPort     int
	QemuProcess *os.Process
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
		fmt.Sprintf("%s@localhost", e.username),
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
		fmt.Sprintf("%s@localhost", e.username),
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

func (e *Engine) scpUploadOutput(ctx context.Context, data []byte, to string, output io.Writer) error {
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
		fmt.Sprintf("%s@localhost:%s", e.username, to),
	)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

// Setup the pipeline environment.
func (e *Engine) Setup(ctx context.Context, specv runtime.Spec) error {
	spec := specv.(*Spec)

	// Find the base image file
	baseImageImg := path.Join(e.ImageDir, fmt.Sprintf("%s.img", spec.Settings.Image))
	baseImageQcow2 := path.Join(e.ImageDir, fmt.Sprintf("%s.qcow2", spec.Settings.Image))
	var baseImage, baseImageFormat string
	if _, e := os.Stat(baseImageQcow2); e == nil {
		baseImage = baseImageQcow2
		baseImageFormat = "qcow2"
	} else if _, e := os.Stat(baseImageImg); e == nil {
		baseImage = baseImageImg
		baseImageFormat = "raw"
	} else {
		return errors.New("No such image file")
	}
	logrus.WithFields(logrus.Fields{
		"image": baseImage,
		"imageFormat": baseImageFormat,
	}).Info("base image selected")

	// Load configuration
	e.username = "root"
	usernameBytes, err := os.ReadFile(baseImage + ".username")
	if err != nil {
		if err.(*os.PathError) == nil {
			return fmt.Errorf("error opening %s: %w", baseImage + ".username", err)
		}
	} else {
		e.username = strings.TrimSpace(string(usernameBytes))
	}
	logrus.WithFields(logrus.Fields{
		"username": e.username,
	}).Info("username selected")

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
		"-b", baseImage,
		"-F", baseImageFormat,
		e.Image,
	).Run()
	if err != nil {
		return fmt.Errorf("qemu-img failed: %w", err)
	}

	// Start Qemu
	logrus.Info("starting qemu-system-x86_64")
	cmd := exec.CommandContext(
		ctx,
		// TODO config
		"qemu-system-x86_64", "-enable-kvm",
		"-cpu", "host",
		"-no-reboot",
		"-drive", fmt.Sprintf("id=root,file=%s,format=qcow2", e.Image),
		"-drive", "id=cidata,file=cloud-init.iso,media=cdrom",
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp:127.0.0.1:%d-:22", e.SshPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-device", "virtio-serial-pci",
		"-nographic",
		"-vga", "none",
		"-display", "none",
		"-m", "1024",
		"-smp", "2",
	)
	//cmd.Stdout = os.Stdout // DEBUG
	//cmd.Stderr = os.Stderr // DEBUG
	err = cmd.Start()
	if err != nil {
		os.Remove(e.Image)
		e.Image = ""
		return fmt.Errorf("qemu-system-x86_64 failed to start: %w", err)
	}
	e.QemuProcess = cmd.Process

	// Wait for SSH connection to succeed
	start := time.Now()
	booted := false
	for time.Since(start) <= 3 * time.Minute {
		time.Sleep(5 * time.Second)

		if err := e.ssh(ctx, "true"); err == nil {
			booted = true
			break
		}
		logrus.Infof("connection failing: %v", err)
	}
	if !booted {
		return errors.New("machine did not come online")
	}
	logrus.WithFields(logrus.Fields{
		"duration": time.Since(start),
	}).Info("machine has started")

	// TODO: upload spec.Files

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

	// Make directories for uploaded files
	makeDirectoryCommand := getMakeDirectoriesCommand(step.Files)
	if err := e.ssh(ctx, makeDirectoryCommand); err != nil {
		return nil, fmt.Errorf("failed to create directories for uploaded files: %w", err)
	}

	// Upload files
	for _, file := range step.Files {
		if file.IsDir {
			continue
		}

		// Upload
		err := e.scpUploadOutput(ctx, file.Data, file.Path, output)
		if err != nil {
			return nil, fmt.Errorf("sftp failed: %w", err)
		}
	}

	// TODO: step.Secrets

	// Build full command
	fullCommand := getStepCommand(
		step.Command,
		step.Args,
		step.Envs,
		step.WorkingDir,
	)
	logrus.WithFields(logrus.Fields{
		"command": fullCommand,
	}).Debug("running command")

	// SSH and run command
	err := e.sshOutput(ctx, fullCommand, output)

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

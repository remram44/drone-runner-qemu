package engine

import (
	"testing"
)

func Test_makeDirectories(t *testing.T) {
	result := getMakeDirectoriesCommand([]*File{
		&File{
			Path: "/some/dir/file one",
		},
		&File{
			Path: "/some/other dir/file two",
		},
	})
	expected := "mkdir -p /some/dir && mkdir -p '/some/other dir'"
	if result != expected {
		t.Errorf("%#v != %#v", result, expected)
	}
}

func Test_stepCommand(t *testing.T) {
	result1 := getStepCommand(
		"/bin/sh",
		[]string{
			"-e",
			"/some/file.sh",
		},
		map[string]string{
		},
		"/wd",
	)
	expected1 := "mkdir -p /wd && cd /wd && env /bin/sh -e /some/file.sh"
	if result1 != expected1 {
		t.Errorf("%#v != %#v", result1, expected1)
	}

	result2 := getStepCommand(
		"/bin/the shell",
		[]string{
			"-e",
			"/some/file name",
		},
		map[string]string{
			"key": "one",
			"key2": "and two",
		},
		"/working dir",
	)
	expected2a := "mkdir -p '/working dir' && cd '/working dir' && env key=one 'key2=and two' '/bin/the shell' -e '/some/file name'"
	expected2b := "mkdir -p '/working dir' && cd '/working dir' && env 'key2=and two' key=one '/bin/the shell' -e '/some/file name'"
	if result2 != expected2a && result2 != expected2b {
		t.Errorf("%#v != %#v", result2, expected2a)
	}
}

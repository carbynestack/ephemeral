//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Executor is an interface for calling a command and process its output.
type Executor interface {
	// CallCMD executes the command and returns the output's STDOUT, STDERR streams as well as any errors
	CallCMD(ctx context.Context, cmd []string, dir string) ([]byte, []byte, error)
}

var (
	defaultCommand = "script"
	defaultOptions = []string{"-e", "-q", "-c"}
)

// NewCommander returns a new commander.
func NewCommander() *Commander {
	return &Commander{
		Command: defaultCommand,
		Options: defaultOptions,
	}
}

// Commander is a wrapper around os/exec.Command().
// It returns a byte array containing the output of stdout and error.
// An error in command execution will land there.
// Stderr is scanned as well, its output is printed out.
type Commander struct {
	Command string
	Options []string
}

// Run is a facade command that runs a single command from the current directory.
func (c *Commander) Run(cmd string) ([]byte, []byte, error) {
	return c.CallCMD(context.TODO(), []string{cmd}, "./")
}

// CallCMD calls a specified command in sh and returns its stdout and stderr as a byte slice and potentially an error.
// As per os/exec doc:
// ```
// If the command fails to run or doesn't complete successfully, the error is of type *ExitError. Other error types may be returned for I/O problems.
// ```
func (c *Commander) CallCMD(ctx context.Context, cmd []string, dir string) ([]byte, []byte, error) {
	baseCmd := c.Options
	baseCmd = append(baseCmd, cmd...)
	command := exec.CommandContext(ctx, c.Command, baseCmd...)
	stderrBuffer := bytes.NewBuffer([]byte{})
	stdoutBuffer := bytes.NewBuffer([]byte{})
	command.Stderr = stderrBuffer
	command.Stdout = stdoutBuffer
	command.Dir = dir
	err := command.Start()
	if err != nil {
		return nil, nil, err
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)
	go func() {
		// Check if the command finished successfully.
		err = command.Wait()
		defer waitGroup.Done()
		if err != nil {
			println(fmt.Sprintf("Error occured!"))
			println(fmt.Sprintf("StdOut: %s", stdoutBuffer.Bytes()))
			println(fmt.Sprintf("StdErr: %s", stderrBuffer.Bytes()))
		}
	}()
	waitGroup.Wait()

	if err != nil {
		switch err.(type) {
		case *exec.ExitError:
			return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), err
		default:
			return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), errors.New("error executing a command")
		}
	}
	return stdoutBuffer.Bytes(), stderrBuffer.Bytes(), nil
}

// ReadFile reads file content for a given file location.
func ReadFile(path string) ([]byte, error) {
	str, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(str)
	defer file.Close()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(file)
}

//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

package utils

import (
	"bufio"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"strings"
	"time"
)

// Fio is a pointer to the shared FileIO implementation
var Fio FileIO = &OSFileIO{}

// File is an interface for basic file based io methods
type File interface {
	io.ReadWriteCloser
	io.StringWriter
	SetWriteDeadline(t time.Time) error
}

// FileIO is an interface for filesystem methods
type FileIO interface {
	CreatePath(path string) error
	CreatePipe(path string) error
	Delete(path string) error
	OpenRead(path string) (File, error)
	OpenWriteOrCreate(name string) (File, error)
	OpenWritePipe(name string) (File, error)
	ReadLine(file File) (string, error)
}

// OSFileIO implements fileIO backed by default os methods
type OSFileIO struct{}

// CreatePath creates a directory and all parents if required. Returns nil on success or an error otherwise.
// This implementation is backed by os.MkdirAll.
func (OSFileIO) CreatePath(path string) error { return os.MkdirAll(path, 0755) }

// CreatePipe creates a named pipe. Returns nil on success or an error otherwise.
// This implementation is backed by unix.Mkfifo.
func (OSFileIO) CreatePipe(path string) error { return unix.Mkfifo(path, 0666) }

// Delete deletes a single file or directory with all contained elements. Returns nil on success or an error otherwise.
// This implementation is backed by os.Remove.
func (OSFileIO) Delete(path string) error { return os.RemoveAll(path) }

// OpenRead opens a file for reading. Returns a file which can be accessed for further processing. If opening the file
// fails, an error is returned instead.
// This implementation is backed by os.Open.
func (OSFileIO) OpenRead(path string) (File, error) { return os.Open(path) }

// OpenWriteOrCreate opens a file for write access. The given file is created in case it does not exist. On success, a file
// is returned for further interaction. Otherwise, an error is returned.
// This implementation is backed by os.OpenFile.
func (OSFileIO) OpenWriteOrCreate(path string) (File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
}

// OpenWritePipe opens a named pipe for write access. Returns a file which can be accessed for further processing.
// Otherwise, an error is returned.
// This implementation is backed by os.OpenFile.
func (OSFileIO) OpenWritePipe(path string) (File, error) {
	return os.OpenFile(path, os.O_WRONLY, os.ModeNamedPipe)
}

// ReadLine reads a line from a file. Returns the line read on success. If an error occurred before finding end of line,
// an error is returned. This can also include io.EOF.
// This implementation is backed by bufio.Reader.
func (OSFileIO) ReadLine(file File) (string, error) {
	r := bufio.NewReader(file)
	str, err := r.ReadString('\n')
	return strings.TrimRight(str, "\n"), err
}

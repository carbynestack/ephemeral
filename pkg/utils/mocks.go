//
// Copyright (c) 2021-2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

package utils

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"time"
)

// MockedRoundTripper mocks http.RoundTripper for testing which always returns successful
type MockedRoundTripper struct {
	ExpectedPath         string
	ExpectedRawQuery     string
	ReturnJSON           []byte
	ExpectedResponseCode int
}

// RoundTrip mocks the execution of a single HTTP request and returns the ExpectedResponseCode
func (m *MockedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var statusCode = m.ExpectedResponseCode
	p := req.URL.Path
	if p != m.ExpectedPath {
		statusCode = http.StatusNotFound
	}
	if m.ExpectedRawQuery != "" && req.URL.RawQuery != m.ExpectedRawQuery {
		statusCode = http.StatusNotFound
	}

	b := bytes.NewBuffer(m.ReturnJSON)
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(b),
	}
	return resp, nil
}

// MockedBrokenRoundTripper mocks http.RoundTripper for testing which will always result in an error
type MockedBrokenRoundTripper struct {
}

// RoundTrip mocks the execution of a single HTTP request and returns an error for each request
func (m *MockedBrokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("some error")
}

// FileErrorPair is a tuple of File and error as returned by some MockedFileIO methods.
type FileErrorPair struct {
	File  File
	Error error
}

// OpenReadResponse is used to define the default response returned by MockedFileIO.OpenRead calls.
type OpenReadResponse FileErrorPair

// OpenWriteOrCreateResponse is used to define the default response returned by MockedFileIO.OpenWriteOrCreate calls.
type OpenWriteOrCreateResponse FileErrorPair

// OpenWritePipeResponse is used to define the default response returned by MockedFileIO.OpenWritePipe calls.
type OpenWritePipeResponse FileErrorPair

// CreatePathResponse is used to define the default response returned by MockedFileIO.CreatePath calls.
type CreatePathResponse error

// CreatePipeResponse is used to define the default response returned by MockedFileIO.CreatePipe calls.
type CreatePipeResponse error

// DeleteResponse is used to define the default response returned by MockedFileIO.Delete calls.
type DeleteResponse error

// ReadLineResponse is a tuple of a string and an error as returned by MockedFileIO.ReadLine.
type ReadLineResponse struct {
	Line  string
	Error error
}

// MockedFileIO implements fileIO as a mock for testing
type MockedFileIO struct {
	CreatePathResponse        CreatePathResponse
	CreatePathCalls           []string
	CreatePipeResponse        CreatePipeResponse
	CreatePipeCalls           []string
	DeleteResponse            DeleteResponse
	DeleteCalls               []string
	OpenReadResponse          OpenReadResponse
	OpenReadCalls             []string
	OpenWriteOrCreateResponse OpenWriteOrCreateResponse
	OpenWriteOrCreateCalls    []string
	OpenWritePipeResponse     OpenWritePipeResponse
	OpenWritePipeCalls        []string
	ReadLineResponse          ReadLineResponse
	ReadLineCalls             []File
}

// CreatePath mocks the creation of a directory. Returns MockedFileIO.CreatePathResponse.
func (mfio *MockedFileIO) CreatePath(path string) error {
	mfio.CreatePathCalls = append(mfio.CreatePathCalls, path)
	return mfio.CreatePathResponse
}

// CreatePipe mocks the creation of a named pipe. Returns MockedFileIO.CreatePipeResponse.
func (mfio *MockedFileIO) CreatePipe(path string) error {
	mfio.CreatePipeCalls = append(mfio.CreatePipeCalls, path)
	return mfio.CreatePipeResponse
}

// Delete mocks the deletion of a file. Returns MockedFileIO.DeleteResponse.
func (mfio *MockedFileIO) Delete(path string) error {
	mfio.DeleteCalls = append(mfio.DeleteCalls, path)
	return mfio.DeleteResponse
}

// OpenRead mocks opening a file for reading. Returns the attributes from MockedFileIO.OpenReadResponse.
func (mfio *MockedFileIO) OpenRead(path string) (File, error) {
	mfio.OpenReadCalls = append(mfio.OpenReadCalls, path)
	return mfio.OpenReadResponse.File, mfio.OpenReadResponse.Error
}

// OpenWriteOrCreate mocks opening a file for write access. Returns the attributes from
// MockedFileIO.OpenWriteOrCreateResponse.
func (mfio *MockedFileIO) OpenWriteOrCreate(path string) (File, error) {
	mfio.OpenWriteOrCreateCalls = append(mfio.OpenWriteOrCreateCalls, path)
	return mfio.OpenWriteOrCreateResponse.File, mfio.OpenWriteOrCreateResponse.Error
}

// OpenWritePipe mocks opening a named pipe for write access. Returns the attributes from
// MockedFileIO.OpenWritePipeResponse.
func (mfio *MockedFileIO) OpenWritePipe(path string) (File, error) {
	mfio.OpenWritePipeCalls = append(mfio.OpenWritePipeCalls, path)
	return mfio.OpenWritePipeResponse.File, mfio.OpenWritePipeResponse.Error
}

// ReadLine mocks reading a string from a file. Returns the attributes from
// // MockedFileIO.ReadStringResponse.
func (mfio *MockedFileIO) ReadLine(file File) (string, error) {
	mfio.ReadLineCalls = append(mfio.ReadLineCalls, file)
	return mfio.ReadLineResponse.Line, mfio.ReadLineResponse.Error
}

// SimpleFileMock mocks os.File i/o for testing.
//
// The error given in IOError is returned on all methods. Data written is appended to IOData on with each call an the
// length of the passed data returned as number of bytes written.
type SimpleFileMock struct {
	ReadWriteCountResponse int
	IOError                error
	SetDeadlineError       error
	CloseError             error
	WrittenData            []byte
	WriteDeadline          time.Time
}

// Write mocks the function Write call, appends given data to WriteData and returns defined CloseError.
func (sfm *SimpleFileMock) Write(d []byte) (int, error) {
	sfm.WrittenData = append(sfm.WrittenData, d...)
	return sfm.ReadWriteCountResponse, sfm.IOError
}

// WriteString mocks the WriteString call, appends given data to WriteData and returns defined CloseError.
func (sfm *SimpleFileMock) WriteString(d string) (int, error) {
	sfm.WrittenData = append(sfm.WrittenData, []byte(d)...)
	return sfm.ReadWriteCountResponse, sfm.IOError
}

// Close mocks the close call and returns defined CloseError
func (sfm *SimpleFileMock) Close() error {
	return sfm.CloseError
}

// Read mocks read calls. Data stored in WrittenData is copied to the given target array 't' at max of length of 't'.
// Returns length of copied data and IOError.
func (sfm *SimpleFileMock) Read(t []byte) (int, error) {
	l := len(t)
	if l > len(sfm.WrittenData) {
		l = len(sfm.WrittenData)
	}
	return copy(t, sfm.WrittenData[:l]), sfm.IOError
}

// SetWriteDeadline sets WriteDeadline
func (sfm *SimpleFileMock) SetWriteDeadline(t time.Time) error {
	sfm.WriteDeadline = t
	return sfm.SetDeadlineError
}

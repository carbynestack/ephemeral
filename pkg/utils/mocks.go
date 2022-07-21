//
// Copyright (c) 2021 - for information on the respective copyright owner
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
)

// MockedRoundTripper mocks http.RoundTripper for testing which always returns successful
type MockedRoundTripper struct {
	ExpectedPath         string
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

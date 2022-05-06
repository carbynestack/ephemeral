//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
)

type MockedRoundTripper struct {
	ExpectedPath string
	ReturnJson   []byte
}

func (m *MockedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var statusCode = http.StatusOK
	p := req.URL.Path
	if p != m.ExpectedPath {
		statusCode = http.StatusNotFound
	}

	b := bytes.NewBuffer(m.ReturnJson)
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(b),
	}
	return resp, nil
}

type MockedBrokenRoundTripper struct {
}

func (m *MockedBrokenRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("some error")
}

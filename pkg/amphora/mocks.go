//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package amphora

import (
	"bytes"
	"io/ioutil"
	"net/http"
)

type MockedRoundTripper struct {
	ExpectedPath         string
	ReturnJson           []byte
	ExpectedResponseCode int
}

func (m *MockedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var statusCode = m.ExpectedResponseCode
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

// Copyright (c) 2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0

package opa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	TagsAction    = "tags"
	ExecuteAction = "execute"
)

type OpaRequest struct {
	Input interface{} `json:"input"`
}

type TagResponse struct {
	Tags []amphora.Tag `json:"result"`
}

type ExecuteResponse struct {
	IsAllowed bool `json:"result"`
}

// AbstractClient is an interface that defines the methods that an OPA client must implement.
type AbstractClient interface {
	GenerateTags(input interface{}) ([]amphora.Tag, error)
	CanExecute(input interface{}) (bool, error)
}

// NewClient creates a new OPA client with the given endpoint and policy package. It returns an error if the endpoint is
// invalid or the policy package is empty.
func NewClient(logger *zap.SugaredLogger, endpoint string, policyPackage string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || !govalidator.IsURL(u.String()) {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if strings.TrimSpace(policyPackage) == "" {
		return nil, fmt.Errorf("invalid policy package")
	}
	return &Client{Logger: logger, HttpClient: http.Client{}, URL: *u, PolicyPackage: strings.TrimSpace(policyPackage)}, nil
}

// Client represents an OPA client that can be used to interact with an OPA server.
type Client struct {
	URL           url.URL
	PolicyPackage string
	HttpClient    http.Client
	Logger        *zap.SugaredLogger
}

// GenerateTags generates tags for the data described by the data provided according to the policy package. It returns
// the tags if the request was successful. An error is returned if the request fails.
func (c *Client) GenerateTags(data interface{}) ([]amphora.Tag, error) {
	result := TagResponse{}
	err := c.makeOpaRequest(TagsAction, data, &result)
	if err != nil {
		return nil, err
	}
	return result.Tags, nil

}

// CanExecute checks if the program can be executed with the input described by the data provided according to the
// policy package. It returns true if the program can be executed, false otherwise. An error is returned if the request
// fails.
func (c *Client) CanExecute(data interface{}) (bool, error) {
	result := ExecuteResponse{}
	err := c.makeOpaRequest(ExecuteAction, data, &result)
	if err != nil {
		return false, err
	}
	return result.IsAllowed, nil
}

func (c *Client) makeOpaRequest(action string, data interface{}, v interface{}) error {
	payload, err := json.Marshal(OpaRequest{Input: data})
	if err != nil {
		return fmt.Errorf("invalid opa input: %w", err)
	}
	bytes.NewBuffer(payload)
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1/data/%s/%s",
			c.URL.String(),
			strings.ReplaceAll(c.PolicyPackage, ".", "/"),
			action),
		bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create opa \"%s\" request: %w", action, err)
	}
	c.Logger.Debugw("Sending OPA request", "url", req.URL.String(), "payload", string(payload))
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check \"%s\" access: %w", action, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to check \"%s\" access (Code %d)", action, resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read opa response body: %w", err)
	}
	err = json.Unmarshal(body, &v)
	if err != nil {
		return fmt.Errorf("failed to unmarshal opa response body: %w", err)
	}
	return nil
}

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
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const (
	TagsAction = "tags"
)

type OpaRequest struct {
	Input interface{} `json:"input"`
}

type TagResponse struct {
	Tags []amphora.Tag `json:"result"`
}

type AbstractClient interface {
	GenerateTags(interface{}) ([]amphora.Tag, error)
}

func NewClient(endpoint string, policyPackage string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil || !govalidator.IsURL(u.String()) {
		return &Client{}, fmt.Errorf("invalid Url: %w", err)
	}
	if strings.TrimSpace(policyPackage) == "" {
		return &Client{}, fmt.Errorf("invalid policy package")
	}
	return &Client{HttpClient: http.Client{}, URL: *u, PolicyPackage: strings.TrimSpace(policyPackage)}, nil
}

type Client struct {
	URL           url.URL
	PolicyPackage string
	HttpClient    http.Client
}

func (c *Client) GenerateTags(data interface{}) ([]amphora.Tag, error) {
	payload, err := json.Marshal(OpaRequest{Input: data})
	if err != nil {
		return nil, fmt.Errorf("invalid opa input: %w", err)
	}
	bytes.NewBuffer(payload)
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1/data/%s/%s",
			c.URL.String(),
			strings.ReplaceAll(c.PolicyPackage, ".", "/"),
			TagsAction),
		bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request to fetch tags from OPA: %w", err)
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags from OPA: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tags from OPA (Code %d)", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read opa response body: %w", err)
	}
	tags := TagResponse{}
	err = json.Unmarshal(body, &tags)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal opa response body: %w", err)
	}
	return tags.Tags, nil
}

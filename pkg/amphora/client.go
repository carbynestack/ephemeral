//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package amphora

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/asaskevich/govalidator"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

// SecretShare is a secret shared value stored in Amphora.
type SecretShare struct {
	SecretID string `json:"secretId"`
	Data     string `json:"data"`
	Tags     []Tag  `json:"tags"`
}

// Tag defines a tag that could be assigned to an secret share.
type Tag struct {
	ValueType string `json:"valueType"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type Metadata struct {
	SecretID string `json:"secretId"`
	Tags     []Tag  `json:"tags"`
}

type MetadataPage struct {
	Content       []Metadata `json:"content"`
	Number        int32      `json:"number"`
	Size          int32      `json:"size"`
	TotalElements int64      `json:"totalElements"`
	TotalPages    int32      `json:"totalPages"`
}

type ObjectListRequestParams struct {
	Filter        string
	PageNumber    int32
	PageSize      int32
	SortProperty  string
	SortDirection string
}

// AbstractClient is an interface for object storage client.
type AbstractClient interface {
	GetObjectList(objectListRequestParams *ObjectListRequestParams) (MetadataPage, error)
	GetSecretShare(string) (SecretShare, error)
	CreateSecretShare(*SecretShare) error
}

// NewAmphoraClient returns a new Amphora client.
func NewAmphoraClient(u url.URL) (*Client, error) {
	ok := govalidator.IsURL(u.String())
	if !ok {
		return &Client{}, errors.New("invalid Url")
	}
	httpClient := http.Client{}
	return &Client{HTTPClient: httpClient, URL: u}, nil
}

// Client is a client for Amphora.
type Client struct {
	URL        url.URL
	HTTPClient http.Client
}

const secretShareURI = "/secret-shares"

//GetObjectList query secretIds based on the filter argument (tags)
func (c *Client) GetObjectList(objectListRequestParams *ObjectListRequestParams) (MetadataPage, error) {
	var result MetadataPage

	urlParams := url.Values{}
	urlParams.Add("filter", objectListRequestParams.Filter)
	urlParams.Add("pageNumber", fmt.Sprintf("%v", objectListRequestParams.PageNumber))
	urlParams.Add("pageSize", fmt.Sprintf("%v", objectListRequestParams.PageSize))
	urlParams.Add("sort", objectListRequestParams.SortProperty)
	urlParams.Add("dir", objectListRequestParams.SortDirection)

	getObjectListUrl := c.URL
	getObjectListUrl.Path += secretShareURI
	getObjectListUrl.RawQuery = urlParams.Encode()
	req, err := http.NewRequest(http.MethodGet, getObjectListUrl.String(), nil)

	if err != nil {
		return result, err
	}

	body, err := c.doRequest(req, http.StatusOK)
	if err != nil {
		return result, err
	}

	json.NewDecoder(body).Decode(&result)

	if err != nil {
		return result, fmt.Errorf("amphora returned an invalid response body: %s", err)
	}
	return result, nil
}

const intraVcpSecretShareURI = "/intra-vcp/secret-shares"

// GetSecretShare creates a new secret share by sending a POST request against Amphora.
func (c *Client) GetSecretShare(id string) (SecretShare, error) {
	var os SecretShare
	req, err := http.NewRequest(http.MethodGet, c.URL.String()+fmt.Sprintf("%s/%s", intraVcpSecretShareURI, id), nil)
	if err != nil {
		return os, err
	}
	body, err := c.doRequest(req, http.StatusOK)
	if err != nil {
		return os, err
	}
	err = json.NewDecoder(body).Decode(&os)
	if err != nil {
		return os, fmt.Errorf("amphora returned an invalid response body: %s", err)
	}
	return os, nil
}

// CreateSecretShare creates a new secret share by sending a POST request against Amphora.
func (c *Client) CreateSecretShare(os *SecretShare) error {
	jsonMarshalled, err := json.Marshal(os)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.URL.String()+fmt.Sprintf("%s", intraVcpSecretShareURI), bytes.NewBuffer(jsonMarshalled))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		return err
	}
	_, err = c.doRequest(req, http.StatusCreated)
	if err != nil {
		return err
	}
	return nil
}

// doRequest is a helper method that sends an HTTP request, compares the returned response code with expected and
// does corresponding error handling.
func (c *Client) doRequest(req *http.Request, expected int) (io.ReadCloser, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http client failed sending request: %s", err)
	}
	if resp.StatusCode != expected {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("server replied with an unexpected response code #%d: %s", resp.StatusCode, string(bodyBytes))
	}
	return resp.Body, nil
}

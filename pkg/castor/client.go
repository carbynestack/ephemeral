//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

// Package castor implements a client to interact with _Carbyne Stack Castor_ services
package castor

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"

	"github.com/asaskevich/govalidator"
)

// AbstractClient is an interface for castor tuple client.
type AbstractClient interface {
	GetTuples(tupleCount int32, tupleType TupleType, requestID uuid.UUID) ([]byte, error)
}

// NewClient returns a new Castor client for the given endpoint
func NewClient(u url.URL) (*Client, error) {
	ok := govalidator.IsURL(u.String())
	if !ok {
		return &Client{}, errors.New("invalid Url")
	}
	httpClient := &http.Client{}
	return &Client{HTTPClient: httpClient, URL: u}, nil
}

// Client is a client for the Castor tuple storage service
type Client struct {
	URL        url.URL
	HTTPClient *http.Client
}

const tupleURI = "/intra-vcp/tuples"
const tupleTypeParam = "tupletype"
const countParam = "count"
const reservationIDParam = "reservationId"

// GetTuples retrieves a list of tuples matching the given criteria from Castor
func (c *Client) GetTuples(count int32, tt TupleType, requestID uuid.UUID) ([]byte, error) {
	values := url.Values{}
	values.Add(tupleTypeParam, tt.Name)
	values.Add(countParam, strconv.Itoa(int(count)))
	values.Add(reservationIDParam, requestID.String())
	requestURL, err := c.URL.Parse(tupleURI)
	if err != nil {
		return nil, err
	}
	requestURL.RawQuery = values.Encode()
	req, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("communication with castor failed: %s", err)
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("getting tuples failed for \"%s\" with response code #%d: %s", req.URL, resp.StatusCode, string(bodyBytes))
	}
	if err != nil {
		return nil, fmt.Errorf("castor has returned an invalid response body: %s", err)
	}
	return bodyBytes, nil
}

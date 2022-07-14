//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/asaskevich/govalidator"
)

// AbstractClient is an interface for castor tuple client.
type AbstractClient interface {
	GetTuples(tupleCount int32, tupleType TupleType, requestId uuid.UUID) (*TupleList, error)
}

// NewCastorClient returns a new Castor client for the given endpoint
func NewCastorClient(u url.URL) (*Client, error) {
	ok := govalidator.IsURL(u.String())
	if !ok {
		return &Client{}, errors.New("invalid Url")
	}
	httpClient := &http.Client{}
	return &Client{HttpClient: httpClient, Url: u}, nil
}

// Client is a client for the Castor tuple storage service
type Client struct {
	Url        url.URL
	HttpClient *http.Client
}

const tupleURI = "/intra-vcp/tuples"
const tupleTypeParam = "tupletype"
const countParam = "count"
const reservationIdParam = "reservationId"

// GetTuples retrieves a list of tuples matching the given criteria from Castor
func (c *Client) GetTuples(count int32, tt TupleType, id uuid.UUID) (*TupleList, error) {
	values := url.Values{}
	values.Add(tupleTypeParam, tt.Name)
	values.Add(countParam, strconv.Itoa(int(count)))
	values.Add(reservationIdParam, id.String())
	requestUrl, err := c.Url.Parse(tupleURI)
	if err != nil {
		return nil, err
	}
	requestUrl.RawQuery = values.Encode()
	req, err := http.NewRequest(http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("communication with castor failed: %s", err))
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(fmt.Sprintf("getting tuples failed for \"%s\" with response code #%d: %s", req.URL, resp.StatusCode, string(bodyBytes)))
	}
	tuples := &TupleList{}
	err = json.NewDecoder(resp.Body).Decode(tuples)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("castor has returned an invalid response body: %s", err))
	}
	return tuples, nil
}

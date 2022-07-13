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
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/asaskevich/govalidator"
)

// AbstractClient is an interface for castor tuple client.
type AbstractClient interface {
	GetTuples(int32, TupleType, string) (TupleList, error)
}

func NewCastorClient(u url.URL) (*Client, error) {
	ok := govalidator.IsURL(u.String())
	if !ok {
		return &Client{}, errors.New("invalid Url")
	}
	httpClient := http.Client{}
	return &Client{HttpClient: httpClient, Url: u}, nil
}

type Client struct {
	Url        url.URL
	HttpClient http.Client
}

const tupleURI = "/intra-vcp/tuples"
const tupleTypeParam = "tupletype"
const countParam = "count"
const reservationIdParam = "reservationId"

func (c *Client) GetTuples(count int32, tt TupleType, id string) (TupleList, error) {
	var tuples TupleList
	req, err := http.NewRequest(http.MethodGet, c.Url.String()+fmt.Sprintf("%s?%s=%s&%s=%d&%s=%s", tupleURI, tupleTypeParam, tt.Name, countParam, count, reservationIdParam, id), nil)
	if err != nil {
		return tuples, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return tuples, errors.New(fmt.Sprintf("communication with castor failed: %s", err))
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return tuples, err
		}
		return tuples, errors.New(fmt.Sprintf("getting tuples failed for \"%s\" with response code #%d: %s", req.URL, resp.StatusCode, string(bodyBytes)))
	}
	err = json.NewDecoder(resp.Body).Decode(&tuples)
	if err != nil {
		return tuples, errors.New(fmt.Sprintf("castor has returned an invalid response body: %s", err))
	}
	return tuples, nil
}

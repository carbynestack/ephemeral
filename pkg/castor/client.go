package castor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/carbynestack/ephemeral/pkg/types"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type InputType string

const (
	BitGfp                   InputType = "BIT_GFP"
	BitGf2n                            = "BIT_GF2N"
	InputMaskGfp                       = "INPUT_MASK_GFP"
	InputMaskGf2n                      = "INPUT_MASK_GF2N"
	InverseTupleGfp                    = "INVERSE_TUPLE_GFP"
	InverseTupleGf2n                   = "INVERSE_TUPLE_GF2N"
	SquareTupleGfp                     = "SQUARE_TUPLE_GFP"
	SquareTupleGf2n                    = "SQUARE_TUPLE_GF2N"
	MultiplicationTripleGfp            = "MULTIPLICATION_TRIPLE_GFP"
	MultiplicationTripleGf2n           = "MULTIPLICATION_TRIPLE_GF2N"
)

type TupleList struct {
	TupleCls string     `json:"tupleCls"`
	Field    TupleField `json:"field"`
	Tuples   []Tuple    `json:"tuples"`
}

type TupleField struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	ElementSize int    `json:"elementSize"`
}

type Tuple struct {
	Type   string        `json:"@type"`
	Field  TupleField    `json:"field"`
	Shares []TupleShares `json:"shares"`
}

type TupleShares struct {
	Value string `json:"value"`
	Mac   string `json:"mac"`
}

type AbstractClient interface {
	DownloadTupleFiles(requestId uuid.UUID, numberOfTuples int, tupleType InputType) (tupleFiles TupleList, err error)
}

type Client struct {
	HTTPClient http.Client
	URL        *url.URL
}

// NewCastorClient returns a new Amphora client.
func NewCastorClient(u *url.URL) (*Client, error) {
	ok := govalidator.IsURL(u.String())
	if !ok {
		return &Client{}, errors.New("invalid Url")
	}
	httpClient := http.Client{}
	return &Client{HTTPClient: httpClient, URL: u}, nil
}

const tupleURI = "/tuples"

// DownloadTupleFiles retrieves Tuple files by sending a
func (c *Client) DownloadTupleFiles(requestId uuid.UUID, numberOfTuples int, tupleType InputType) (tupleFiles TupleList, err error) {

	var result TupleList

	urlParams := url.Values{}
	urlParams.Add("tupletype", string(tupleType))
	urlParams.Add("count", strconv.Itoa(numberOfTuples))
	urlParams.Add("reservationId", requestId.String())

	getObjectListUrl := c.URL
	getObjectListUrl.Path += tupleURI
	getObjectListUrl.RawQuery = urlParams.Encode()
	req, err := http.NewRequest(http.MethodGet, getObjectListUrl.String(), nil)

	if err != nil {
		return result, err
	}

	body, err := c.doRequest(req, http.StatusOK)
	if err != nil {
		return result, err
	}

	err = json.NewDecoder(body).Decode(&result)
	if err != nil {
		return result, fmt.Errorf("castor returned an invalid response body: %s", err)
	}
	return result, nil
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

func TupleFileNameFor(inputType InputType, threadNumber int, config *types.SPDZEngineTypedConfig) string {
	switch inputType {
	case BitGfp:
		return fmt.Sprintf("%d-p-%d/Bits-p-P%d-T%d", config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case InputMaskGfp:
		return fmt.Sprintf("%d-p-%d/Inputs-p-P%d-T%d", config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case InverseTupleGfp:
		return fmt.Sprintf("%d-p-%d/Inverses-p-P%d-T%d", config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case SquareTupleGfp:
		return fmt.Sprintf("%d-p-%d/Squares-p-P%d-T%d", config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case MultiplicationTripleGfp:
		return fmt.Sprintf("%d-p-%d/Triples-p-P%d-T%d", config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	}

	panic("Unknown type for Name " + inputType)
}

func ProtocolDescriptorFor(inputType InputType) string {
	switch inputType {
	case BitGfp, MultiplicationTripleGfp, InputMaskGfp, InverseTupleGfp, SquareTupleGfp:
		return "SPDZ gfp"
	case InputMaskGf2n, InverseTupleGf2n, SquareTupleGf2n, MultiplicationTripleGf2n, BitGf2n:
		return "SPDZ gf2n"
	}

	panic("Unknown type for Descriptor " + inputType)
}

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

//region Structs that are retrieved from Castor as JSON Payloads

// TupleList is the whole Payload from a "DownloadTuples" call to Castor
type TupleList struct {
	TupleCls string     `json:"tupleCls"`
	Field    TupleField `json:"field"` // ToDo: Do we need this in the Go Code or can we ignore it?
	Tuples   []Tuple    `json:"tuples"`
}

// TupleField describes the Field (Galouis Prime, GF2N, ...) inside Castor
// Not sure if we need this here?
type TupleField struct {
	Type        string `json:"@type"` // ToDo: While this is part of the JSON, it's only used by Java Code, do we need it here?
	Name        string `json:"name"`
	ElementSize int    `json:"elementSize"`
}

// Tuple is a holder of actual Data.
// Depending on its Type it can hold 1 Share (e.g. InputMask) or multiple Shares (e.g. MultiplicationTriple contains 3)
type Tuple struct {
	Type   string        `json:"@type"` // ToDo: While this is part of the JSON, it's only used by Java Code, do we need it here?
	Field  TupleField    `json:"field"` // ToDo: Do we need this in the Go Code or can we ignore it?
	Shares []TupleShares `json:"shares"`
}

// TupleShares are the actual Binary Tuple Values, encoded in Base64
// MP-SPDZ expects first the Value, then the Mac as base64 decoded ByteStream
type TupleShares struct {
	Value string `json:"value"`
	Mac   string `json:"mac"`
}

//endregion

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

const tupleURI = "intra-vcp/tuples"

// DownloadTupleFiles retrieves Tuple files by sending a
func (c *Client) DownloadTupleFiles(requestId uuid.UUID, numberOfTuples int, tupleType InputType) (tupleFiles TupleList, err error) {

	var result TupleList

	urlParams := url.Values{}
	urlParams.Add("tupletype", string(tupleType))
	urlParams.Add("count", strconv.Itoa(numberOfTuples))
	urlParams.Add("reservationId", requestId.String())

	downloadTuplesURL := *c.URL
	downloadTuplesURL.Path += tupleURI
	downloadTuplesURL.RawQuery = urlParams.Encode()
	req, err := http.NewRequest(http.MethodGet, downloadTuplesURL.String(), nil)

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
// ToDo: This method was copied from AmphoraClient
//       -> Maybe it should be moved to moved to an io package and be made public?
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

// ToDo: Can this be constant -> Do we assume the Base-Image never overrides PREP_DIR on compilation?
const tupleBaseFolder = "Player-Data"

func TupleFileNameFor(inputType InputType, threadNumber int, config *types.SPDZEngineTypedConfig) string {
	// ToDo: Clean up a bit (All that changes is the Bits/Inputs/Inverses/... part for Prime Tuples)
	switch inputType {
	case BitGfp:
		return fmt.Sprintf("%s/%d-p-%d/Bits-p-P%d-T%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case InputMaskGfp:
		return fmt.Sprintf("%s/%d-p-%d/Inputs-p-P%d-T%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case InverseTupleGfp:
		return fmt.Sprintf("%s/%d-p-%d/Inverses-p-P%d-T%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case SquareTupleGfp:
		return fmt.Sprintf("%s/%d-p-%d/Squares-p-P%d-T%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	case MultiplicationTripleGfp:
		return fmt.Sprintf("%s/%d-p-%d/Triples-p-P%d-T%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen(), config.PlayerID, threadNumber)
	}
	// ToDo: Add GF2N Tuples?
	panic("Unknown type for Name " + inputType)
}

//func WriteInformationFor(config *types.SPDZEngineTypedConfig) error {
//	primeFolder := fmt.Sprintf("%s/%d-p-%d", tupleBaseFolder, config.PlayerCount, config.Prime.BitLen())
//
//	primeParamFile := fmt.Sprintf("%s/Params-Data", primeFolder)
//	primeMacFile := fmt.Sprintf("%s/Player-MAC-Keys-p-P%d", primeFolder, config.PlayerID)
//
//	err := os.WriteFile(primeParamFile, []byte(config.Prime.Text(10)), 0666)
//	if err != nil {
//		return err
//	}
//
//	// ToDo: Where do we get the MAC from?
//	//       Do we even want this information written as Part of Ephemeral vs. mounting a Secret/Configmap into the Pod?
//	return nil
//}

func ProtocolDescriptorFor(inputType InputType) string {
	switch inputType {
	case BitGfp, MultiplicationTripleGfp, InputMaskGfp, InverseTupleGfp, SquareTupleGfp:
		return "SPDZ gfp"
	case InputMaskGf2n, InverseTupleGf2n, SquareTupleGf2n, MultiplicationTripleGf2n, BitGf2n:
		return "SPDZ gf2n"
	}

	panic("Unknown type for Descriptor " + inputType)
}

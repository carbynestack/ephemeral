//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrSizeTooBig is thrown when the size of the parameters is exceeded.
const ErrSizeTooBig = "size array must be <= 32 bits"

// ErrInvalidBodySize is thrown when a message of invalid length is provided.
const ErrInvalidBodySize = "Body size must be a multiple of 32"

// ErrInvalidResponseSize is thrown when a message of invalid length is returned.
const ErrInvalidResponseSize = "Response size must be equal to 20"

// ErrInvalidWordSize is thrown when a SPDZ runtime socket response is not equal to word size.
const ErrInvalidWordSize = "plain text message size must be a multiple of 16"

// ErrSPDZToParcel is thrown when an error in spdzToParcels method has occured.
const ErrSPDZToParcel = "spdzToParcels error: "

// ErrParcelToSPDZ is thrown when an error in parcelsToSPDZ method has occured.
const ErrParcelToSPDZ = "parcelsToSPDZ error: "

// ErrMarshal is thrown when provided input is < 1.
const ErrMarshal = "at 1 object must be provided to marshal"

// BodySize equals to 32 = 16 bytes secret share + 16 bytes MAC.
const BodySize = 32

// MessageSize equals to 36 = 4 bytes Length + 32 bytes of message
const MessageSize = 36

// WordSize is the size of a single word in SPDZ runtime notation, e.g. secret share or MAC.
const WordSize = 16

// ResponseSizeCipher is the size of a byte array containing plain text.
const ResponseSizeCipher = 36

// ParcelSizeLength is the length of the Parcel size header in bytes.
const ParcelSizeLength = 4

// MaxLength is the max size of Param's octet.
var MaxLength = int(math.Pow(2, 32))

// Packer is an interface to marshal and unmarshal strings to the format specified by a given MPC runtime.
type Packer interface {
	Marshal([]string, *[]byte) error
	Unmarshal(*[]byte, ResponseConverter, bool) ([]string, error)
}

// SPDZPacker is an implementation of Packer interface for SPDZ runtime.
type SPDZPacker struct {
	// maxBulkSize is the maximum size of bulk objects received as parameters.
	MaxBulkSize int32
}

// Marshal converts a base64 encoded string into a byte array consumable by SPDZ runtime.
func (p *SPDZPacker) Marshal(b64 []string, dst *[]byte) (err error) {
	if len(b64) < 1 {
		return errors.New(ErrMarshal)
	}
	parcels, err := p.base64ToParcels(b64)
	if err != nil {
		return err
	}
	err = parcelsToSPDZ(parcels, dst)
	if err != nil {
		return err
	}
	return nil
}

// Unmarshal converts a byte sequence to a base64 encoded secret share representation consumable by Amphora.
func (p *SPDZPacker) Unmarshal(in *[]byte, conv ResponseConverter, bulkSecrets bool) ([]string, error) {
	prc, err := spdzToParcels(in, conv)
	if err != nil {
		return nil, err
	}
	strings := []string{}
	if bulkSecrets {
		bulky := []byte{}
		for _, pr := range prc {
			bulky = append(bulky, pr.Body...)
		}
		b64Body := base64.StdEncoding.EncodeToString(bulky)
		return []string{b64Body}, nil
	}
	for i := range prc {
		strings = append(strings, prc[i].BodyBase64)
	}
	return strings, nil
}

// base64ToParcels parses a base64 encoded byte array and returns a slice of parcels.
// The byte array is a list of concatenated 32 bytes strings.
func (p *SPDZPacker) base64ToParcels(b64 []string) (prc []Parcel, err error) {
	for i := range b64 {
		body, err := base64.StdEncoding.DecodeString(b64[i])
		if err != nil {
			return nil, err
		}
		if len(body)%BodySize != 0 {
			return nil, errors.New(ErrInvalidBodySize)
		}
		for i := 0; i < len(body)-(BodySize-1); i += BodySize {
			j := i + BodySize
			chunk := body[i:j]
			size, err := lenToBytes(chunk)
			if err != nil {
				return nil, err
			}
			prc = append(prc,
				Parcel{
					Size: size,
					Body: chunk,
					// We do not need the base64 encoded body for now.
					BodyBase64: "",
				})
		}
	}
	return prc, nil
}

// Parcel is an internal representation of a message consumed by SPDZ.
type Parcel struct {
	Size       []byte
	Body       []byte
	BodyBase64 string
}

// parcelsToSPDZ creates a message accepted by SPDZ socket. It is a concatenation of its size and body.
// The size bytes must correspond to the length of the body slice.
func parcelsToSPDZ(prc []Parcel, o *[]byte) error {
	concatBody := []byte{}
	for i := range prc {
		p := prc[i]
		size := binary.LittleEndian.Uint32(p.Size)
		if size != uint32(len(p.Body)) {
			return errors.New(ErrParcelToSPDZ + ErrInvalidBodySize)
		}
		concatBody = append(concatBody, p.Body...)
	}
	overallSizeInBytes, err := lenToBytes(concatBody)
	if err != nil {
		return errors.New(ErrParcelToSPDZ + err.Error())
	}
	out := make([]byte, ParcelSizeLength+ParcelSizeLength)
	out = append(overallSizeInBytes, concatBody...)
	*o = out
	return nil
}

// spdzToParcels decodes the bytes received from a SPDZ socket.
// i - input byte slice received from SPDZ runtime.
// p - resulting Parcel
// converter - a function to mutate the output, e.g. to specify different logic for plain text and secret shared values.
func spdzToParcels(i *[]byte, converter ResponseConverter) ([]Parcel, error) {
	size := (*i)[:ParcelSizeLength]
	body := (*i)[ParcelSizeLength:]
	s := binary.LittleEndian.Uint32(size)
	if uint32(len(body))%s != 0 {
		return nil, errors.New(ErrSPDZToParcel + ErrInvalidBodySize + fmt.Sprintf(", actual size is %d\n", len(body)))
	}
	parcels, err := converter.convert(body)
	if err != nil {
		return nil, errors.New(ErrSPDZToParcel + err.Error())
	}
	return parcels, nil
}

// lenToBytes is a helper method to convert byte slice len to a Little Endian 4 bytes sequence.
func lenToBytes(i []byte) ([]byte, error) {
	if len(i) > int(MaxLength) {
		return []byte{}, errors.New(ErrSizeTooBig)
	}
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(len(i)))
	return b, nil
}

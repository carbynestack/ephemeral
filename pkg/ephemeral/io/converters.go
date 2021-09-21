//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"math/big"
)

// ResponseConverter is an interface for a struct that mutates the response from SPDZ runtime to a required format.
type ResponseConverter interface {
	convert(in []byte) ([]Parcel, error)
}

// SecretSharesConverter is to be used for encoding base64 secret shared responses received from SPDZ runtime.
type SecretSharesConverter struct {
	Params []interface{}
}

// Convert encodes a byte array in base64.
func (b *SecretSharesConverter) convert(in []byte) ([]Parcel, error) {
	shareSize := WordSize * 2 // it is 32 bytes, value + MAC
	rem := math.Remainder(float64(len(in)), float64(shareSize))
	if rem > 0 {
		return nil, fmt.Errorf("received secret shared value length is not a multiple of %d", shareSize)
	}
	chunks := len(in) / (shareSize)
	parcels := []Parcel{}
	for i := 0; i < chunks; i++ {
		begin := i * shareSize
		end := begin + shareSize
		chunk := in[begin:end]
		bodyBase64 := base64.StdEncoding.EncodeToString(chunk)
		size, err := lenToBytes(chunk)
		if err != nil {
			return nil, err
		}
		prc := Parcel{
			Size:       size,
			Body:       chunk,
			BodyBase64: bodyBase64,
		}
		parcels = append(parcels, prc)
	}
	return parcels, nil
}

// PlaintextConverter expects plain-text response from SPDZ and converts it into a human readable base64 encoded int64 param.
// base64 encoding is used to comply with the format we use when storing objects in Amphora.
type PlaintextConverter struct {
	Params []interface{}
}

// convert converts a binary output delivered by SPDZ runtime to a human readable int64 number.
// rInv - is the inverse of R in Montgomery notation.
// p - is the prime number used in MPC computation.
func (s *PlaintextConverter) convert(in []byte) ([]Parcel, error) {
	rInv := s.Params[0].(*big.Int)
	p := s.Params[1].(*big.Int)
	rem := math.Remainder(float64(len(in)), float64(WordSize))
	if rem > float64(0) {
		return nil, errors.New(ErrInvalidWordSize + fmt.Sprintf(": received %d", len(in)))
	}
	chunks := len(in) / WordSize
	parcels := []Parcel{}
	for i := 0; i < chunks; i++ {
		begin := i * WordSize
		end := begin + WordSize
		chunk := in[begin:end]
		limb1 := s.littleToBigEndian(chunk[:8])
		limb2 := s.littleToBigEndian(chunk[8:])
		arr := s.swapLimbs(limb1, limb2)
		inputBigInt := new(big.Int)
		inputBigInt.SetBytes(arr)
		result := new(big.Int)
		result.Mul(inputBigInt, rInv)
		result.Mod(result, p)
		resp := base64.StdEncoding.EncodeToString([]byte(result.String()))
		size, err := lenToBytes(in)
		if err != nil {
			return nil, err
		}
		prc := Parcel{
			Body:       in,
			BodyBase64: resp,
			Size:       size,
		}
		parcels = append(parcels, prc)
	}
	return parcels, nil
}

// littleToBigEndian converts Little Endian notation to Big Endian.
func (s *PlaintextConverter) littleToBigEndian(in []byte) []byte {
	out := make([]byte, len(in))
	for i, j := 0, len(in)-1; i < len(in); i++ {
		out[i] = in[j]
		j--
	}
	return out
}

// swapLimbs swaps two limbs and returns a new slice containing both.
func (s *PlaintextConverter) swapLimbs(l1, l2 []byte) []byte {
	return append(l2, l1...)
}

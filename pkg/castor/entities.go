//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor

type TupleList struct {
	TupleCls string  `json:"tupleCls"`
	Field    Field   `json:"field"`
	Tuples   []Tuple `json:"tuples"`
}

type Field struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	ElementSize int64  `json:"elementSize"`
}

type Tuple struct {
	Type   string  `json:"@type"`
	Field  Field   `json:"field"`
	Shares []Share `json:"shares"`
}

type Share struct {
	Value string `json:"value"`
	Mac   string `json:"mac"'`
}

const (
	BitGfp                   TupleType = "BIT_GFP"
	BitGf2n                  TupleType = "BIT_GF2N"
	InputMaskGfp             TupleType = "INPUT_MASK_GFP"
	InputMaskGf2n            TupleType = "INPUT_MASK_GF2N"
	InverseTupleGfp          TupleType = "INVERSE_TUPLE_GFP"
	InverseTupleGf2n         TupleType = "INVERSE_TUPLE_GF2N"
	SquareTupleGfp           TupleType = "SQUARE_TUPLE_GFP"
	SquareTupleGf2n          TupleType = "SQUARE_TUPLE_GF2N"
	MultiplicationTripleGfp  TupleType = "MULTIPLICATION_TRIPLE_GFP"
	MultiplicationTripleGf2n TupleType = "MULTIPLICATION_TRIPLE_GF2N"
)

var TupleTypes = []TupleType{
	BitGfp,
	BitGf2n,
	InputMaskGfp,
	InputMaskGf2n,
	InverseTupleGfp,
	InverseTupleGf2n,
	SquareTupleGfp,
	SquareTupleGf2n,
	MultiplicationTripleGfp,
	MultiplicationTripleGf2n}

type TupleType string

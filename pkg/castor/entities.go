//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor

// TupleList is a collection of a specific type of tuples
type TupleList struct {
	Tuples []Tuple `json:"tuples"`
}

// Tuple describes the actual tuple and its shares
type Tuple struct {
	Shares []Share `json:"shares"`
}

// Share represents a single share of a tuple with its shared value and mac data
type Share struct {
	Value string `json:"value"`
	Mac   string `json:"mac"`
}

// SPDZProtocol describes the protocol used for the MPC computation
type SPDZProtocol struct {
	Descriptor string
	Shorthand  string
}

var (
	SpdzGfp  = SPDZProtocol{"SPDZ gfp", "p"}
	SpdzGf2n = SPDZProtocol{"SPDZ gf2n_", "2"}
)

var SupportedSpdzProtocols = []SPDZProtocol{
	SpdzGfp,
	SpdzGf2n}

// TupleType describes a type of Tuples provided by Castor
type TupleType struct {
	Name              string
	PreprocessingName string
	SpdzProtocol      SPDZProtocol
}

var (
	BitGfp                   = TupleType{"BIT_GFP", "Bits", SpdzGfp}
	BitGf2n                  = TupleType{"BIT_GF2N", "Bits", SpdzGf2n}
	InputMaskGfp             = TupleType{"INPUT_MASK_GFP", "Inputs", SpdzGfp}
	InputMaskGf2n            = TupleType{"INPUT_MASK_GF2N", "Inputs", SpdzGf2n}
	InverseTupleGfp          = TupleType{"INVERSE_TUPLE_GFP", "Inverses", SpdzGfp}
	InverseTupleGf2n         = TupleType{"INVERSE_TUPLE_GF2N", "Inverses", SpdzGf2n}
	SquareTupleGfp           = TupleType{"SQUARE_TUPLE_GFP", "Squares", SpdzGfp}
	SquareTupleGf2n          = TupleType{"SQUARE_TUPLE_GF2N", "Squares", SpdzGf2n}
	MultiplicationTripleGfp  = TupleType{"MULTIPLICATION_TRIPLE_GFP", "Triples", SpdzGfp}
	MultiplicationTripleGf2n = TupleType{"MULTIPLICATION_TRIPLE_GF2N", "Triples", SpdzGf2n}
)

var SupportedTupleTypes = []TupleType{
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

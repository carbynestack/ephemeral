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
	Mac   string `json:"mac"`
}

type SPDZProtocol struct {
	Descriptor string
	Shorthand  string
}

var (
	SpdzGfp  = SPDZProtocol{"SPDZ gfp", "p"}
	SpdzGf2n = SPDZProtocol{"SPDZ gf2n_", "2"}
)

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

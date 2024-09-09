//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//

package castor

// TupleList is a collection of a specific type of tuples.
type TupleList struct {
	Tuples []Tuple `json:"tuples"`
}

// Tuple describes the actual tuple and its shares.
type Tuple struct {
	Shares []Share `json:"shares"`
}

// Share represents a single share of a tuple with its shared value and mac data.
type Share struct {
	Value string `json:"value"`
	Mac   string `json:"mac"`
}

// SPDZProtocol describes the protocol used for the MPC computation.
type SPDZProtocol struct {
	Descriptor string
	Shorthand  string
}

var (
	// SPDZGfp provides the SPDZProtocol protocol definition following the Modulo a prime domain.
	SPDZGfp = SPDZProtocol{"SPDZ gfp", "p"}
	// SPDZGf2n provides the SPDZProtocol protocol definition following the GF(2^n) domain.
	SPDZGf2n = SPDZProtocol{"SPDZ gf2n_", "2"}
)

// SupportedSPDZProtocols is a list of all SPDZProtocol supported by castor and ephemeral.
var SupportedSPDZProtocols = []SPDZProtocol{
	SPDZGfp,
	SPDZGf2n,
}

// TupleType describes a type of Tuples provided by Castor.
type TupleType struct {
	Name              string
	PreprocessingName string
	SpdzProtocol      SPDZProtocol
	Arity             int8
}

var (
	// BitGfp describes the Bits tuple type in the Mudulo a Prime domain.
	BitGfp = TupleType{"BIT_GFP", "Bits", SPDZGfp, 1}
	// BitGf2n describes the Bits tuple type in the GF(2^n) domain.
	BitGf2n = TupleType{"BIT_GF2N", "Bits", SPDZGf2n, 1}
	// InputMaskGfp describes the Inputs tuple type in the Mudulo a Prime domain.
	InputMaskGfp = TupleType{"INPUT_MASK_GFP", "Inputs", SPDZGfp, 2}
	// InputMaskGf2n describes the Inputs tuple type in the GF(2^n) domain.
	InputMaskGf2n = TupleType{"INPUT_MASK_GF2N", "Inputs", SPDZGf2n, 2}
	// InverseTupleGfp describes the Inverses tuple type in the Mudulo a Prime domain.
	InverseTupleGfp = TupleType{"INVERSE_TUPLE_GFP", "Inverses", SPDZGfp, 2}
	// InverseTupleGf2n describes the Inverses tuple type in the GF(2^n) domain.
	InverseTupleGf2n = TupleType{"INVERSE_TUPLE_GF2N", "Inverses", SPDZGf2n, 2}
	// SquareTupleGfp describes the Squares tuple type in the Mudulo a Prime domain.
	SquareTupleGfp = TupleType{"SQUARE_TUPLE_GFP", "Squares", SPDZGfp, 2}
	// SquareTupleGf2n describes the Squares tuple type in the GF(2^n) domain.
	SquareTupleGf2n = TupleType{"SQUARE_TUPLE_GF2N", "Squares", SPDZGf2n, 2}
	// MultiplicationTripleGfp describes the Triples tuple type in the Mudulo a Prime domain.
	MultiplicationTripleGfp = TupleType{"MULTIPLICATION_TRIPLE_GFP", "Triples", SPDZGfp, 3}
	// MultiplicationTripleGf2n describes the Triples tuple type in the GF(2^n) domain.
	MultiplicationTripleGf2n = TupleType{"MULTIPLICATION_TRIPLE_GF2N", "Triples", SPDZGf2n, 3}
)

// SupportedTupleTypes is a list of all tuple types supported by the castor client.
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
	MultiplicationTripleGf2n,
}

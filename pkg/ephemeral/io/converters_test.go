// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package io

import (
	"encoding/base64"
	"math/big"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// . "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
)

var _ = Describe("Converters", func() {
	Context("when converting plaintext output", func() {
		var (
			rInv, p big.Int
			params  []interface{}
			conv    PlaintextConverter
		)
		BeforeEach(func() {
			rInv.SetString("116525037434575252203671714714489805504", 10)
			p.SetString("172035116406933162231178957667602464769", 10)
			params = []interface{}{&rInv, &p}
			conv = PlaintextConverter{
				Params: params,
			}
		})
		Context("when converting a single valued response", func() {
			It("returns a plain-text integer", func() {
				spdzFormat := "Jf8uKaLlN9MhlQdaTPP1Rw==" // 25ff 2e29 a2e5 37d3 2195 075a 4cf3 f547
				bytes, _ := base64.StdEncoding.DecodeString(spdzFormat)
				parcels, err := conv.convert(bytes)
				decoded, _ := base64.StdEncoding.DecodeString(parcels[0].BodyBase64)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(decoded)).To(Equal("111"))
			})
			It("returns an error when invalid message size is provided", func() {
				bytes := make([]byte, 1)
				_, err := conv.convert(bytes)
				Expect(err.Error()).To(Equal(ErrInvalidWordSize + ": received 1"))
			})
		})
		Context("when converting multiple values", func() {
			It("should recognise individual messages from the body", func() {
				spdzFormat := "Jf8uKaLlN9MhlQdaTPP1Rw==" // 25ff 2e29 a2e5 37d3 2195 075a 4cf3 f547
				bytes, _ := base64.StdEncoding.DecodeString(spdzFormat)
				message := append(bytes, bytes...)
				parcels, err := conv.convert(message)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(parcels)).To(Equal(2))
			})
		})
	})
})

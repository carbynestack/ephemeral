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
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Packer operations", func() {

	var (
		// This is how the expectedByteSeq could be derived (tested on MacOS):
		// $ echo -n -e "Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=" | base64 -D > /tmp/test.b64
		// And then verify it:
		// $ xxd /tmp/test.b64
		expectedByteSeq = "532fe7e70d838626c27cd9cc9c7769105ecd3d7e42e960f49d8b2a3a293ed203"
	)
	var (
		p SPDZPacker
	)
	BeforeEach(func() {
		p = SPDZPacker{
			MaxBulkSize: 64,
		}
	})

	Context("when converting slice length", func() {
		It("should return the length of the input array in hex format", func() {
			arr := []byte("abcd")
			size, err := lenToBytes(arr)
			// This is Little Endian 32 bits long hex value, e.g. '4' -> '\x04\x00\x00\x00' -> '04000000'
			expected := "04000000"
			Expect(err).NotTo(HaveOccurred())
			Expect(hex.EncodeToString(size)).To(Equal(expected))
		})
		It("returns an error if the len of the base64 encoded string exceeds max length", func() {
			overflow := make([]byte, MaxLength+1)
			_, err := lenToBytes(overflow)
			Expect(err.Error()).To(Equal(ErrSizeTooBig))
		})
	})

	Context("when converting Amphora message parts to parcels", func() {
		Context("when a single object is provided", func() {
			It("creates a parcel", func() {
				// 32 bytes, former 16 is the secret shared value, later 16 is the MAC key as per MP-SPDZ notation.
				b64 := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM="}
				parcels, err := p.base64ToParcels(b64)
				Expect(err).NotTo(HaveOccurred())
				Expect(hex.EncodeToString(parcels[0].Body)).To(Equal(expectedByteSeq))
				Expect(len(parcels[0].Body)).To(Equal(32))
				Expect(hex.EncodeToString(parcels[0].Size)).To(Equal("20000000"))
			})
			It("returns an error if a malformed base64 string is given", func() {
				b64 := []string{"abc"}
				_, err := p.base64ToParcels(b64)
				Expect(err).To(HaveOccurred())
			})
			It("returns an error if the octets size is not equal to 32", func() {
				// This a body with is 33 bytes long instead of 32.
				b64WithExtraCharacters := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gMA"}
				_, err := p.base64ToParcels(b64WithExtraCharacters)
				Expect(err.Error()).To(Equal(ErrInvalidBodySize))
			})
		})
		Context("when several objects are provided", func() {
			It("returns a slice of parcels", func() {
				b64 := []string{
					"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
					"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
				}
				parcels, _ := p.base64ToParcels(b64)
				Expect(len(parcels)).To(Equal(2))
				Expect(hex.EncodeToString(parcels[0].Body)).To(Equal(expectedByteSeq))
				Expect(hex.EncodeToString(parcels[1].Body)).To(Equal(expectedByteSeq))
			})
		})

		Context("when bulk objects are provided", func() {

			Context("containing only a single bulk object", func() {
				Context("object size is a multiple of BodySize", func() {
					It("creates a slice of parcels", func() {
						b64Bulk := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gNTL+fnDYOGJsJ82cycd2kQXs09fkLpYPSdiyo6KT7SAw=="}
						parcels, err := p.base64ToParcels(b64Bulk)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(parcels)).To(Equal(2))
						Expect(hex.EncodeToString(parcels[0].Body)).To(Equal(expectedByteSeq))
						Expect(hex.EncodeToString(parcels[1].Body)).To(Equal(expectedByteSeq))
					})
				})
				Context("object size is bigger then the MaxBulkSize", func() {
					It("returns an error", func() {
						p.MaxBulkSize = 1
						// 2 bytes.
						b64Bulk := []string{"Jf8="}
						_, err := p.base64ToParcels(b64Bulk)
						Expect(err.Error()).To(Equal(ErrInvalidBodySize))
					})
				})
				Context("object size is not a multiple of BodySize", func() {
					It("returns an error", func() {
						// 34 bytes.
						b64Bulk := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gNKZjg9Cg=="}
						_, err := p.base64ToParcels(b64Bulk)
						Expect(err).To(HaveOccurred())
					})
				})
			})
			Context("containing several bulk objects", func() {
				It("creates a slice of parcels", func() {
					b64Bulk := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gNTL+fnDYOGJsJ82cycd2kQXs09fkLpYPSdiyo6KT7SAw==", "Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gNTL+fnDYOGJsJ82cycd2kQXs09fkLpYPSdiyo6KT7SAw=="}
					parcels, err := p.base64ToParcels(b64Bulk)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(parcels)).To(Equal(4))
					Expect(hex.EncodeToString(parcels[0].Body)).To(Equal(expectedByteSeq))
					Expect(hex.EncodeToString(parcels[1].Body)).To(Equal(expectedByteSeq))
					Expect(hex.EncodeToString(parcels[2].Body)).To(Equal(expectedByteSeq))
					Expect(hex.EncodeToString(parcels[3].Body)).To(Equal(expectedByteSeq))
				})
			})
		})

	})

	Context("when converting from parcel to SPDZ format", func() {
		Context("when single parcel is provided", func() {
			var (
				b64     []string
				parcels []Parcel
			)
			BeforeEach(func() {
				b64 = []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM="}
				parcels, _ = p.base64ToParcels(b64)
			})
			It("creates a message out of valid parcels", func() {
				message := make([]byte, 36) // 36 = 4 (size) + 16 (secret share) + 16 (MAC)
				err := parcelsToSPDZ(parcels, &message)
				Expect(err).NotTo(HaveOccurred())
				size := make([]byte, 4)
				binary.LittleEndian.PutUint32(size, 32)
				Expect(message[:4]).To(Equal(size))
				Expect(message[4:]).To(Equal(parcels[0].Body))
			})
			It("returns an error if size is not equal to the actual octet's slice lenght", func() {
				malformedSize := make([]byte, 5)
				parcels[0].Size = malformedSize
				message := make([]byte, 36)
				err := parcelsToSPDZ(parcels, &message)
				Expect(err.Error()).To(Equal(ErrParcelToSPDZ + ErrInvalidBodySize))
			})
		})
		Context("when several parcels are provided", func() {
			It("concatenates them into a single byte stream", func() {
				// This is the size of 2 amphora secrets.
				sizeHeader := []byte{64, 0, 0, 0}
				b64 := []string{
					"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
					"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
				}
				parcels, _ := p.base64ToParcels(b64)
				out := []byte{}
				parcelsToSPDZ(parcels, &out)
				Expect(len(out)).To(Equal(68)) // 4 + 32 + 32
				Expect(out[:4]).To(Equal(sizeHeader))
			})
		})
	})
	Context("when converting from SPDZ bytes to parcel", func() {

		var (
			converter = SecretSharesConverter{}
		)
		Context("when plaintext result is returned", func() {
			Context("when a single object is returned", func() {
				It("returns a slice containing a single parcel", func() {
					size := make([]byte, 4)
					body := make([]byte, 32)
					size[0] = 32
					body[0] = 42
					message := append(size, body...)
					b64Body := base64.StdEncoding.EncodeToString(body)
					p := Parcel{
						Size:       size,
						Body:       body,
						BodyBase64: b64Body,
					}
					parcels, err := spdzToParcels(&message, &converter)
					Expect(err).NotTo(HaveOccurred())
					Expect(parcels[0]).To(Equal(p))
				})
			})
			Context("when several objects are returned", func() {
				It("returns a slice containing several parcels", func() {
					size := make([]byte, 4)
					body1 := make([]byte, 32)
					body2 := make([]byte, 32)
					size[0] = 32
					body1[0] = 42
					body2[0] = 42
					message := append(size, body1...)
					message = append(message, body2...)
					parcels, err := spdzToParcels(&message, &converter)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(parcels)).To(Equal(2))
					Expect(parcels[0].Size[0]).To(Equal(uint8(32)))
					Expect(parcels[1].Size[0]).To(Equal(uint8(32)))
					Expect(len(parcels[0].Body)).To(Equal(32))
					Expect(len(parcels[1].Body)).To(Equal(32))
				})
			})
			Context("when spdz output is corrupt", func() {
				It("throws an error when the size doesn't correspond to the actual body size", func() {
					bytes := make([]byte, 20)
					size := bytes[:4]
					size[0] = 42
					_, err := spdzToParcels(&bytes, &converter)
					Expect(err.Error()).To(Equal(ErrSPDZToParcel + ErrInvalidBodySize + ", actual size is 16\n"))
				})
			})
		})

	})

	Context("when using SPDZPacker", func() {
		Context("when marshalling objects", func() {
			Context("when no objects are provided", func() {
				It("returns an error", func() {
					packer := SPDZPacker{}
					src := []string{}
					dst := []byte{}
					err := packer.Marshal(src, &dst)
					Expect(err.Error()).To(Equal(ErrMarshal))
				})
			})
			Context("when corrupted input is provided", func() {
				It("returns an error", func() {
					packer := SPDZPacker{}
					corruptedInput := []string{"abc"}
					dst := []byte{}
					err := packer.Marshal(corruptedInput, &dst)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("illegal base64 data at input byte 0"))
				})
			})
			Context("when a single object is given", func() {
				var (
					packer      SPDZPacker
					maxBulkSize int
				)
				BeforeEach(func() {
					maxBulkSize = 64
					packer = SPDZPacker{
						MaxBulkSize: int32(maxBulkSize),
					}
				})
				Context("when a message containing a single object is provided", func() {
					It("returns a byte array", func() {
						packer := SPDZPacker{}
						src := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM="}
						dst := []byte{}
						packer.Marshal(src, &dst)
						Expect(len(dst)).To(Equal(MessageSize))
					})
				})
				Context("bulk parameters are provided", func() {
					It("marshalls them into a byte array", func() {
						src := []string{"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gNTL+fnDYOGJsJ82cycd2kQXs09fkLpYPSdiyo6KT7SAw=="}
						dst := []byte{}
						packer.Marshal(src, &dst)
						Expect(len(dst)).To(Equal(maxBulkSize + ParcelSizeLength))
					})
				})
			})
			Context("when several objects are given", func() {
				It("returns a concatenated byte stream", func() {
					packer := SPDZPacker{}
					src := []string{
						"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
						"Uy/n5w2DhibCfNnMnHdpEF7NPX5C6WD0nYsqOik+0gM=",
					}
					dst := []byte{}
					packer.Marshal(src, &dst)
					Expect(len(dst)).To(Equal(68))
				})
			})
		})
		Context("when unmarshalling the object", func() {
			It("returns a response", func() {
				packer := SPDZPacker{}
				size := make([]byte, 4)
				body := make([]byte, 32)
				size[0] = 32
				body[0] = 32
				message := append(size, body...)
				b64Body := base64.StdEncoding.EncodeToString(body)
				resp, err := packer.Unmarshal(&message, &SecretSharesConverter{}, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp[0]).To(Equal(b64Body))
			})
			Context("when a response for amphora is required", func() {
				It("packs several parcels into one base64 string", func() {
					packer := SPDZPacker{}
					size := make([]byte, 4)
					body1 := make([]byte, 32)
					body2 := make([]byte, 32)
					size[0] = 64
					body1[0] = 32
					body2[0] = 42
					message := append(size, body1...)
					message = append(message, body2...)
					body := append(body1, body2...)
					b64Body := base64.StdEncoding.EncodeToString(body)
					resp, err := packer.Unmarshal(&message, &SecretSharesConverter{}, true)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(resp)).To(Equal(1))
					Expect(resp[0]).To(Equal(b64Body))
				})
			})
		})
	})
})

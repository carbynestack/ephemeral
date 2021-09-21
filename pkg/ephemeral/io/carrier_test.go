//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package io_test

import (
	"context"
	"fmt"
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/carbynestack/ephemeral/pkg/amphora"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
)

var _ = Describe("Carrier", func() {
	var ctx = context.TODO()
	It("connects to a socket", func() {
		var connected bool
		conn := FakeNetConnection{}
		fakeDialer := func(ctx context.Context, addr, port string) (net.Conn, error) {
			connected = true
			return &conn, nil
		}
		carrier := Carrier{
			Dialer: fakeDialer,
		}
		err := carrier.Connect(context.TODO(), "", "")
		Expect(connected).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})
	It("closes socket connection", func() {
		conn := FakeNetConnection{}
		fakeDialer := func(ctx context.Context, addr, port string) (net.Conn, error) {
			return &conn, nil
		}
		carrier := Carrier{
			Dialer: fakeDialer,
		}
		err := carrier.Connect(context.TODO(), "", "")
		Expect(err).NotTo(HaveOccurred())
		err = carrier.Close()
		Expect(err).NotTo(HaveOccurred())
		Expect(conn.Closed).To(BeTrue())
	})

	var (
		secret         []amphora.SecretShare
		output         []byte
		client, server net.Conn
		dialer         func(ctx context.Context, addr, port string) (net.Conn, error)
	)
	BeforeEach(func() {
		secret = []amphora.SecretShare{
			amphora.SecretShare{},
		}
		output = make([]byte, 1)
		client, server = net.Pipe()
		dialer = func(ctx context.Context, addr, port string) (net.Conn, error) {
			return client, nil
		}
	})
	Context("when sending secret shares through the carrier", func() {
		It("sends an amphora secret to the socket", func() {
			serverResponse := []byte{byte(1)}
			packer := &FakePacker{
				MarshalResponse: serverResponse,
			}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
			}
			carrier.Connect(ctx, "", "")
			go server.Read(output)
			err := carrier.Send(secret)
			carrier.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(output[0]).To(Equal(byte(1)))
		})
		It("returns an error when it fails to marshal the object", func() {
			packer := &FakeBrokenPacker{}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
			}
			carrier.Connect(ctx, "", "")
			go server.Read(output)
			err := carrier.Send(secret)
			carrier.Close()
			Expect(err).To(HaveOccurred())
		})
		It("returns an error when it fails to write to the connection", func() {
			serverResponse := []byte{byte(1)}
			packer := &FakePacker{
				MarshalResponse: serverResponse,
			}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
			}
			carrier.Connect(ctx, "", "")
			// Closing the connection to trigger a failure due to writing into the closed socket.
			server.Close()
			err := carrier.Send(secret)
			carrier.Close()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when reading secret shares from the carrier", func() {
		It("sends back the message from the socket", func() {
			serverResponse := []byte{byte(1)}
			packerResponse := fmt.Sprintf("%x", serverResponse[0])
			packer := FakePacker{
				UnmarshalResponse: []string{packerResponse},
			}
			carrier := Carrier{
				Dialer: dialer,
				Packer: &packer,
			}
			carrier.Connect(ctx, "", "")
			go func() {
				server.Write(serverResponse)
				server.Close()
			}()
			anyConverter := &PlaintextConverter{}
			res, err := carrier.Read(anyConverter, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Response[0]).To(Equal("1"))
		})
		It("returns an error when reading from the socket fails", func() {
			serverResponse := []byte{byte(1)}
			packerResponse := fmt.Sprintf("%x", serverResponse[0])
			packer := FakePacker{
				UnmarshalResponse: []string{packerResponse},
			}
			carrier := Carrier{
				Dialer: dialer,
				Packer: &packer,
			}
			carrier.Connect(ctx, "", "")
			server.Close()
			anyConverter := &PlaintextConverter{}
			_, err := carrier.Read(anyConverter, false)
			Expect(err).To(HaveOccurred())
		})
		It("returns an error when unmarshalling the response fails", func() {
			serverResponse := []byte{byte(1)}
			packer := &FakeBrokenPacker{}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
			}
			carrier.Connect(ctx, "", "")
			go func() {
				server.Write(serverResponse)
				server.Close()
			}()
			anyConverter := &PlaintextConverter{}
			_, err := carrier.Read(anyConverter, false)
			Expect(err).To(HaveOccurred())
		})
	})
})

// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package io_test

import (
	"context"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/amphora"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"net"
	"sync"
)

var _ = Describe("Carrier", func() {
	var ctx = context.TODO()
	var playerID = int32(1) // PlayerID 1, since PlayerID==0 contains another check when connecting

	It("connects to a socket", func() {
		var connected bool
		conn := FakeNetConnection{}
		fakeDialer := func(ctx context.Context, addr, port string) (net.Conn, error) {
			connected = true
			return &conn, nil
		}
		carrier := Carrier{
			Dialer: fakeDialer,
			Logger: zap.NewNop().Sugar(),
		}
		err := carrier.Connect(context.TODO(), playerID, "", "")
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
			Logger: zap.NewNop().Sugar(),
		}
		err := carrier.Connect(context.TODO(), playerID, "", "")
		Expect(err).NotTo(HaveOccurred())
		err = carrier.Close()
		Expect(err).NotTo(HaveOccurred())
		Expect(conn.Closed).To(BeTrue())
	})

	var (
		secret           []amphora.SecretShare
		output           []byte
		connectionOutput []byte //Will contain (length 4 byte, playerID 1 byte)
		client, server   net.Conn
		dialer           func(ctx context.Context, addr, port string) (net.Conn, error)
	)
	BeforeEach(func() {
		secret = []amphora.SecretShare{
			amphora.SecretShare{},
		}
		output = make([]byte, 1)
		connectionOutput = make([]byte, 5)
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
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
			go server.Read(output)
			err := carrier.Send(secret)
			carrier.Close()
			Expect(err).NotTo(HaveOccurred())
			Expect(output[0]).To(Equal(byte(1)))
			Expect(connectionOutput).To(Equal([]byte{1, 0, 0, 0, fmt.Sprintf("%d", playerID)[0]}))
		})
		It("returns an error when it fails to marshal the object", func() {
			packer := &FakeBrokenPacker{}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
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
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
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
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
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
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
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
				Logger: zap.NewNop().Sugar(),
			}
			go server.Read(connectionOutput)
			carrier.Connect(ctx, playerID, "", "")
			go func() {
				server.Write(serverResponse)
				server.Close()
			}()
			anyConverter := &PlaintextConverter{}
			_, err := carrier.Read(anyConverter, false)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when connecting as Player0", func() {
		playerID := int32(0)
		It("will receive and handle the server's fileHeader", func() {
			// Arrange
			// ToDo: Better Response for real-life scenario?
			serverResponse := []byte{1, 0, 0, 0, 1} // 4 byte length + header, in this case "1". In real case Descriptor + Prime
			packer := &FakeBrokenPacker{}
			carrier := Carrier{
				Dialer: dialer,
				Packer: packer,
				Logger: zap.NewNop().Sugar(),
			}
			waitGroup := sync.WaitGroup{}
			waitGroup.Add(1)
			go server.Read(connectionOutput)

			// Act
			var errConnecting error
			go func() {
				errConnecting = carrier.Connect(ctx, playerID, "", "")
				waitGroup.Done()
			}()

			numberOfBytesWritten, errWrite := server.Write(serverResponse)
			errClose := server.Close()

			// Make sure we wait until the Connect and Write are done
			waitGroup.Wait()

			// Assert
			Expect(connectionOutput).To(Equal([]byte{1, 0, 0, 0, fmt.Sprintf("%d", playerID)[0]}))
			Expect(errConnecting).NotTo(HaveOccurred())
			Expect(errWrite).NotTo(HaveOccurred())
			Expect(numberOfBytesWritten).To(Equal(len(serverResponse)))
			Expect(errClose).NotTo(HaveOccurred())
		})
	})
})

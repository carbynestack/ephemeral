//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package client

import (
	"context"
	"errors"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type BrokenDiscoveryClient struct {
}

func (f *BrokenDiscoveryClient) Events(ctx context.Context, opts ...grpc.CallOption) (pb.Discovery_EventsClient, error) {
	return nil, errors.New("crazyLama")
}

type FakeTransportConn struct {
}

func (f *FakeTransportConn) Close() error {
	return nil
}

type FakeStream struct {
	sendCh      chan struct{}
	closeSendCh chan struct{}
}

func (f *FakeStream) Send(*pb.Event) error {
	f.sendCh <- struct{}{}
	return nil
}

func (f *FakeStream) Recv() (*pb.Event, error) {
	return nil, nil
}

func (f *FakeStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (f *FakeStream) Trailer() metadata.MD {
	return nil
}

// CloseSend is called when the connection is being closed.
func (f *FakeStream) CloseSend() error {
	f.closeSendCh <- struct{}{}
	return nil
}

func (f *FakeStream) Context() context.Context {
	return context.TODO()
}

func (f *FakeStream) SendMsg(m interface{}) error {
	return nil
}

func (f *FakeStream) RecvMsg(m interface{}) error {
	return nil
}

type BrokenStream struct {
}

func (b *BrokenStream) Send(*pb.Event) error {
	return errors.New("crazyFrog")
}

func (b *BrokenStream) Recv() (*pb.Event, error) {
	return nil, io.EOF
}

func (b *BrokenStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (b *BrokenStream) Trailer() metadata.MD {
	return nil
}

func (b *BrokenStream) CloseSend() error {
	return errors.New("crazyMonkey")
}

func (b *BrokenStream) Context() context.Context {
	return context.TODO()
}

func (b *BrokenStream) SendMsg(m interface{}) error {
	return nil
}

func (b *BrokenStream) RecvMsg(m interface{}) error {
	return nil
}

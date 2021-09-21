//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package server

import (
	"context"
	"errors"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"net"

	"google.golang.org/grpc/metadata"
)

type FakeStream struct {
	sendCh  chan struct{}
	context context.Context
}

func (f *FakeStream) Send(*pb.Event) error {
	f.sendCh <- struct{}{}
	return nil
}

func (f *FakeStream) Recv() (*pb.Event, error) {
	return nil, nil
}

func (f *FakeStream) SetHeader(metadata.MD) error {
	return nil
}

func (f *FakeStream) SendHeader(metadata.MD) error {
	return nil
}

func (f *FakeStream) SetTrailer(metadata.MD) {
	return
}

func (f *FakeStream) Context() context.Context {
	return f.context
}

func (f *FakeStream) SendMsg(m interface{}) error {
	return nil
}

func (f *FakeStream) RecvMsg(m interface{}) error {
	return nil
}

type BrokenStream struct {
	sendCh  chan struct{}
	context context.Context
}

func (f *BrokenStream) Send(*pb.Event) error {
	return errors.New("funkyWolf")
}

func (f *BrokenStream) Recv() (*pb.Event, error) {
	return nil, nil
}

func (f *BrokenStream) SetHeader(metadata.MD) error {
	return nil
}

func (f *BrokenStream) SendHeader(metadata.MD) error {
	return nil
}

func (f *BrokenStream) SetTrailer(metadata.MD) {
	return
}

func (f *BrokenStream) Context() context.Context {
	return f.context
}

func (f *BrokenStream) SendMsg(m interface{}) error {
	return nil
}

func (f *BrokenStream) RecvMsg(m interface{}) error {
	return nil
}

type BrokenListener struct {
}

func (f *BrokenListener) Accept() (net.Conn, error) {
	return nil, errors.New("funkyRacoon")
}

func (f *BrokenListener) Close() error {
	return nil
}

func (f *BrokenListener) Addr() net.Addr {
	return nil
}

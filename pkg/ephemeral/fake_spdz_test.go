// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package ephemeral

import (
	"context"
	"errors"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/types"

	mb "github.com/vardius/message-bus"
	"google.golang.org/grpc"
)

type FakeSPDZEngine struct {
}

func (s *FakeSPDZEngine) Execute(event *pb.Event) error {
	return nil
}

type FakeForwarder struct {
}

func (f *FakeForwarder) Run() error {
	return nil
}

type FakeTransportClient struct {
}

func (f *FakeTransportClient) GetIn() chan *pb.Event {
	return nil
}
func (f *FakeTransportClient) GetOut() chan *pb.Event {
	return nil
}
func (f *FakeTransportClient) Connect() (*grpc.ClientConn, error) {
	return nil, nil
}
func (f *FakeTransportClient) Run(client pb.DiscoveryClient) {
	return
}
func (f *FakeTransportClient) Stop() error {
	return nil
}

type BrokenFakeTransportClient struct {
}

func (f *BrokenFakeTransportClient) GetIn() chan *pb.Event {
	return nil
}
func (f *BrokenFakeTransportClient) GetOut() chan *pb.Event {
	return nil
}
func (f *BrokenFakeTransportClient) Connect() (*grpc.ClientConn, error) {
	return nil, errors.New("some error")
}
func (f *BrokenFakeTransportClient) Run(client pb.DiscoveryClient) {
	return
}
func (f *BrokenFakeTransportClient) Stop() error {
	return nil
}

type FakePlayer struct {
	Initialized bool
	history     *fsm.History
}

func (f *FakePlayer) Init() {
	f.Initialized = true
	return
}
func (f *FakePlayer) Stop() {
	return
}
func (f *FakePlayer) History() *fsm.History {
	return f.history
}
func (f *FakePlayer) Bus() mb.MessageBus {
	return nil
}
func (f *FakePlayer) PublishEvent(name, topic string, event *pb.Event) {
	return
}

type FakeExecutor struct {
}

func (f *FakeExecutor) CallCMD(ctx context.Context, cmd []string, dir string) ([]byte, []byte, error) {
	return []byte{}, []byte{}, nil
}

type BrokenFakeExecutor struct {
}

func (f *BrokenFakeExecutor) CallCMD(ctx context.Context, cmd []string, dir string) ([]byte, []byte, error) {
	return []byte{}, []byte{}, errors.New("some error")
}

type CallbackFakeExecutor struct {
	fts      *FakeTupleStreamer
	callback func(fts *FakeTupleStreamer)
}

func (cfe *CallbackFakeExecutor) CallCMD(ctx context.Context, cmd []string, dir string) ([]byte, []byte, error) {
	cfe.callback(cfe.fts)
	return []byte{}, []byte{}, nil
}

type FakeProxy struct {
}

func (f *FakeProxy) Run(*CtxConfig, chan error) error {
	return nil
}
func (f *FakeProxy) Stop() {
	return
}

type BrokenFakeProxy struct {
}

func (f *BrokenFakeProxy) Run(*CtxConfig, chan error) error {
	return errors.New("some error")
}
func (f *BrokenFakeProxy) Stop() {
	return
}

type FakeFeeder struct {
}

func (f *FakeFeeder) LoadFromSecretStoreAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error) {
	return []byte(ctx.Act.AmphoraParams[0]), nil
}
func (f *FakeFeeder) LoadFromRequestAndFeed(act *Activation, feedPort string, ctx *CtxConfig) ([]byte, error) {
	return []byte(ctx.Act.SecretParams[0]), nil
}
func (f *FakeFeeder) Close() error {
	return nil
}

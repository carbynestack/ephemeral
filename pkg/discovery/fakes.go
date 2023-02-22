// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package discovery

import (
	"errors"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"

	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	. "github.com/carbynestack/ephemeral/pkg/types"

	mb "github.com/vardius/message-bus"
	"google.golang.org/grpc"
)

func NewFakeDiscoveryClient(bus mb.MessageBus, name string) *FakeDiscoveryClient {
	return &FakeDiscoveryClient{
		bus:        bus,
		playerName: name,
	}
}

type FakeDiscoveryClient struct {
	bus        mb.MessageBus
	playerName string
}

func (fdc *FakeDiscoveryClient) Run() {
	fdc.bus.Subscribe(DiscoveryTopic, func(e interface{}) {
		ev := e.(*fsm.Event)
		switch ev.Name {
		case PlayerReady:
			playersReady := fsm.Event{
				Meta: ev.Meta,
				Name: PlayersReady,
			}
			fdc.bus.Publish(fdc.playerName, &playersReady)
		case TCPCheckSuccess:
			event := fsm.Event{
				Meta: ev.Meta,
				Name: TCPCheckSuccessAll,
			}
			fdc.bus.Publish(fdc.playerName, &event)
		}
	})
}

// GetIn returns In channel of the client.
func (fdc *FakeDiscoveryClient) GetIn() chan *pb.Event {
	return make(chan *pb.Event)
}

// GetOut returns Out channel of the client.
func (fdc *FakeDiscoveryClient) GetOut() chan *pb.Event {
	return make(chan *pb.Event)
}

// FakeDClient is used in the tests.
type FakeDClient struct {
}

// Connect belongs to a fake, it is used in the tests.
func (f *FakeDClient) Connect() (*grpc.ClientConn, error) {
	return nil, nil
}

// Run belongs to a fake struct, it is not really used.
func (f *FakeDClient) Run(client pb.DiscoveryClient) {
	return
}

// GetIn returns In channel of the client.
func (f *FakeDClient) GetIn() chan *pb.Event {
	return make(chan *pb.Event)
}

// GetOut returns Out channel of the client.
func (f *FakeDClient) GetOut() chan *pb.Event {
	return make(chan *pb.Event)
}

func NewFakeBrokenDiscoveryClient(bus mb.MessageBus, name string, registered, playersReady bool) *FakeBrokenDiscoveryClient {
	return &FakeBrokenDiscoveryClient{
		bus:          bus,
		playerName:   name,
		registered:   registered,
		playersReady: playersReady,
	}
}

type FakeBrokenDiscoveryClient struct {
	bus                      mb.MessageBus
	registered, playersReady bool
	playerName               string
}

func (fbdc *FakeBrokenDiscoveryClient) Run() {
	fbdc.bus.Subscribe(DiscoveryTopic, func(e interface{}) {
		ev := e.(*fsm.Event)
		switch ev.Name {
		case PlayerReady:
			if fbdc.playersReady {
				playersReady := fsm.Event{
					Meta: &fsm.Metadata{FSM: ev.Meta.FSM},
					Name: PlayersReady,
				}
				fbdc.bus.Publish(fbdc.playerName, &playersReady)
			}

			gameError := fsm.Event{
				Meta: &fsm.Metadata{FSM: ev.Meta.FSM},
				Name: GameError,
			}
			fbdc.bus.Publish(fbdc.playerName, &gameError)
		case TCPCheckSuccess:
			event := fsm.Event{
				Meta: &fsm.Metadata{FSM: ev.Meta.FSM},
				Name: TCPCheckSuccessAll,
			}
			fbdc.bus.Publish(fbdc.playerName, &event)
		}
	})
}

// GetIn returns In channel of the client.
func (fbdc *FakeBrokenDiscoveryClient) GetIn() chan *pb.Event {
	return make(chan *pb.Event)
}

// GetOut returns Out channel of the client.
func (fbdc *FakeBrokenDiscoveryClient) GetOut() chan *pb.Event {
	return make(chan *pb.Event)
}

type BrokenNetworkChecker struct {
}

func (f *BrokenNetworkChecker) Verify(host, port string) error {
	return errors.New("some network error")
}

type BrokenSPDZEngine struct {
}

func (b *BrokenSPDZEngine) Execute(*pb.Event) error {
	return errors.New("some SPDZ error")
}

type FakeTransport struct {
}

func (t *FakeTransport) Run(f func()) error {
	f()
	return nil
}

func (t *FakeTransport) Stop() {
	return

}

func (t *FakeTransport) GetIn() chan *pb.Event {
	return make(chan *pb.Event)
}

func (t *FakeTransport) GetOut() chan *pb.Event {
	return make(chan *pb.Event)
}

func (t *FakeTransport) Events(stream pb.Discovery_EventsServer) error {
	return nil
}

type FakeNetworker struct {
	FreePorts []int32
}

func (f *FakeNetworker) CreateNetwork(pl *pb.Player) (int32, error) {
	port := f.FreePorts[0]
	f.FreePorts = f.FreePorts[1:]
	return port, nil
}

//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package ephemeral

import (
	"context"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/types"

	"go.uber.org/zap"
)

// ForwarderConf is the configuration of the event forwarder.
type ForwarderConf struct {
	Logger *zap.SugaredLogger
	Ctx    context.Context
	Player *Player1
	InCh   chan *pb.Event
	OutCh  chan *pb.Event
	Topic  string
}

// AbstractForwarder an interface for an entity that forwards events.
type AbstractForwarder interface {
	Run() error
}

// NewForwarder returns a new forwarder.
func NewForwarder(conf *ForwarderConf) *Forwarder {
	return &Forwarder{conf: conf}
}

// Forwarder forwards events from/out an FSM.
type Forwarder struct {
	conf *ForwarderConf
}

// Run forwards events from _in_ channel to player's internal message bus.
// It returns nil once the context is canceled.
func (f *Forwarder) Run() error {
	// Forward events to discovery service.
	f.conf.Player.Bus().Subscribe(DiscoveryTopic, func(e interface{}) {
		ev := e.(*fsm.Event)
		event := ev.Meta.TransportMsg
		f.conf.OutCh <- event
	})
	for {
		select {
		case e := <-f.conf.InCh:
			f.conf.Player.PublishEvent(e.Name, f.conf.Topic, e)
		case <-f.conf.Ctx.Done():
			f.conf.Logger.Info("Context canceled, cleaning up assigned resources...")
			f.conf.Player.Bus().Close(DiscoveryTopic)
			f.conf.Player.Stop()
			return nil
		}
	}
}

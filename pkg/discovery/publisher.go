//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	mb "github.com/vardius/message-bus"
)

// NewPublisher returns a new publisher.
func NewPublisher(bus mb.MessageBus) *Publisher {
	return &Publisher{
		Fsm: &fsm.FSM{},
		Bus: bus,
	}
}

// Publisher sends an event to the message bus.
type Publisher struct {
	Fsm *fsm.FSM
	Bus mb.MessageBus
}

// PublishExternalEvent publishes an instance of Event as received from
// the external source.
func (p *Publisher) PublishExternalEvent(ev *pb.Event, topic string) {
	p.Bus.Publish(topic, ev)
}

// Publish sends an FSM event to a given topic of the message bus.
// Not every call to Publish will have an srcTopic, thus make it of variable size.
func (p *Publisher) Publish(name, targetTopic string, srcTopics ...string) {
	event := fsm.Event{
		Name: name,
		Meta: &fsm.Metadata{
			FSM:         p.Fsm,
			TargetTopic: targetTopic,
			SrcTopics:   srcTopics,
		},
	}
	p.Bus.Publish(targetTopic, &event)
}

// PublishWithBody wraps a protobuf event into the fsm.Event and publishes it to the message bus.
func (p *Publisher) PublishWithBody(name, targetTopic string, body *pb.Event, srcTopics ...string) {
	event := fsm.Event{
		Name: name,
		Meta: &fsm.Metadata{
			FSM:          p.Fsm,
			TargetTopic:  targetTopic,
			SrcTopics:    srcTopics,
			TransportMsg: body,
		},
	}
	p.Bus.Publish(targetTopic, &event)
}

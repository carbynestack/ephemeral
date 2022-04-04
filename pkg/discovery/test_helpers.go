//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
)

// WaitDoneOrTimeout waits until either something came from the done channel or a timeout occured
func WaitDoneOrTimeout(done chan struct{}, t ...time.Duration) {
	var timeout time.Duration
	if t == nil {
		timeout = 1 * time.Second
	} else {
		timeout = t[0]
	}
	select {
	case <-done:
		// the ping is on time, just continue.
	case <-time.After(timeout):
		err := fmt.Sprintf("test timeout, exit after %s", timeout)
		// make the test fail.
		Expect(err).NotTo(Equal(err))
	}
}

// GenerateEvents id is a helper method to generate a list of events produced by discovery clients.
func GenerateEvents(name string, IDs ...string) []*pb.Event {
	events := make([]*pb.Event, len(IDs))
	for i, id := range IDs {
		ev := &pb.Event{
			Name:    name,
			GameID:  id,
			Players: []*pb.Player{&pb.Player{}},
		}
		events[i] = ev
	}
	return events
}

// Assert subscribes for a topic in the message bus, waits until the 'event' received, executes the assertions and signals to the done channel once finished.
// if no assetions are specified, it just verifies that the event was received.
func Assert(event string, f FSMWithBus, done chan struct{}, assert func([]string), topic ...string) {
	var t string
	if topic != nil {
		t = topic[0]
	} else {
		t = DiscoveryTopic
	}
	f.Bus().Subscribe(t, func(e interface{}) error {
		defer GinkgoRecover()
		ev := e.(*fsm.Event)
		if ev.Name == event {
			defer func() {
				done <- struct{}{}
			}()
			states := f.History().GetStates()
			assert(states)
		}
		return nil
	})
}

// GamesWithBus is tuple of Game and MessageBus
type GamesWithBus struct {
	Games map[string]*Game
	Bus   mb.MessageBus
}

// assertExternalEvent runs assertions against received games.
// Note that games slice should be already propagated before calling this method.
func assertExternalEvent(event *pb.Event, topic string, g *GamesWithBus, done chan struct{}, assert func([]string)) {
	g.Bus.Subscribe(topic, func(e interface{}) error {
		defer GinkgoRecover()
		ev := e.(*pb.Event)
		if ev.GameID == event.GameID && ev.Name == event.Name {
			defer func() {
				done <- struct{}{}
			}()
			states := g.Games[ev.GameID].History().GetStates()
			assert(states)
		}
		return nil
	})
}

// assertExternalEventWithBody is nearly the same as assertExternalEvent but it passes the event itself as a parameter to the assert function.
func assertExternalEventBody(event *pb.Event, topic string, g *GamesWithBus, done chan struct{}, assert func(*pb.Event)) {
	g.Bus.Subscribe(topic, func(e interface{}) error {
		defer GinkgoRecover()
		ev := e.(*pb.Event)
		if ev.GameID == event.GameID && ev.Name == event.Name {
			defer func() {
				done <- struct{}{}
			}()
			assert(ev)
		}
		return nil
	})
}

// StatesAsserter allows checking for returned states from a Game more easily
type StatesAsserter struct {
	states       []string
	currentIndex int
}

// NewStatesAsserter creates a new StatesAsserter that checks the provided states slice
func NewStatesAsserter(states []string) *StatesAsserter {
	return &StatesAsserter{
		states: states,
	}
}

// ExpectNext returns an Assertion over the next element of the internal states slice.
//  This method does not perform any bounds checking, so calling this one time too many will panic
func (s *StatesAsserter) ExpectNext() Assertion {
	state := s.states[s.currentIndex]
	s.currentIndex++
	return Expect(state)
}

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
)

var _ = Describe("Publisher", func() {

	Context("when publishing gRPC events", func() {
		It("sends an event to the bus", func() {
			name := "testEvent"
			topic := "testTopic"
			src := "origin"
			bus := mb.New(100)
			pub := NewPublisher(bus)
			Expect(pub.Bus).NotTo(BeNil())
			grpcMsg := &pb.Event{
				GameID: "abc",
			}
			done := make(chan struct{})
			bus.Subscribe(topic, func(e interface{}) {
				defer func() {
					GinkgoRecover()
					done <- struct{}{}
				}()
				ev := e.(*fsm.Event)
				Expect(ev.Name).To(Equal(name))
				Expect(ev.Meta.TransportMsg.GameID).To(Equal(grpcMsg.GameID))
				return
			})
			// Subscribing to the topic is asynchronous, it could race with publishing to the topic.
			// The sleep below should be long enough to prevent it.
			time.Sleep(100 * time.Millisecond)
			pub.PublishWithBody(name, topic, grpcMsg, src)
			select {
			case <-done:
				// Success, do nothing
			case <-time.After(10 * time.Second):
				Expect("").To(Equal("timeout doesn't happen"))
			}
		})
	})
})

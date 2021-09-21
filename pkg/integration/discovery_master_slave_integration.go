//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package integration

import (
	"context"
	d "github.com/carbynestack/ephemeral/pkg/discovery"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	c "github.com/carbynestack/ephemeral/pkg/discovery/transport/client"
	proto "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/discovery/transport/server"
	"time"

	. "github.com/carbynestack/ephemeral/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
)

var _ = Describe("Discovery cluster", func() {
	// Please note, this test doesn't require a real k8s cluster with ephemeral, it runs locally.
	It("forwards events from slave to master and backwards", func() {
		portMaster := "8081"
		portSlave := "8082"
		logger := zap.NewNop().Sugar()
		busMaster := mb.New(10000)
		busSlave := mb.New(10000)
		masterFrontend := "192.168.0.1"
		slaveFrontend := "192.168.0.2"
		master := getDiscovery(portMaster, logger, busMaster, masterFrontend, ModeMaster)
		slave := getDiscovery(portSlave, logger, busSlave, slaveFrontend, ModeSlave)
		done := make(chan struct{})
		busMaster.Subscribe(ClientIncomingEventsTopic, func(e interface{}) {
			defer GinkgoRecover()
			ev := e.(*proto.Event)
			if ev.Name == PlayerReady {
				defer func() {
					done <- struct{}{}
				}()
			}
		})
		busSlave.Subscribe(ClientOutgoingEventsTopic, func(e interface{}) {
			defer GinkgoRecover()
			ev := e.(*proto.Event)
			if ev.Name == PlayersReady {
				defer func() {
					done <- struct{}{}
				}()
			}
		})
		go master.Start()
		go slave.Start()
		err := master.WaitUntilReady(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())
		err = slave.WaitUntilReady(5 * time.Second)
		Expect(err).NotTo(HaveOccurred())
		ready := d.GenerateEvents(PlayerReady, "0")[0]
		pb := d.Publisher{
			Bus: busSlave,
			Fsm: &fsm.FSM{},
		}
		pb.PublishExternalEvent(ready,
			MasterOutgoingEventsTopic)
		pb.PublishExternalEvent(ready,
			MasterOutgoingEventsTopic)
		d.WaitDoneOrTimeout(done)
		d.WaitDoneOrTimeout(done)
		d.WaitDoneOrTimeout(done)
	})
})

func getDiscovery(port string, logger *zap.SugaredLogger, bus mb.MessageBus, frontend string, mode string) *d.ServiceNG {
	in := make(chan *proto.Event)
	out := make(chan *proto.Event)
	errCh := make(chan error)
	serverConf := &server.TransportConfig{
		In:     in,
		Out:    out,
		ErrCh:  errCh,
		Port:   port,
		Logger: logger,
	}
	tr := server.NewTransportServer(serverConf)
	pb := d.NewPublisher(bus)
	stateTimeout := 10 * time.Second
	n := &d.FakeNetworker{
		FreePorts: []int32{30000, 30001, 30002},
	}
	inClient := make(chan *proto.Event)
	outClient := make(chan *proto.Event)

	clientConf := &c.TransportClientConfig{
		In:         inClient,
		Out:        outClient,
		ErrCh:      errCh,
		Host:       "localhost",
		Port:       "8081",
		Logger:     logger,
		ConnID:     "abc",
		EventScope: EventScopeAll,
		Timeout:    10 * time.Second,
		Context:    context.TODO(),
	}
	cl, _ := c.NewClient(clientConf)
	s := d.NewServiceNG(bus, pb, stateTimeout, tr, n, frontend, logger, mode, cl)
	return s
}

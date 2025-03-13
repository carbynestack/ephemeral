// Copyright (c) 2021-2025 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/discovery"
	c "github.com/carbynestack/ephemeral/pkg/discovery/transport/client"
	cl "github.com/carbynestack/ephemeral/pkg/discovery/transport/client"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	proto "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/discovery/transport/server"
	l "github.com/carbynestack/ephemeral/pkg/logger"
	"github.com/carbynestack/ephemeral/pkg/types"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"github.com/carbynestack/ephemeral/pkg/utils"
	mb "github.com/vardius/message-bus"
	"go.uber.org/zap"
	"time"
)

const (
	// DefaultPort is the port the server will be listen on.
	DefaultPort = "8080"
	// DefaultBusSize is the size of the in-memory message bus used for FSM and communication with clients.
	DefaultBusSize = 10000
	// DefaultPortRange is the range of ports used for MCP communication between the players.
	DefaultPortRange      = "30000:30100"
	defaultConfigLocation = "/etc/config/config.json"
)

func main() {
	config, err := ParseConfig(defaultConfigLocation)
	if err != nil {
		panic(err)
	}
	logger, err := l.NewDevelopmentLogger()
	if err != nil {
		panic(err)
	}
	SetDefaults(config)
	logger.Infof("Starting with the config %v", config)
	bus := mb.New(config.BusSize)
	tr := NewTransportServer(logger, config.Port)
	pb := discovery.NewPublisher(bus)
	doneCh := make(chan string)
	errCh := make(chan error, 1)

	n, err := discovery.NewIstioNetworker(logger, config.PortRange, doneCh)
	if err != nil {
		panic(err)
	}
	var upstreamConfig *DiscoveryClientTypedConfig
	if config.Slave {
		upstreamConfig = &DiscoveryClientTypedConfig{
			Host:           config.MasterHost,
			Port:           config.MasterPort,
			ConnectTimeout: config.ConnectTimeout,
		}
	}
	client, mode, err := NewClient(upstreamConfig, logger, errCh)
	if err != nil {
		panic(err)
	}
	// TODO: extract this Istio address dynamically.
	s := discovery.NewServiceNG(bus, pb, config.StateTimeout, config.ComputationTimeout, tr, n, config.FrontendURL, logger, mode, client, config.PlayerCount)
	if err != nil {
		panic(err)
	}

	err = n.Run()
	if err != nil {
		panic(err)
	}
	go RunDeletion(doneCh, errCh, logger, s)
	if err = s.Start(); err != nil {
		errCh <- err
	}
}

// NewClient returns a new client with parameters specific to the server mode. If upstreamClient is defined, the client
// will be configured to forward incoming events to an upstream master server. With upstreamClient set to nil, the
// service is considered to be the master service.
func NewClient(upstreamConfig *types.DiscoveryClientTypedConfig, logger *zap.SugaredLogger, errCh chan error) (*cl.Client, string, error) {
	logger.Debug("Creating new discovery client")
	mode := ModeMaster
	client := &cl.Client{}
	var err error
	if upstreamConfig != nil { // If Follower/Slave -> Open GRPc Connection to Master
		inCh := make(chan *proto.Event)
		outCh := make(chan *proto.Event)
		grpcClientConf := &c.TransportClientConfig{
			In:             inCh,
			Out:            outCh,
			ErrCh:          errCh,
			Host:           upstreamConfig.Host,
			Port:           upstreamConfig.Port,
			EventScope:     EventScopeAll,
			ConnID:         "slave",
			ConnectTimeout: upstreamConfig.ConnectTimeout,
			Logger:         logger,
			Context:        context.Background(),
		}
		client, err = c.NewClient(grpcClientConf)
		if err != nil {
			return nil, "", err
		}
		mode = ModeSlave
	}
	return client, mode, nil
}

// NewTransportServer returns a gRPC transport server.
func NewTransportServer(logger *zap.SugaredLogger, port string) *server.TransportServer {
	serverIn := make(chan *pb.Event)
	serverOut := make(chan *pb.Event)
	serverErr := make(chan error)
	grpcServerConf := &server.TransportConfig{
		In:     serverIn,
		Out:    serverOut,
		ErrCh:  serverErr,
		Logger: logger,
		Port:   port,
	}
	return server.NewTransportServer(grpcServerConf)
}

// RunDeletion removes the Networks depending on the scale down of the Knative services.
func RunDeletion(doneCh chan string, errCh chan error, logger *zap.SugaredLogger, s *discovery.ServiceNG) {
	for {
		select {
		case name := <-doneCh:
			logger.Debugf("Deleting the network %s from our bookkeeping\n", name)
			s.DeleteCallback(name)
		case err := <-errCh:
			panic(err)
		}
	}
}

// ParseConfig parses the configuration file of the discovery service.
func ParseConfig(path string) (*DiscoveryTypedConfig, error) {
	bytes, err := utils.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var conf DiscoveryConfig
	err = json.Unmarshal(bytes, &conf)
	if err != nil {
		return nil, err
	}
	if conf.FrontendURL == "" {
		return nil, errors.New("missing config error, FrontendURL must be defined")
	}
	if conf.MasterHost == "" && conf.Slave {
		return nil, errors.New("missing config error, MasterHost must be defined")
	}
	if conf.MasterPort == "" && conf.Slave {
		return nil, errors.New("missing config error, MasterPort must be defined")
	}
	if conf.PlayerCount == 0 {
		return nil, errors.New("missing config error, PlayerCount must be defined")
	}
	if conf.PlayerCount < 2 {
		return nil, errors.New("invalid config error, PlayerCount must be 2 or higher")
	}
	stateTimeout, err := time.ParseDuration(conf.StateTimeout)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("invalid state timeout format: %v", err))
	}
	computationTimeout, err := time.ParseDuration(conf.ComputationTimeout)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("invalid computation timeout format: %v", err))
	}
	connectTimeout, err := time.ParseDuration(conf.ConnectTimeout)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("invalid connection timeout format: %v", err))
	}
	return &DiscoveryTypedConfig{
		FrontendURL:        conf.FrontendURL,
		MasterHost:         conf.MasterHost,
		MasterPort:         conf.MasterPort,
		Slave:              conf.Slave,
		StateTimeout:       stateTimeout,
		ComputationTimeout: computationTimeout,
		ConnectTimeout:     connectTimeout,
		Port:               conf.Port,
		BusSize:            conf.BusSize,
		PortRange:          conf.PortRange,
		PlayerCount:        conf.PlayerCount,
	}, nil
}

// SetDefaults sets the default values for config properties if they are not set.
func SetDefaults(conf *DiscoveryTypedConfig) {
	if conf.Port == "" {
		conf.Port = DefaultPort
	}
	if conf.BusSize == 0 {
		conf.BusSize = DefaultBusSize
	}
	if conf.PortRange == "" {
		conf.PortRange = DefaultPortRange
	}
}

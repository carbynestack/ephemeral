//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package ephemeral

import (
	d "github.com/carbynestack/ephemeral/pkg/discovery"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/network"
	. "github.com/carbynestack/ephemeral/pkg/types"
	. "github.com/carbynestack/ephemeral/pkg/utils"

	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"

	"go.uber.org/zap"
)

const (
	proxyAddress      = "localhost"
	basePort          = int32(10000)
	appName           = "mpc-program"
	baseDir           = "/mp-spdz"
	ipFile            = baseDir + "/ip-file"
	timeout           = 20 * time.Second
	tcpCheckerTimeout = 50 * time.Millisecond
	defaultPath       = baseDir + "/Programs/Source/" + appName + ".mpc"
)

// MPCEngine is an interface for an MPC runtime that performs the computation.
type MPCEngine interface {
	Execute(*pb.Event) error
}

// NewSPDZWrapper returns a new SPDZ wrapper.
func NewSPDZWrapper(ctx *CtxConfig, respCh chan []byte, errCh chan error, logger *zap.SugaredLogger, act func(*CtxConfig) ([]byte, error)) *SPDZWrapper {
	return &SPDZWrapper{
		ctx:      ctx,
		activate: act,
		respCh:   respCh,
		errCh:    errCh,
		logger:   logger,
	}
}

// SPDZWrapper starts the computation and reads the output parameters.
// It is also used to update player's proxy configuration based on the discovery output.
type SPDZWrapper struct {
	ctx      *CtxConfig
	activate func(*CtxConfig) ([]byte, error)
	respCh   chan []byte
	errCh    chan error
	logger   *zap.SugaredLogger
}

// Execute runs the MPC computation.
func (s *SPDZWrapper) Execute(event *pb.Event) error {
	ip, port, err := s.getPlayerIPAndPort(event.Players)
	if err != nil {
		return err
	}
	s.ctx.Proxy.Host = ip
	s.ctx.Proxy.Port = port
	s.ctx.Proxy.LocalPort = s.getPort(s.ctx.Spdz.PlayerID)
	s.ctx.ErrCh = s.errCh
	s.logger.Debug("Starting MPC execution")
	res, err := s.activate(s.ctx)
	if err != nil {
		s.errCh <- err
		return err
	}
	s.logger.Debugw("SPDZWrapper is writing response", GameID, s.ctx.Act.GameID)
	s.respCh <- res
	return err
}

func (s *SPDZWrapper) getPlayerIPAndPort(pls []*pb.Player) (string, string, error) {
	for _, player := range pls {
		// The port always includes the second player, change it if the number of players > 2.
		if player.Id-100 == 1 {
			return player.Ip, strconv.Itoa(int(player.Port)), nil
		}
	}
	return "", "", fmt.Errorf("player with ID %d not found", s.ctx.Spdz.PlayerID)
}

// getPort returns back the port that is set by the proxy.
// Note, it only works until the ExpectedPlayers is equal to 2.
func (s *SPDZWrapper) getPort(id int32) string {
	if id == 0 {
		return "5001"
	}
	return "5000"
}

// NewSPDZEngine returns a new instance of SPDZ engine that knows how to compile and trigger an execution of SPDZ runtime.
func NewSPDZEngine(logger *zap.SugaredLogger, cmder Executor, config *SPDZEngineTypedConfig) *SPDZEngine {
	c := &network.TCPCheckerConf{
		DialTimeout:  tcpCheckerTimeout,
		RetryTimeout: timeout,
		Logger:       logger,
	}
	feeder := NewAmphoraFeeder(logger, config)
	checker := network.NewTCPChecker(c)
	proxy := network.NewProxy(logger, config, checker)
	return &SPDZEngine{logger: logger,
		cmder:          cmder,
		config:         config,
		checker:        checker,
		feeder:         feeder,
		sourceCodePath: defaultPath,
		proxy:          proxy,
		baseDir:        baseDir,
		ipFile:         ipFile,
	}
}

// SPDZEngine compiles, executes, provides IO operations for SPDZ based runtimes.
type SPDZEngine struct {
	logger         *zap.SugaredLogger
	cmder          Executor
	config         *SPDZEngineTypedConfig
	doneCh         chan struct{}
	checker        network.NetworkChecker
	feeder         Feeder
	sourceCodePath string
	proxy          network.AbstractProxy
	baseDir        string
	ipFile         string
}

// Activate starts a proxy, writes an IP file, start SPDZ execution, unpacks inputs parameters, sends them to the runtime and waits for the response.
func (s *SPDZEngine) Activate(ctx *CtxConfig) ([]byte, error) {
	errCh := make(chan error, 1)
	act := ctx.Act
	err := s.proxy.Run(ctx, errCh)
	if err != nil {
		msg := "error starting the tcp proxy"
		s.logger.Errorw(msg, GameID, act.GameID)
		return nil, fmt.Errorf("%s: %s", msg, err)
	}
	defer func() {
		select {
		case err := <-errCh:
			s.logger.Errorw(err.Error(), GameID, act.GameID)
		default:
			s.proxy.Stop()
		}
	}()
	err = s.writeIPFile(s.ipFile, proxyAddress, d.ExpectedPlayers)
	if err != nil {
		msg := "error due to writing to the ip file"
		s.logger.Errorw(msg, GameID, act.GameID)
		return nil, fmt.Errorf("%s: %s", msg, err)
	}
	go s.startMPC(ctx)

	feedPort := s.getFeedPort()
	go func() {
		select {
		case <-ctx.Context.Done():
			s.logger.Debug("Closing the TCP socket connection - context cancelled")
			s.feeder.Close()
		case <-time.After(s.config.RetryTimeout):
			s.logger.Debug("Closing the TCP socket connection - retry timeout exceeded")
			s.feeder.Close()
		}
	}()
	// Read the secret shares either from Amphora or from the http request.
	if len(act.AmphoraParams) > 0 {
		return s.feeder.LoadFromSecretStoreAndFeed(act, feedPort, ctx)
	}
	if len(act.SecretParams) > 0 {
		return s.feeder.LoadFromRequestAndFeed(act, feedPort, ctx)
	}
	// The line below should be never reached, since we check activations parameters in the request handlers. However, leaving it here for completeness.
	return nil, errors.New("no MPC parameters specified")
}

// Compile compiles a SPDZ application.
func (s *SPDZEngine) Compile(ctx *CtxConfig) error {
	act := ctx.Act
	path := s.sourceCodePath
	data := []byte(act.Code)
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	_, err = s.cmder.CallCMD([]string{fmt.Sprintf("./compile.py %s", appName)}, s.baseDir)
	if err != nil {
		return err
	}
	return nil
}

// getFeedPort returns the port on which SPDZ accepts input parameters.
func (s *SPDZEngine) getFeedPort() string {
	return strconv.FormatInt(int64(basePort+s.config.PlayerID), 10)
}

func (s *SPDZEngine) startMPC(ctx *CtxConfig) {
	s.logger.Infow("Starting Player-Online.x", GameID, ctx.Act.GameID)
	stdout, err := s.cmder.CallCMD([]string{fmt.Sprintf("./Player-Online.x %s %s -N %s --ip-file-name %s", fmt.Sprint(s.config.PlayerID), appName, fmt.Sprint(d.ExpectedPlayers), ipFile)}, s.baseDir)
	if err != nil {
		err := fmt.Errorf("error while executing the user code: %v", err)
		ctx.ErrCh <- err
		s.logger.Errorw(err.Error(), GameID, ctx.Act.GameID)
	}
	s.logger.Infow(fmt.Sprintf("===== Begin of stdout from the user container =====\n%s", string(stdout)), GameID, ctx.Act.GameID)
	s.logger.Infow("===== End of stdout from the user container =====", GameID, ctx.Act.GameID)
}

func (s *SPDZEngine) writeIPFile(path string, addr string, parties int32) error {
	var addrs string
	for i := int32(0); i < parties; i++ {
		addrs = addrs + fmt.Sprintf("%s\n", addr)
	}
	data := []byte(addrs)
	return ioutil.WriteFile(path, data, 0644)
}

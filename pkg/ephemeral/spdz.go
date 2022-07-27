//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package ephemeral

import (
	"context"
	"errors"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/castor"
	d "github.com/carbynestack/ephemeral/pkg/discovery"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	. "github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	"github.com/carbynestack/ephemeral/pkg/ephemeral/network"
	. "github.com/carbynestack/ephemeral/pkg/types"
	. "github.com/carbynestack/ephemeral/pkg/utils"
	"github.com/google/uuid"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	proxyAddress        = "localhost"
	basePort            = int32(10000)
	appName             = "mpc-program"
	baseDir             = "/mp-spdz"
	ipFile              = baseDir + "/ip-file"
	timeout             = 20 * time.Second
	tcpCheckerTimeout   = 50 * time.Millisecond
	defaultPath         = baseDir + "/Programs/Source/" + appName + ".mpc"
	defaultSchedulePath = baseDir + "/Programs/Schedules/" + appName + ".sch"
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
	entries, err := s.getProxyEntries(event.Players)
	if err != nil {
		return err
	}
	s.ctx.ProxyEntries = entries
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

func (s *SPDZWrapper) getProxyEntries(pls []*pb.Player) ([]*ProxyConfig, error) {
	if len(pls) == 1 {
		return nil, errors.New("you must provide at least two players")
	}
	// Copy to new Slice so that we don't modify the original Slice (just in case)
	players := make([]*pb.Player, len(pls))
	copy(players, pls)
	sort.Slice(players, func(left, right int) bool {
		return players[left].Id < players[right].Id
	})
	var proxyEntries []*ProxyConfig
	for _, player := range players {
		// TODO: remove this 100 hack, it is a temp workaround for protobuf3.
		// Create proxy entries for all OTHER players
		if (player.Id - 100) != s.ctx.Spdz.PlayerID {
			proxyEntries = append(proxyEntries, &ProxyConfig{
				Host:      player.Ip,
				Port:      strconv.Itoa(int(player.Port)),
				LocalPort: s.getLocalPortForPlayer(player.Id - 100),
			})
		}
	}
	s.logger.Infow("Created ProxyEntries", "ProxyEntries", proxyEntries, "Players", players)
	if len(proxyEntries) != len(players)-1 {
		return nil, errors.New("could not get all ProxyEntries")
	}
	return proxyEntries, nil
}

// getLocalPortForPlayer returns the port that is set by the proxy.
func (s *SPDZWrapper) getLocalPortForPlayer(id int32) string {
	return strconv.Itoa(int(d.BasePort + id))
}

// TupleStreamerFactory is a factory method to create new io.TupleStreamer.
//
// It accepts a logger, the type of tuples it provides, the spdz config, path to the player data base directory, the
// game ID, as well as the thread it serves. It either returns a pointer to the new TupleStreamer or an error if
// creation failed.
type TupleStreamerFactory func(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, playerDataDir string, gameID uuid.UUID, threadNr int) (TupleStreamer, error)

// DefaultCastorTupleStreamerFactory default implementation of TupleStreamerFactory creating new io.CastorTupleStreamer
func DefaultCastorTupleStreamerFactory(l *zap.SugaredLogger, tt castor.TupleType, conf *SPDZEngineTypedConfig, playerDataDir string, gameID uuid.UUID, threadNr int) (TupleStreamer, error) {
	return NewCastorTupleStreamer(l, tt, conf, playerDataDir, gameID, threadNr)
}

// NewSPDZEngine returns a new instance of SPDZ engine that knows how to compile and trigger an execution of SPDZ runtime.
func NewSPDZEngine(logger *zap.SugaredLogger, cmder Executor, config *SPDZEngineTypedConfig) (*SPDZEngine, error) {
	c := &network.TCPCheckerConf{
		DialTimeout:  tcpCheckerTimeout,
		RetryTimeout: timeout,
		Logger:       logger,
	}
	feeder := NewAmphoraFeeder(logger, config)
	checker := network.NewTCPChecker(c)
	proxy := network.NewProxy(logger, config, checker)
	playerDataPaths, err := preparePlayerData(config)
	if err != nil {
		return nil, err
	}
	return &SPDZEngine{logger: logger,
		cmder:           cmder,
		config:          config,
		checker:         checker,
		feeder:          feeder,
		playerDataPaths: playerDataPaths,
		sourceCodePath:  defaultPath,
		schedulePath:    defaultSchedulePath,
		proxy:           proxy,
		baseDir:         baseDir,
		ipFile:          ipFile,
		streamerFactory: DefaultCastorTupleStreamerFactory,
	}, nil
}

// SPDZEngine compiles, executes, provides IO operations for SPDZ based runtimes.
type SPDZEngine struct {
	logger          *zap.SugaredLogger
	cmder           Executor
	config          *SPDZEngineTypedConfig
	doneCh          chan struct{}
	checker         network.NetworkChecker
	feeder          Feeder
	playerDataPaths map[castor.SPDZProtocol]string
	sourceCodePath  string
	schedulePath    string
	proxy           network.AbstractProxy
	baseDir         string
	ipFile          string
	streamerFactory TupleStreamerFactory
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
	err = s.writeIPFile(s.ipFile, proxyAddress, ctx.Spdz.PlayerCount)
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
			_ = s.feeder.Close()
		case <-time.After(s.config.RetryTimeout):
			s.logger.Debug("Closing the TCP socket connection - retry timeout exceeded")
			_ = s.feeder.Close()
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

func (s *SPDZEngine) getNumberOfThreads() (int, error) {
	file, err := Fio.OpenRead(s.schedulePath)
	if err != nil {
		msg := "error accessing the program's schedule"
		return 0, fmt.Errorf("%s: %s", msg, err)
	}
	defer file.Close()
	nThreads, err := Fio.ReadLine(file)
	if err != nil {
		msg := "error reading number of threads"
		return 0, fmt.Errorf("%s: %s", msg, err)
	}
	return strconv.Atoi(nThreads)
}

// Compile compiles a SPDZ application and returns the number of threads declared by the program.
func (s *SPDZEngine) Compile(ctx *CtxConfig) error {
	act := ctx.Act
	path := s.sourceCodePath
	data := []byte(act.Code)
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return err
	}
	var stdoutSlice []byte
	var stderrSlice []byte
	command := fmt.Sprintf("./compile.py -M %s", appName)
	stdoutSlice, stderrSlice, err = s.cmder.CallCMD(context.TODO(), []string{command}, s.baseDir)
	stdOut := string(stdoutSlice)
	stdErr := string(stderrSlice)
	s.logger.Debugw("Compiled Successfully", "Command", command, "StdOut", stdOut, "StdErr", stdErr)
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
	s.logger.Debugw("Starting MPC", GameID, ctx.Act.GameID)
	nThreads, err := s.getNumberOfThreads()
	if err != nil {
		ctx.ErrCh <- fmt.Errorf("failed to determine the number of threads: %v", err)
		return
	}
	wg := new(sync.WaitGroup)
	defer func() {
		gracefully := make(chan struct{})
		go func() {
			wg.Wait()
			close(gracefully)
		}()
		select {
		case <-gracefully:
		case <-time.After(time.Second * 30):
			s.logger.Error("Tuple streamers have not terminated gracefully")
		}
	}()

	var tupleStreamers = []TupleStreamer{}
	gameUUID, err := uuid.Parse(ctx.Act.GameID)
	if err != nil {
		ctx.ErrCh <- fmt.Errorf("error parsing gameID: %v", err)
		return
	}
	for _, tt := range castor.SupportedTupleTypes {
		for thread := 0; thread < nThreads; thread++ {
			streamer, err := s.streamerFactory(s.logger, tt, s.config, s.playerDataPaths[tt.SpdzProtocol], gameUUID, thread)
			if err != nil {
				s.logger.Errorw("Error when initializing tuple streamer", GameID, ctx.Act.GameID, TupleType, tt, "Error", err)
				ctx.ErrCh <- err
				return
			}
			tupleStreamers = append(tupleStreamers, streamer)
		}
	}
	computationFinished := make(chan struct{})
	terminateStreams := make(chan struct{})
	streamErrCh := make(chan error, len(castor.SupportedTupleTypes))
	for _, s := range tupleStreamers {
		wg.Add(1)
		s.StartStreamTuples(terminateStreams, streamErrCh, wg)
	}
	command := []string{fmt.Sprintf("./Player-Online.x %s %s -N %s --ip-file-name %s --file-prep-per-thread", fmt.Sprint(s.config.PlayerID), appName, fmt.Sprint(ctx.Spdz.PlayerCount), ipFile)}
	s.logger.Infow("Starting Player-Online.x", GameID, ctx.Act.GameID, "command", command)
	go func() {
		stdout, stderr, err := s.cmder.CallCMD(ctx.Context, command, s.baseDir)
		if err != nil {
			s.logger.Errorw("Error while executing the user code", GameID, ctx.Act.GameID, "StdErr", string(stderr), "StdOut", string(stdout), "error", err)
			err := fmt.Errorf("error while executing the user code: %v", err)
			ctx.ErrCh <- err
		} else {
			s.logger.Debugw("Computation finished", GameID, ctx.Act.GameID, "StdErr", string(stderr), "StdOut", string(stdout))
		}
		close(computationFinished)
	}()
	select {
	case <-computationFinished:
	case err := <-streamErrCh:
		error := fmt.Errorf("error while streaming tuples: %v", err)
		s.logger.Error(error)
		ctx.ErrCh <- error
	}
	close(terminateStreams)
}

func (s *SPDZEngine) writeIPFile(path string, addr string, parties int32) error {
	var addrs string
	for i := int32(0); i < parties; i++ {
		addrs = addrs + fmt.Sprintf("%s\n", addr)
	}
	data := []byte(addrs)
	s.logger.Infow("Writing to IPFile: ", "path", path, "content", addrs, "proxy address", addr, "parties", parties)
	return ioutil.WriteFile(path, data, 0644)
}

// preparePlayerData Returns the directories for the supported protocol's preprocessing data. It therefore creates
// the required directories and writes the mac keys  and other required parameters to the files expected by SPDZ.
func preparePlayerData(conf *SPDZEngineTypedConfig) (map[castor.SPDZProtocol]string, error) {
	playerDataDirs := make(map[castor.SPDZProtocol]string)
	for _, p := range castor.SupportedSPDZProtocols {
		path, err := createPlayerDataForProtocol(p, conf)
		if err != nil {
			return nil, fmt.Errorf("failed to create preprocessing data directories: %v", err)
		}
		playerDataDirs[p] = path
	}
	err := writeGfpParams(playerDataDirs[castor.SPDZGfp], conf.Prime)
	if err != nil {
		return nil, fmt.Errorf("failed to create gfp Player-params: %v", err)
	}
	return playerDataDirs, nil
}

func createPlayerDataForProtocol(p castor.SPDZProtocol, conf *SPDZEngineTypedConfig) (string, error) {
	var playerDataDir, macKey string
	switch p {
	case castor.SPDZGfp:
		playerDataDir = fmt.Sprintf("%s/%d-%s-%d/",
			conf.PrepFolder, conf.PlayerCount, castor.SPDZGfp.Shorthand, conf.Prime.BitLen())
		macKey = conf.GfpMacKey.String()
	case castor.SPDZGf2n:
		playerDataDir = fmt.Sprintf("%s/%d-%s-%d/",
			conf.PrepFolder, conf.PlayerCount, castor.SPDZGf2n.Shorthand, conf.Gf2nBitLength)
		macKey = conf.Gf2nMacKey
	default:
		panic("Unsupported SpdzProtocol " + p.Descriptor)
	}
	err := Fio.CreatePath(playerDataDir)
	if err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("error creating directory path: %v", err)
	}
	macKeyFileName := fmt.Sprintf("Player-MAC-Keys-%s-P%d", p.Shorthand, conf.PlayerID)
	err = writeMacKey(playerDataDir+macKeyFileName, conf.PlayerCount, macKey)
	if err != nil {
		return "", fmt.Errorf("failed to write mac key to file: %v", err)
	}

	return playerDataDir, nil
}

func writeMacKey(macKeyFilePath string, playerCount int32, macKey string) error {
	file, err := Fio.OpenWriteOrCreate(macKeyFilePath)
	if err != nil {
		return fmt.Errorf("failed creating mac key file: %v", err)
	}
	defer file.Close()
	_, err = file.WriteString(fmt.Sprintf("%d %s", playerCount, macKey))
	return err
}

func writeGfpParams(playerDataDir string, prime big.Int) error {
	file, err := Fio.OpenWriteOrCreate(filepath.Join(playerDataDir, "Params-Data"))
	if err != nil {
		return fmt.Errorf("failed creating gfp params file: %v", err)
	}
	defer file.Close()
	_, err = file.WriteString(prime.String())
	return err
}

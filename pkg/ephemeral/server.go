// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package ephemeral

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/carbynestack/ephemeral/pkg/discovery/fsm"
	. "github.com/carbynestack/ephemeral/pkg/types"
	. "github.com/carbynestack/ephemeral/pkg/utils"
	"io/ioutil"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	c "github.com/carbynestack/ephemeral/pkg/discovery/transport/client"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"

	mb "github.com/vardius/message-bus"

	"go.uber.org/zap"
)

type contextConf string

const paramsMsg = "either secret params or amphora secret share UUIDs must be specified, %s"

var (
	// The number of parallel games that could run per container.
	parallelGames  = 1
	defaultBusSize = 10000
	ctxConf        = contextConf("contextConf")
)

// NewServer returns a new server.
func NewServer(compile func(*CtxConfig) error, activate func(*CtxConfig) ([]byte, error), logger *zap.SugaredLogger, config *SPDZEngineTypedConfig) *Server {
	return &Server{
		player:   &PlayerWithIO{},
		compile:  compile,
		activate: activate,
		logger:   logger,
		config:   config,
		executor: NewCommander(),
	}
}

// Server is a HTTP server which wraps the handling of incoming requests that trigger the MPC computation.
type Server struct {
	player    AbstractPlayerWithIO
	compile   func(*CtxConfig) error
	activate  func(*CtxConfig) ([]byte, error)
	logger    *zap.SugaredLogger
	config    *SPDZEngineTypedConfig
	respCh    chan []byte
	errCh     chan error
	execErrCh chan error
	executor  Executor
}

// MethodFilter assures that only HTTP POST requests are able to get through.
func (s *Server) MethodFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "POST":
			if s.hasContentType(req, "application/json") {
				next.ServeHTTP(writer, req)
			} else {
				msg := "application/json content type must be provided"
				writer.WriteHeader(http.StatusUnsupportedMediaType)
				writer.Write([]byte(msg))
				s.logger.Error(msg)
				return
			}
		default:
			msg := "POST requests must be used to trigger a computation"
			writer.WriteHeader(http.StatusMethodNotAllowed)
			s.logger.Error(msg)
			writer.Write([]byte(msg))
		}
	})
}

// BodyFilter verifies all necessary parameters are set in the request body.
// Also sets the CtxConfig to the request
func (s *Server) BodyFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		var act Activation
		if req.Body == nil {
			msg := "request body is nil"
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(msg))
			s.logger.Error(msg)
			return
		}
		bodyBytes, _ := ioutil.ReadAll(req.Body)
		req.Body.Close()
		req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		err := json.Unmarshal(bodyBytes, &act)
		if err != nil {
			msg := "error decoding the request body"
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(msg))
			s.logger.Error(msg)
			return
		}
		if !isValidUUID(act.GameID) {
			msg := fmt.Sprintf("GameID %s is not a valid UUID", act.GameID)
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(msg))
			s.logger.Error(msg)
			return
		}
		if len(act.SecretParams) > 0 && len(act.AmphoraParams) > 0 {
			msg := fmt.Sprintf(paramsMsg, "not both of them")
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(msg))
			s.logger.Error(msg)
			return
		}
		if len(act.SecretParams) == 0 && len(act.AmphoraParams) == 0 {
			msg := fmt.Sprintf(paramsMsg, "none of them given")
			writer.WriteHeader(http.StatusBadRequest)
			writer.Write([]byte(msg))
			s.logger.Error(msg)
			return
		}
		if len(act.SecretParams) > 0 {
			for _, str := range act.SecretParams {
				_, err := base64.StdEncoding.DecodeString(str)
				if err != nil {
					msg := fmt.Sprintf("error decoding secret parameters: %s", err.Error())
					writer.WriteHeader(http.StatusBadRequest)
					writer.Write([]byte(msg))
					s.logger.Error(msg)
					return
				}
			}
		}
		con := context.Background()
		ctx := &CtxConfig{
			Act:  &act,
			Spdz: s.config,
		}
		con = context.WithValue(con, ctxConf, ctx)
		r := req.Clone(con)
		s.logger.Debug("Bodyfilter handler done")
		next.ServeHTTP(writer, r)
	})
}

// CompilationHandler parses the JSON payload and adds it to the request context.
func (s *Server) CompilationHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		conf, ok := req.Context().Value(ctxConf).(*CtxConfig)
		if !ok {
			writer.WriteHeader(http.StatusBadRequest)
			s.logger.Error("No context config provided")
			return
		}
		s.logger.Debugf("Executing Compilation Handler: %v", conf.Act)
		// These channels initialized here, because they must be unique
		// for each incoming request.
		s.respCh = make(chan []byte)
		s.errCh = make(chan error, parallelGames)
		s.execErrCh = make(chan error, parallelGames)

		// Compile the code if the parameter is specified.
		compileParam := req.URL.Query().Get("compile")
		if compileParam != "" {
			compile, err := strconv.ParseBool(compileParam)
			if err != nil {
				msg := fmt.Sprintf("error when reading the compile parameter: %s\n", err)
				writer.WriteHeader(http.StatusBadRequest)
				writer.Write([]byte(msg))
				s.logger.Errorw(msg, GameID, conf.Act.GameID)
				return
			}
			if compile {
				s.logger.Infow("Compiling the application", GameID, conf.Act.GameID)
				err := s.compile(conf)
				if err != nil {
					msg := fmt.Sprintf("error compiling the code: %s\n", err)
					writer.WriteHeader(http.StatusServiceUnavailable)
					writer.Write([]byte(msg))
					s.logger.Errorw(msg, GameID, conf.Act.GameID)
					return
				}
				s.logger.Debugw("Finished compiling the application", GameID, conf.Act.GameID)
			}
		}
		s.logger.Debug("Compilation handler done")
		next.ServeHTTP(writer, req)
	})
}

// ActivationHandler is the http handler starts the Player FSM.
func (s *Server) ActivationHandler(writer http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	ctxConfig := ctx.Value(ctxConf).(*CtxConfig)
	con, cancel := context.WithTimeout(ctx, ctxConfig.Spdz.StateTimeout*3+ctxConfig.Spdz.ComputationTimeout)
	defer cancel()
	deadline, _ := con.Deadline()
	s.logger.Debugw("Created Activation context", "Context", con, "Deadline", deadline)
	ctxConfig.Context = con
	pod, err := s.getPodName()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		s.logger.Errorw(fmt.Sprintf("Error retrieving pod name: %s", err), GameID, ctxConfig.Act.GameID)
	}
	s.logger.Debugf("Retrieved pod name %v", pod)

	spdz := NewSPDZWrapper(ctxConfig, s.respCh, s.execErrCh, s.logger, s.activate)
	plIO := s.getPlayer(func() AbstractPlayerWithIO {
		pl, err := NewPlayerWithIO(ctxConfig, &s.config.DiscoveryConfig, pod, spdz, s.config.StateTimeout, s.config.ComputationTimeout, s.errCh, s.logger)
		if err != nil {
			s.logger.Errorf("Failed to initialize Player: %v", err)
		}
		return pl
	})

	plIO.Start()

	select {
	case stdout := <-s.respCh:
		writer.WriteHeader(http.StatusOK)
		writer.Write(stdout)
	case err := <-s.errCh:
		msg := fmt.Sprintf("error while talking to Discovery: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(msg))
		s.logger.Errorw(msg, GameID, ctxConfig.Act.GameID)
	case err := <-s.execErrCh:
		msg := fmt.Sprintf("error during MPC execution: %s", err)
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(msg))
		s.logger.Errorw(msg, GameID, ctxConfig.Act.GameID)
	case <-con.Done():
		msg := fmt.Sprintf("timeout during activation procedure")
		writer.WriteHeader(http.StatusInternalServerError)
		writer.Write([]byte(msg))
		s.logger.Errorw(msg, GameID, ctxConfig.Act.GameID, "FSM History", plIO.History())
	}
	s.logger.Debug("Activation finalized")
}

// getPlayer is main purpose to test activation handler using a custom PlayerWithIO
func (s *Server) getPlayer(initializer func() AbstractPlayerWithIO) AbstractPlayerWithIO {
	switch s.player.(type) {
	case *PlayerWithIO:
		return initializer()
	default:
		return s.player
	}
}

// AbstractPlayerWithIO is an interface type for a PlayerWithIO.
type AbstractPlayerWithIO interface {
	Start()
	History() *fsm.History
}

// NewPlayerWithIO returns a new instance of PlayerWithIO.
func NewPlayerWithIO(ctx *CtxConfig, dcConf *DiscoveryClientTypedConfig, pod string, spdz MPCEngine, stateTimeout time.Duration, computationTimeout time.Duration, errCh chan error, logger *zap.SugaredLogger) (*PlayerWithIO, error) {
	bus := mb.New(defaultBusSize)

	name := NewTopicFromPlayerID(ctx)
	params := &PlayerParams{
		// probuf3 will omit playerID=0.
		PlayerID: ctx.Spdz.PlayerID + 100,
		Players:  ctx.Spdz.PlayerCount,
		Pod:      pod,
		IP:       ctx.Spdz.FrontendURL,
		GameID:   ctx.Act.GameID,
		Name:     name,
	}
	pl, _ := NewPlayer(ctx.Context, bus, stateTimeout, computationTimeout, spdz, params, errCh, logger)

	wires := &Wires{
		In:  make(chan *pb.Event, 1),
		Out: make(chan *pb.Event, 1),
		Err: errCh,
	}
	fConf := &ForwarderConf{
		Logger: logger,
		Ctx:    ctx.Context,
		Player: pl,
		InCh:   wires.In,
		OutCh:  wires.Out,
		Topic:  name,
	}
	forwarder := NewForwarder(fConf)

	cl, err := NewTransportClientFromDiverseConfigs(dcConf, ctx, logger, wires)
	if err != nil {
		return nil, err
	}
	return &PlayerWithIO{
		Forwarder: forwarder,
		Player:    pl,
		Wires:     wires,
		Client:    cl,
	}, nil
}

// PlayerWithIO contains the forwarder, player FSM, wires with input and output channels and discovery client.
type PlayerWithIO struct {
	Forwarder AbstractForwarder
	Player    AbstractPlayer
	Wires     *Wires
	Client    c.TransportClient
}

// Start activates the state machine of the player.
func (p *PlayerWithIO) Start() {
	// Receive events from the discovery service.
	go p.Forwarder.Run()
	conn, err := p.Client.Connect()
	if err != nil {
		p.Wires.Err <- err
		return
	}
	dc := pb.NewDiscoveryClient(conn)
	go p.Client.Run(dc)
	p.Player.Init()
}

// History returns the [fsm.History] of the game's statemachine.
func (p *PlayerWithIO) History() *fsm.History {
	return p.Player.History()
}

func (s *Server) getPodName() (string, error) {
	// TODO: this is brittle, read the pod name from more reliable place.
	//       use something like os.Getenv("HOST_NAME")?
	cmder := s.executor
	name, _, err := cmder.CallCMD(context.TODO(), []string{"hostname"}, "/")
	if err != nil {
		return "", err
	}
	hostname := strings.TrimSuffix(string(name), "\n")
	hostname = strings.TrimSuffix(hostname, "\r")
	return hostname, err
}

// Determine whether the request `content-type` includes a
// server-acceptable mime-type
//
// Failure should yield an HTTP 415 (`http.StatusUnsupportedMediaType`)
// Rented from https://gist.github.com/rjz/fe283b02cbaa50c5991e1ba921adf7c9.
func (s *Server) hasContentType(r *http.Request, mimetype string) bool {
	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		return mimetype == "application/octet-stream"
	}

	for _, v := range strings.Split(contentType, ",") {
		t, _, err := mime.ParseMediaType(v)
		if err != nil {
			break
		}
		if t == mimetype {
			return true
		}
	}
	return false
}

// Wires defines the channels used to communicate with the player.
type Wires struct {
	In  chan *pb.Event
	Out chan *pb.Event
	Err chan error
}

// NewTransportClientFromDiverseConfigs returns a new transport client.
func NewTransportClientFromDiverseConfigs(dcConf *DiscoveryClientTypedConfig, ctx *CtxConfig, logger *zap.SugaredLogger, ch *Wires) (*c.Client, error) {
	clientConf := &c.TransportClientConfig{
		In:             ch.In,
		Out:            ch.Out,
		ErrCh:          ch.Err,
		Host:           dcConf.Host,
		Port:           dcConf.Port,
		Logger:         logger,
		ConnID:         ctx.Act.GameID,
		EventScope:     EventScopeSelf,
		ConnectTimeout: dcConf.ConnectTimeout,
		Context:        ctx.Context,
	}
	cl, err := c.NewClient(clientConf)
	if err != nil {
		return nil, err
	}
	return cl, nil
}

// NewTopicFromPlayerID converts player ID so it can be used as topic name.
func NewTopicFromPlayerID(ctx *CtxConfig) string {
	return strconv.Itoa(int(ctx.Spdz.PlayerID))
}

// isValidUUID returns true if the uuid is valid, false otherwise.
func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package ephemeral

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/carbynestack/ephemeral/pkg/ephemeral/io"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Server", func() {

	var (
		act        *Activation
		handler200 http.Handler
		rr         *httptest.ResponseRecorder
		s          *Server
		l          *zap.SugaredLogger
	)

	Context("when sending http requests", func() {
		BeforeEach(func() {
			act = &Activation{
				AmphoraParams: []string{"a"},
			}
			handler200 = http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
				writer.WriteHeader(http.StatusOK)
			})
			rr = httptest.NewRecorder()

			l = zap.NewNop().Sugar()
			s = NewServer(func(*CtxConfig) error { return nil }, func(*CtxConfig) ([]byte, error) { return nil, nil }, l, &SPDZEngineTypedConfig{})
		})

		Context("when going through body filter", func() {
			It("add ctxConfig to the request", func() {
				handler200 = http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
					ctx := req.Context()
					ctxConfig := ctx.Value(ctxConf).(*CtxConfig)
					Expect(ctxConfig).NotTo(BeNil())
					Expect(ctxConfig.Act.GameID).To(Equal(act.GameID))
					writer.WriteHeader(http.StatusOK)
				})
				body, _ := json.Marshal(&act)
				req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
				s.BodyFilter(handler200).ServeHTTP(rr, req)
			})
			Context("when the game id is not a valid UUID", func() {
				It("responds with 400 http code", func() {
					act.GameID = "123"
					body, _ := json.Marshal(act)
					req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
					s.BodyFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					respBody := rr.Body.String()
					Expect(respCode).To(Equal(http.StatusBadRequest))
					Expect(respBody).To(Equal("GameID 123 is not a valid UUID"))
				})
			})
			Context("when the game id is valid a UUID", func() {
				It("responds 200 http code", func() {
					act.GameID = "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4"
					body, _ := json.Marshal(&act)
					req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
					s.BodyFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					Expect(respCode).To(Equal(http.StatusOK))
				})
			})
			Context("when the body is empty", func() {
				It("returns a 400 response code", func() {
					req, _ := http.NewRequest("POST", "/", nil)
					s.BodyFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					respBody := rr.Body.String()
					Expect(respCode).To(Equal(http.StatusBadRequest))
					Expect(respBody).To(Equal("request body is nil"))
				})
			})
			Context("when a not-valid JSON is provided in the body", func() {
				It("returns a 400 response code", func() {
					body := []byte("a")
					checker := http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {})
					req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
					s.BodyFilter(checker).ServeHTTP(rr, req)
					respCode := rr.Code
					respBody := rr.Body.String()
					Expect(respCode).To(Equal(http.StatusBadRequest))
					Expect(respBody).To(Equal("error decoding the request body"))
				})
			})
		})

		Context("when going through method filter handler", func() {
			Context("when a get request is being sent", func() {
				It("returns a 405 response code", func() {
					req, _ := http.NewRequest("GET", "/", nil)
					s.MethodFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					respBody := rr.Body.String()
					Expect(respCode).To(Equal(http.StatusMethodNotAllowed))
					Expect(respBody).To(Equal("POST requests must be used to trigger a computation"))
				})
			})

			Context("when a non-application/json content type is provided", func() {
				It("returns a 415 response code", func() {
					body, _ := json.Marshal(&act)
					req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
					s.MethodFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					respBody := rr.Body.String()
					Expect(respCode).To(Equal(http.StatusUnsupportedMediaType))
					Expect(respBody).To(Equal("application/json content type must be provided"))
				})
			})
			Context("when POST with application/json content type is provided", func() {
				It("returns a 200", func() {
					act.GameID = "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4"
					body, _ := json.Marshal(&act)
					req, _ := http.NewRequest("POST", "/", bytes.NewReader(body))
					req.Header.Add("Content-Type", "application/json")
					s.MethodFilter(handler200).ServeHTTP(rr, req)
					respCode := rr.Code
					Expect(respCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when going through compilation handler", func() {
			Context("when compile parameter is not set", func() {
				It("forwards to the next handler without compilation", func() {
					req := requestWithContext("/", act)
					var compiled bool
					compile := func(*CtxConfig) error {
						compiled = true
						return nil
					}
					s.compile = compile
					s.CompilationHandler(handler200).ServeHTTP(rr, req)
					Expect(rr.Code).To(Equal(http.StatusOK))
					Expect(compiled).To(BeFalse())
				})
			})
			Context("when compile parameter is set", func() {
				Context("when compile is false", func() {
					It("doesn't compile the code", func() {
						var compiled bool
						req := requestWithContext("/?compile=false", act)
						compile := func(*CtxConfig) error {
							compiled = true
							return nil
						}
						s.compile = compile
						s.CompilationHandler(handler200).ServeHTTP(rr, req)
						Expect(rr.Code).To(Equal(http.StatusOK))
						Expect(compiled).To(BeFalse())
					})
				})
				Context("when compile param is ambiguous", func() {
					It("returns an error", func() {
						var compiled bool
						req := requestWithContext("/?compile=abc", act)
						compile := func(*CtxConfig) error {
							compiled = true
							return nil
						}
						s.compile = compile
						s.CompilationHandler(handler200).ServeHTTP(rr, req)
						respBody := rr.Body.String()
						Expect(rr.Code).To(Equal(http.StatusBadRequest))
						Expect(respBody).To(Equal("error when reading the compile parameter: strconv.ParseBool: parsing \"abc\": invalid syntax\n"))
						Expect(compiled).To(BeFalse())
					})
				})
				Context("when compile is true", func() {
					Context("when compilation succeeds", func() {
						It("returns 200", func() {
							var compiled bool
							compile := func(*CtxConfig) error {
								compiled = true
								return nil
							}
							req := requestWithContext("/?compile=true", act)
							s.compile = compile
							s.CompilationHandler(handler200).ServeHTTP(rr, req)
							Expect(rr.Code).To(Equal(http.StatusOK))
							Expect(compiled).To(BeTrue())
						})
					})
					Context("when compilation fails", func() {
						It("returns a 503 response code", func() {
							s.compile = func(*CtxConfig) error {
								return errors.New("some error")
							}
							req := requestWithContext("/?compile=true", act)
							s.CompilationHandler(handler200).ServeHTTP(rr, req)
							respCode := rr.Code
							Expect(respCode).To(Equal(http.StatusServiceUnavailable))
						})
					})
					Context("when additional parameters are provided", func() {
						It("still compiles the code", func() {
							var compiled bool
							req := requestWithContext("/?compile=true&other_param=abc", act)
							conf := &CtxConfig{
								Act: &Activation{
									GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
								},
								Spdz: &SPDZEngineTypedConfig{},
							}
							ctx := context.Background()
							ctx = context.WithValue(ctx, ctxConf, conf)
							req = req.WithContext(ctx)
							compile := func(*CtxConfig) error {
								compiled = true
								return nil
							}
							s.compile = compile
							s.CompilationHandler(handler200).ServeHTTP(rr, req)
							Expect(rr.Code).To(Equal(http.StatusOK))
							Expect(compiled).To(BeTrue())
						})
					})
				})
				Context("when no context config was specified", func() {
					It("returns a 400", func() {
						ctx := context.TODO()
						req := requestWithContext("", act)
						// override the context.
						req = req.WithContext(ctx)
						s.CompilationHandler(handler200).ServeHTTP(rr, req)
						respCode := rr.Code
						Expect(respCode).To(Equal(http.StatusBadRequest))
					})
				})
			})
		})
		Context("when going through activation handler", func() {
			var (
				req    *http.Request
				conf   *CtxConfig
				respCh chan []byte
				errCh  chan error
			)
			BeforeEach(func() {
				body, _ := json.Marshal(&act)
				req, _ = http.NewRequest("POST", "/?compile=true", bytes.NewReader(body))
				conf = &CtxConfig{
					Act: &Activation{
						GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
					},
					Context: context.Background(),
					Spdz:    &SPDZEngineTypedConfig{},
				}
				ctx := context.Background()
				ctx = context.WithValue(ctx, ctxConf, conf)
				req = req.WithContext(ctx)
				respCh = make(chan []byte, 1)
				errCh = make(chan error, parallelGames)
				player := &FakePlayerWithIO{
					respCh: respCh,
					errCh:  errCh,
				}
				s.player = player
				s.respCh = respCh
				s.errCh = errCh
				s.activate = func(*CtxConfig) ([]byte, error) {
					return []byte{}, nil
				}
				s.executor = &FakeExecutor{}
			})
			Context("when execution finishes with success", func() {
				It("responds with 200", func() {
					respCh <- []byte{}
					s.ActivationHandler(rr, req)
					code := rr.Code
					Expect(code).To(Equal(http.StatusOK))
				})
			})
			Context("when execution finishes with error", func() {
				Context("when ephemeral error happens", func() {
					It("responds with a 500", func() {
						s.errCh <- errors.New("some error")
						s.ActivationHandler(rr, req)
						code := rr.Code
						respBody := rr.Body.String()
						Expect(code).To(Equal(http.StatusInternalServerError))
						Expect(respBody).To(Equal("error while talking to Discovery: some error"))
					})
				})
				Context("when the timeout is reached during the execution", func() {
					It("responds with a 500", func() {
						conf.Spdz = &SPDZEngineTypedConfig{
							RetryTimeout: 1 * time.Millisecond,
						}
						s.ActivationHandler(rr, req)
						code := rr.Code
						respBody := rr.Body.String()
						Expect(code).To(Equal(http.StatusInternalServerError))
						Expect(respBody).To(Equal("timeout during MPC execution"))
					})
				})
			})
		})
	})
	Context("when getting the discovery client", func() {
		var (
			ioConf  *io.Config
			ctx     *CtxConfig
			timeout time.Duration
			logger  *zap.SugaredLogger
			wr      *Wires
		)
		BeforeEach(func() {
			ioConf = &io.Config{
				Host: "host",
				Port: "port",
			}
			ctx = &CtxConfig{
				Act: &Activation{
					GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
				},
				Context: context.TODO(),
			}
			timeout = time.Second
			logger = zap.NewNop().Sugar()
			wr = &Wires{}
		})
		It("succeeds when all required properties are set", func() {
			cl, err := NewTransportClientFromDiverseConfigs(ioConf, ctx, timeout, logger, wr)
			Expect(err).NotTo(HaveOccurred())
			Expect(cl).NotTo(BeNil())
		})
		It("returns an error when some client properties are missing", func() {
			ioConf.Host = ""
			cl, err := NewTransportClientFromDiverseConfigs(ioConf, ctx, timeout, logger, wr)
			Expect(err).To(HaveOccurred())
			Expect(cl).To(BeNil())
		})
	})

	Context("when starting the player", func() {
		It("starts all required subcomponents", func() {
			forwarder := &FakeForwarder{}
			cl := &FakeTransportClient{}
			wr := &Wires{}
			pl := &FakePlayer{}
			plIO := PlayerWithIO{
				Forwarder: forwarder,
				Client:    cl,
				Wires:     wr,
				Player:    pl,
			}
			plIO.Start()
			Expect(pl.Initialized).To(BeTrue())
		})
		Context("when the transport client is broken", func() {
			It("emits an error to the error channel without initialization of the player", func() {
				forwarder := &FakeForwarder{}
				cl := &BrokenFakeTransportClient{}
				wr := &Wires{
					Err: make(chan error, 1),
				}
				pl := &FakePlayer{}
				plIO := PlayerWithIO{
					Forwarder: forwarder,
					Client:    cl,
					Wires:     wr,
					Player:    pl,
				}
				plIO.Start()
				err := <-wr.Err
				Expect(err.Error()).To(Equal("some error"))
				Expect(pl.Initialized).To(BeFalse())
			})
		})
		Context("when creating a new instance of PlayerWithIO", func() {
			It("creates it without an error", func() {
				ctx := &CtxConfig{
					Spdz: &SPDZEngineTypedConfig{
						PlayerID: 0,
					},
					Act: &Activation{
						GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
					},
				}
				conf := &io.Config{
					Host: "host",
					Port: "port",
				}
				pod := "somePod"
				spdz := &SPDZWrapper{}
				errCh := make(chan error)
				logger := zap.NewNop().Sugar()
				pl, err := NewPlayerWithIO(ctx, conf, pod, spdz, errCh, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(pl).NotTo(BeNil())
			})
		})
	})
})

type FakePlayerWithIO struct {
	respCh chan []byte
	errCh  chan error
}

func (f *FakePlayerWithIO) Start() {
	return
}

func requestWithContext(path string, act *Activation) *http.Request {
	body, _ := json.Marshal(&act)
	req, _ := http.NewRequest("POST", path, bytes.NewReader(body))
	conf := &CtxConfig{
		Act: &Activation{
			GameID: "71b2a100-f3f6-11e9-81b4-2a2ae2dbcce4",
		},
		Spdz: &SPDZEngineTypedConfig{},
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxConf, conf)
	req = req.WithContext(ctx)
	return req
}

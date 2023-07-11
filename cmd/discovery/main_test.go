// Copyright (c) 2021-2023 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/carbynestack/ephemeral/pkg/discovery"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"github.com/carbynestack/ephemeral/pkg/utils"
)

var _ = Describe("Main", func() {
	It("returns a new client", func() {
		conf := &DiscoveryClientTypedConfig{
			Host:           "abc",
			Port:           "8080",
			ConnectTimeout: time.Second,
		}
		logger := zap.NewNop().Sugar()
		errCh := make(chan error)
		cl, mode, err := NewClient(conf, logger, errCh)
		Expect(err).NotTo(HaveOccurred())
		Expect(mode).To(Equal(ModeSlave))
		Expect(cl).NotTo(BeNil())
	})

	Context("when parsing the config", func() {

		var (
			random int64
			cmder  utils.Commander
			path   string
		)
		BeforeEach(func() {
			cmder = utils.Commander{
				Command: "bash",
				Options: []string{"-c"},
			}
			rand.Seed(time.Now().UnixNano())
			random = rand.Int63()
			path = fmt.Sprintf("/tmp/test-%d", random)
		})
		Context("all required parameters are specified", func() {
			AfterEach(func() {
				_, _, err := cmder.CallCMD(context.TODO(), []string{fmt.Sprintf("rm %s", path)}, "./")
				Expect(err).NotTo(HaveOccurred())
			})
			Context("parameters are plausible", func() {
				It("succeeds", func() {
					data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
					err := ioutil.WriteFile(path, data, 0644)
					Expect(err).NotTo(HaveOccurred())
					conf, err := ParseConfig(path)
					Expect(err).NotTo(HaveOccurred())
					Expect(conf.FrontendURL).To(Equal("apollo.test.specs.cloud"))
					Expect(conf.MasterHost).To(Equal("apollo.test.specs.cloud"))
					Expect(conf.MasterPort).To(Equal("31400"))
					Expect(conf.Slave).To(BeFalse())
				})
			})

			Context("parameters are invalid", func() {
				Context("playerCount is invalid", func() {
					It("returns an error on PlayerCount == 1", func() {
						data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": 1, "stateTimeout": "1s", "connectTimeout": "2s"}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						_, err = ParseConfig(path)
						Expect(err).To(HaveOccurred())
					})
					It("returns an error on negative PlayerCount", func() {
						data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": -2, "stateTimeout": "1s", "connectTimeout": "2s"}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						_, err = ParseConfig(path)
						Expect(err).To(HaveOccurred())
					})
				})
				Context("stateTimeout is invalid", func() {
					It("return an error on invalid format", func() {
						data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1", "connectTimeout": "2s", "computationTimeout": "3s"}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						conf, err := ParseConfig(path)
						Expect(conf).To(BeNil())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid state timeout format: time: missing unit in duration \"1\""))
					})
				})
				Context("connectTimeout is invalid", func() {
					It("return an error on invalid format", func() {
						data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2", "computationTimeout": "3s"}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						conf, err := ParseConfig(path)
						Expect(conf).To(BeNil())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid connection timeout format: time: missing unit in duration \"2\""))
					})
				})
				Context("computationTimeout is invalid", func() {
					It("return an error on invalid format", func() {
						data := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud",
		"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3"}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						conf, err := ParseConfig(path)
						Expect(conf).To(BeNil())
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid computation timeout format: time: missing unit in duration \"3\""))
					})
				})
			})

		})
		Context("one of the required parameters is missing", func() {
			AfterEach(func() {
				_, _, err := cmder.CallCMD(context.TODO(), []string{fmt.Sprintf("rm %s", path)}, "./")
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns an error", func() {
				path := fmt.Sprintf("/tmp/test-%d", random)
				noFrontendURLConfig := []byte(`{"masterHost": "apollo.test.specs.cloud",
			"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err := ioutil.WriteFile(path, noFrontendURLConfig, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())

				noMasterHostConfigSlave := []byte(`{"frontendURL": "apollo.test.specs.cloud",
					"masterPort": "31400","slave": true, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noMasterHostConfigSlave, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())

				noMasterHostConfigMaster := []byte(`{"frontendURL": "apollo.test.specs.cloud",
					"masterPort": "31400","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noMasterHostConfigMaster, 0644)
				Expect(err).NotTo(HaveOccurred())
				conf, err := ParseConfig(path)
				Expect(err).NotTo(HaveOccurred())
				Expect(conf).NotTo(BeNil())

				noMasterPortConfigSlave := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud","slave": false, "playerCount": 2, "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noMasterPortConfigSlave, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())

				noPlayerCountConfig := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud","slave": false, "masterPort": "31400", "stateTimeout": "1s", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noPlayerCountConfig, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())

				noStateTimeoutConfig := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud","slave": false, "masterPort": "31400", "playerCount": "2", "connectTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noStateTimeoutConfig, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())

				noConnectTimeoutConfig := []byte(`{"frontendURL": "apollo.test.specs.cloud","masterHost": "apollo.test.specs.cloud","slave": false, "masterPort": "31400", "playerCount": "2", "stateTimeout": "2s", "computationTimeout": "3s"}`)
				err = ioutil.WriteFile(path, noConnectTimeoutConfig, 0644)
				Expect(err).NotTo(HaveOccurred())
				_, err = ParseConfig(path)
				Expect(err).To(HaveOccurred())
			})
		})
		Context("when port|busSize|portRange|configLocation are not defined", func() {
			It("sets the default values", func() {
				conf := &DiscoveryTypedConfig{}
				SetDefaults(conf)
				Expect(conf.Port).To(Equal(DefaultPort))
				Expect(conf.BusSize).To(Equal(DefaultBusSize))
				Expect(conf.PortRange).To(Equal(DefaultPortRange))
			})
		})
		Context("when initializing the gRPC server", func() {
			It("sets its parameters", func() {
				logger := zap.NewNop().Sugar()
				port := "8080"
				tr := NewTransportServer(logger, port)
				Expect(tr.GetIn()).NotTo(BeNil())
				Expect(tr.GetOut()).NotTo(BeNil())
			})
		})
		Context("when starting the network deletion", func() {
			It("deletes the network with the given name", func() {
				doneCh := make(chan string, 1)
				errCh := make(chan error, 1)
				logger := zap.NewNop().Sugar()
				s := &discovery.ServiceNG{}
				doneCh <- "network"
				errCh <- errors.New("some error")
				runDeletion := func() {
					defer func() {
						if r := recover(); r == nil {
							Fail("the code must panic, but it didn't")
						}
					}()
					RunDeletion(doneCh, errCh, logger, s)
				}
				runDeletion()
			})
		})
	})
})

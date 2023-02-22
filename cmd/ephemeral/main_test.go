// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package main_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/carbynestack/ephemeral/cmd/ephemeral"
	. "github.com/carbynestack/ephemeral/pkg/types"
	"github.com/carbynestack/ephemeral/pkg/utils"

	"go.uber.org/zap"
)

var _ = Describe("Main", func() {

	Context("when manipulating ephemeral configuration", func() {

		Context("when working with real config file", func() {
			var (
				random int64
				cmder  utils.Commander
				path   string
			)
			Context("when the file is present on the file system", func() {
				BeforeEach(func() {
					cmder = utils.Commander{
						Command: "bash",
						Options: []string{"-c"},
					}
					rand.Seed(time.Now().UnixNano())
					random = rand.Int63()
					path = fmt.Sprintf("/tmp/test-%d", random)
				})
				AfterEach(func() {
					_, _, err := cmder.CallCMD(context.TODO(), []string{fmt.Sprintf("rm %s", path)}, "./")
					Expect(err).NotTo(HaveOccurred())
				})
				Context("when it succeeds", func() {
					It("initializes the config", func() {
						data := []byte(
							`{
								"retrySleep":"50ms",
								"retryTimeout":"1m",
								"prime":"p",
								"rInv":"r",
								"gfpMacKey":"gfpKey",
								"gf2nMacKey":"gf2nKey",
								"gf2nBitLength":40,
								"gf2nStorageSize":8,
								"prepFolder":"Player-Data",
								"amphoraConfig": {
									"host":"mock-server:1080",
									"scheme":"http","path":"/amphora1"
								},
								"castorConfig": {
									"host":"mock-server:1081",
									"scheme":"http",
									"path":"/castor1",
									"tupleStock":1000
								},
								"frontendURL":"apollo.test.specs.cloud",
								"playerID":0,
								"maxBulkSize":32000,
								"discoveryAddress":"discovery.default.svc.cluster.local"
							}`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						conf, err := ParseConfig(path)
						Expect(err).NotTo(HaveOccurred())
						Expect(conf.RetrySleep).To(Equal("50ms"))
					})
				})
				Context("when JSON format is corrupt", func() {
					It("returns an error", func() {
						data := []byte(`abc`)
						err := ioutil.WriteFile(path, data, 0644)
						Expect(err).NotTo(HaveOccurred())
						conf, err := ParseConfig(path)
						Expect(err).To(HaveOccurred())
						Expect(conf).To(BeNil())
					})
				})
			})
			Context("when reading a file fails", func() {
				It("returns an error", func() {
					conf, err := ParseConfig(fmt.Sprintf("/non-existing-location-%d", random))
					Expect(err).To(HaveOccurred())
					Expect(conf).To(BeNil())
				})
			})
		})
		Context("when initializing typed config", func() {
			It("succeeds when all parameters are specified", func() {
				conf := &SPDZEngineConfig{
					RetryTimeout:    "2s",
					RetrySleep:      "1s",
					Prime:           "198766463529478683931867765928436695041",
					RInv:            "133854242216446749056083838363708373830",
					GfpMacKey:       "1113507028231509545156335486838233835",
					Gf2nBitLength:   40,
					Gf2nStorageSize: 8,
					AmphoraConfig: AmphoraConfig{
						Host:   "localhost",
						Scheme: "http",
						Path:   "amphoraPath",
					},
					CastorConfig: CastorConfig{
						Host:   "localhost",
						Scheme: "http",
						Path:   "castorPath",
					},
				}
				typedConf, err := InitTypedConfig(conf)
				Expect(err).NotTo(HaveOccurred())
				Expect(typedConf.RetryTimeout).To(Equal(2 * time.Second))
				Expect(typedConf.RetrySleep).To(Equal(1 * time.Second))
			})
			Context("when non-valid parameters are specified", func() {
				Context("retry timeout format is corrupt", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout: "2",
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(typedConf).To(BeNil())
					})
				})
				Context("retry sleep format is corrupt", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout: "2s",
							RetrySleep:   "1",
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(typedConf).To(BeNil())
					})
				})
				Context("prime number is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout: "2s",
							RetrySleep:   "1s",
							Prime:        "",
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("wrong prime number format"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("inverse R is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout: "2s",
							RetrySleep:   "1s",
							Prime:        "123",
							RInv:         "",
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("wrong rInv format"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("gfpMacKey is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout: "2s",
							RetrySleep:   "1s",
							Prime:        "123",
							RInv:         "123",
							GfpMacKey:    "",
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("wrong gfpMacKey format"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("amphora URL is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout:    "2s",
							RetrySleep:      "1s",
							Prime:           "123",
							RInv:            "123",
							GfpMacKey:       "123",
							Gf2nBitLength:   40,
							Gf2nStorageSize: 8,
							AmphoraConfig: AmphoraConfig{
								Host: "",
							},
							CastorConfig: CastorConfig{
								Host:       "localhost",
								Scheme:     "http",
								Path:       "castorPath",
								TupleStock: 1000,
							},
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid Url"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("amphora scheme is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout:    "2s",
							RetrySleep:      "1s",
							Prime:           "123",
							RInv:            "123",
							GfpMacKey:       "123",
							Gf2nBitLength:   40,
							Gf2nStorageSize: 8,
							AmphoraConfig: AmphoraConfig{
								Host:   "localhost",
								Scheme: "",
							},
							CastorConfig: CastorConfig{
								Host:       "localhost",
								Scheme:     "http",
								Path:       "castorPath",
								TupleStock: 1000,
							},
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid Url"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("castor URL is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout:    "2s",
							RetrySleep:      "1s",
							Prime:           "123",
							RInv:            "123",
							GfpMacKey:       "123",
							Gf2nBitLength:   40,
							Gf2nStorageSize: 8,
							AmphoraConfig: AmphoraConfig{
								Host:   "localhost",
								Scheme: "http",
								Path:   "amphoraPath",
							},
							CastorConfig: CastorConfig{
								Host: "",
							},
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid Url"))
						Expect(typedConf).To(BeNil())
					})
				})
				Context("castor scheme is not specified", func() {
					It("returns an error", func() {
						conf := &SPDZEngineConfig{
							RetryTimeout:    "2s",
							RetrySleep:      "1s",
							Prime:           "123",
							RInv:            "123",
							GfpMacKey:       "123",
							Gf2nBitLength:   40,
							Gf2nStorageSize: 8,
							AmphoraConfig: AmphoraConfig{
								Host:   "localhost",
								Scheme: "http",
								Path:   "amphoraPath",
							},
							CastorConfig: CastorConfig{
								Host:   "localhost",
								Scheme: "",
							},
						}
						typedConf, err := InitTypedConfig(conf)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("invalid Url"))
						Expect(typedConf).To(BeNil())
					})
				})
			})
		})
	})
	Context("when retrieving the handler", func() {
		Context("when no error happens", func() {
			It("returns the handler chain and write mac keys", func() {
				tmpPrepDir, _ := ioutil.TempDir("", "ephemeral_prep_folder_")
				defer os.RemoveAll(tmpPrepDir)
				logger := zap.NewNop().Sugar()
				conf := &SPDZEngineConfig{
					RetryTimeout:    "2s",
					RetrySleep:      "1s",
					Prime:           "198766463529478683931867765928436695041",
					RInv:            "133854242216446749056083838363708373830",
					GfpMacKey:       "1113507028231509545156335486838233835",
					Gf2nMacKey:      "0xb660b323e6",
					Gf2nBitLength:   40,
					Gf2nStorageSize: 8,
					PlayerCount:     2,
					PrepFolder:      tmpPrepDir,
					AmphoraConfig: AmphoraConfig{
						Host:   "localhost",
						Scheme: "http",
						Path:   "amphoraPath",
					},
					CastorConfig: CastorConfig{
						Host:       "localhost",
						Scheme:     "http",
						Path:       "castorPath",
						TupleStock: 1000,
					},
				}
				handler, err := GetHandlerChain(conf, logger)
				Expect(err).NotTo(HaveOccurred())
				Expect(handler).NotTo(BeNil())
			})
		})
		Context("when an error in config convertion happens", func() {
			It("is returned", func() {
				logger := zap.NewNop().Sugar()
				conf := &SPDZEngineConfig{
					RetryTimeout:    "2s",
					RetrySleep:      "1s",
					Prime:           "198766463529478683931867765928436695041",
					RInv:            "133854242216446749056083838363708373830",
					GfpMacKey:       "1113507028231509545156335486838233835",
					Gf2nBitLength:   40,
					Gf2nStorageSize: 8,
					// an empty amphora config is given to provoke an error.
					AmphoraConfig: AmphoraConfig{},
					CastorConfig: CastorConfig{
						Host:       "localhost",
						Scheme:     "http",
						Path:       "castorPath",
						TupleStock: 1000,
					},
				}
				handler, err := GetHandlerChain(conf, logger)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid Url"))
				Expect(handler).To(BeNil())
			})
		})
	})
})

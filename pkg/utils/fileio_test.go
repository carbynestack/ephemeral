//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//

package utils

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

var _ = Describe("FileIO utils", func() {
	var fileIO FileIO
	var testFolderPath string
	BeforeSuite(func() {
		rand.Seed(time.Now().UnixNano())
		fileIO = &OSFileIO{}
	})
	BeforeEach(func() {
		testFolderPath = filepath.Join("/tmp", fmt.Sprintf("ephemeral_%d", rand.Int31()))
	})
	AfterEach(func() {
		_ = os.RemoveAll(testFolderPath)
	})
	Context("when using OSFileIO", func() {
		Context("when CreatePath", func() {
			It("create path and return nil", func() {
				err := fileIO.CreatePath(testFolderPath)
				Expect(err).NotTo(HaveOccurred())
				fi, err := os.Stat(testFolderPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(fi.IsDir()).To(BeTrue())
			})
		})
		Context("when CreatePipe", func() {
			It("create pipe", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				pipePath := filepath.Join(testFolderPath, "testPipe")

				err = fileIO.CreatePipe(pipePath)
				Expect(err).NotTo(HaveOccurred())
				stats, err := os.Stat(pipePath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stats.Mode() & os.ModeNamedPipe).To(Equal(os.ModeNamedPipe))
			})
		})
		Context("when Delete", func() {
			It("delete given file", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				testFile := filepath.Join(testFolderPath, "testFile")
				_, _ = os.Create(testFile)
				err = fileIO.Delete(testFolderPath)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.Stat(testFile)
				Expect(err).To(HaveOccurred())
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
			It("delete given folder containing child elements", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				testFile := filepath.Join(testFolderPath, "testFile")
				_, _ = os.Create(testFile)
				err = fileIO.Delete(testFolderPath)
				Expect(err).NotTo(HaveOccurred())
				_, err = os.Stat(testFolderPath)
				Expect(err).To(HaveOccurred())
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})
		Context("when OpenRead", func() {
			It("return file", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				testFile := filepath.Join(testFolderPath, "testFile")
				_, _ = os.Create(testFile)
				file, err := fileIO.OpenRead(testFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(file).NotTo(BeNil())
			})
		})
		Context("when OpenWriteOrCreate", func() {
			Context("when file does not exist", func() {
				It("create and return file", func() {
					testFolderPath, err := ioutil.TempDir("", "ephemeral_")
					if err != nil {
						Fail("failed to create temp dir for test")
					}
					testFile := filepath.Join(testFolderPath, "testFile")
					file, err := fileIO.OpenWriteOrCreate(testFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(file).NotTo(BeNil())
				})
			})
			Context("when file exists with content", func() {
				It("wipe content and return file", func() {
					testFolderPath, err := ioutil.TempDir("", "ephemeral_")
					if err != nil {
						Fail("failed to create temp dir for test")
					}
					testFile := filepath.Join(testFolderPath, "testFile")
					existingFile, _ := os.Create(testFile)
					_, err = existingFile.Write([]byte(fmt.Sprintf("some data")))
					if err != nil {
						Fail("failed to initialize test data")
					}
					_ = existingFile.Close()
					file, err := fileIO.OpenWriteOrCreate(testFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(file).NotTo(BeNil())
					dataInFile, _ := ioutil.ReadAll(file)
					Expect(len(dataInFile)).To(Equal(0))
				})
			})
		})
		Context("when OpenWritePipe", func() {
			It("return file", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				testFile := filepath.Join(testFolderPath, "testFile")
				_, _ = os.Create(testFile)
				file, err := fileIO.OpenWritePipe(testFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(file).NotTo(BeNil())
			})
		})
		Context("when ReadLine", func() {
			It("return first line in file", func() {
				testFolderPath, err := ioutil.TempDir("", "ephemeral_")
				if err != nil {
					Fail("failed to create temp dir for test")
				}
				firstLine := "first line data"
				secondLine := "irrelevant data"
				testFile := filepath.Join(testFolderPath, "testFile")
				existingFile, _ := os.Create(testFile)
				_, err = existingFile.Write([]byte(fmt.Sprintf("%s\n%s", firstLine, secondLine)))
				if err != nil {
					Fail("failed to initialize test data")
				}
				_, _ = existingFile.Seek(0, 0)
				line, err := fileIO.ReadLine(existingFile)
				Expect(err).NotTo(HaveOccurred())
				Expect(line).To(Equal(firstLine))
			})
		})
	})
})

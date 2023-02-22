// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package utils_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/carbynestack/ephemeral/pkg/utils"
	. "github.com/carbynestack/ephemeral/pkg/utils"
)

var _ = Describe("OS utils", func() {
	Context("when creating a new commander", func() {
		It("creates a commander with default params", func() {
			c := NewCommander()
			Expect(c.Command).To(Equal("script"))
			Expect(c.Options).To(Equal([]string{"-e", "-q", "-c"}))
		})
	})
	Context("when executing a command", func() {
		It("runs it and catches its output", func() {
			cmder := Commander{
				Command: "bash",
				Options: []string{"-c"},
			}
			resp, _, err := cmder.Run("echo 1")
			Expect(err).To(BeNil())
			Expect(string(resp)).To(Equal("1\n"))
		})
	})
	Context("when an error occurs executing a command", func() {
		Context("when the command returns an error to stderr", func() {
			It("returns the error", func() {
				cmder := Commander{
					Command: "bash",
					Options: []string{"-c"},
				}
				rand.Seed(time.Now().UnixNano())
				random := rand.Int31()
				resp, _, err := cmder.Run(fmt.Sprintf("cat /tmp/non-existing-file-%d", random))
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("exit status 1"))
				Expect(resp).NotTo(BeNil())
			})
		})
		Context("when starting the command fails", func() {
			It("returns an error", func() {
				cmder := Commander{
					Command: "non-existing-command",
					Options: []string{"-c"},
				}
				resp, _, err := cmder.Run("some command")
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("exec: \"non-existing-command\": executable file not found in $PATH"))
				Expect(len(resp)).To(Equal(0))
			})
		})
	})
	Context("when reading a file", func() {
		var (
			fileName string
			random   int32
			cmder    Commander
		)
		BeforeEach(func() {
			rand.Seed(time.Now().UnixNano())
			random = rand.Int31()
			fileName = fmt.Sprintf("/tmp/program-%d.mpc", random)
			cmder = utils.Commander{
				Command: "bash",
				Options: []string{"-c"},
			}
		})
		AfterEach(func() {
			cmder.CallCMD(context.TODO(), []string{fmt.Sprintf("rm %s", fileName)}, "./")
		})
		It("reads file content", func() {
			data := []byte(`a`)
			err := ioutil.WriteFile(fileName, data, 0644)
			content, err := ReadFile(fileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("a"))
		})
		Context("when file does not exists", func() {
			It("returns an error", func() {
				content, err := ReadFile(fileName)
				Expect(err).To(HaveOccurred())
				Expect(len(content)).To(Equal(0))
			})
		})
	})
})

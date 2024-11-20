// Copyright (c) 2024 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package opa_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOpa(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opa Suite")
}

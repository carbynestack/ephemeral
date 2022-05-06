//
// Copyright (c) 2022 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/castor.
//
// SPDX-License-Identifier: Apache-2.0
//
package castor_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestCastorGoClient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Castor Suite")
}

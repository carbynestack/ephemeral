// Copyright (c) 2021-2025 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
// The original file created by the Operator SDK has been modified to add Carbyne Stack Ephemeral network controller
// logic.
package controller

import (
	"github.com/carbynestack/ephemeral/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManagerFuncs is a list of functions to add all Controllers to the Manager
var AddToManagerFuncs []func(manager.Manager, *types.NetworkControllerConfig) error

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager, config *types.NetworkControllerConfig) error {
	for _, f := range AddToManagerFuncs {
		if err := f(m, config); err != nil {
			return err
		}
	}
	return nil
}

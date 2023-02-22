// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
package amphora

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/google/uuid"
)

// This is a test to run during the demo.
// TODO: remove the test once we have a system test that covers this functionality.
// Amphora and Castor must be deployed upfront.
func TestPostAndGet(t *testing.T) {
	demo := os.Getenv("DEMO")
	if demo == "true" {
		minikubeIP := os.Getenv("MINIKUBE_IP")
		if minikubeIP == "" {
			t.Error(errors.New("MINIKUBE_IP is not set"))
		}
		amphoraPort := os.Getenv("AMPHORA_MASTER_PORT")
		if amphoraPort == "" {
			t.Error(errors.New("AMPHORA_MASTER_PORT is not set"))
		}
		url := url.URL{Host: fmt.Sprintf("%s:%s", minikubeIP, amphoraPort), Scheme: "http"}
		client, err := NewClient(url)
		if err != nil {
			t.Error(err)
		}

		secretID := uuid.New().String()
		fmt.Printf("Operating on shared secret with UUID: %s\n", secretID)
		os := SecretShare{
			SecretID: secretID,
			// Secred share of 42.
			Data: "4tAU7fnIMu667ulrnjKLO4H6heQo0HPGSwJD8ZwVsh4=",
		}

		err = client.CreateSecretShare(&os)
		if err != nil {
			t.Error(err)
		}

		retrieved, err := client.GetSecretShare(secretID)
		if retrieved.Data != os.Data {
			t.Error("Retrieved object data is not equal to expected.")
		}
		if err != nil {
			t.Error(err)
		}

		fmt.Println(retrieved)
	} else {
		fmt.Println("Not a demo, skipping the test")
	}
}

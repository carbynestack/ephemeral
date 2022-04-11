//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package network

import (
	"crypto/tls"
	"fmt"
	"net"
)

// NewTLSConnector creates a TLS connector function in the default path "Player-Data".
// Simply delegates to NewTLSConnectorWithPath
func NewTLSConnector() func(conn net.Conn, playerID int32) (net.Conn, error) {
	return NewTLSConnectorWithPath("Player-Data")
}

// NewTLSConnectorWithPath creates a new TLS connector function.
// The function will accept the socket connection and the playerID and upgrade it to a TLS encrypted one.
// Will search for certificates in the provided folder path.
// Certificates must be named in the format that MP-SPDZ uses (<folder>/C<playerID>.pem and .key).
func NewTLSConnectorWithPath(folder string) func(conn net.Conn, playerID int32) (net.Conn, error) {
	return func(conn net.Conn, playerID int32) (net.Conn, error) {
		tlsConfig, err := getTLSConfig(playerID, folder)
		if err != nil {
			return nil, err
		}

		tlsClient := tls.Client(conn, tlsConfig)
		err = tlsClient.Handshake()
		if err != nil {
			return nil, err
		}

		return net.Conn(tlsClient), nil
	}
}

// getTLSConfig Loads the TLS config for the provided playerID located in the given folder.
// Certificates must be named in the format that MP-SPDZ uses (<folder>/C<playerID>.pem and .key)
func getTLSConfig(playerID int32, folder string) (*tls.Config, error) {
	certFile := fmt.Sprintf("%s/C%d.pem", folder, playerID)
	keyFile := fmt.Sprintf("%s/C%d.key", folder, playerID)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// For future improvement, see https://github.com/carbynestack/ephemeral/issues/22
		InsecureSkipVerify: true,
	}
	return tlsConfig, nil
}

/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package transport

import (
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/transport"
)

// OutboundTransport interface definition for transport layer
// This is the client side of the agent
type OutboundTransport interface {
	// Send send a2a exchange data
	Send(data []byte, destination string) (string, error)
	// Accept url
	Accept(string) bool
}

// InboundMessageHandler handles the inbound requests. The transport will unpack the payload prior to the
// message handle invocation.
type InboundMessageHandler func(message []byte) error

// InboundProvider contains dependencies for starting the inbound transport.
// It is typically created by using aries.Context().
type InboundProvider interface {
	InboundMessageHandler() InboundMessageHandler
	Packager() transport.Packager
}

// InboundTransport interface definition for inbound transport layer
type InboundTransport interface {
	// starts the inbound transport
	Start(prov InboundProvider) error

	// stops the inbound transport
	Stop() error

	// returns the endpoint
	Endpoint() string
}

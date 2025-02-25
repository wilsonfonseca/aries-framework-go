/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package context

import (
	"fmt"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	commontransport "github.com/hyperledger/aries-framework-go/pkg/didcomm/common/transport"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/dispatcher"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/packer"
	"github.com/hyperledger/aries-framework-go/pkg/didcomm/transport"
	"github.com/hyperledger/aries-framework-go/pkg/framework/aries/api"
	vdriapi "github.com/hyperledger/aries-framework-go/pkg/framework/aries/api/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/kms"
	"github.com/hyperledger/aries-framework-go/pkg/storage"
)

// Provider supplies the framework configuration to client objects.
type Provider struct {
	services                 []dispatcher.Service
	storeProvider            storage.Provider
	transientStoreProvider   storage.Provider
	kms                      kms.KMS
	packager                 commontransport.Packager
	primaryPacker            packer.Packer
	packers                  []packer.Packer
	inboundTransportEndpoint string
	outboundDispatcher       dispatcher.Outbound
	outboundTransports       []transport.OutboundTransport
	vdriRegistry             vdriapi.Registry
}

// New instantiates a new context provider.
func New(opts ...ProviderOption) (*Provider, error) {
	ctxProvider := Provider{}

	for _, opt := range opts {
		err := opt(&ctxProvider)
		if err != nil {
			return nil, fmt.Errorf("option failed: %w", err)
		}
	}

	return &ctxProvider, nil
}

// OutboundDispatcher returns an outbound dispatcher.
func (p *Provider) OutboundDispatcher() dispatcher.Outbound {
	return p.outboundDispatcher
}

// OutboundTransports returns an outbound transports.
func (p *Provider) OutboundTransports() []transport.OutboundTransport {
	return p.outboundTransports
}

// Service return protocol service
func (p *Provider) Service(id string) (interface{}, error) {
	for _, v := range p.services {
		if v.Name() == id {
			return v, nil
		}
	}

	return nil, api.ErrSvcNotFound
}

// KMS returns a kms service.
func (p *Provider) KMS() kms.KeyManager {
	return p.kms
}

// Packager returns a packager service.
func (p *Provider) Packager() commontransport.Packager {
	return p.packager
}

// Packers returns a list of enabled packers.
func (p *Provider) Packers() []packer.Packer {
	return p.packers
}

// PrimaryPacker returns the main inbound/outbound Packer service.
func (p *Provider) PrimaryPacker() packer.Packer {
	return p.primaryPacker
}

// Signer returns a kms signing service.
func (p *Provider) Signer() kms.Signer {
	return p.kms
}

// InboundTransportEndpoint returns an inbound transport endpoint.
func (p *Provider) InboundTransportEndpoint() string {
	return p.inboundTransportEndpoint
}

// InboundMessageHandler return an inbound message handler.
func (p *Provider) InboundMessageHandler() transport.InboundMessageHandler {
	return func(message []byte) error {
		msg, err := service.NewDIDCommMsg(message)
		if err != nil {
			return err
		}

		// find the service which accepts the message type
		for _, svc := range p.services {
			if svc.Accept(msg.Header.Type) {
				_, err = svc.HandleInbound(msg)
				return err
			}
		}
		return fmt.Errorf("no message handlers found for the message type: %s", msg.Header.Type)
	}
}

// StorageProvider return a storage provider.
func (p *Provider) StorageProvider() storage.Provider {
	return p.storeProvider
}

// TransientStorageProvider return a transient storage provider.
func (p *Provider) TransientStorageProvider() storage.Provider {
	return p.transientStoreProvider
}

// VDRIRegistry returns a vdri registry
func (p *Provider) VDRIRegistry() vdriapi.Registry {
	return p.vdriRegistry
}

// ProviderOption configures the framework.
type ProviderOption func(opts *Provider) error

// WithOutboundTransports injects an outbound transports into the context.
func WithOutboundTransports(transports ...transport.OutboundTransport) ProviderOption {
	return func(opts *Provider) error {
		opts.outboundTransports = transports
		return nil
	}
}

// WithOutboundDispatcher injects an outbound dispatcher into the context.
func WithOutboundDispatcher(outboundDispatcher dispatcher.Outbound) ProviderOption {
	return func(opts *Provider) error {
		opts.outboundDispatcher = outboundDispatcher
		return nil
	}
}

// WithProtocolServices injects a protocol services into the context.
func WithProtocolServices(services ...dispatcher.Service) ProviderOption {
	return func(opts *Provider) error {
		opts.services = services
		return nil
	}
}

// WithKMS injects a kms service into the context.
func WithKMS(w kms.KMS) ProviderOption {
	return func(opts *Provider) error {
		opts.kms = w
		return nil
	}
}

// WithVDRIRegistry injects a vdri service into the context.
func WithVDRIRegistry(vdri vdriapi.Registry) ProviderOption {
	return func(opts *Provider) error {
		opts.vdriRegistry = vdri
		return nil
	}
}

// WithInboundTransportEndpoint injects an inbound transport endpoint into the context.
func WithInboundTransportEndpoint(endpoint string) ProviderOption {
	return func(opts *Provider) error {
		opts.inboundTransportEndpoint = endpoint
		return nil
	}
}

// WithStorageProvider injects a storage provider into the context.
func WithStorageProvider(s storage.Provider) ProviderOption {
	return func(opts *Provider) error {
		opts.storeProvider = s
		return nil
	}
}

// WithTransientStorageProvider injects a transient storage provider into the context.
func WithTransientStorageProvider(s storage.Provider) ProviderOption {
	return func(opts *Provider) error {
		opts.transientStoreProvider = s
		return nil
	}
}

// WithPackager injects a packager into the context.
func WithPackager(p commontransport.Packager) ProviderOption {
	return func(opts *Provider) error {
		opts.packager = p
		return nil
	}
}

// WithPacker injects at least one Packer into the context,
// with the primary Packer being used for inbound/outbound communication
// and the additional packers being available for unpacking inbound messages.
func WithPacker(primary packer.Packer, additionalPackers ...packer.Packer) ProviderOption {
	return func(opts *Provider) error {
		opts.primaryPacker = primary
		opts.packers = append(opts.packers, additionalPackers...)
		return nil
	}
}

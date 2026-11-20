package harness

import (
	"context"
	"sync"

	"github.com/openrtc/conformance/openrtc"
)

// RTCFailure is the public portable failure type re-exported by the harness.
type RTCFailure = openrtc.RTCFailure

// CredentialSource supplies credentials for instrumentation. Invalid is optional.
type CredentialSource struct {
	Valid   func(context.Context, openrtc.CredentialRequest) (openrtc.CredentialInput, error)
	Invalid func(context.Context, openrtc.CredentialRequest) (openrtc.CredentialInput, error)
}

const (
	ExplicitCreate      ProvisioningModeName = "explicit-create"
	CreateOnJoin        ProvisioningModeName = "create-on-join"
	ExternalProvisioned ProvisioningModeName = "external-provisioned"
	BackendBrokered     ProvisioningModeName = "backend-brokered"
)

type ClientConfigInput struct {
	Identity    openrtc.ApplicationIdentity
	Credentials openrtc.CredentialProvider
}

// ConformanceTarget is the adapter-author contract. All factories must return fresh
// objects; the harness never shares state between tests. [design-proposal]
type ConformanceTarget interface {
	Name() string
	Author() string
	Evidence() EvidenceType
	Environment() EnvironmentDescription
	Timing() TimingPolicy
	CreateRegistry() openrtc.RTCAdapterRegistry
	CreateAdapter() openrtc.RTCAdapter
	ProviderConfig() openrtc.ProviderConfig
	ClientConfig(ClientConfigInput) openrtc.ClientConfig
	Credentials() CredentialSource
	// A nil target means that the fixture cannot produce the provisioning mode.
	ConferenceTarget(context.Context, ProvisioningModeName) (*openrtc.ConferenceTarget, error)
}

// Optional facilities are separate interfaces so their absence yields not-run.
type AdministrationTarget interface {
	Administration(context.Context) (openrtc.RTCServerAdministration, error)
}
type JoinOptionsTarget interface {
	JoinOptions() openrtc.JoinSessionOptions
}
type CallingTarget interface{ Calling() CallingFixture }
type CallingFixture struct {
	Account     openrtc.ProviderIdentity
	Destination openrtc.SessionAddress
}

// Deferred is an externally resolvable promise. The first settlement determines the result.
type Deferred[T any] struct {
	mu       sync.Mutex
	done     chan struct{}
	settled  bool
	value    T
	reason   any
	rejected bool
}

func NewDeferred[T any]() *Deferred[T]   { return &Deferred[T]{done: make(chan struct{})} }
func (d *Deferred[T]) Resolve(value T)   { d.settle(value, nil, false) }
func (d *Deferred[T]) Reject(reason any) { var zero T; d.settle(zero, reason, true) }
func (d *Deferred[T]) settle(value T, reason any, rejected bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settled {
		return
	}
	d.value, d.reason, d.rejected, d.settled = value, reason, rejected, true
	close(d.done)
}
func (d *Deferred[T]) Settled() bool { d.mu.Lock(); defer d.mu.Unlock(); return d.settled }

// Await returns the rejection reason separately to preserve arbitrary rejection values.
func (d *Deferred[T]) Await(ctx context.Context) (T, any, bool, error) {
	select {
	case <-d.done:
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.value, d.reason, d.rejected, nil
	case <-ctx.Done():
		var zero T
		return zero, nil, false, ctx.Err()
	}
}

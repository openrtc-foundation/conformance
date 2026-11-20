package selftest

import (
	"context"
	"fmt"
	"github.com/openrtc/conformance/openrtc"
	"sync"
)

func mergeRaw(value any, extra map[string]any) map[string]any {
	m := objectMap(value)
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func (c *fakeClient) ConfigureRaw(ctx context.Context, command openrtc.ClientRuntimeCommand, extra map[string]any) (openrtc.ConfigurationResult, error) {
	if !c.defects.IgnoreUnknownOptions {
		allowed := map[string]bool{"target": true, "diagnostics": true, "extensions": true, "id": true, "metadata": true, "request": true, "key": true, "payload": true}
		for k := range extra {
			if !allowed[k] {
				return openrtc.ConfigurationResult{}, validationError("unsupported-option", "configure", fmt.Sprintf("field '%s' does not belong to a runtime command", k), []string{k}, "runtime")
			}
		}
	}
	op := c.Configure(ctx, command)
	if _, err := op.Result(ctx); err != nil {
		return openrtc.ConfigurationResult{}, err
	}
	r, err := c.Inspection().Operation(op.OperationID())
	if err != nil {
		return openrtc.ConfigurationResult{}, err
	}
	return r.Details.Configuration.Value.Outcome, nil
}

func (p *fakeProvider) CreateClientRaw(ctx context.Context, config openrtc.ClientConfig, extra map[string]any) (openrtc.RTCClient, error) {
	if _, err := validateHolder("client-init", mergeRaw(config, extra), p.defects, "create-client"); err != nil {
		return nil, err
	}
	return p.CreateClient(ctx, config).Result(ctx)
}

func (r *fakeRegistry) InitializeRaw(ctx context.Context, id openrtc.AdapterID, config openrtc.ProviderConfig, extra map[string]any) (openrtc.RTCProvider, error) {
	if _, err := validateHolder("provider-init", mergeRaw(config, extra), r.defects, "initialize"); err != nil {
		return nil, err
	}
	return r.Initialize(ctx, id, config).Result(ctx)
}

var _ openrtc.RTCClient = (*fakeClient)(nil)
var _ openrtc.RTCProvider = (*fakeProvider)(nil)
var _ openrtc.RTCAdapterRegistry = (*fakeRegistry)(nil)
var _ openrtc.RTCAdapter = (*bindingAdapter)(nil)

func newFakeClient(config openrtc.ClientConfig, w *world, defects FakeDefects) *fakeClient {
	return &fakeClient{id: openrtc.ClientID(newID("client")), identity: config.Identity, credentialProvider: config.Credentials, world: w, defects: defects, capabilities: newFakeCapabilities(defects), sessionMap: make(map[openrtc.LocalSessionID]openrtc.RTCSession)}
}
func (c *fakeClient) ID() openrtc.ClientID                  { return c.id }
func (c *fakeClient) Capabilities() openrtc.RTCCapabilities { return c.capabilities }
func (c *fakeClient) Sessions() map[openrtc.LocalSessionID]openrtc.RTCSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[openrtc.LocalSessionID]openrtc.RTCSession, len(c.sessionMap))
	for id, session := range c.sessionMap {
		result[id] = session
	}
	return result
}
func (c *fakeClient) OpenSession(target openrtc.ConferenceTarget) openrtc.RTCConferenceSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	if target.Provisioning.Mode == "" {
		panic(fail("invalid-option", "open-session", "missing provisioning mode", failureExtras{}))
	}
	if c.disposed && !c.defects.OpenAfterDispose {
		panic(fail("invalid-state", "open-session", "client disposed", failureExtras{}))
	}
	if target.Provisioning.Mode == "" {
		panic(fail("invalid-option", "open-session", "missing provisioning mode", failureExtras{}))
	}
	if f := c.world.takeFault("open-session", "admission"); f != nil {
		if c.defects.OpenFailureCreatesSession {
			s := newFakeSession(c, target, c.world, c.defects)
			c.sessionMap[s.id] = s
			c.sessionOrder = append(c.sessionOrder, s)
		}
		panic(faultError(f))
	}
	if f := c.world.takeFault("open-session", "execution"); f != nil {
		c.world.acquireUnreturned(c, c.defects)
		panic(faultError(f))
	}
	s := newFakeSession(c, target, c.world, c.defects)
	c.sessionMap[s.id] = s
	c.sessionOrder = append(c.sessionOrder, s)
	return s
}
func (c *fakeClient) Configure(_ context.Context, command openrtc.ClientRuntimeCommand) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(c, func() (openrtc.ConfigurationResult, error) {
		if !c.defects.IgnoreUnknownOptions && command.Extensions != nil {
			if err := validateExtensions(command.Extensions, "runtime", "configure"); err != nil {
				return openrtc.ConfigurationResult{}, err
			}
		}
		result := emptyConfigurationResult(nil)
		if command.Target == "client" && command.Diagnostics != nil {
			var value openrtc.JSONValue = command.Diagnostics.Level
			result.Fields = []openrtc.ConfigurationFieldResult{{Path: openrtc.OptionPath{"diagnostics", "level"}, Requested: openrtc.KnownValue(value), Accepted: openrtc.KnownValue(value), Effective: openrtc.Unknown[openrtc.JSONValue](), Selection: "selected", Reason: "accepted; effect on a fake is not claimed", Activation: "immediate"}}
		}
		return result, nil

	}))
}
func completeCleanup() openrtc.CleanupReport {
	return openrtc.CleanupReport{Status: openrtc.CleanupStatus{Kind: "complete"}, Generation: openrtc.Unknown[openrtc.GenerationID]()}
}
func (c *fakeClient) Dispose(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(c, func() (openrtc.CleanupReport, error) {
		if f := c.world.takeFault("client-dispose", "fence"); f != nil {
			if c.defects.FenceFailureCloses {
				c.mu.Lock()
				c.disposed = true
				sessions := append([]*fakeSession(nil), c.sessionOrder...)
				c.mu.Unlock()
				for _, s := range sessions {
					s.setState("ended")
				}
			}
			return openrtc.CleanupReport{}, faultError(f)
		}
		c.disposeOnce.Do(func() {
			c.mu.Lock()
			c.disposed = true
			c.mu.Unlock()
			closeOwnerListeners(c, c.defects)
			c.mu.Lock()
			sessions := append([]*fakeSession(nil), c.sessionOrder...)
			c.mu.Unlock()
			for _, session := range sessions {
				if _, err := session.Dispose(ctx).Result(ctx); err != nil {
					c.disposeErr = err
					return
				}
			}
			c.disposeReport = completeCleanup()
		})
		return c.disposeReport, c.disposeErr

	}))
}

type fakeProvider struct {
	descriptor   openrtc.ProviderDescriptor
	world        *world
	defects      FakeDefects
	capabilities *fakeCapabilities
	mu           sync.Mutex
	clients      []*fakeClient
	disposed     bool
}

func (p *fakeProvider) Descriptor() openrtc.ProviderDescriptor { return p.descriptor }
func (p *fakeProvider) Capabilities() openrtc.RTCCapabilities  { return p.capabilities }
func (*fakeProvider) Schemas(lifetime openrtc.ConfigurationLifetime) []openrtc.ConfigurationSchema {
	return (bindingAdapter{}).Schemas(lifetime)
}
func (p *fakeProvider) CreateClient(ctx context.Context, config openrtc.ClientConfig) openrtc.Async[openrtc.RTCClient] {
	return startOperation(p, func() (openrtc.RTCClient, error) {
		p.world.mu.Lock()
		gate := p.world.createGate
		p.world.createGate = nil
		p.world.mu.Unlock()
		if gate != nil {
			close(gate.reachedCh)
			native := make(chan struct{})
			go func() { _, _, _, _ = gate.outcome.Await(context.Background()); close(native) }()
			if p.defects.CreateCancellationWaitsNative {
				<-native
			} else {
				select {
				case <-native:
				case <-ctx.Done():
					code := openrtc.ErrorCode("cancelled")
					if ctx.Err() == context.DeadlineExceeded {
						code = "timeout"
					}
					return nil, fail(code, "create-client", "invocation ended before native completion", failureExtras{})
				}
			}
		}

		if _, err := validateHolder("client-init", config, p.defects, "create-client"); err != nil {
			return nil, err
		}
		p.mu.Lock()
		if p.disposed && !p.defects.CreateAfterDispose {
			p.mu.Unlock()
			return nil, fail("invalid-state", "create-client", "provider disposed", failureExtras{})
		}
		p.mu.Unlock()
		if f := p.world.takeFault("create-client", "admission"); f != nil {
			if p.defects.AdmissionChangesState {
				p.world.acquireUnreturned(p, p.defects)
			}
			return nil, faultError(f)
		}
		if f := p.world.takeFault("create-client", "execution"); f != nil {
			p.world.acquireUnreturned(p, p.defects)
			return nil, faultError(f)
		}
		client := newFakeClient(config, p.world, p.defects)
		client.provider = p
		p.world.mu.Lock()
		p.world.counts.ClientsCreated++
		p.world.mu.Unlock()
		p.mu.Lock()
		if p.disposed && !p.defects.CreateAfterDispose {
			p.mu.Unlock()
			_, _ = client.Dispose(context.Background()).Result(context.Background())
			return nil, fail("cancelled", "create-client", "provider fence won", failureExtras{})
		}
		p.clients = append(p.clients, client)
		p.mu.Unlock()
		return client, nil

	})
}
func (p *fakeProvider) Dispose(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(p, func() (openrtc.CleanupReport, error) {
		p.mu.Lock()
		p.disposed = true
		clients := append([]*fakeClient(nil), p.clients...)
		p.mu.Unlock()
		for _, client := range clients {
			if _, err := client.Dispose(ctx).Result(ctx); err != nil {
				return openrtc.CleanupReport{}, err
			}
		}
		closeOwnerListeners(p, p.defects)
		return completeCleanup(), nil

	}))
}

type fakeRegistry struct {
	mu       sync.Mutex
	adapters map[openrtc.AdapterID]openrtc.RTCAdapter
	world    *world
	defects  FakeDefects
}

func makeRegistry(w *world, defects FakeDefects) *fakeRegistry {
	return &fakeRegistry{world: w, defects: defects, adapters: make(map[openrtc.AdapterID]openrtc.RTCAdapter)}
}
func (r *fakeRegistry) Register(adapter openrtc.RTCAdapter) (openrtc.Registration, error) {
	if adapter == nil {
		return openrtc.Registration{}, fail("incompatible-adapter", "initialize", "nil adapter", failureExtras{})
	}
	d := adapter.Descriptor()
	if f := r.world.takeFault("register", "admission"); f != nil {
		return openrtc.Registration{}, faultError(f)
	}
	if err := validateAdapterDescriptor(d); err != nil {
		return openrtc.Registration{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[d.AdapterID]; exists {
		if r.defects.RegistrationPoisoned {
			delete(r.adapters, d.AdapterID)
		}
		return openrtc.Registration{}, fail("incompatible-adapter", "initialize", "adapter ID is already registered", failureExtras{})
	}
	r.adapters[d.AdapterID] = adapter
	return openrtc.Registration{AdapterID: d.AdapterID, SelectedVersion: hostSpec}, nil
}
func (r *fakeRegistry) Initialize(_ context.Context, id openrtc.AdapterID, config openrtc.ProviderConfig) openrtc.InitializationOperation {
	return newInitialization(r, startOperation(r, func() (openrtc.RTCProvider, error) {
		if _, err := validateHolder("provider-init", config, r.defects, "initialize"); err != nil {
			return nil, err
		}
		r.mu.Lock()
		adapter, exists := r.adapters[id]
		r.mu.Unlock()
		if !exists {
			code := openrtc.ErrorCode("invalid-option")
			if r.defects.UnknownInitCode {
				code = "incompatible-adapter"
			}
			return nil, fail(code, "initialize", "no such adapter", failureExtras{})
		}
		if _, err := validateHolder("provider-init", config, r.defects, "initialize"); err != nil {
			return nil, err
		}
		if f := r.world.takeFault("initialize", "admission"); f != nil {
			if r.defects.AdmissionChangesState {
				r.world.acquireUnreturned(r, r.defects)
			}
			return nil, faultError(f)
		}
		if f := r.world.takeFault("initialize", "execution"); f != nil {
			r.world.acquireUnreturned(r, r.defects)
			return nil, faultError(f)
		}
		return &fakeProvider{descriptor: adapter.Descriptor(), world: r.world, defects: r.defects, capabilities: newFakeCapabilities(r.defects)}, nil

	}))
}

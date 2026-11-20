package selftest

import (
	"context"
	"fmt"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type effectObserver struct{ world *world }

var _ harness.NativeEffectObserver = effectObserver{}
var _ harness.JoinGate = joinGateControl{}
var _ harness.TransportInterruption = interruption{}
var _ harness.AdmissionControl = admissionControl{}
var _ harness.RemoteEndpoint = (*remoteControl)(nil)

func (o effectObserver) Counts() harness.NativeEffectCounts {
	o.world.mu.Lock()
	defer o.world.mu.Unlock()
	return o.world.counts
}

type joinGateControl struct{ gate *worldGate }

func (g joinGateControl) Reached() <-chan struct{} { return g.gate.reachedCh }
func (g joinGateControl) ReleaseSuccess()          { g.gate.outcome.Resolve("success") }
func (g joinGateControl) ReleaseFailure(v any) {
	if _, ok := v.(error); !ok {
		v = fmt.Errorf("%v", v)
	}
	g.gate.outcome.Resolve(v)
}

type interruption struct{ restore func(context.Context) error }

func (i interruption) Restore(ctx context.Context) error { return i.restore(ctx) }

type admissionControl struct{ world *world }

func (a admissionControl) Hold() {
	a.world.mu.Lock()
	a.world.admissionMode = "hold"
	a.world.mu.Unlock()
}
func (a admissionControl) Deny() {
	a.world.mu.Lock()
	a.world.admissionMode = "deny"
	a.world.mu.Unlock()
}
func (a admissionControl) Admit() {
	a.world.mu.Lock()
	a.world.admissionMode = "admit"
	held := make([]*fakeSession, 0, len(a.world.held))
	for session := range a.world.held {
		held = append(held, session)
		delete(a.world.held, session)
	}
	a.world.mu.Unlock()
	for _, session := range held {
		session.admitHeld()
	}
}

type remoteControl struct {
	world    *world
	remote   *remote
	mu       sync.Mutex
	received []string
}

func (r *remoteControl) Identity() openrtc.ApplicationIdentity { return r.remote.identity }
func (r *remoteControl) IsConnected() bool {
	r.world.mu.Lock()
	defer r.world.mu.Unlock()
	return r.remote.connected
}
func (*remoteControl) Publish(context.Context, []string) error { return nil }
func (*remoteControl) SetMuted(context.Context, bool) error    { return nil }
func (r *remoteControl) SendText(_ context.Context, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, text)
	return nil
}
func (r *remoteControl) ReceivedText() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.received...)
}
func (r *remoteControl) Leave(context.Context) error {
	r.world.mu.Lock()
	defer r.world.mu.Unlock()
	r.remote.connected = false
	return nil
}

func (t *FakeTarget) Controls() harness.FixtureControls {
	w := t.world
	return harness.FixtureControls{
		NativeEffects: effectObserver{w},
		GateNextJoin: func() harness.JoinGate {
			g := &worldGate{reached: harness.NewDeferred[struct{}](), outcome: harness.NewDeferred[any](), reachedCh: make(chan struct{})}
			w.mu.Lock()
			w.gate = g
			w.mu.Unlock()
			return joinGateControl{g}
		},
		InterruptTransport: func(_ context.Context, session openrtc.RTCSession) (harness.TransportInterruption, error) {
			s, ok := any(session).(interface {
				interrupt() func(context.Context) error
			})
			if !ok {
				return nil, fmt.Errorf("session is not a fake session")
			}
			return interruption{s.interrupt()}, nil
		},
		TriggerCredentialRenewal: func(ctx context.Context, session openrtc.RTCSession) error {
			s, ok := any(session).(interface{ renewCredentials(context.Context) })
			if !ok {
				return fmt.Errorf("session is not a fake session")
			}
			s.renewCredentials(ctx)
			return nil
		},
		FailNextNativeOperation: func(operation string, failure any) {
			w.mu.Lock()
			w.nextFailure = &worldFailure{operation: operation, failure: failure}
			w.mu.Unlock()
		},
		RemoteEndpoint: func(_ context.Context, target openrtc.ConferenceTarget, intent harness.RemoteEndpointIntent) (harness.RemoteEndpoint, error) {
			value := "room-selftest"
			p := target.Provisioning
			if p.Mode == "explicit-create" && p.Definition != nil {
				value = p.Definition.Address.Value
			} else if p.Mode != "backend-brokered" && p.Address != nil {
				value = p.Address.Value
			}
			r := &remote{identity: intent.Identity, connected: true}
			w.mu.Lock()
			w.remotes[value] = append(w.remotesAt(value), r)
			w.mu.Unlock()
			return &remoteControl{world: w, remote: r, received: []string{}}, nil
		},
		Admission: func() harness.AdmissionControl { return admissionControl{w} },
	}
}

package selftest

import (
	"context"
	"sync"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
)

// This direct public synthetic profile binding validates shared stream oracles.
// Its in-memory evidence cannot establish any provider implementation result.
type profileObservationTarget struct {
	*FakeTarget
	defect    string
	transient bool
	omit      string
}

func (t profileObservationTarget) ClaimDeclaration() harness.ClaimDeclaration {
	return harness.ClaimDeclaration{Core: true, Profiles: []harness.ProfileClaim{{Name: "calling", Version: 1}, {Name: "data-messaging", Version: 1}, {Name: "recording", Version: 1}, {Name: "placement", Version: 1}, {Name: "devices-and-statistics", Version: 1}}}
}
func (t profileObservationTarget) ProfileObservation(_ context.Context, profile string) (harness.ProfileObservationFixture, error) {
	if profile == t.omit {
		return harness.ProfileObservationFixture{}, nil
	}
	stateful := profile != "data-messaging" && !t.transient
	b := &profileObservationBinding{scope: profile + "/controlled-owner", defect: t.defect, stateful: stateful, state: map[string]string{}}
	return harness.ProfileObservationFixture{Profile: profile, Scope: b.scope, PublicRepresentation: "syntheticProfileBinding.Observe/Snapshot", Documentation: "selftest/profile_observation_test.go; direct synthetic public representation", Stateful: stateful, Capability: func(context.Context) (openrtc.CapabilityDescriptor, error) {
		return openrtc.CapabilityDescriptor{ID: "observeProfile", Support: openrtc.SupportDescription{Level: "supported"}, Scope: openrtc.CapabilityDescriptorScope{Resource: b.scope, Profile: profile}}, nil
	}, Observe: b.Observe, Snapshot: b.Snapshot, Commit: b.Commit, LoseNext: b.LoseNext}, nil
}

type profileWatch struct {
	callback        func(harness.ProfileObservationUpdate)
	active, current bool
}
type profileObservationBinding struct {
	mu                          sync.Mutex
	scope, defect               string
	stateful, lose, dispatching bool
	state                       map[string]string
	watchers                    []*profileWatch
	queue                       []func()
}

func copyProfileFacts(v map[string]string) map[string]string {
	out := map[string]string{}
	for k, s := range v {
		out[k] = s
	}
	return out
}
func (b *profileObservationBinding) stateLocked() harness.ProfileObservationState {
	facts := copyProfileFacts(b.state)
	if b.defect == "snapshot-alias" {
		facts = b.state
	}
	completeness := "complete"
	if b.defect == "unknown-coverage" {
		completeness = "unknown"
	}
	return harness.ProfileObservationState{Scope: b.scope, Completeness: completeness, Facts: facts}
}
func (b *profileObservationBinding) Snapshot(context.Context) (harness.ProfileObservationState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stateLocked(), nil
}
func (b *profileObservationBinding) Observe(_ context.Context, callback func(harness.ProfileObservationUpdate)) (openrtc.Unsubscribe, error) {
	b.mu.Lock()
	w := &profileWatch{callback: callback, active: true, current: true}
	b.watchers = append(b.watchers, w)
	state := b.stateLocked()
	if b.defect == "stale-resnapshot" && state.Facts["marker"] != "baseline" {
		state.Facts = map[string]string{"marker": "baseline", "related": "baseline/related"}
	}
	b.queue = append(b.queue, func() { callback(harness.ProfileObservationUpdate{Snapshot: &state, Current: true}) })
	if b.defect == "duplicate-handoff" && state.Facts["marker"] == "fresh-next" {
		b.queue = append(b.queue, func() {
			callback(harness.ProfileObservationUpdate{Marker: "fresh-next", Related: map[string]string{"related": "fresh-next/related"}, Current: true})
		})
	}
	b.mu.Unlock()
	b.drain()
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		w.active = false
		if b.defect == "remove-all" {
			for _, watch := range b.watchers {
				watch.active = false
			}
		}
	}, nil
}
func (b *profileObservationBinding) LoseNext(context.Context) error {
	b.mu.Lock()
	b.lose = true
	b.mu.Unlock()
	return nil
}
func (b *profileObservationBinding) Commit(_ context.Context, marker string) error {
	b.mu.Lock()
	if b.stateful {
		if b.defect == "snapshot-alias" {
			b.state["marker"] = marker
			b.state["related"] = marker + "/related"
		} else {
			b.state = map[string]string{"marker": marker, "related": marker + "/related"}
		}
	}
	watches := append([]*profileWatch(nil), b.watchers...)
	if b.lose {
		b.lose = false
		for _, w := range watches {
			w.current = false
		}
		if b.defect != "hide-loss" {
			b.queue = append(b.queue, func() {
				for _, w := range watches {
					b.mu.Lock()
					active := w.active
					b.mu.Unlock()
					if active {
						w.callback(harness.ProfileObservationUpdate{Loss: true, Current: false})
					}
				}
			})
		}
		b.mu.Unlock()
		b.drain()
		return nil
	}
	related := map[string]string{"related": marker + "/related"}
	if b.defect == "split-related" {
		related = map[string]string{}
	}
	deliver := func() {
		for _, w := range watches {
			b.mu.Lock()
			active, current := w.active, w.current
			b.mu.Unlock()
			if active {
				w.callback(harness.ProfileObservationUpdate{Marker: marker, Related: copyProfileFacts(related), Current: current})
			}
		}
	}
	if b.defect == "drop-intermediate" && marker == "first" {
		b.mu.Unlock()
		return nil
	}
	if b.defect == "reentrant-inversion" && marker == "reentrant" && b.dispatching {
		b.mu.Unlock()
		deliver()
		return nil
	}
	b.queue = append(b.queue, deliver)
	if b.defect == "duplicate-handoff" && marker == "fresh-next" {
		b.queue = append(b.queue, deliver)
	}
	b.mu.Unlock()
	b.drain()
	return nil
}
func (b *profileObservationBinding) drain() {
	b.mu.Lock()
	if b.dispatching {
		b.mu.Unlock()
		return
	}
	b.dispatching = true
	b.mu.Unlock()
	for {
		b.mu.Lock()
		if len(b.queue) == 0 {
			b.dispatching = false
			b.mu.Unlock()
			return
		}
		next := b.queue[0]
		b.queue = b.queue[1:]
		b.mu.Unlock()
		next()
	}
}

// RFC §8.3:467–477, §7.0:63, A.3:1335–1339/1425.
func TestProfileObservationContract(t *testing.T) {
	for _, defect := range []string{"", "drop-intermediate", "duplicate-handoff", "hide-loss", "stale-resnapshot", "reentrant-inversion", "snapshot-alias", "remove-all", "split-related"} {
		t.Run(defect, func(t *testing.T) {
			target := profileObservationTarget{FakeTarget: MakeFakeTarget(NoDefects), defect: defect}
			report := harness.RunConformance(context.Background(), target, capability.ProfileObservationTests, harness.RunOptions{})
			want := harness.Pass
			if defect != "" {
				want = harness.Fail
			}
			for _, r := range report.Results {
				if r.TestID == "CT-PROFILE-OBS-001" && r.Status != want {
					t.Fatalf("%s: %s", r.Status, r.Reason)
				}
			}
		})
	}
	t.Run("missing one claimed profile", func(t *testing.T) {
		target := profileObservationTarget{FakeTarget: MakeFakeTarget(NoDefects), omit: "recording"}
		r := harness.RunConformance(context.Background(), target, capability.ProfileObservationTests, harness.RunOptions{})
		if statusOf(r, "CT-PROFILE-OBS-001") != harness.Incomplete {
			t.Fatalf("missing mapping status %s", statusOf(r, "CT-PROFILE-OBS-001"))
		}
	})
	t.Run("transient facts do not require retained payload", func(t *testing.T) {
		target := profileObservationTarget{FakeTarget: MakeFakeTarget(NoDefects), transient: true}
		r := harness.RunConformance(context.Background(), target, capability.ProfileObservationTests, harness.RunOptions{})
		if statusOf(r, "CT-PROFILE-OBS-001") != harness.Pass {
			t.Fatalf("transient mapping status %s", statusOf(r, "CT-PROFILE-OBS-001"))
		}
	})
}

// RFC §§4:23,8.3:475: unavailable initial/resnapshot coverage remains incomplete.
func TestUnknownProfileObservationCoverageIsIncomplete(t *testing.T) {
	target := profileObservationTarget{FakeTarget: MakeFakeTarget(NoDefects), defect: "unknown-coverage"}
	r := harness.RunConformance(context.Background(), target, capability.ProfileObservationTests, harness.RunOptions{})
	if statusOf(r, "CT-PROFILE-OBS-001") != harness.Incomplete {
		t.Fatalf("unknown profile coverage status %s", statusOf(r, "CT-PROFILE-OBS-001"))
	}
}

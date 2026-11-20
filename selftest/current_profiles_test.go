package selftest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
)

// Each mapping below calls the test implementation's actual operation and uses
// separate peer/service observations. These are harness fixtures, not evidence
// for a real provider or replacements for the target's public API.
type currentProfileTarget struct {
	*FakeTarget
	defect string
}

func currentProfileFake(defect string) *currentProfileTarget {
	return &currentProfileTarget{MakeFakeTarget(FakeDefects{}), defect}
}
func (t *currentProfileTarget) MessageScenario(context.Context) (harness.MessageScenario, error) {
	var mu sync.Mutex
	online := true
	submissions := 0
	unknown := false
	next := 0
	observers := map[int]func(harness.MessageEvent){}
	var received, queued [][]byte
	modes := []harness.MessageRecipients{{Kind: "direct", IDs: []string{"presence-peer"}}, {Kind: "broadcast", Scope: "room-incarnation", MembershipBoundary: "current-authorized-members"}}
	send := func(_ context.Context, scope harness.MessageRecipients, payload []byte) (harness.MessageSubmission, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(payload) > 64 && t.defect != "message-overflow" {
			return harness.MessageSubmission{}, fail("invalid-argument", "sendMessage", "payload exceeds 64 octets", failureExtras{})
		}
		submissions++
		if unknown || t.defect == "message-unknown-positive" {
			unknown = false
			if t.defect == "message-retry" {
				submissions++
			}
			return harness.MessageSubmission{State: "unknown", Recipients: scope}, nil
		}
		if online {
			received = append(received, append([]byte(nil), payload...))
		} else if t.defect == "message-offline-replay" {
			queued = append(queued, append([]byte(nil), payload...))
		}
		if t.defect == "message-recipient-scope" {
			scope = modes[1]
		}
		return harness.MessageSubmission{State: "submitted", Recipients: scope}, nil
	}
	return harness.MessageScenario{Documentation: "selftest public Send/Observe v1; peer delivery at payload boundary", Modes: modes, MaximumPayloadOctets: openrtc.KnownValue(64), Send: send,
		Observe: func(cb func(harness.MessageEvent)) (func() error, error) {
			mu.Lock()
			next++
			id := next
			observers[id] = cb
			mu.Unlock()
			return func() error {
				mu.Lock()
				defer mu.Unlock()
				if t.defect != "message-removal" {
					delete(observers, id)
				}
				return nil
			}, nil
		},
		PeerSend: func(_ context.Context, scope harness.MessageRecipients, payload []byte) error {
			mu.Lock()
			callbacks := make([]func(harness.MessageEvent), 0, len(observers))
			for _, f := range observers {
				callbacks = append(callbacks, f)
			}
			mu.Unlock()
			sender := openrtc.Unknown[string]()
			if t.defect == "message-sender-omitted" {
				sender = openrtc.Known[string]{}
			}
			event := harness.MessageEvent{Kind: "message", Sender: sender, Recipients: scope, Payload: append([]byte(nil), payload...)}
			for _, f := range callbacks {
				f(event)
			}
			return nil
		},
		LoseDelivery: func(context.Context) error {
			mu.Lock()
			callbacks := make([]func(harness.MessageEvent), 0, len(observers))
			for _, f := range observers {
				callbacks = append(callbacks, f)
			}
			mu.Unlock()
			if t.defect != "message-loss-hidden" {
				for _, f := range callbacks {
					f(harness.MessageEvent{Kind: "loss"})
				}
			}
			return nil
		},
		SetOnline: func(_ context.Context, v bool) error {
			mu.Lock()
			defer mu.Unlock()
			online = v
			if v && len(queued) > 0 {
				received = append(received, queued...)
				queued = nil
			}
			return nil
		},
		Checkpoint: func(context.Context) error { return nil }, RecipientPayloads: func() [][]byte {
			mu.Lock()
			defer mu.Unlock()
			out := make([][]byte, len(received))
			for i, p := range received {
				out[i] = append([]byte(nil), p...)
			}
			return out
		},
		DefaultSurfaces: func() []any {
			if t.defect == "message-payload-retained" {
				return []any{map[string]string{"last": "transient-secret-message-5af3"}}
			}
			return []any{map[string]string{"snapshot": "no-payload"}, []string{"sendMessage settled"}}
		},
		SubmissionCount: func() int { mu.Lock(); defer mu.Unlock(); return submissions }, NextSubmissionUnknown: func() { mu.Lock(); unknown = true; mu.Unlock() }}, nil
}

type profileDeviceState struct {
	mu         sync.Mutex
	defect     string
	selections []openrtc.DeviceSelection
	counts     harness.NativeEffectCounts
}
type profileDevices struct{ state *profileDeviceState }

func (d profileDevices) Counts() harness.NativeEffectCounts {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	return d.state.counts
}
func (d profileDevices) Snapshot() openrtc.DeviceSnapshot {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	coverage := "complete"
	if d.state.defect == "device-list-completeness" {
		coverage = ""
	}
	return openrtc.DeviceSnapshot{Inventory: openrtc.DeviceInventory{Coverage: coverage, Entries: []openrtc.DeviceView{{ID: "audio-device", Kind: "audio", Direction: "input", Label: openrtc.Unknown[string](), NativeID: openrtc.Unknown[openrtc.ScopedNativeID]()}}}, Permissions: map[openrtc.MediaKind]openrtc.Known[string]{"audio": openrtc.KnownValue("granted")}, Selections: append([]openrtc.DeviceSelection(nil), d.state.selections...), Freshness: openrtc.SourceFreshness{Owner: "client-incarnation", Status: "current"}}
}
func (d profileDevices) Enumerate(context.Context) openrtc.Async[openrtc.DeviceInventory] {
	return openrtc.StartOperation(d, func() (openrtc.DeviceInventory, error) {
		if d.state.defect == "device-list-captures" {
			d.state.mu.Lock()
			d.state.counts.Captures++
			d.state.mu.Unlock()
		}
		return d.Snapshot().Inventory, nil
	})
}
func (d profileDevices) Select(_ context.Context, selection openrtc.DeviceSelection) openrtc.Async[openrtc.Void] {
	return openrtc.StartOperation(d, func() (openrtc.Void, error) {
		d.state.mu.Lock()
		defer d.state.mu.Unlock()
		if d.state.defect != "device-selection-noop" {
			d.state.selections = []openrtc.DeviceSelection{selection}
		}
		if d.state.defect == "device-selection-captures" {
			d.state.counts.Captures++
		}
		return openrtc.Void{}, nil
	})
}
func (d profileDevices) Watch(f func(openrtc.DeviceSnapshot)) openrtc.Unsubscribe {
	f(d.Snapshot())
	return func() {}
}

type profileStatistics struct{ state *profileDeviceState }

func (s profileStatistics) Sample(_ context.Context, target openrtc.StatisticsTarget) openrtc.Async[openrtc.StatsSample] {
	return openrtc.StartOperation(s, func() (openrtc.StatsSample, error) {
		unit := "dB"
		if s.state.defect == "statistics-unit-omitted" {
			unit = ""
		}
		collectedAt := openrtc.Instant("2026-11-20T00:00:00Z")
		if s.state.defect == "statistics-time-unknown" {
			collectedAt = ""
		}
		return openrtc.StatsSample{Target: target, Coverage: "complete", CollectedAt: collectedAt, Interval: openrtc.KnownValue(openrtc.Duration{Milliseconds: 50}), Measurements: []openrtc.Measurement{{Metric: "signal-level", Unit: unit, Target: target, Value: openrtc.KnownValue(float64(-3)), Interval: openrtc.KnownValue(openrtc.Duration{Milliseconds: 50})}}}, nil
	})
}
func (t *currentProfileTarget) DeviceScenario(context.Context) (harness.DeviceScenario, error) {
	s := &profileDeviceState{defect: t.defect}
	manager := profileDevices{s}
	stats := profileStatistics{s}
	target := openrtc.StatisticsTarget{Kind: "source", ID: "source-incarnation"}
	return harness.DeviceScenario{Documentation: "selftest device ownership client-incarnation, source-incarnation; empty collectedAt explicitly unknown", Manager: manager, Kind: "audio", Selection: openrtc.DeviceSelection{Device: "audio-device", Target: openrtc.DeviceSelectionTarget{Kind: "source", ID: target.ID}}, Effects: manager, Statistics: stats, StatisticsTarget: target, CachedSample: func(ctx context.Context) (openrtc.StatsSample, error) {
		sample, e := stats.Sample(ctx, target).Result(ctx)
		if t.defect == "statistics-cached-time" {
			sample.CollectedAt = "2030-01-01T00:00:00Z"
		}
		return sample, e
	}}, nil
}

type profileGate struct {
	reached, release  chan struct{}
	once, releaseOnce sync.Once
}

func newProfileGate() *profileGate {
	return &profileGate{reached: make(chan struct{}), release: make(chan struct{})}
}
func (g *profileGate) Reached() <-chan struct{} { return g.reached }
func (g *profileGate) ReleaseSuccess()          { g.releaseOnce.Do(func() { close(g.release) }) }
func (g *profileGate) ReleaseFailure(any)       { g.ReleaseSuccess() }
func (t *currentProfileTarget) RenewalScenario(context.Context) (harness.RenewalScenario, error) {
	var mu sync.Mutex
	next := 0
	submissions := 0
	installed := "old"
	connected := true
	var gate *profileGate
	failNext := false
	ackState := "applied"
	renew := func(ctx context.Context, candidate string) (harness.RenewalResult, error) {
		mu.Lock()
		if candidate == "mismatched" && t.defect != "renewal-mismatch-submitted" {
			mu.Unlock()
			return harness.RenewalResult{}, fail("permission-denied", "renewCredential", "known destination mismatch", failureExtras{})
		}
		submissions++
		next++
		sequence := next
		g := gate
		gate = nil
		failure := failNext
		failNext = false
		mu.Unlock()
		if g != nil {
			g.once.Do(func() { close(g.reached) })
			select {
			case <-g.release:
			case <-ctx.Done():
				return harness.RenewalResult{}, ctx.Err()
			}
		}
		mu.Lock()
		defer mu.Unlock()
		if failure {
			if t.defect == "renewal-failure-disconnects" {
				connected = false
			}
			return harness.RenewalResult{}, fail("unavailable", "renewCredential", "replacement service unavailable", failureExtras{})
		}
		if ackState != "applied" {
			if t.defect == "renewal-unacknowledged-installs" {
				installed = candidate
			}
			state := ackState
			if t.defect == "renewal-deferred-reports-applied" {
				state = "applied"
			}
			return harness.RenewalResult{State: state, Acknowledged: false, Boundary: "provider-live-reauthorization"}, nil
		}
		if sequence == next || t.defect == "renewal-stale-installs" {
			installed = candidate
		}
		ack := t.defect != "renewal-unacknowledged-applied"
		return harness.RenewalResult{State: "applied", Acknowledged: ack, Boundary: "provider-live-reauthorization"}, nil
	}
	return harness.RenewalScenario{Documentation: "selftest actual Renew operation with independent applied-version observation", SetAcknowledgementState: func(state string) { mu.Lock(); ackState = state; mu.Unlock() }, Renew: renew, First: "candidate-A", Second: "candidate-B", Mismatched: "mismatched", HoldNext: func() harness.JoinGate { mu.Lock(); defer mu.Unlock(); gate = newProfileGate(); return gate }, Installed: func() openrtc.Known[string] { mu.Lock(); defer mu.Unlock(); return openrtc.KnownValue(installed) }, SubmissionCount: func() int { mu.Lock(); defer mu.Unlock(); return submissions }, PublicConnected: func() openrtc.Known[bool] { mu.Lock(); defer mu.Unlock(); return openrtc.KnownValue(connected) }, FailNext: func() { mu.Lock(); failNext = true; mu.Unlock() }, Checkpoint: func(context.Context) error { return nil }}, nil
}
func (t *currentProfileTarget) AdministrationScenario(context.Context) (harness.AdministrationScenario, error) {
	var mu sync.Mutex
	next := 0
	conferences := map[string]string{}
	participants := map[string]string{}
	server := "trusted-server"
	minted := ""
	denied := func(operation string) error {
		return fail("permission-denied", operation, "trusted-server authority required", failureExtras{})
	}
	return harness.AdministrationScenario{Documentation: "selftest actual administration operations; service-state witness separate from command acknowledgments", Server: server,
		Create: func(_ context.Context, authority string) (harness.ConferenceFacts, error) {
			mu.Lock()
			defer mu.Unlock()
			if authority != server && t.defect != "admin-client-escalation" {
				return harness.ConferenceFacts{}, denied("createConference")
			}
			next++
			id := fmt.Sprintf("conference-%d", next)
			conferences[id] = "active"
			state := "active"
			if t.defect == "admin-create-not-active" {
				state = "unknown"
			}
			return harness.ConferenceFacts{ID: id, Owner: server, State: state}, nil
		},
		End: func(_ context.Context, authority, id string) (harness.AdministrationRelease, error) {
			mu.Lock()
			defer mu.Unlock()
			if authority != server {
				return harness.AdministrationRelease{}, denied("endConference")
			}
			if t.defect == "admin-end-unconfirmed" {
				conferences[id] = "pending"
				return harness.AdministrationRelease{Scope: id, RemoteState: openrtc.Unknown[string]()}, nil
			}
			conferences[id] = "ended"
			if t.defect == "admin-end-other-incarnation" {
				for other := range conferences {
					if other != id {
						conferences[other] = "ended"
					}
				}
			}
			return harness.AdministrationRelease{Scope: id, RemoteState: openrtc.KnownValue("ended")}, nil
		},
		Populate: func(_ context.Context, conference string) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			next++
			id := fmt.Sprintf("%s/presence-%d", conference, next)
			participants[id] = "present"
			return id, nil
		},
		Remove: func(_ context.Context, authority, conference, presence string) (harness.AdministrationRelease, error) {
			mu.Lock()
			defer mu.Unlock()
			if authority != server {
				return harness.AdministrationRelease{}, denied("removeParticipant")
			}
			participants[presence] = "departed"
			if t.defect == "admin-remove-other-presence" {
				for id := range participants {
					participants[id] = "departed"
				}
			}
			return harness.AdministrationRelease{Scope: presence, RemoteState: openrtc.KnownValue("departed")}, nil
		},
		Mint: func(_ context.Context, authority string, claims harness.CredentialFacts) (harness.CredentialFacts, error) {
			mu.Lock()
			defer mu.Unlock()
			if authority != server {
				return harness.CredentialFacts{}, denied("mintCredential")
			}
			minted = "issued-secret-only-authorized-channel"
			claims.ID = minted
			claims.AuthorizedChannel = t.defect != "admin-mint-wrong-channel"
			return claims, nil
		},
		Conference: func(id string) openrtc.Known[string] {
			mu.Lock()
			defer mu.Unlock()
			s, ok := conferences[id]
			if s == "pending" {
				return openrtc.Unknown[string]()
			}
			if !ok {
				return openrtc.Unknown[string]()
			}
			return openrtc.KnownValue(s)
		}, Participant: func(id string) openrtc.Known[string] {
			mu.Lock()
			defer mu.Unlock()
			s, ok := participants[id]
			if !ok {
				return openrtc.Unknown[string]()
			}
			return openrtc.KnownValue(s)
		},
		DefaultSurfaces: func() []any {
			mu.Lock()
			defer mu.Unlock()
			if t.defect == "admin-credential-leak" {
				return []any{map[string]string{"credential": minted}}
			}
			return []any{map[string]string{"conference": "active"}, []string{"mintCredential settled"}}
		}, Checkpoint: func(context.Context) error { return nil }}, nil
}

type profileVersionEffects struct {
	mu     sync.Mutex
	counts harness.NativeEffectCounts
}

func (e *profileVersionEffects) Counts() harness.NativeEffectCounts {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.counts
}
func (t *currentProfileTarget) VersionScenario(context.Context) (harness.VersionScenario, error) {
	effects := &profileVersionEffects{}
	return harness.VersionScenario{Documentation: "selftest screen-sharing versioned captureScreen operation", Profile: "screen-sharing", Namespace: "openrtc/screen-sharing", SupportedVersion: 1, Effects: effects, Execute: func(_ context.Context, version int, namespace string) (harness.VersionResult, error) {
		effects.mu.Lock()
		defer effects.mu.Unlock()
		if version != 1 || namespace != "openrtc/screen-sharing" {
			if t.defect == "version-effects-before-reject" {
				effects.counts.Captures++
			}
			return harness.VersionResult{}, fail("unsupported", "captureScreen", "unsupported version or namespace", failureExtras{})
		}
		effects.counts.Captures++
		op := "captureScreen"
		if t.defect == "version-accessor-only" {
			op = "accessor"
		}
		return harness.VersionResult{Operation: op, Resource: "screen-source-incarnation", State: "active"}, nil
	}}, nil
}

// RFC §§7.0:65, 11.5, 11.9–11.11 and 14.3:1125–1149: each reusable
// positive/adversarial oracle executes against a conforming mapping and one or
// more focused defects. A fake pass proves the oracle, never provider service.
func TestCurrentProfileOperationOracles(t *testing.T) {
	mutants := map[string][]string{
		"CT-MSG-001": {"message-recipient-scope"}, "CT-MSG-002": {"message-removal", "message-sender-omitted", "message-loss-hidden"}, "CT-MSG-003": {"message-overflow"}, "CT-MSG-004": {"message-offline-replay", "message-retry", "message-payload-retained"},
		"CT-DEVICE-001": {"device-list-completeness", "device-list-captures"}, "CT-DEVICE-002": {"device-selection-noop", "device-selection-captures"}, "CT-DEVICE-003": {"statistics-unit-omitted", "statistics-cached-time"},
		"CT-RENEW-001": {"renewal-unacknowledged-applied"}, "CT-RENEW-002": {"renewal-mismatch-submitted", "renewal-failure-disconnects"}, "CT-RENEW-003": {"renewal-stale-installs"}, "CT-RENEW-004": {"renewal-unacknowledged-installs", "renewal-deferred-reports-applied"},
		"CT-ADMIN-001": {"admin-create-not-active"}, "CT-ADMIN-002": {"admin-end-other-incarnation"}, "CT-ADMIN-003": {"admin-remove-other-presence"}, "CT-ADMIN-004": {"admin-client-escalation", "admin-mint-wrong-channel", "admin-credential-leak"},
		"CT-VERSION-001": {"version-effects-before-reject"}, "CT-VERSION-002": {"version-accessor-only"}}
	for _, test := range capability.CurrentProfileTests {
		t.Run(test.ID, func(t *testing.T) {
			for _, defect := range append([]string{""}, mutants[test.ID]...) {
				name := defect
				if name == "" {
					name = "conforming"
				}
				t.Run(name, func(t *testing.T) {
					target := currentProfileFake(defect)
					tc := harness.NewTestContext(target, target.CreateAdapter().Descriptor())
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					err := test.Run(ctx, tc)
					if defect == "" && err != nil {
						t.Fatalf("conforming fake: %v", err)
					}
					if defect != "" && err == nil {
						t.Fatalf("defective fake %s passed", defect)
					}
				})
			}
		})
	}
}

// RFC §11.10:999: unknown collection time is a legitimate qualified value,
// not a failed timestamp parse and not evidence of collection now.
func TestStatisticsUnknownCollectionTimeRemainsPermitted(t *testing.T) {
	target := currentProfileFake("statistics-time-unknown")
	tc := harness.NewTestContext(target, target.CreateAdapter().Descriptor())
	for _, test := range capability.CurrentProfileTests {
		if test.ID == "CT-DEVICE-003" {
			if err := test.Run(context.Background(), tc); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatal("statistics test unregistered")
}

// RFC §§11.5:770–775,14.2:1102–1107: unknown submission is unknown
// evidence, not a demonstrated positive for a claimed messaging operation.
func TestUnknownPositiveMessageSubmissionIsIncomplete(t *testing.T) {
	target := currentProfileFake("message-unknown-positive")
	tc := harness.NewTestContext(target, target.CreateAdapter().Descriptor())
	for _, test := range capability.CurrentProfileTests {
		if test.ID == "CT-MSG-001" {
			err := test.Run(context.Background(), tc)
			var inc *harness.Inconclusive
			if !errors.As(err, &inc) || inc.Status != harness.NotRun {
				t.Fatalf("unknown submission became success/failure: %v", err)
			}
			return
		}
	}
	t.Fatal("messaging positive unregistered")
}

// RFC §§11.9:929–935,14.2:1102–1107: a local administrative end
// acknowledgment with withheld remote outcome leaves remote release incomplete.
func TestUnconfirmedRemoteAdministrationIsIncomplete(t *testing.T) {
	target := currentProfileFake("admin-end-unconfirmed")
	tc := harness.NewTestContext(target, target.CreateAdapter().Descriptor())
	for _, test := range capability.CurrentProfileTests {
		if test.ID == "CT-ADMIN-002" {
			if err := test.Run(context.Background(), tc); err != nil {
				t.Fatal(err)
			}
			if len(tc.Partials) == 0 {
				t.Fatal("unknown remote end became complete success")
			}
			return
		}
	}
	t.Fatal("administrative end scenario missing")
}

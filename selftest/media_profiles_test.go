package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
	"strings"
	"sync"
	"testing"
)

type currentMediaTarget struct {
	*FakeTarget
	fixture harness.CurrentMediaProfileFixture
}

func (t currentMediaTarget) CurrentMediaProfiles() harness.CurrentMediaProfileFixture {
	return t.fixture
}
func (t currentMediaTarget) CapabilityFor(_ context.Context, d harness.CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool) {
	v := descriptorFor(openrtc.PortableCapabilityID(d.ID), 1)
	v.Support.Level = "supported"
	return []openrtc.CapabilityDescriptor{v}, true
}
func completedVoid(work func() error) openrtc.Async[openrtc.Void] {
	return openrtc.StartOperation(nil, func() (openrtc.Void, error) { return openrtc.Void{}, work() })
}

type currentCallFake struct {
	mu        sync.Mutex
	v         openrtc.CurrentCallView
	defect    string
	submitted *string
	prefix    *int
}

func (c *currentCallFake) Snapshot() openrtc.CurrentCallView {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.v
	v.Kinds = append([]openrtc.MediaKind{}, v.Kinds...)
	v.Updates = append([]openrtc.RenegotiationUpdate{}, v.Updates...)
	return v
}
func (c *currentCallFake) Accept(context.Context, openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.v.Answered = true
		c.v.State = "active"
		if c.defect == "accept-pending" {
			c.v.Answered = false
		}
		return nil
	})
}
func (c *currentCallFake) Reject(context.Context) openrtc.Async[openrtc.Void] {
	return c.Hangup(context.Background())
}
func (c *currentCallFake) Hangup(context.Context) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error { c.mu.Lock(); defer c.mu.Unlock(); c.v.State = "ended"; return nil })
}
func (c *currentCallFake) SetHold(_ context.Context, v bool, _ openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.v.State == "ended" {
			return fail(openrtc.InvalidState, "setHold", "ended Call", failureExtras{})
		}
		c.v.LocalHold = openrtc.KnownValue(v)
		if c.defect == "resume-clears-remote" && !v {
			c.v.RemoteHold = openrtc.KnownValue(false)
		}
		c.v.State = openrtc.DisplayedCallState(c.v.Answered, c.v.TransferActive, c.v.LocalHold, c.v.RemoteHold)
		return nil
	})
}
func (c *currentCallFake) Transfer(context.Context, openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.v.TransferActive = true
		c.v.State = "transferring"
		return nil
	})
}
func (c *currentCallFake) SendDigits(_ context.Context, d string, _ []openrtc.DtmfMethod, _ openrtc.OperationOptions) openrtc.Async[openrtc.DtmfResult] {
	return openrtc.StartOperation(c, func() (openrtc.DtmfResult, error) {
		n := *c.prefix
		if n > 0 && n < len(d) {
			*c.submitted = d[:n]
		} else {
			*c.submitted = d
		}
		if c.defect == "duplicate-digits" {
			*c.submitted += d
		}
		return openrtc.DtmfResult{SubmittedPrefix: openrtc.KnownValue(*c.submitted), Methods: openrtc.KnownValue([]openrtc.DtmfMethod{"rtp-event"})}, nil
	})
}
func callingProfileFake(defect string) *harness.CallingProfileFixture {
	submitted := ""
	prefix := 0
	offers := 0
	queued := false
	disposed := false
	account := openrtc.ProviderIdentity{Namespace: "selftest", ID: "account-A"}
	registration := openrtc.RegistrationStatus{Status: "unregistered", Account: openrtc.KnownValue(account), Validity: "unknown", Refresh: "idle", ExpiresAt: openrtc.Unknown[openrtc.Instant](), IncomingDeliveryEnabled: openrtc.KnownValue(false)}
	listeners := map[int]func(openrtc.RegistrationStatus){}
	next := 0
	emit := func() {
		for _, listener := range listeners {
			listener(registration)
		}
	}
	makeCall := func(answered bool) *currentCallFake {
		v := openrtc.CurrentCallView{ID: newID("call"), State: "active", Answered: answered, LocalHold: openrtc.KnownValue(false), RemoteHold: openrtc.KnownValue(false)}
		if !answered {
			v.State = "ringing"
		}
		return &currentCallFake{v: v, defect: defect, submitted: &submitted, prefix: &prefix}
	}
	return &harness.CallingProfileFixture{Account: account, Effects: func() harness.MediaFixtureEffects { return harness.MediaFixtureEffects{} },
		Dial: func(context.Context) openrtc.Async[openrtc.CurrentCall] {
			return openrtc.StartOperation(nil, func() (openrtc.CurrentCall, error) { return makeCall(true), nil })
		},
		Incoming: func(context.Context) (openrtc.CurrentCall, error) {
			call := makeCall(false)
			if defect == "incoming-answers" {
				call.v.Answered = true
				call.v.State = "active"
			}
			return call, nil
		},
		RemoteHold: func(_ context.Context, call openrtc.CurrentCall, v openrtc.Known[bool]) error {
			c := call.(*currentCallFake)
			c.mu.Lock()
			defer c.mu.Unlock()
			c.v.RemoteHold = v
			c.v.State = openrtc.DisplayedCallState(c.v.Answered, c.v.TransferActive, c.v.LocalHold, v)
			if defect == "unknown-hold-active" && v.Status == "unknown" {
				c.v.State = "active"
			}
			return nil
		},
		TransferClaimed: true, FinishTransfer: func(_ context.Context, call openrtc.CurrentCall) error {
			c := call.(*currentCallFake)
			c.mu.Lock()
			defer c.mu.Unlock()
			c.v.TransferActive = false
			c.v.State = openrtc.DisplayedCallState(c.v.Answered, false, c.v.LocalHold, c.v.RemoteHold)
			return nil
		},
		RemoteOffer: func(_ context.Context, call openrtc.CurrentCall, p openrtc.RemoteUpdatePolicy, enforceable bool) error {
			if !enforceable {
				return fail(openrtc.Unsupported, "remoteUpdate", "selected policy unavailable", failureExtras{})
			}
			c := call.(*currentCallFake)
			c.mu.Lock()
			defer c.mu.Unlock()
			decision := "rejected"
			if p == "accept-without-new-kinds" {
				decision = "accepted-without-new-kinds"
			}
			if defect == "offer-adds-kind" {
				c.v.Kinds = append(c.v.Kinds, "video")
			}
			c.v.Updates = append(c.v.Updates, openrtc.RenegotiationUpdate{Policy: p, Decision: decision})
			return nil
		},
		WithholdTransferDecision: func(_ context.Context, call openrtc.CurrentCall) error {
			if defect == "withheld-transfer-executes" {
				c := call.(*currentCallFake)
				c.mu.Lock()
				c.v.TransferActive = true
				c.mu.Unlock()
			}
			return nil
		},
		StartReceiving: func(context.Context) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error {
				registration.Status = "registered"
				registration.Validity = "valid"
				registration.IncomingDeliveryEnabled = openrtc.KnownValue(true)
				registration.ExpiresAt = openrtc.KnownValue(openrtc.Instant("2030-01-01T00:00:00Z"))
				emit()
				return nil
			})
		},
		StopReceiving: func(context.Context) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error {
				registration.Status = "unregistered"
				registration.IncomingDeliveryEnabled = openrtc.KnownValue(false)
				emit()
				return nil
			})
		}, Registration: func() openrtc.RegistrationStatus { return registration },
		RefreshRegistration: func(context.Context) error {
			registration.Refresh = "refreshing"
			emit()
			registration.Refresh = "idle"
			emit()
			return nil
		},
		ExpireRegistration: func(context.Context) error {
			if defect != "expired-stays-valid" {
				registration.Validity = "expired"
				registration.Status = "unregistered"
				registration.IncomingDeliveryEnabled = openrtc.KnownValue(false)
			}
			emit()
			return nil
		},
		ObserveRegistration: func(listener func(openrtc.RegistrationStatus)) openrtc.Unsubscribe {
			next++
			id := next
			listeners[id] = listener
			listener(registration)
			return func() { delete(listeners, id) }
		},
		QueueIncoming: func(context.Context) error { queued = true; return nil }, DeliverQueuedIncoming: func(context.Context) error {
			if queued {
				queued = false
				if (!disposed && registration.Status == "registered") || defect == "late-offer" {
					offers++
				}
			}
			return nil
		}, IncomingOffers: func() int { return offers },
		DisposeReceiver: func(context.Context) error {
			disposed = true
			registration.Status = "unregistered"
			registration.IncomingDeliveryEnabled = openrtc.KnownValue(false)
			emit()
			return nil
		},
		FailDigitsAfter: func(_ context.Context, n int) error { prefix = n; return nil }, SubmittedDigits: func() string { return submitted },
	}
}

type subscriptionFake struct {
	mu          sync.Mutex
	publication openrtc.TrackPublication
	subs        map[openrtc.SubscriptionID]openrtc.TrackSubscription
	defect      string
}

func (s *subscriptionFake) Subscribe(_ context.Context, id openrtc.PublicationID, k []openrtc.MediaKind) openrtc.Async[openrtc.TrackSubscription] {
	return openrtc.StartOperation(s, func() (openrtc.TrackSubscription, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, v := range s.subs {
			if v.State == "subscribed" {
				return openrtc.TrackSubscription{}, fail(openrtc.InvalidState, "subscribe", "duplicate selection", failureExtras{})
			}
		}
		v := openrtc.TrackSubscription{ID: openrtc.SubscriptionID(newID("sub")), State: "subscribed", Source: openrtc.ReceiveSourceReference{Kind: "publication", Publication: id}, Kinds: k}
		if s.defect == "subscribe-pending" {
			v.State = "subscribing"
		}
		s.subs[v.ID] = v
		return v, nil
	})
}
func (s *subscriptionFake) Unsubscribe(_ context.Context, id openrtc.SubscriptionID) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		v := s.subs[id]
		v.State = "unsubscribed"
		s.subs[id] = v
		if s.defect == "unsubscribe-removes-remote" {
			s.publication.State = "unpublished"
		}
		return nil
	})
}
func subscriptionProfileFake(defect string) *harness.SubscriptionProfileFixture {
	s := &subscriptionFake{publication: openrtc.TrackPublication{ID: "remote", State: "published", Kinds: []openrtc.MediaKind{"audio"}}, subs: map[openrtc.SubscriptionID]openrtc.TrackSubscription{}, defect: defect}
	return &harness.SubscriptionProfileFixture{Control: s, Publication: s.publication, Snapshot: func(id openrtc.SubscriptionID) openrtc.TrackSubscription {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.subs[id]
	}, RemotePublication: func() openrtc.TrackPublication { s.mu.Lock(); defer s.mu.Unlock(); return s.publication }, Teardown: func(context.Context) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if defect == "subscription-teardown-noop" {
			return nil
		}
		for id, v := range s.subs {
			v.State = "unsubscribed"
			s.subs[id] = v
		}
		return nil
	}}
}

type grantSourceFake struct{ v openrtc.CaptureSourceView }

func (s *grantSourceFake) Snapshot() openrtc.CaptureSourceView { return s.v }
func (s *grantSourceFake) Inspection() openrtc.OwnerInspection { return nil }
func (s *grantSourceFake) Tracks() []openrtc.LocalTrack        { return nil }
func (s *grantSourceFake) Stop(context.Context, ...openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error { s.v.State = "stopped"; return nil })
}

type grantsFake struct {
	mu     sync.Mutex
	grants map[string]openrtc.SourceGrant
	defect string
}

func (g *grantsFake) ShareSource(_ context.Context, source openrtc.CaptureSourceID, r openrtc.ResourceRef, scope openrtc.CapabilityScope, _ openrtc.OperationOptions) openrtc.Async[openrtc.SourceGrant] {
	return openrtc.StartOperation(g, func() (openrtc.SourceGrant, error) {
		g.mu.Lock()
		defer g.mu.Unlock()
		v := openrtc.SourceGrant{Owner: openrtc.ResourceRef{Kind: "client", ID: "A"}, ID: newID("grant"), Source: source, Recipient: r, Scope: scope, State: "active"}
		g.grants[v.ID] = v
		return v, nil
	})
}
func (g *grantsFake) RevokeSourceGrant(_ context.Context, id string, _ openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		g.mu.Lock()
		defer g.mu.Unlock()
		v := g.grants[id]
		if g.defect != "revoke-noop" {
			v.State = "revoked"
		}
		g.grants[id] = v
		return nil
	})
}
func (g *grantsFake) Grant(id string) openrtc.SourceGrant {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.grants[id]
}
func grantProfileFake(defect string) *harness.GrantProfileFixture {
	g := &grantsFake{grants: map[string]openrtc.SourceGrant{}, defect: defect}
	src := &grantSourceFake{v: openrtc.CaptureSourceView{ID: "owner-source", State: "active", Owner: openrtc.ResourceRef{Kind: "client", ID: "A"}}}
	return &harness.GrantProfileFixture{Replacement: replacementProfileFake(defect), Control: g, Source: src, B: openrtc.ResourceRef{Kind: "client", ID: "B"}, C: openrtc.ResourceRef{Kind: "client", ID: "C"}, Scope: openrtc.CapabilityScope{Resource: "session", Authority: "client"},
		Use: func(_ context.Context, r openrtc.ResourceRef, scope openrtc.CapabilityScope) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error {
				g.mu.Lock()
				defer g.mu.Unlock()
				if defect == "ambient-grant" {
					return nil
				}
				for _, v := range g.grants {
					if v.State == "active" && v.Recipient.ID == r.ID && (v.Scope.Resource == scope.Resource || defect == "grant-scope-ignored") {
						return nil
					}
				}
				return fail(openrtc.PermissionDenied, "publish", "source use not granted", failureExtras{})
			})
		},
		TeardownRecipient: func(context.Context, openrtc.ResourceRef) error {
			if defect == "recipient-stops-owner" {
				src.v.State = "stopped"
			}
			return nil
		},
	}
}

func screenProfileFake(defect string) *harness.ScreenProfileFixture {
	sources := map[openrtc.CaptureSourceID]openrtc.SelectedScreen{}
	dismiss := false
	effects := harness.MediaFixtureEffects{}
	listeners := map[int]func(openrtc.SelectedScreen){}
	next := 0
	emit := func(v openrtc.SelectedScreen) {
		for _, listener := range listeners {
			listener(v)
		}
	}
	return &harness.ScreenProfileFixture{
		Capture: func(_ context.Context, o openrtc.OperationOptions) openrtc.Async[openrtc.SelectedScreen] {
			return openrtc.StartOperation(nil, func() (openrtc.SelectedScreen, error) {
				if dismiss {
					dismiss = false
					e := fail(openrtc.Cancelled, "captureScreen", "selection dismissed", failureExtras{}).(*openrtc.RTCOperationError)
					e.Reasons = []string{"selection-dismissed"}
					return openrtc.SelectedScreen{}, e
				}
				if len(o.Conditions) > 0 {
					if defect == "screen-rejected-acquires" {
						effects.Captures++
					}
					return openrtc.SelectedScreen{}, fail(openrtc.Unsupported, "captureScreen", "condition unavailable", failureExtras{})
				}
				effects.Captures++
				r := openrtc.SelectedScreen{Source: openrtc.CaptureSourceView{ID: openrtc.CaptureSourceID(newID("screen")), Owner: openrtc.ResourceRef{Kind: "local-session", ID: "screen-session"}, State: "active"}, Selection: openrtc.KnownValue("display-1"), AudioAssociation: openrtc.NotApplicable[openrtc.CaptureSourceID]("not selected")}
				if defect == "screen-auto-publishes" {
					r.Publications = []openrtc.PublicationID{"implicit"}
				}
				sources[r.Source.ID] = r
				emit(r)
				return r, nil
			})
		},
		DismissNext: func(context.Context) error { dismiss = true; return nil },
		EndSelection: func(_ context.Context, id openrtc.CaptureSourceID) error {
			r := sources[id]
			r.Source.State = "stopped"
			sources[id] = r
			emit(r)
			return nil
		},
		Snapshot: func(id openrtc.CaptureSourceID) openrtc.SelectedScreen { return sources[id] }, Effects: func() harness.MediaFixtureEffects { return effects }, UnsupportedCondition: openrtc.Condition{Name: "unsupported-window"},
		Observe: func(listener func(openrtc.SelectedScreen)) openrtc.Unsubscribe {
			next++
			id := next
			listeners[id] = listener
			for _, source := range sources {
				listener(source)
			}
			return func() { delete(listeners, id) }
		},
		ChangedSelection: openrtc.KnownValue("window-2"), ChangedAudioAssociation: openrtc.Unknown[openrtc.CaptureSourceID](),
		ChangeSelection: func(_ context.Context, id openrtc.CaptureSourceID) error {
			r := sources[id]
			r.Selection = openrtc.KnownValue("window-2")
			r.AudioAssociation = openrtc.Unknown[openrtc.CaptureSourceID]()
			sources[id] = r
			if defect != "screen-scope-update-dropped" {
				emit(r)
			}
			return nil
		},
	}
}

type recordingFake struct {
	mu     sync.Mutex
	v      openrtc.MediaJobView
	defect string
}

func (r *recordingFake) Inspection() openrtc.OwnerInspection { return nil }
func (r *recordingFake) Snapshot() openrtc.MediaJobView      { r.mu.Lock(); defer r.mu.Unlock(); return r.v }
func (r *recordingFake) Refresh(context.Context) openrtc.Async[openrtc.MediaJobView] {
	return openrtc.StartOperation(r, func() (openrtc.MediaJobView, error) { return r.Snapshot(), nil })
}
func (r *recordingFake) Stop(context.Context) openrtc.Async[openrtc.JobStopReceipt] {
	return openrtc.StartOperation(r, func() (openrtc.JobStopReceipt, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.v.Status = "stopping"
		if r.defect == "ack-is-stop" {
			r.v.Status = "stopped"
		}
		return openrtc.JobStopReceipt{Status: "accepted", Job: r.v}, nil
	})
}
func (r *recordingFake) OnChanged(func(openrtc.MediaJobView)) openrtc.Unsubscribe { return func() {} }
func (r *recordingFake) Dispose(ctx context.Context) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error { _, e := r.Stop(ctx).Result(ctx); return e })
}
func recordingProfileFake(defect string) *harness.RecordingProfileFixture {
	consent := "allowed"
	content := 0
	jobs := map[openrtc.JobID]*recordingFake{}
	return &harness.RecordingProfileFixture{ServiceContentBoundary: "selftest:recording-service/content", ServiceContentEvidence: "synthetic recorder output instrumentation", ServiceContent: func() openrtc.Known[int] { return openrtc.KnownValue(content) },
		Start: func(context.Context) openrtc.Async[openrtc.MediaJob] {
			return openrtc.StartOperation(nil, func() (openrtc.MediaJob, error) {
				if consent != "allowed" && defect != "record-without-consent" {
					if defect == "record-content-without-consent" {
						content++
					}
					return nil, fail(openrtc.PermissionDenied, "startRecording", "required consent missing", failureExtras{})
				}
				r := &recordingFake{v: openrtc.MediaJobView{ID: openrtc.JobID(newID("rec")), Status: "running"}, defect: defect}
				if defect == "recording-start-pending" {
					r.v.Status = "pending"
				}
				jobs[r.v.ID] = r
				return r, nil
			})
		},
		SetConsent: func(_ context.Context, v string) error { consent = v; return nil }, ServiceJobs: func() openrtc.Known[int] { return openrtc.KnownValue(len(jobs)) },
		ObserveRelease: func(_ context.Context, id openrtc.JobID) error {
			r := jobs[id]
			r.mu.Lock()
			r.v.Status = "stopped"
			r.mu.Unlock()
			return nil
		},
		LeaveOwner: func(context.Context) error {
			if defect == "recording-leave-noop" {
				return nil
			}
			for _, r := range jobs {
				r.mu.Lock()
				r.v.Status = "stopping"
				r.mu.Unlock()
			}
			return nil
		},
	}
}

type encryptionFake struct {
	mu     sync.Mutex
	v      openrtc.EncryptionCoverage
	defect string
}

func (e *encryptionFake) Coverage() openrtc.EncryptionCoverage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.v
}
func (e *encryptionFake) Configure(_ context.Context, r openrtc.EncryptionRequest) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.v = openrtc.EncryptionCoverage{Mode: openrtc.KnownValue(r.Mode), Effective: openrtc.KnownValue(true), Directions: r.Directions, MediaKinds: r.MediaKinds, TrustedEndpoints: r.TrustedEndpoints, Data: openrtc.KnownValue(r.Data), Boundary: r.Boundary, KeyResponsibility: openrtc.KnownValue("application"), Evidence: []openrtc.EvidenceReference{{Kind: "harness-self-test", Source: "synthetic only"}}}
		if e.defect == "encryption-not-effective" {
			e.v.Effective = openrtc.Unknown[bool]()
		}
		if e.defect == "encryption-wrong-direction" {
			e.v.Directions = []string{"receive"}
		}
		return nil
	})
}
func encryptionProfileFake(defect string) *harness.EncryptionProfileFixture {
	e := &encryptionFake{defect: defect}
	effects := 0
	return &harness.EncryptionProfileFixture{Control: e, Request: openrtc.EncryptionRequest{Mode: "end-to-end", MediaKinds: []openrtc.MediaKind{"audio"}, Directions: []string{"send"}, TrustedEndpoints: []openrtc.ProviderIdentity{{}}},
		WithdrawEvidence: func(context.Context) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.v.Effective = openrtc.Unknown[bool]()
			e.v.Evidence = nil
			return nil
		}, ProtectedEffects: func() int { return effects },
		SendProtected: func(context.Context) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error {
				if defect == "encryption-fallback" {
					effects++
					return nil
				}
				return fail(openrtc.Unsupported, "send", "encryption verification lost", failureExtras{})
			})
		},
		KeyCanary: "protected-key-canary-012345", DefaultSurfaces: func() []any {
			if defect == "key-leak" {
				return []any{"protected-key-canary-012345"}
			}
			return []any{e.Coverage()}
		},
	}
}
func newCurrentMediaTarget(defect string) currentMediaTarget {
	return currentMediaTarget{FakeTarget: MakeFakeTarget(NoDefects), fixture: harness.CurrentMediaProfileFixture{Calling: callingProfileFake(defect), Subscription: subscriptionProfileFake(defect), Grants: grantProfileFake(defect), Screen: screenProfileFake(defect), Recording: recordingProfileFake(defect), Encryption: encryptionProfileFake(defect)}}
}

// All these fixtures execute actual equivalent public operations; they validate
// harness oracles only. RFC §11.1–11.8, §14.2:1102–1115.
func TestCurrentMediaProfileScenarios(t *testing.T) {
	prefixes := []string{"CT-CALL-POS-", "CT-HOLD-", "CT-CALL-POLICY-", "CT-DIGIT-", "CT-SUB-", "CT-GRANT-", "CT-SCREEN-", "CT-REC-", "CT-ENC-"}
	for _, scenario := range capability.CapabilityTests {
		if scenario.ID == "CT-SUB-004" || scenario.ID == "CT-REC-005" {
			continue
		} // Dedicated scope-specific controls are validated in profile_edges_test.go.
		selected := false
		for _, prefix := range prefixes {
			selected = selected || strings.HasPrefix(scenario.ID, prefix)
		}
		if !selected {
			continue
		}
		t.Run(scenario.ID, func(t *testing.T) {
			report := harness.RunConformance(context.Background(), newCurrentMediaTarget(""), []harness.ConformanceTest{scenario}, harness.RunOptions{})
			if got := statusOf(report, scenario.ID); got != harness.Pass {
				for _, r := range report.Results {
					if r.TestID == scenario.ID {
						t.Fatalf("%s: %s", got, r.Reason)
					}
				}
			}
		})
	}
}
func TestCurrentMediaProfileMutants(t *testing.T) {
	for _, tc := range []struct{ id, defect string }{
		{"CT-SUB-003", "subscription-teardown-noop"}, {"CT-GRANT-002", "grant-scope-ignored"},
		{"CT-SCREEN-003", "screen-rejected-acquires"}, {"CT-REC-004", "recording-leave-noop"},
		{"CT-ENC-001", "encryption-wrong-direction"}, {"CT-REC-002", "record-content-without-consent"}, {"CT-CALL-POLICY-003", "expired-stays-valid"}, {"CT-CALL-POLICY-003", "late-offer"}, {"CT-GRANT-004", "replacement-clears-mute"}, {"CT-GRANT-004", "replacement-unknown-succeeds"}, {"CT-GRANT-004", "replacement-drops-survivors"}, {"CT-SCREEN-002", "screen-scope-update-dropped"}, {"CT-CALL-POS-002", "incoming-answers"}, {"CT-CALL-POS-003", "accept-pending"}, {"CT-HOLD-001", "unknown-hold-active"}, {"CT-CALL-POLICY-001", "offer-adds-kind"}, {"CT-CALL-POLICY-002", "withheld-transfer-executes"}, {"CT-DIGIT-001", "duplicate-digits"}, {"CT-SUB-001", "subscribe-pending"}, {"CT-SUB-002", "unsubscribe-removes-remote"}, {"CT-GRANT-001", "ambient-grant"}, {"CT-GRANT-003", "revoke-noop"}, {"CT-GRANT-004", "recipient-stops-owner"}, {"CT-SCREEN-001", "screen-auto-publishes"}, {"CT-REC-001", "recording-start-pending"}, {"CT-REC-002", "record-without-consent"}, {"CT-REC-003", "ack-is-stop"}, {"CT-ENC-001", "encryption-not-effective"}, {"CT-ENC-002", "encryption-fallback"}, {"CT-ENC-003", "key-leak"},
	} {
		t.Run(tc.defect, func(t *testing.T) {
			for _, scenario := range capability.CapabilityTests {
				if scenario.ID != tc.id {
					continue
				}
				report := harness.RunConformance(context.Background(), newCurrentMediaTarget(tc.defect), []harness.ConformanceTest{scenario}, harness.RunOptions{})
				if got := statusOf(report, scenario.ID); got != harness.Fail {
					t.Fatalf("%s got %s, want fail", tc.id, got)
				}
				return
			}
			t.Fatal("scenario unregistered")
		})
	}
}

func callMediaProfileFake(defect string) *harness.CallMediaProfileFixture {
	call := callingProfileFake("").Dial(context.Background())
	handle, _ := call.Result(context.Background())
	foreign := &grantSourceFake{v: openrtc.CaptureSourceView{ID: "foreign-source", Owner: openrtc.ResourceRef{Kind: "client", ID: "A"}, State: "active"}}
	owned := &grantSourceFake{v: openrtc.CaptureSourceView{ID: "call-source", Kinds: []openrtc.MediaKind{"audio"}, Owner: openrtc.ResourceRef{Kind: "call", ID: handle.Snapshot().ID}, State: "active"}}
	if defect == "capture-call-wrong-owner" {
		owned.v.Owner.Kind = "session"
	}
	publications := map[openrtc.PublicationID]openrtc.TrackPublication{}
	wrapper := &callMediaHangupFake{CurrentCall: handle, after: func() {
		for id, p := range publications {
			p.State = "unpublished"
			publications[id] = p
		}
		if defect != "capture-call-not-stopped" {
			owned.v.State = "stopped"
		}
		if defect == "call-stops-foreign" {
			foreign.v.State = "stopped"
		}
	}}
	return &harness.CallMediaProfileFixture{Call: wrapper, GrantedSource: foreign, CaptureCall: func(context.Context) openrtc.Async[openrtc.CaptureSource] {
		return openrtc.StartOperation(nil, func() (openrtc.CaptureSource, error) { return owned, nil })
	},
		PublishCall: func(_ context.Context, source openrtc.CaptureSource) openrtc.Async[openrtc.TrackPublication] {
			return openrtc.StartOperation(nil, func() (openrtc.TrackPublication, error) {
				p := openrtc.TrackPublication{ID: openrtc.PublicationID(newID("call-publication")), Owner: openrtc.ResourceRef{Kind: "call", ID: handle.Snapshot().ID}, Source: openrtc.KnownValue(source.Snapshot().ID), State: "published"}
				if defect == "call-session-owner" {
					p.Owner.Kind = "local-session"
				}
				publications[p.ID] = p
				return p, nil
			})
		},
		Publication: func(id openrtc.PublicationID) openrtc.TrackPublication { return publications[id] }, ConferenceEffects: func() int { return 0 },
	}
}

type callMediaHangupFake struct {
	openrtc.CurrentCall
	after func()
}

func (c *callMediaHangupFake) Hangup(ctx context.Context) openrtc.Async[openrtc.Void] {
	return completedVoid(func() error { _, e := c.CurrentCall.Hangup(ctx).Result(ctx); c.after(); return e })
}

// RFC §11.1:592, A.1:1241: Call ownership is not a synthetic Session.
func TestCallOnlyPublicationOwnership(t *testing.T) {
	for _, defect := range []string{"", "call-session-owner", "call-stops-foreign", "capture-call-wrong-owner", "capture-call-not-stopped"} {
		t.Run(defect, func(t *testing.T) {
			target := newCurrentMediaTarget(defect)
			target.fixture.CallMedia = callMediaProfileFake(defect)
			for _, scenario := range capability.CapabilityTests {
				if scenario.ID != "CT-SHAPE-001" {
					continue
				}
				report := harness.RunConformance(context.Background(), target, []harness.ConformanceTest{scenario}, harness.RunOptions{})
				want := harness.Pass
				if defect != "" {
					want = harness.Fail
				}
				if got := statusOf(report, scenario.ID); got != want {
					for _, r := range report.Results {
						if r.TestID == scenario.ID {
							t.Fatalf("%s want %s: %s", got, want, r.Reason)
						}
					}
				}
				return
			}
			t.Fatal("shape scenario unregistered")
		})
	}
}

func replacementProfileFake(defect string) *harness.SourceReplacementFixture {
	old := &grantSourceFake{v: openrtc.CaptureSourceView{ID: "old-source", Owner: openrtc.ResourceRef{Kind: "client", ID: "owner-A"}, State: "active", SourceDisableIntent: openrtc.KnownValue(false)}}
	replacement := &grantSourceFake{v: openrtc.CaptureSourceView{ID: "replacement", Owner: openrtc.ResourceRef{Kind: "client", ID: "owner-B"}, State: "active", SourceDisableIntent: openrtc.KnownValue(false)}}
	publication := openrtc.TrackPublication{ID: "replacement-target", Source: openrtc.KnownValue(old.v.ID), State: "published", LocalMuteIntent: openrtc.KnownValue(false), SetSendingMuteIntent: openrtc.KnownValue(false)}
	continuity := "established"
	partial := false
	effects := harness.MediaFixtureEffects{}
	return &harness.SourceReplacementFixture{Publication: publication.ID, OldSource: old, Replacement: replacement,
		SetMuted: func(_ context.Context, v bool) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error { publication.LocalMuteIntent = openrtc.KnownValue(v); return nil })
		},
		SetDisabled: func(_ context.Context, v bool) openrtc.Async[openrtc.Void] {
			return completedVoid(func() error {
				replacement.v.SourceDisableIntent = openrtc.KnownValue(v)
				if v {
					replacement.v.State = "disabled"
				} else {
					replacement.v.State = "active"
				}
				return nil
			})
		},
		SetContinuity: func(_ context.Context, v string) error { continuity = v; return nil }, PartialNext: func(context.Context) error { partial = true; return nil }, Effects: func() harness.MediaFixtureEffects { return effects }, Snapshot: func() openrtc.TrackPublication { return publication },
		Replace: func(_ context.Context, id openrtc.PublicationID, source openrtc.CaptureSourceID, requireIdentity bool, _ openrtc.OperationOptions) openrtc.Async[harness.ReplacementObservation] {
			return openrtc.StartOperation(nil, func() (harness.ReplacementObservation, error) {
				if requireIdentity && continuity == "unknown" && defect != "replacement-unknown-succeeds" {
					return harness.ReplacementObservation{}, fail(openrtc.Unsupported, "replaceSource", "required identity continuity unverified", failureExtras{})
				}
				effects.Mutations++
				if partial {
					partial = false
					e := fail(openrtc.UnknownCode, "replaceSource", "replacement partially applied", failureExtras{}).(*openrtc.RTCOperationError)
					e.Outcome = "partial"
					e.Details.KnownEffects = []openrtc.Effect{{Target: openrtc.EffectTarget{Resource: &openrtc.ResourceRef{Kind: "publication", ID: string(publication.ID)}}, Action: "replace", Certainty: "guaranteed"}}
					e.Details.UnknownEffects = []openrtc.Effect{{Target: openrtc.EffectTarget{Resource: &openrtc.ResourceRef{Kind: "publication", ID: string(publication.ID)}}, Action: "remote-replacement", Certainty: "unknown"}}
					e.Details.SurvivingResources = []openrtc.ResourceRef{{Kind: "capture", ID: string(old.v.ID)}, {Kind: "capture", ID: string(replacement.v.ID)}}
					if defect == "replacement-drops-survivors" {
						e.Details.SurvivingResources = nil
					}
					return harness.ReplacementObservation{}, e
				}
				publication.Source = openrtc.KnownValue(source)
				if defect == "replacement-clears-mute" {
					publication.LocalMuteIntent = openrtc.KnownValue(false)
				}
				return harness.ReplacementObservation{Publication: publication, Identity: "preserved", Atomic: true}, nil
			})
		},
	}
}

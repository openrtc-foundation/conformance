package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
	"github.com/openrtc/conformance/tests/core"
	"testing"
)

type mediaMutantTarget struct {
	*FakeTarget
	mutation string
}

func (t mediaMutantTarget) MediaContracts() harness.MediaContractControl {
	return mediaMutantControl{MediaContractControl: t.FakeTarget.MediaContracts(), mutation: t.mutation}
}

type mediaMutantControl struct {
	harness.MediaContractControl
	mutation string
}

func (c mediaMutantControl) Prepare(ctx context.Context, s openrtc.RTCConferenceSession) error {
	if e := c.MediaContractControl.Prepare(ctx, s); e != nil {
		return e
	}
	s.(*fakeSession).media.testMutation = c.mutation
	return nil
}

// RFC §4:23, §7.4:282/310, §7.1:86–88: unknown/nil false and missing
// unpublished populations cannot pass; consent is checked before extra effects.
func TestMediaOraclesRejectUnknownAndMissingEvidence(t *testing.T) {
	for _, tc := range []struct{ mutation, id string }{
		{"unknown-false", "CT-MOP-005"}, {"nil-false", "CT-MOP-005"}, {"omit-unpublished", "CT-SHAPE-002"}, {"effect-without-acceptance", "CT-EFFECT-001"},
		{"effect-accepted-noop", "CT-EFFECT-002"}, {"effect-wrong-acceptance", "CT-EFFECT-003"},
		{"effect-destroys-target", "CT-EFFECT-004"}, {"release-acquisition-unaccepted", "CT-EFFECT-005"},
		{"late-release-false-confirmation", "CT-EFFECT-006"},
	} {
		t.Run(tc.mutation, func(t *testing.T) {
			var scenario harness.ConformanceTest
			for _, candidate := range core.CoreTests {
				if candidate.ID == tc.id {
					scenario = candidate
				}
			}
			if scenario.Run == nil {
				t.Fatal("scenario unregistered")
			}
			for _, defective := range []bool{false, true} {
				target := mediaMutantTarget{FakeTarget: MakeFakeTarget(NoDefects)}
				want := harness.Pass
				if defective {
					target.mutation = tc.mutation
					want = harness.Fail
				}
				report := harness.RunConformance(context.Background(), target, []harness.ConformanceTest{scenario}, harness.RunOptions{})
				if got := statusOf(report, tc.id); got != want {
					for _, r := range report.Results {
						if r.TestID == tc.id {
							t.Fatalf("defective=%v got=%s want=%s: %s", defective, got, want, r.Reason)
						}
					}
				}
			}
		})
	}
}

// RFC §7.1:86–88, §7.4:310–323: every effect fixture must actually execute.
func TestEffectAcceptanceScenarios(t *testing.T) {
	for _, scenario := range core.EffectTests {
		t.Run(scenario.ID, func(t *testing.T) {
			report := harness.RunConformance(context.Background(), MakeFakeTarget(NoDefects), []harness.ConformanceTest{scenario}, harness.RunOptions{})
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

func (c mediaMutantControl) Kinds(ctx context.Context, s openrtc.RTCConferenceSession, operation string) ([]openrtc.MediaKind, error) {
	return c.MediaContractControl.(harness.MediaKindControl).Kinds(ctx, s, operation)
}

type singleKindTarget struct {
	*FakeTarget
	kind        openrtc.MediaKind
	publishOnly bool
}

func (t singleKindTarget) MediaContracts() harness.MediaContractControl {
	return singleKindControl{MediaContractControl: t.FakeTarget.MediaContracts(), target: t}
}

type singleKindControl struct {
	harness.MediaContractControl
	target singleKindTarget
}

func (c singleKindControl) Prepare(ctx context.Context, s openrtc.RTCConferenceSession) error {
	if e := c.MediaContractControl.Prepare(ctx, s); e != nil {
		return e
	}
	m := s.(*fakeSession).media
	m.mu.Lock()
	m.allowedKinds = []openrtc.MediaKind{c.target.kind}
	m.mu.Unlock()
	if c.target.publishOnly {
		s.(*fakeSession).capabilities.mu.Lock()
		s.(*fakeSession).capabilities.overrides["media.capture"] = "unsupported"
		s.(*fakeSession).capabilities.mu.Unlock()
	}
	return nil
}
func (c singleKindControl) Kinds(_ context.Context, _ openrtc.RTCConferenceSession, operation string) ([]openrtc.MediaKind, error) {
	if c.target.publishOnly && operation == "capture" {
		return nil, nil
	}
	return []openrtc.MediaKind{c.target.kind}, nil
}
func (c singleKindControl) EligibleSource(_ context.Context, s openrtc.RTCConferenceSession, kind openrtc.MediaKind) (openrtc.CaptureSource, error) {
	m := s.(*fakeSession).media
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.capture(openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{kind}, Ownership: "application"})
	src.view.State = "disabled"
	src.view.SourceDisableIntent = openrtc.KnownValue(true)
	return src, nil
}
func (c singleKindControl) ObserveOutput(ctx context.Context, s openrtc.RTCConferenceSession, id openrtc.CaptureSourceID, kind openrtc.MediaKind) (harness.MediaOutputEvidence, error) {
	return c.MediaContractControl.(harness.MediaOutputControl).ObserveOutput(ctx, s, id, kind)
}

// RFC §7.3:175–188, §7.4:245–293: neither audio-only nor video-only
// implies combined support; publish-only must not depend on capture support.
func TestPerKindMediaAndPublishOnlyFixtures(t *testing.T) {
	for _, kind := range []openrtc.MediaKind{"audio", "video"} {
		for _, id := range []string{"CT-MOP-001", "CT-MOP-002", "CT-MOP-005", "CT-MOP-011"} {
			t.Run(string(kind)+"/"+id, func(t *testing.T) {
				target := singleKindTarget{FakeTarget: MakeFakeTarget(NoDefects), kind: kind}
				for _, scenario := range core.CoreTests {
					if scenario.ID != id {
						continue
					}
					report := harness.RunConformance(context.Background(), target, []harness.ConformanceTest{scenario}, harness.RunOptions{})
					if got := statusOf(report, id); got != harness.Pass {
						for _, r := range report.Results {
							if r.TestID == id {
								t.Fatalf("%s: %s", got, r.Reason)
							}
						}
					}
					return
				}
				t.Fatal("scenario absent")
			})
		}
	}
	t.Run("publish-only", func(t *testing.T) {
		target := singleKindTarget{FakeTarget: MakeFakeTarget(NoDefects), kind: "audio", publishOnly: true}
		for _, scenario := range core.MediaPositiveTests {
			if scenario.ID != "CT-MEDIA-POS-002" {
				continue
			}
			report := harness.RunConformance(context.Background(), target, []harness.ConformanceTest{scenario}, harness.RunOptions{})
			if got := statusOf(report, scenario.ID); got != harness.Pass {
				for _, r := range report.Results {
					if r.TestID == scenario.ID {
						t.Fatalf("%s: %s", got, r.Reason)
					}
				}
			}
			return
		}
		t.Fatal("scenario absent")
	})
}
func TestMediaPositiveScenarios(t *testing.T) {
	for _, scenario := range core.MediaPositiveTests {
		t.Run(scenario.ID, func(t *testing.T) {
			report := harness.RunConformance(context.Background(), MakeFakeTarget(NoDefects), []harness.ConformanceTest{scenario}, harness.RunOptions{})
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

// RFC §7.0:45–65, §7.4:245–293: optional media is rejected before effects
// on a media-free Core; mandatory join/error operations remain usable.
func TestMediaFreeCoreRejectsOptionalEntrypointsBeforeEffects(t *testing.T) {
	target := MakeFakeTarget(NoDefects)
	ctx := context.Background()
	c := &harness.TestContext{Target: target, Descriptor: target.CreateAdapter().Descriptor(), Spy: harness.NewCredentialSpy(target.Credentials())}
	session, err := c.Joined(ctx, harness.JoinedOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := session.Session
	defer c.Cleanup(ctx)
	before := target.world.counts
	calls := []func() error{
		func() error {
			_, e := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"audio"}, Ownership: "session"}).Result(ctx)
			return e
		},
		func() error {
			_, e := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"video"}, Ownership: "session"}).Result(ctx)
			return e
		},
		func() error {
			_, e := s.Media().SetSending(ctx, openrtc.SendRequest{Publication: "unavailable", Sending: true}).Result(ctx)
			return e
		},
		func() error {
			_, e := s.Media().SetMuted(ctx, "unavailable", true, openrtc.OperationOptions{}).Result(ctx)
			return e
		},
		func() error {
			_, e := s.Media().SetDisabled(ctx, "unavailable", true, openrtc.OperationOptions{}).Result(ctx)
			return e
		},
	}
	for _, invoke := range calls {
		e := invoke()
		if e == nil {
			t.Fatal("media-free target accepted optional media")
		}
		if _, bad := c.ExpectError(e, harness.ExpectedFailure{}, "optional entrypoint"); bad != nil {
			t.Fatal(bad)
		}
	}
	after := target.world.counts
	if before.Captures != after.Captures || before.Publications != after.Publications {
		t.Fatal("unsupported media crossed effects")
	}
}

type exchangeTarget struct {
	*FakeTarget
	control exchangeControl
}

func (t exchangeTarget) MediaExchange() harness.MediaExchangeControl { return t.control }

type exchangeControl struct{ state string }

func (c exchangeControl) ClaimedBoundaries() []harness.MediaOutputEvidence {
	return []harness.MediaOutputEvidence{{Boundary: "selftest:peer", Source: "source", Kind: "audio", Direction: "send"}}
}
func (c exchangeControl) Exchange(_ context.Context, e harness.MediaOutputEvidence) (harness.MediaOutputEvidence, error) {
	e.Status = "observed"
	e.Content = "identifiable-tone-440"
	e.Evidence = "selftest synthetic peer only"
	e.Interval = "frame 1..10"
	if c.state == "unknown" {
		e.Status = "unknown"
	}
	if c.state == "metadata-only" {
		e.Content = ""
	}
	return e, nil
}

// RFC §14.1:1088–1098, §14.2:1108–1115: exchange claims need positive
// content evidence; missing observations are incomplete, metadata alone fails.
func TestMediaExchangeRequiresPositiveContentEvidence(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  harness.ConformanceStatus
	}{{"", harness.Pass}, {"unknown", harness.Incomplete}, {"metadata-only", harness.Fail}} {
		t.Run(tc.state, func(t *testing.T) {
			for _, scenario := range capability.CapabilityTests {
				if scenario.ID != "CT-MEDIA-EXCHANGE-001" {
					continue
				}
				report := harness.RunConformance(context.Background(), exchangeTarget{FakeTarget: MakeFakeTarget(NoDefects), control: exchangeControl{state: tc.state}}, []harness.ConformanceTest{scenario}, harness.RunOptions{})
				if got := statusOf(report, scenario.ID); got != tc.want {
					t.Fatalf("%s want %s", got, tc.want)
				}
				return
			}
			t.Fatal("exchange scenario absent")
		})
	}
}

package core

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func boundaryFixture(c *harness.TestContext) harness.BoundaryLossControl {
	t, ok := c.Target.(harness.BoundaryLossTarget)
	if !ok {
		must(c.NotRun("fixture provides no BoundaryLossTarget"))
	}
	f := t.BoundaryLoss()
	if f == nil {
		must(c.NotRun("fixture provides no boundary loss control"))
	}
	return f
}
func leakageFixture(c *harness.TestContext) harness.LeakageControl {
	t, ok := c.Target.(harness.LeakageTarget)
	if !ok {
		must(c.NotRun("fixture provides no LeakageTarget"))
	}
	f := t.Leakage()
	if f == nil {
		must(c.NotRun("fixture provides no leakage control"))
	}
	return f
}

// A native cause/cycle stays in process. Every serialization check below uses
// encoding/json directly: a serializer failure cannot mask a leak as success.
type mediaCanaryError struct {
	Message, URL, Key string
	Cause, Cycle      any
}

func (e *mediaCanaryError) Error() string { return e.Message }
func noMediaCanaries(c *harness.TestContext, v any, canaries []string) {
	b, err := json.Marshal(v)
	must(err)
	for _, canary := range canaries {
		check(c, !strings.Contains(string(b), canary), "public surface disclosed an injected credential or payload canary")
	}
}

func leakageCase(ctx context.Context, c *harness.TestContext, surface string) {
	f := leakageFixture(c)
	s := c.Open(client(ctx, c), value(c.Conference(ctx)))
	marker := c.FreshIdentity("secret").Subject
	canaries := []string{marker + "-token", marker + "-key", "https://credential.invalid/" + marker + "?token=" + marker, marker + "-message-payload"}
	native := &mediaCanaryError{Message: canaries[0], Key: canaries[1], URL: canaries[2]}
	native.Cause = &mediaCanaryError{Message: canaries[1]}
	native.Cycle = native
	var mu sync.Mutex
	var events []openrtc.EventEnvelope[any]
	stop := s.Observation().On("credentialChanged", func(v openrtc.EventEnvelope[any]) { mu.Lock(); events = append(events, v); mu.Unlock() })
	defer stop()
	must(f.Inject(ctx, s, canaries, native))
	// Safe defaults apply to the mandatory client/join error surface even for a
	// media-free Core. Optional profile surfaces have their own applicable cases.
	fault := value(c.RequireControl("failNextNativeOperation")).(func(string, any))
	fault("join", native)
	attempt := s.Join(opts(c))
	_, err := attempt.Result(ctx).Result(ctx)
	e := expect(c, err, []openrtc.ErrorCode{openrtc.UnknownCode}, "join canary failure")
	switch surface {
	case "error":
		check(c, e.Cause == nil && (e.ProviderError.Value == nil || e.ProviderError.Value.Original == nil), "default error exposed unrestricted native original (RFC §9:544)")
		noMediaCanaries(c, e, canaries)
		noMediaCanaries(c, e.ToJSON(), canaries)
		check(c, !strings.Contains(e.Error(), canaries[0]), "error string leaked native message")
		check(c, e.Operation.Name == "join", "failure lost known operation correlation")
	case "snapshot":
		noMediaCanaries(c, s.Observation().Snapshot(), canaries)
		mu.Lock()
		copyEvents := append([]openrtc.EventEnvelope[any](nil), events...)
		mu.Unlock()
		check(c, len(copyEvents) > 0, "no credential event observed")
		noMediaCanaries(c, copyEvents, canaries)
	case "inspection":
		noMediaCanaries(c, s.Inspection().Snapshot(), canaries)
		noMediaCanaries(c, value(s.Inspection().Operation(attempt.Result(ctx).OperationID())), canaries)
	case "logs":
		logs, err := f.DefaultLogs(ctx)
		if err != nil {
			must(c.NotRun("default logs unavailable: " + err.Error()))
		}
		noMediaCanaries(c, logs, canaries)
	case "extension":
		payload := s.Extension(f.DiagnosticKey())
		check(c, payload != nil, "diagnostic extension payload absent")
		noMediaCanaries(c, payload, canaries)
	}
}

var MediaAdversarialTests = []harness.ConformanceTest{
	mediaCase("CT-MADV-001", "required media boundary loss fails closed before outside fallback", "conformance.md Region guarantees becoming unverifiable", []string{"CFG-007", "CFG-008", "CFG-110", "CFG-111", "CFG-112", "CFG-121", "CFG-122", "CFG-125", "CAP-113", "ADP-124"}, func(ctx context.Context, c *harness.TestContext) {
		f := boundaryFixture(c)
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		pubs := mediaPublications(ctx, c, s, src)
		must(f.Establish(ctx, s))
		placement := s.Observation().Snapshot().Placement
		check(c, len(placement) > 0 && placement[0].Enforcement == "enforced" && placement[0].Requested.Strength == "required", "fixture lacks initially enforced required boundary")
		before := s.Capabilities().Snapshot().Revision
		fallbacks := f.FallbackEffects()
		var mu sync.Mutex
		var failures []any
		stop := s.Observation().On("error", func(e openrtc.EventEnvelope[any]) { mu.Lock(); failures = append(failures, e.Payload); mu.Unlock() })
		defer stop()
		must(f.Withdraw(ctx, s))
		mu.Lock()
		observed := append([]any(nil), failures...)
		mu.Unlock()
		check(c, len(observed) > 0, "boundary loss emitted no failure")
		value(c.ExpectError(observed[0], harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"placement-unsatisfied"}}, "required boundary loss"))
		check(c, f.FallbackEffects() == fallbacks, "outside-boundary fallback ran before fail-closed suspension")
		for _, p := range pubs {
			v := s.Observation().Snapshot().Publications[p.ID]
			check(c, v.State == "interrupted" || v.State == "unpublished" || mediaBool(v.EffectiveSuppression.Suppressed), "affected outbound activity remained enabled")
		}
		after := s.Capabilities().Snapshot()
		check(c, after.Revision > before, "placement loss did not invalidate descriptor revision")
		for _, d := range after.Entries {
			if d.ID == "media.send" {
				check(c, d.Support.Level != "supported", "unverifiable send guarantee still advertised")
			}
		}
		placement = s.Observation().Snapshot().Placement
		check(c, len(placement) > 0 && placement[0].Enforcement != "enforced" && placement[0].Effective.Status == "unknown", "placement retained invented enforcement")
		_, err := s.Media().SetSending(ctx, openrtc.SendRequest{Publication: pubs[0].ID, Sending: true}).Result(ctx)
		expect(c, err, []openrtc.ErrorCode{"placement-unsatisfied"}, "sending after enforcement loss")
	}),
	mediaCase("CT-MADV-002", "mandatory join errors keep native canaries off default public surfaces", "conformance.md Credential leakage; Original access without secret serialization", []string{"CFG-009", "ERR-106", "ERR-107", "ERR-111", "ERR-114", "IDL-106", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) { leakageCase(ctx, c, "error") }),
	mediaCase("CT-MADV-003", "credential event and application snapshot omit secret canaries", "conformance.md Credential leakage", []string{"CFG-113", "CFG-114", "CFG-115", "ERR-111", "IDL-106"}, func(ctx context.Context, c *harness.TestContext) { leakageCase(ctx, c, "snapshot") }),
	mediaCase("CT-MADV-004", "inspection and retained message results omit secret and payload canaries", "conformance.md Credential leakage", []string{"CFG-113", "CFG-114", "CFG-115", "EXT-111", "IDL-106", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) { leakageCase(ctx, c, "inspection") }),
	mediaCase("CT-MADV-005", "default logs redact credential canaries", "conformance.md Credential leakage; Original access without secret serialization", []string{"CFG-009", "ERR-107", "ERR-111", "ERR-114"}, func(ctx context.Context, c *harness.TestContext) { leakageCase(ctx, c, "logs") }),
	mediaCase("CT-MADV-006", "schema-valid adapter extension diagnostics redact credential canaries", "conformance.md Credential leakage", []string{"CFG-009", "CFG-113", "ERR-111", "EXT-111", "IDL-106"}, func(ctx context.Context, c *harness.TestContext) { leakageCase(ctx, c, "extension") }),
}

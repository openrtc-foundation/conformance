package selftest

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// failureTarget is a scoped synthetic public binding used only to validate the
// error/authority oracles. Its metadata cannot establish any provider boundary.
type failureTarget struct {
	*FakeTarget
	mutation string
}
type diagnosticAuthorization struct{}
type failureBinding struct {
	mu                 sync.Mutex
	scenario, mutation string
	observed           harness.FailureObservation
}

func (t failureTarget) FailureScenario(_ context.Context, name string) (harness.FailureScenario, error) {
	b := &failureBinding{scenario: name, mutation: t.mutation, observed: harness.FailureObservation{ForeignOwnerLive: true, ArtifactAuditReference: "selftest/fake_failure_contracts.go (synthetic binding source only)"}}
	return harness.FailureScenario{ExpectedResource: openrtc.ResourceRef{Kind: "session", ID: "controlled-session"}, PublicOperation: "syntheticBinding.execute(" + name + ")", Boundary: "in-memory synthetic service submission boundary", Invoke: b.Execute, Observe: b.Observe, Canary: "secret-diagnostic-canary-938721", DiagnosticsOffered: name == "secret-diagnostics", UnauthorizedDiagnosticContext: context.Background(), AuthorizedDiagnosticContext: context.WithValue(context.Background(), diagnosticAuthorization{}, true)}, nil
}
func (b *failureBinding) Observe(context.Context) (harness.FailureObservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.observed, nil
}

// Execute is this fake binding's public operation. It commits its independent
// synthetic service facts before completing a portable result, exactly once.
func (b *failureBinding) Execute(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.StartOperation(b, func() (openrtc.Void, error) {
		b.mu.Lock()
		defer b.mu.Unlock()
		resource := openrtc.ResourceRef{Kind: "session", ID: "controlled-session"}
		effect := openrtc.Effect{Target: openrtc.EffectTarget{Resource: &resource}, Action: "admit", Certainty: "possible"}
		d := openrtc.RTCErrorDetails{ID: openrtc.ErrorID(newID("controlled-error")), Code: "unknown", Reasons: []string{"unknown"}, Outcome: "none", Operation: openrtc.OperationRef{Name: "join"}, AdapterID: openrtc.KnownValue(adapterID), Retryability: "never", ProviderError: openrtc.Unknown[openrtc.ProviderError]()}
		switch b.scenario {
		case "outcome-none":
			d.Code = "invalid-argument"
		case "outcome-partial":
			b.observed.Effects = 1
			d.Outcome = "partial"
			d.Details.KnownEffects = []openrtc.Effect{effect}
			d.Details.SurvivingResources = []openrtc.ResourceRef{resource}
			d.Details.UnstartedSteps = []string{"initial media"}
		case "outcome-unknown", "unknown-submission":
			d.Outcome = "unknown"
			d.Details.UnknownEffects = []openrtc.Effect{effect}
			b.observed.Submissions = 1
		case "simultaneous-causes":
			d.Code = "permission-denied"
			d.Reasons = []string{"authorization-denied", "network-failure"}
		case "rejected-options-conditions":
			d.Code = "unsupported"
			d.Details.RejectedOptions = []openrtc.RejectedOption{{Path: "join.media.kind", Operation: "join", Problem: "unsupported-semantics"}}
			d.Details.RejectedConditions = []openrtc.RejectedCondition{{Condition: openrtc.Condition{Name: "required-placement"}, Assessment: "pending", Resolution: &openrtc.Resolution{Mechanism: "policy decision callback", EvaluationBoundary: "before admission"}}}
		case "primary-and-cleanup":
			d.Code = "unavailable"
			d.Reasons = []string{"network-failure"}
			d.Outcome = "partial"
			d.Details.UnknownEffects = []openrtc.Effect{effect}
			d.Details.CleanupFailures = []openrtc.ResourceEffect{{Resource: resource, Effect: "cleanup-failure"}}
		case "secret-diagnostics":
			d.Outcome = "unknown"
			cycle := map[string]any{"token": "secret-diagnostic-canary-938721"}
			cycle["cycle"] = cycle
			d.Cause = cycle
			d.ProviderError = openrtc.KnownValue(openrtc.ProviderError{Original: cycle})
			d.DiagnosticAccess = func(ctx context.Context) (any, error) {
				if ctx.Value(diagnosticAuthorization{}) != true {
					return nil, errors.New("diagnostic access denied")
				}
				return cycle, nil
			}
			b.observed.DefaultSurfaces = []any{map[string]any{"state": "disconnected", "errorCode": "unknown"}, "safe default log"}
		default:
			d.Code = "permission-denied"
			d.Reasons = []string{"authorization-denied"}
			if strings.Contains(b.scenario, "unknown") {
				d.Code = "unsupported"
				d.Reasons = []string{"unknown"}
			}
		}
		e := openrtc.NewRTCOperationError("controlled public operation failed safely", d)
		switch b.mutation {
		case "omit-outcome":
			e.Outcome = ""
		case "unknown-as-none":
			if e.Outcome == "unknown" {
				e.Outcome = "none"
				e.Details.UnknownEffects = nil
			}
		case "omit-reasons":
			e.Reasons = nil
		case "empty-rejections":
			e.Details.RejectedOptions = nil
			e.Details.RejectedConditions = nil
		case "hide-cleanup":
			e.Details.CleanupFailures = nil
		case "automatic-retry":
			if b.scenario == "unknown-submission" {
				b.observed.Submissions++
			}
		case "leak-original":
			if b.scenario == "secret-diagnostics" {
				e.Cause = map[string]any{"nested": "secret-diagnostic-canary-938721"}
			}
		case "ambient-authority":
			b.observed.ProtectedEffects++
		}
		return openrtc.Void{}, e
	})
}

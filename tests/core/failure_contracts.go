package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func failureScenario(ctx context.Context, c *harness.TestContext, name string) harness.FailureScenario {
	target, ok := c.Target.(harness.FailureContractTarget)
	if !ok {
		must(c.NotRun("controlled public failure/authority fixture unavailable: " + name))
	}
	fixture := value(target.FailureScenario(ctx, name))
	if fixture.Invoke == nil || fixture.Observe == nil || fixture.PublicOperation == "" || fixture.Boundary == "" {
		must(c.NotRun("public mapping or independent boundary observation missing: " + name))
	}
	return fixture
}
func invokeFailure(ctx context.Context, c *harness.TestContext, fixture harness.FailureScenario) *openrtc.RTCFailure {
	_, err := fixture.Invoke(ctx).Result(ctx)
	return value(c.ExpectError(err, harness.ExpectedFailure{}, fixture.PublicOperation))
}
func effectCovers(effects []openrtc.Effect, r openrtc.ResourceRef) bool {
	for _, effect := range effects {
		if effect.Target.Resource != nil && *effect.Target.Resource == r && effect.Action != "" {
			return true
		}
	}
	return false
}
func survivorCovers(resources []openrtc.ResourceRef, r openrtc.ResourceRef) bool {
	for _, resource := range resources {
		if resource == r {
			return true
		}
	}
	return false
}
func reasonPresent(c *harness.TestContext, e *openrtc.RTCFailure, reason string) {
	check(c, slices.Contains(e.Reasons, reason), "missing established reason "+reason)
}

// Every case invokes an identified real public operation under a controlled
// established condition, then examines its default error and covered boundary.
var FailureContractTests = []harness.ConformanceTest{
	boundTest("CT-ERROR-001", "none, partial and unknown outcomes preserve established effects, survivors and unstarted steps", []string{"RFC §9:552–570", "RFC A.1:1217–1227"}, func(ctx context.Context, c *harness.TestContext) {
		for _, outcome := range []string{"none", "partial", "unknown"} {
			f := failureScenario(ctx, c, "outcome-"+outcome)
			e := invokeFailure(ctx, c, f)
			observed := value(f.Observe(ctx))
			check(c, e.Outcome == outcome, "failure outcome contradicts established fixture facts")
			switch outcome {
			case "none":
				check(c, observed.Effects == 0 && len(e.Details.KnownEffects) == 0, "none erased attributable effects")
			case "partial":
				check(c, observed.Effects > 0 && effectCovers(e.Details.KnownEffects, f.ExpectedResource) && survivorCovers(e.Details.SurvivingResources, f.ExpectedResource) && len(e.Details.UnstartedSteps) > 0, "partial effects lost survivor or unstarted step")
			case "unknown":
				check(c, effectCovers(e.Details.UnknownEffects, f.ExpectedResource), "unestablished remote effects lack requested resource coverage")
			}
		}
	}),
	boundTest("CT-ERROR-002", "simultaneous established causes retain every reason without imposing precedence", []string{"RFC §9:546–548"}, func(ctx context.Context, c *harness.TestContext) {
		e := invokeFailure(ctx, c, failureScenario(ctx, c, "simultaneous-causes"))
		check(c, e.Code == "permission-denied" || e.Code == "unavailable", "no established condition supports returned code")
		reasonPresent(c, e, "authorization-denied")
		reasonPresent(c, e, "network-failure")
	}),
	boundTest("CT-ERROR-003", "rejected requested options and pending conditions retain exact operation paths and resolution boundaries", []string{"RFC §9:550–560", "RFC A.1:1214–1227"}, func(ctx context.Context, c *harness.TestContext) {
		e := invokeFailure(ctx, c, failureScenario(ctx, c, "rejected-options-conditions"))
		check(c, len(e.Details.RejectedOptions) == 1 && len(e.Details.RejectedConditions) == 1, "requested rejection observations missing")
		o := e.Details.RejectedOptions[0]
		check(c, o.Path == "join.media.kind" && o.Operation == "join" && o.Problem == "unsupported-semantics", "rejected option lost complete path, operation or problem")
		condition := e.Details.RejectedConditions[0]
		check(c, condition.Condition.Name == "required-placement" && condition.Assessment == "pending" && condition.Resolution != nil && condition.Resolution.Mechanism != "" && condition.Resolution.EvaluationBoundary != "", "pending required condition lacks identifiable active resolution")
	}),
	boundTest("CT-ERROR-004", "initiating failure remains distinct from later cleanup failure", []string{"RFC §8.6:510", "RFC §9:570"}, func(ctx context.Context, c *harness.TestContext) {
		f := failureScenario(ctx, c, "primary-and-cleanup")
		e := invokeFailure(ctx, c, f)
		check(c, e.Code == "unavailable", "cleanup failure replaced initiating failure")
		reasonPresent(c, e, "network-failure")
		check(c, len(e.Details.CleanupFailures) > 0 && effectCovers(e.Details.UnknownEffects, f.ExpectedResource), "separate failed cleanup or continuing uncertainty missing")
		check(c, e.Details.CleanupFailures[0].Resource == f.ExpectedResource, "cleanup failure names an unrelated resource")
	}),
	boundTest("CT-ERROR-005", "unknown remote submission does not trigger duplication-prone retry without idempotency evidence", []string{"RFC §9:562–570"}, func(ctx context.Context, c *harness.TestContext) {
		f := failureScenario(ctx, c, "unknown-submission")
		e := invokeFailure(ctx, c, f)
		check(c, e.Outcome == "unknown" && len(e.Details.UnknownEffects) > 0, "unobserved remote submission treated as absent")
		must(c.Settle(ctx))
		o := value(f.Observe(ctx))
		check(c, o.Submissions == 1, "implementation retried ambiguous submission without idempotency evidence")
	}),
	boundTest("CT-SECRET-001", "default error surfaces remain safe and unrestricted diagnostics require explicit authorized access", []string{"RFC §9:544", "RFC §12.1:1044–1046", "RFC A.1:1220–1227"}, func(ctx context.Context, c *harness.TestContext) {
		f := failureScenario(ctx, c, "secret-diagnostics")
		if f.Canary == "" {
			must(c.NotRun("diagnostic canary fixture unavailable"))
		}
		e := invokeFailure(ctx, c, f)
		o := value(f.Observe(ctx))
		surfaces := append([]any{e, fmt.Sprintf("%+v", e), e.Error(), harness.SafeJSON(e)}, o.DefaultSurfaces...)
		for _, surface := range surfaces {
			where := harness.FindLeak(surface, []string{f.Canary})
			check(c, where == nil, "unrestricted canary exposed on default surface")
			if text, ok := surface.(string); ok {
				check(c, !strings.Contains(text, f.Canary), "formatted diagnostic leaked canary")
			}
		}
		if f.DiagnosticsOffered {
			if f.UnauthorizedDiagnosticContext == nil || f.AuthorizedDiagnosticContext == nil {
				must(c.NotRun("diagnostic authorization fixture unavailable"))
			}
			denied, err := e.RequestDiagnostics(f.UnauthorizedDiagnosticContext)
			check(c, err != nil && denied == nil, "unrestricted diagnostics available without authorization")
			original, err := e.RequestDiagnostics(f.AuthorizedDiagnosticContext)
			must(err)
			check(c, harness.FindLeak(original, []string{f.Canary}) != nil, "authorized explicit diagnostic access did not deliver fixture payload")
		}
	}),
}

var AuthorityContractTests = []harness.ConformanceTest{
	boundTest("CT-AUTH-001", "known credential claim mismatches and unknown required match reject before protected effects", []string{"RFC §6:33", "RFC §7.1:69–84", "RFC §12.2:1048–1058"}, func(ctx context.Context, c *harness.TestContext) {
		for _, claim := range []string{"audience", "subject", "provider", "purpose", "destination", "unknown-required-match"} {
			f := failureScenario(ctx, c, "credential-"+claim)
			e := invokeFailure(ctx, c, f)
			check(c, e.Code == "invalid-argument" || e.Code == "permission-denied" || e.Code == "authentication-failed" || e.Code == "unsupported", "credential mismatch returned an inapplicable code")
			check(c, value(f.Observe(ctx)).ProtectedEffects == 0, "credential mismatch allowed protected submission")
		}
	}),
	boundTest("CT-AUTH-002", "denied or unknown required authority and consent fail before their protected effects", []string{"RFC §6:33", "RFC §7.1:69–84", "RFC §12.3:1060–1064"}, func(ctx context.Context, c *harness.TestContext) {
		for _, condition := range []string{"authority-denied", "authority-unknown", "consent-denied", "consent-unknown"} {
			f := failureScenario(ctx, c, condition)
			e := invokeFailure(ctx, c, f)
			if strings.HasSuffix(condition, "denied") {
				check(c, e.Code == "permission-denied", "known denial was not permission-denied")
			}
			check(c, value(f.Observe(ctx)).ProtectedEffects == 0, "denied or unknown required authorization allowed effects")
		}
	}),
	boundTest("CT-AUTH-003", "capturing consent does not grant publication or recording authority and discovery grants no authority", []string{"RFC §6:33–35", "RFC §7.3:173–186", "RFC §12.3:1060–1064"}, func(ctx context.Context, c *harness.TestContext) {
		for _, condition := range []string{"capture-consent-only", "ungranted-foreign-source", "authority-changed-after-discovery"} {
			f := failureScenario(ctx, c, condition)
			e := invokeFailure(ctx, c, f)
			check(c, e.Code == "permission-denied" || e.Code == "unsupported", "ungranted protected operation was accepted or misclassified")
			check(c, value(f.Observe(ctx)).ProtectedEffects == 0, "discovery/capture consent conferred publication authority")
		}
	}),
	boundTest("CT-AUTH-004", "client credentials cannot confer trusted-server administration or expose trusted-server secrets", []string{"RFC §12.1:1044–1046", "RFC §14.2:1115"}, func(ctx context.Context, c *harness.TestContext) {
		f := failureScenario(ctx, c, "client-administration")
		e := invokeFailure(ctx, c, f)
		check(c, e.Code == "permission-denied" || e.Code == "unsupported", "client credentials obtained trusted-server authority")
		o := value(f.Observe(ctx))
		check(c, o.ProtectedEffects == 0 && o.ForeignOwnerLive, "client administration affected another owner")
		if o.ArtifactAuditReference == "" {
			c.Partial("published client artifact audit absent; runtime rejection does not establish secret-free published artifact")
		}
	}),
}

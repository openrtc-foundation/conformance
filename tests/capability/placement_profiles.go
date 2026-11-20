package capability

import (
	"context"
	"slices"
	"time"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func placementFixture(ctx context.Context, c *harness.TestContext) harness.PlacementScenario {
	t, ok := c.Target.(harness.PlacementScenarioTarget)
	if !ok {
		must(c.NotRun("actual placement operations and exact boundary witness unavailable"))
	}
	f := value(t.PlacementScenario(ctx))
	check(c, f.Documentation != "" && f.Scope != "" && f.Activity != "" && f.Direction != "" && f.Boundary != "" && len(f.Regions) >= 2, "placement fixture lacks scoped request/evidence setup")
	check(c, f.Control != nil, "placement fixture has no actual operations")
	return f
}
func placementEntries(f harness.PlacementScenario) []openrtc.PlacementEntry {
	return []openrtc.PlacementEntry{{ID: "required-current", Scope: f.Scope, Activities: []string{f.Activity}, AllowedRegions: f.Regions, Transit: true, Fallback: true, Requirement: "required"}, {ID: "required-inherited", Scope: f.Scope, Activities: []string{f.Activity}, AllowedRegions: f.Regions[:1], Transit: true, Fallback: true, Requirement: "required"}, {ID: "prefer-other", Scope: f.Scope, Activities: []string{f.Activity}, AllowedRegions: f.Regions[1:2], Transit: true, Fallback: true, Requirement: "preference"}}
}
func requirePreferenceCoverage(c *harness.TestContext, entries []openrtc.PlacementEntry, result openrtc.CurrentPlacementResult) {
	for _, entry := range entries {
		if entry.Requirement != "preference" {
			continue
		}
		found := 0
		for _, p := range result.Preferences {
			if p.RequestID != entry.ID {
				continue
			}
			found++
			check(c, slices.Contains([]string{"selected", "not-selected", "unknown"}, p.Selection), "invalid preference selection")
			if p.Selection == "not-selected" {
				check(c, p.Reason != "", "unselected preference has no reason")
			}
		}
		check(c, found == 1, "requested placement preference omitted, duplicated or uncorrelated")
	}
}
func exactPlacementEvidence(c *harness.TestContext, f harness.PlacementScenario, r openrtc.CurrentPlacementResult) {
	check(c, r.Assessment == "supported" && r.State == "verified", "successful applyPlacement did not establish enforcement")
	check(c, r.Boundary.Status == "known" && r.Boundary.Value != nil && *r.Boundary.Value == f.Boundary, "placement result verified an unrelated or unknown boundary")
	covered := false
	for _, e := range r.Evidence {
		if e.Boundary == f.Boundary && e.Activity == f.Activity && e.Direction == f.Direction {
			check(c, e.Mechanism != "" && e.Authority != "" && e.Source != "" && e.Version != "" && e.ValidUntil != "", "exact placement evidence lacks mechanism/authority/source/version/validity")
			expiry, err := time.Parse(time.RFC3339, string(e.ValidUntil))
			check(c, err == nil && time.Now().Before(expiry), "placement evidence expired or invalid")
			covered = true
		}
	}
	check(c, covered, "no current evidence for exact requested placement boundary/activity/direction")
}
func placementError(c *harness.TestContext, err error, label string, assessment string, fixture harness.PlacementScenario) *openrtc.RTCFailure {
	failure, e := c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"unsupported"}}, label)
	must(e)
	check(c, len(failure.Details.RejectedConditions) > 0, "placement rejection omitted required condition assessment")
	found := false
	for _, r := range failure.Details.RejectedConditions {
		if r.Assessment == assessment {
			found = true
			check(c, r.Condition.Name != "", "rejected placement condition unidentified")
			check(c, r.Condition.Parameters["scope"] == fixture.Scope && r.Condition.Parameters["boundary"] == fixture.Boundary, "placement rejection does not identify requested scope/boundary")
			if assessment == "pending" {
				check(c, r.Resolution != nil && r.Resolution.Mechanism != "" && r.Resolution.EvaluationBoundary != "", "pending condition lacks resolution mechanism/boundary")
			}
		}
	}
	check(c, found, "placement rejection does not report actual required assessment")
	return failure
}

var PlacementProfileTests = []harness.ConformanceTest{
	currentProfileTest("CT-PLC-PROFILE-001", "required placement intersections exclude preferences and assessment has no protected effects", "placement", "placement.profile", []string{"RFC §11.7:831–845", "RFC A.3:1386–1391"}, func(ctx context.Context, c *harness.TestContext) {
		f := placementFixture(ctx, c)
		if f.Effects == nil {
			must(c.NotRun("assessment protected-effect boundary unobserved"))
		}
		entries := placementEntries(f)
		before := f.Effects.Counts()
		assessment := value(f.Control.AssessPlacement(ctx, f.Scope, entries).Result(ctx))
		check(c, assessment.Assessment == "supported", "disjoint preference incorrectly weakened required intersection")
		check(c, assessment.State != "verified", "capability assessment falsely claimed current enforcement")
		requirePreferenceCoverage(c, entries, assessment)
		check(c, f.Effects.Counts() == before, "assessPlacement caused protected acquisition/admission effects")
		applied := value(f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx))
		requirePreferenceCoverage(c, entries, applied)
		exactPlacementEvidence(c, f, applied)
	}),
	currentProfileTest("CT-PLC-PROFILE-002", "empty required intersection namespace mismatch and unknown conditions fail before effects", "placement", "placement.profile", []string{"RFC §11.7:845–865", "RFC §9:551–558"}, func(ctx context.Context, c *harness.TestContext) {
		f := placementFixture(ctx, c)
		if f.Effects == nil || f.SetRequiredAssessment == nil {
			must(c.NotRun("required condition/effect controls unavailable"))
		}
		entries := placementEntries(f)[:2]
		entries[0].AllowedRegions = f.Regions[:1]
		entries[1].AllowedRegions = f.Regions[1:2]
		before := f.Effects.Counts()
		_, err := f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx)
		placementError(c, err, "empty intersection", "unsupported", f)
		check(c, f.Effects.Counts() == before, "empty intersection acquired dependent resources")
		entries = placementEntries(f)[:2]
		foreign := entries[1]
		foreign.AllowedRegions = []openrtc.RegionRef{{Namespace: "unmapped-foreign.example", Code: f.Regions[0].Code}}
		entries[1] = foreign
		_, err = f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx)
		placementError(c, err, "unmapped region namespace", "unsupported", f)
		check(c, f.Effects.Counts() == before, "foreign namespace treated as equivalent")
		for _, state := range []string{"unknown", "pending"} {
			f.SetRequiredAssessment(state)
			entries = placementEntries(f)
			_, err = f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx)
			placementError(c, err, "required "+state, state, f)
			check(c, f.Effects.Counts() == before, "unresolved required condition caused dependent effects")
		}
	}),
	currentProfileTest("CT-PLC-PROFILE-003", "recovery retains exact enforcement and verification loss prevents outside fallback", "placement", "placement.profile", []string{"RFC §11.7:849–873", "RFC §14.2:1111–1115"}, func(ctx context.Context, c *harness.TestContext) {
		f := placementFixture(ctx, c)
		if f.Recover == nil || f.WithdrawVerification == nil || f.Snapshot == nil || f.OutsideEffects == nil {
			must(c.NotRun("recovery/verification-loss boundary evidence unavailable"))
		}
		entries := placementEntries(f)
		r := value(f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx))
		exactPlacementEvidence(c, f, r)
		before := f.OutsideEffects()
		must(f.Recover(ctx))
		exactPlacementEvidence(c, f, f.Snapshot())
		must(f.WithdrawVerification(ctx))
		requireCheckpoint(ctx, c, f.Checkpoint)
		lost := f.Snapshot()
		check(c, lost.State == "unverified" || lost.State == "violated", "lost verification remained verified")
		check(c, lost.Boundary.Status == "known" && lost.Boundary.Value != nil && *lost.Boundary.Value == f.Boundary, "verification loss omitted affected boundary")
		check(c, f.OutsideEffects() == before, "lost verification permitted out-of-bound protected fallback")
	}),
	currentProfileTest("CT-PLC-PROFILE-004", "supported runtime placement preserves session and exact enforcement", "placement", "placement.profile", []string{"RFC §8.8:521–525", "RFC §11.7:849–873"}, func(ctx context.Context, c *harness.TestContext) {
		f := placementFixture(ctx, c)
		if !f.RuntimeApplySupported {
			must(c.NotApplicable("fixture documents no supported runtime apply boundary"))
		}
		if f.SessionIncarnation == nil || f.Effects == nil {
			must(c.NotRun("runtime identity/effect evidence unavailable"))
		}
		entries := placementEntries(f)
		value(f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx))
		identity := f.SessionIncarnation()
		before := f.Effects.Counts()
		r := value(f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx))
		exactPlacementEvidence(c, f, r)
		requirePreferenceCoverage(c, entries, r)
		check(c, f.SessionIncarnation() == identity, "runtime placement silently recreated/rejoined session")
		after := f.Effects.Counts()
		check(c, after.Admissions == before.Admissions && after.Captures == before.Captures && after.Publications == before.Publications, "runtime placement silently rejoined or reacquired media")
	}),
	currentProfileTest("CT-COMB-001", "individually supported placement entries do not prove combined support", "placement", "placement.profile", []string{"RFC §7.3:180", "RFC §11.7:835/851", "RFC §8.8:525"}, func(ctx context.Context, c *harness.TestContext) {
		f := placementFixture(ctx, c)
		if f.SetCombinationAssessment == nil || f.Effects == nil {
			must(c.NotRun("complete-combination applicability control unavailable"))
		}
		entries := placementEntries(f)[:2]
		for _, entry := range entries {
			r := value(f.Control.AssessPlacement(ctx, f.Scope, []openrtc.PlacementEntry{entry}).Result(ctx))
			check(c, r.Assessment == "supported", "fixture individual prerequisite not supported")
		}
		before := f.Effects.Counts()
		for _, state := range []string{"unsupported", "unknown"} {
			f.SetCombinationAssessment(state)
			r := value(f.Control.AssessPlacement(ctx, f.Scope, entries).Result(ctx))
			check(c, r.Assessment == state, "combined assessment inferred from individual support")
			_, err := f.Control.ApplyPlacement(ctx, f.Scope, entries, openrtc.OperationOptions{}).Result(ctx)
			placementError(c, err, "combined required assessment", state, f)
			check(c, f.Effects.Counts() == before, "unsupported/unknown combined request had dependent effects")
		}
	}),
}

func init() { CapabilityTests = append(CapabilityTests, PlacementProfileTests...) }

package harness

import (
	"context"
	"github.com/openrtc/conformance/inventory"
	"github.com/openrtc/conformance/openrtc"
	"strings"
	"testing"
	"time"
)

// RFC §§14.1:1088–1098,14.2:1102–1115: catalog/filter mutations must
// not create a claim, optional positives remain independently applicable.
func TestInventoryDrivenClaimMutants(t *testing.T) {
	good := []TestResult{{TestID: "core", Suite: Core, Status: Pass}, {TestID: "media", Suite: Capability, Status: Pass}}
	required := []string{"core", "media"}
	if a := AssessClaim(RealProvider, good, required, true); a.Status != Pass {
		t.Fatal(a)
	}
	for _, tc := range []struct {
		name    string
		results []TestResult
		want    ConformanceStatus
	}{
		{"capability-fails", []TestResult{good[0], {TestID: "media", Suite: Capability, Status: Fail}}, Fail},
		{"capability-omitted", good[:1], Incomplete},
		{"unknown", []TestResult{good[0], {TestID: "media", Status: Unknown}}, Incomplete},
		{"not-run", []TestResult{good[0], {TestID: "media", Status: NotRun}}, Incomplete},
		{"unjustified-exclusion", []TestResult{good[0], {TestID: "media", Status: NotApplicable, Reason: "optional"}}, Incomplete},
		{"verified-exclusion", []TestResult{good[0], {TestID: "media", Status: NotApplicable, Reason: "unsupported exact scope", ExclusionEvidence: "public descriptor revision 9 session S audio"}}, Pass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if a := AssessClaim(RealProvider, tc.results, required, true); a.Status != tc.want {
				t.Fatal(a)
			}
		})
	}
	r := RunConformance(context.Background(), testTarget{}, []ConformanceTest{{ID: "core", Suite: Core, Run: func(context.Context, *TestContext) error { return nil }}}, RunOptions{})
	if r.Claim.Status != Incomplete || len(r.Results) < 2 {
		t.Fatal("one-test default slice hid omitted inventory", r.Claim)
	}
	found := false
	for _, result := range r.Results {
		if strings.HasPrefix(result.TestID, "OR-") && result.Status == Incomplete {
			found = true
		}
	}
	if !found {
		t.Fatal("missing assertions not materialized")
	}
}

// RFC §§14.2:1111–1115,12.3:1064: current exact-boundary provenance is
// separate from runtime successes and from a metadata evidence label.
func TestBoundaryEvidenceMutants(t *testing.T) {
	now := time.Now()
	b := BoundaryEvidence{Activity: "processing", Direction: "both", Mechanism: "audited enforcement", Authority: "service", Boundary: "service processing exact region A", Reference: "audit://current", Version: "v1", ValidUntil: now.Add(time.Hour), Origin: RealProvider}
	c := ClaimDeclaration{Core: true, SpecRevision: "amended", SpecSHA256: "hash", InventoryRevision: "inventory", ArtifactRevision: "artifact", ImplementationRevision: "impl", BindingRevision: "binding", EnvironmentRevision: "env", ArtifactAudit: &b, Boundaries: []BoundaryEvidence{b}}
	if s, r := evidenceAssessment(c, RealProvider, now); s != Pass {
		t.Fatal(s, r)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*BoundaryEvidence)
		want   ConformanceStatus
	}{
		{"expired", func(b *BoundaryEvidence) { b.ValidUntil = now }, Incomplete},
		{"withdrawn", func(b *BoundaryEvidence) { b.Withdrawn = true }, Incomplete},
		{"contradicted", func(b *BoundaryEvidence) { b.Contradicted = true }, Incomplete},
		{"fake-origin", func(b *BoundaryEvidence) { b.Origin = HarnessSelfTest }, Incomplete},
		{"established-violation", func(b *BoundaryEvidence) { b.Violation = true }, Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := b
			tc.mutate(&v)
			x := c
			x.Boundaries = []BoundaryEvidence{v}
			if s, r := evidenceAssessment(x, RealProvider, now); s != tc.want {
				t.Fatal(s, r)
			}
		})
	}
	c.ArtifactAudit = nil
	if s, _ := evidenceAssessment(c, RealProvider, now); s != Incomplete {
		t.Fatal("missing artifact audit passed")
	}
}

// RFC §7.0:45–65 and §14.1:1092–1098: no-media is a valid Core scope;
// claimed media adds positives, while unsupported entry/security tests remain.
func TestNoMediaCoreApplicability(t *testing.T) {
	c := ClaimDeclaration{Core: true}
	for _, a := range []string{"core", "operation-entry"} {
		if !obligationApplies(inventory.Entry{Applicability: a}, c) {
			t.Fatal(a)
		}
	}
	e := inventory.Entry{Applicability: "supported-operation", Operations: []string{"capture"}}
	if obligationApplies(e, c) {
		t.Fatal("media mandatory on no-media Core")
	}
	c.Operations = []OperationClaim{{Operation: "capture", Kinds: []string{"video"}, Scope: "session", Support: "supported"}}
	if !obligationApplies(e, c) {
		t.Fatal("claimed positive omitted")
	}
}

type scopedClaimTestTarget struct {
	testTarget
	claim ClaimDeclaration
	probe func(CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool)
}

func (t scopedClaimTestTarget) ClaimDeclaration() ClaimDeclaration { return t.claim }
func (t scopedClaimTestTarget) CapabilityFor(_ context.Context, d CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool) {
	return t.probe(d)
}

// RFC §§7.0:45–65,7.3:173–186,14.1:1092–1098: optional positives use
// actual exact-scope capability evidence; legacy labels never suppress Core.
func TestScopedCapabilityClaimEvaluation(t *testing.T) {
	for _, kind := range []string{"audio", "video"} {
		t.Run(kind, func(t *testing.T) {
			called := false
			target := scopedClaimTestTarget{claim: ClaimDeclaration{Core: true}, probe: func(d CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool) {
				if d.Operation != "capture" || len(d.Kinds) != 1 || d.Kinds[0] != kind || d.OwnerScope != "session" {
					t.Error("lost exact query", d)
				}
				return []openrtc.CapabilityDescriptor{{ID: "media.capture", Scope: openrtc.CapabilityScope{Resource: "session"}, Support: openrtc.SupportDescription{Level: "supported"}}}, true
			}}
			test := ConformanceTest{ID: "positive", Suite: Capability, Profile: Calling, Requirements: []string{"RFC §7.4:243–291"}, Capability: &CapabilityDependence{ID: "media.capture", Operation: "capture", Kinds: []string{kind}, OwnerScope: "session"}, Run: func(context.Context, *TestContext) error { called = true; return nil }}
			r := RunConformance(context.Background(), target, []ConformanceTest{test}, RunOptions{})
			if !called || r.Results[0].Status != Pass {
				t.Fatal(r.Results[0])
			}
		})
	}
	combined := ConformanceTest{Capability: &CapabilityDependence{ID: "media.capture", Kinds: []string{"audio", "video"}, OwnerScope: "session"}}
	d := []openrtc.CapabilityDescriptor{{ID: "media.capture", Scope: openrtc.CapabilityScope{Resource: "session"}, Support: openrtc.SupportDescription{Level: "partial", Subset: []string{"audio", "video"}}}}
	if s, _ := capabilityGate(combined, d, true); s != Unknown {
		t.Fatal("individual support invented joint support")
	}
	d[0].Scope.Resource = "provider"
	d[0].Support.Level = "unknown"
	if s, _ := capabilityGate(combined, d, true); s != Unknown {
		t.Fatal("provider unknown settled session support")
	}
	profileTarget := scopedClaimTestTarget{claim: ClaimDeclaration{Core: true, Profiles: []ProfileClaim{{Name: "calling", Version: 1}}}, probe: func(CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool) {
		return []openrtc.CapabilityDescriptor{{ID: "calling.dial", Support: openrtc.SupportDescription{Level: "unsupported"}}}, true
	}}
	positive := ConformanceTest{ID: "dial", Suite: Capability, Capability: &CapabilityDependence{ID: "calling.dial"}, Run: func(context.Context, *TestContext) error { t.Fatal("unsupported positive invoked"); return nil }}
	r := RunConformance(context.Background(), profileTarget, []ConformanceTest{positive}, RunOptions{})
	if r.Results[0].Status != Fail || r.Claim.Status != Fail {
		t.Fatal("unsupported defining operation passed", r.Claim.Status)
	}
}

// RFC §§14.1:1088–1098,14.2:1102–1115: unresolved applicability,
// empty scopes and undefined profiles must not turn a subset into a claim.
func TestClaimApplicabilityAuditMutants(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b := BoundaryEvidence{Activity: "artifact audit", Direction: "both", Mechanism: "published artifact inspection", Authority: "auditor", Boundary: "published-client", Reference: "audit://release", Version: "artifact", Origin: RealProvider, ValidUntil: now.Add(time.Hour)}
	c := ClaimDeclaration{Core: true, SpecRevision: "amended", SpecSHA256: "hash", InventoryRevision: "inventory", ArtifactRevision: "artifact", ImplementationRevision: "implementation", BindingRevision: "binding", EnvironmentRevision: "environment", ArtifactAudit: &b}
	for _, tc := range []struct {
		name   string
		mutate func(*ClaimDeclaration)
	}{
		{"empty-scope", func(c *ClaimDeclaration) { c.Core = false }},
		{"unknown-operation", func(c *ClaimDeclaration) {
			c.Operations = []OperationClaim{{Operation: "capture", Scope: "session", Support: "unknown"}}
		}},
		{"undefined-profile", func(c *ClaimDeclaration) { c.Profiles = []ProfileClaim{{Name: "invented-profile", Version: 1}} }},
		{"missing-exact-boundary", func(c *ClaimDeclaration) {
			c.ClaimedBoundaries = []ClaimedBoundary{{Boundary: "service-region-a", Activity: "processing", Direction: "send", Mechanism: "enforcement", Authority: "service"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := c
			tc.mutate(&v)
			if s, r := evidenceAssessment(v, RealProvider, now); s != Incomplete {
				t.Fatal(s, r)
			}
		})
	}
	e := inventory.Entry{Applicability: "supported-operation", Operations: []string{"capture"}}
	c.Operations = []OperationClaim{{Operation: "capture", Scope: "session", Support: "unknown"}}
	if !obligationApplies(e, c) {
		t.Fatal("unknown operation disappeared from applicability inventory")
	}
	test := ConformanceTest{Capability: &CapabilityDependence{ID: "media.capture", OwnerScope: "session"}}
	descriptors := []openrtc.CapabilityDescriptor{
		{ID: "media.capture", Scope: openrtc.CapabilityScope{Resource: "client"}, Support: openrtc.SupportDescription{Level: "supported"}},
		{ID: "media.capture", Scope: openrtc.CapabilityScope{Resource: "session"}, Support: openrtc.SupportDescription{Level: "supported"}},
	}
	if s, r := capabilityGate(test, descriptors, true); s != "" {
		t.Fatal("earlier descriptor obscured exact session scope", s, r)
	}
}

// RFC §14.2:1115: the recorded evaluation clock, rather than wall time,
// determines validity of exact-revision documentary evidence.
func TestAssertionEvidenceUsesEvaluationClock(t *testing.T) {
	d, err := inventory.Load()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var entry inventory.Entry
	for _, e := range d.Entries {
		if e.EvidenceMethod == "binding-documentation" && e.Profile == "" {
			entry = e
			break
		}
	}
	if len(entry.Assertions) == 0 {
		t.Fatal("no documentary assertion")
	}
	id := entry.Assertions[0].ID
	c := ClaimDeclaration{Core: true, SpecRevision: "amended", SpecSHA256: d.SpecSHA256, InventoryRevision: d.Revision, BindingRevision: "binding-r1", Evidence: []AssertionEvidence{{AssertionID: id, Method: "binding-documentation", Evidence: BoundaryEvidence{Activity: "binding audit", Direction: "both", Mechanism: "document inspection", Authority: "auditor", Boundary: "binding", Reference: "audit://binding-r1", Version: "binding-r1", Origin: RealProvider, ValidUntil: now.Add(time.Hour)}}}}
	results, _, _, _ := materializeAssertions(testTarget{}, nil, c, now)
	found := false
	for _, r := range results {
		if r.TestID == id {
			found = true
			if r.Status != Pass {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("documentary assertion omitted")
	}
	results, _, _, _ = materializeAssertions(testTarget{}, nil, c, now.Add(2*time.Hour))
	for _, r := range results {
		if r.TestID == id && r.Status != Incomplete {
			t.Fatal("expired evidence passed", r)
		}
	}
}

// RFC §14.1:1092–1098: a successful audio/session fixture cannot establish
// a video/call or conditional combination that was never exercised.
func TestExactOperationContextCoverage(t *testing.T) {
	audio := OperationClaim{Operation: "capture", Kinds: []string{"audio"}, Scope: "session", Support: "supported"}
	video := OperationClaim{Operation: "capture", Kinds: []string{"video"}, Scope: "call", Support: "supported"}
	if executionCovers([]OperationClaim{audio}, video) {
		t.Fatal("audio/session invented video/call coverage")
	}
	if !executionCovers([]OperationClaim{audio}, audio) {
		t.Fatal("exact observed context was lost")
	}
	conditional := audio
	conditional.Conditions = []string{"permission-granted"}
	if executionCovers([]OperationClaim{audio}, conditional) {
		t.Fatal("unconditional fixture invented required condition coverage")
	}
}

// RFC §§7.0:45–65,14.1:1092–1098: a justified unsupported optional
// positive does not require a fixture; negative entry/security checks still run.
func TestMediaFreeClaimKeepsNegativesAndExcludesJustifiedPositives(t *testing.T) {
	target := scopedClaimTestTarget{claim: ClaimDeclaration{Core: true, SpecRevision: "amended", Operations: []OperationClaim{{Operation: "capture", Scope: "session", Support: "unsupported", UnsupportedReason: "no media acquisition", Evidence: "public capability capture/session revision 3"}}}}
	negativeRan := false
	tests := []ConformanceTest{
		{ID: "positive", Suite: Core, OptionalOperation: "capture", Run: func(context.Context, *TestContext) error {
			t.Fatal("unsupported optional positive invoked")
			return nil
		}},
		{ID: "negative", Suite: Core, Run: func(context.Context, *TestContext) error { negativeRan = true; return nil }},
	}
	r := RunConformance(context.Background(), target, tests, RunOptions{})
	if !negativeRan || r.Results[0].Status != NotApplicable || r.Results[0].ExclusionEvidence == "" || r.Results[1].Status != Pass {
		t.Fatal("optional gate weakened mandatory negatives", r.Results[:2])
	}
	target.claim.Operations[0].Evidence = ""
	r = RunConformance(context.Background(), target, tests[:1], RunOptions{})
	if r.Results[0].Status != Incomplete {
		t.Fatal("missing exclusion evidence became unsupported", r.Results[0])
	}
}

// RFC §14.1:1092–1098 and §14.2:1102–1115: a Calling catalog
// location cannot hide a failed applicable Core media postcondition.
func TestScopedCoreRetainsCapabilityMediaFailure(t *testing.T) {
	results := []TestResult{{TestID: "call-publication-owner", Suite: Capability, Profile: Calling, Requirements: []string{"RFC A.1:1241–1242"}, Status: Fail, Reason: "publication lost owner"}}
	scopes := scopedAssessments(ClaimDeclaration{Core: true, Profiles: []ProfileClaim{{Name: "calling", Version: 1}}}, results, ClaimAssessment{Status: Fail})
	if len(scopes) == 0 || scopes[0].Scope != "core" || scopes[0].Status != Fail {
		t.Fatal("Core failure hidden by catalog profile", scopes)
	}
}

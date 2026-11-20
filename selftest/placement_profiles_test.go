package selftest

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
)

type placementProfileTarget struct {
	*FakeTarget
	defect string
}
type placementProfileState struct {
	mu                            sync.Mutex
	defect, required, combination string
	counts                        harness.NativeEffectCounts
	current                       openrtc.CurrentPlacementResult
	outside, applications         int
	incarnation                   string
}

func (s *placementProfileState) Counts() harness.NativeEffectCounts {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts
}
func placementIntersection(entries []openrtc.PlacementEntry, includePreferences bool) map[openrtc.RegionRef]bool {
	var intersection map[openrtc.RegionRef]bool
	for _, e := range entries {
		if e.Requirement != "required" && !includePreferences {
			continue
		}
		current := map[openrtc.RegionRef]bool{}
		for _, region := range e.AllowedRegions {
			current[region] = true
		}
		if intersection == nil {
			intersection = current
		} else {
			for region := range intersection {
				if !current[region] {
					delete(intersection, region)
				}
			}
		}
	}
	return intersection
}
func (s *placementProfileState) assess(scope string, entries []openrtc.PlacementEntry) openrtc.CurrentPlacementResult {
	intersection := placementIntersection(entries, s.defect == "placement-preference-required")
	assessment := "supported"
	if len(intersection) == 0 && s.defect != "placement-ignores-intersection" {
		assessment = "unsupported"
	}
	if s.required != "" && s.required != "supported" {
		assessment = s.required
	}
	if len(entries) > 1 && s.combination != "" && s.defect != "placement-infers-combination" {
		assessment = s.combination
	}
	result := openrtc.CurrentPlacementResult{Assessment: assessment, State: "unverified", Boundary: openrtc.Unknown[string](), Preferences: []openrtc.PlacementPreferenceResult{}}
	for _, e := range entries {
		if e.Requirement != "preference" {
			continue
		}
		if s.defect == "placement-preference-omitted" {
			continue
		}
		selection := "not-selected"
		reason := "required intersection excludes preferred region"
		for _, r := range e.AllowedRegions {
			if intersection[r] {
				selection = "selected"
				reason = ""
			}
		}
		if s.defect == "placement-preference-no-reason" {
			reason = ""
		}
		result.Preferences = append(result.Preferences, openrtc.PlacementPreferenceResult{RequestID: e.ID, Selection: selection, Reason: reason})
	}
	if s.defect == "placement-assessment-enforcement" {
		result.State = "verified"
	}
	return result
}
func (s *placementProfileState) AssessPlacement(_ context.Context, scope string, entries []openrtc.PlacementEntry) openrtc.Async[openrtc.CurrentPlacementResult] {
	return openrtc.StartOperation(s, func() (openrtc.CurrentPlacementResult, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.defect == "placement-assess-effects" {
			s.counts.Captures++
		}
		return s.assess(scope, entries), nil
	})
}
func (s *placementProfileState) ApplyPlacement(_ context.Context, scope string, entries []openrtc.PlacementEntry, _ openrtc.OperationOptions) openrtc.Async[openrtc.CurrentPlacementResult] {
	return openrtc.StartOperation(s, func() (openrtc.CurrentPlacementResult, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		result := s.assess(scope, entries)
		if result.Assessment != "supported" {
			if s.defect == "placement-rejected-effects" {
				s.counts.Captures++
			}
			rejected := openrtc.RejectedCondition{Condition: openrtc.Condition{Name: "placement-combination", Parameters: map[string]string{"scope": scope, "boundary": "provider-media-processing"}}, Assessment: result.Assessment}
			if result.Assessment == "pending" {
				rejected.Resolution = &openrtc.Resolution{Mechanism: "authoritative-policy-lookup", EvaluationBoundary: "before-first-protected-effect"}
			}
			details := openrtc.FailureDetails{RejectedConditions: []openrtc.RejectedCondition{rejected}}
			if s.defect == "placement-rejection-no-assessment" {
				details.RejectedConditions = nil
			}
			return openrtc.CurrentPlacementResult{}, openrtc.NewRTCOperationError("complete placement requirement not established", openrtc.RTCErrorDetails{Code: "unsupported", Reasons: []string{"unknown"}, Outcome: "none", Operation: openrtc.OperationRef{Name: "applyPlacement"}, Details: details})
		}
		result.State = "verified"
		boundary := "provider-media-processing"
		if s.defect == "placement-irrelevant-boundary" {
			boundary = "recording-storage-only"
		}
		result.Boundary = openrtc.KnownValue(boundary)
		result.Evidence = []openrtc.PlacementEnforcementEvidence{{Activity: "media-processing", Direction: "implementation-to-provider", Boundary: boundary, Mechanism: "fixture-enforced-process-policy", Authority: "fixture-independent-policy-witness", Source: "harness-self-test", Version: "1", ValidUntil: "2030-01-01T00:00:00Z"}}
		if s.defect == "placement-evidence-missing" {
			result.Evidence = nil
		}
		s.applications++
		if s.applications > 1 && s.defect == "placement-runtime-rejoin" {
			s.incarnation = "silently-recreated"
			s.counts.Admissions++
		}
		s.current = result
		return result, nil
	})
}
func (t *placementProfileTarget) PlacementScenario(context.Context) (harness.PlacementScenario, error) {
	s := &placementProfileState{defect: t.defect, incarnation: "session-one"}
	return harness.PlacementScenario{Documentation: "selftest scoped assessPlacement/applyPlacement; exact provider-media-processing synthetic policy witness", Scope: "session-one/media-processors", Activity: "media-processing", Direction: "implementation-to-provider", Boundary: "provider-media-processing", Regions: []openrtc.RegionRef{{Namespace: "provider-region-v1", Code: "west"}, {Namespace: "provider-region-v1", Code: "east"}}, Control: s, Effects: s, Snapshot: func() openrtc.CurrentPlacementResult { s.mu.Lock(); defer s.mu.Unlock(); return s.current }, SetRequiredAssessment: func(v string) { s.mu.Lock(); s.required = v; s.mu.Unlock() }, SetCombinationAssessment: func(v string) { s.mu.Lock(); s.combination = v; s.mu.Unlock() }, Recover: func(context.Context) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.defect == "placement-recovery-loses-boundary" {
			s.current.Boundary = openrtc.KnownValue("unrelated-fallback")
		}
		return nil
	}, WithdrawVerification: func(context.Context) error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.defect != "placement-loss-hidden" {
			s.current.State = "unverified"
		}
		if s.defect == "placement-outside-fallback" {
			s.outside++
		}
		return nil
	}, Checkpoint: func(context.Context) error { return nil }, OutsideEffects: func() int { s.mu.Lock(); defer s.mu.Unlock(); return s.outside }, SessionIncarnation: func() string { s.mu.Lock(); defer s.mu.Unlock(); return s.incarnation }, RuntimeApplySupported: true}, nil
}

// RFC §§7.3:180, 11.7:831–873, amended A.3:1386–1388: a preference
// never changes mandatory intersections; unsupported/unknown required complete
// combinations cannot reach protected effects or claim unrelated enforcement.
func TestPlacementProfileOracles(t *testing.T) {
	mutants := map[string][]string{
		"CT-PLC-PROFILE-001": {"placement-preference-required", "placement-preference-omitted", "placement-preference-no-reason", "placement-assessment-enforcement", "placement-assess-effects"},
		"CT-PLC-PROFILE-002": {"placement-ignores-intersection", "placement-rejected-effects", "placement-rejection-no-assessment"},
		"CT-PLC-PROFILE-003": {"placement-recovery-loses-boundary", "placement-loss-hidden", "placement-outside-fallback"},
		"CT-PLC-PROFILE-004": {"placement-irrelevant-boundary", "placement-evidence-missing", "placement-runtime-rejoin"},
		"CT-COMB-001":        {"placement-infers-combination"}}
	for _, test := range capability.PlacementProfileTests {
		t.Run(test.ID, func(t *testing.T) {
			for _, defect := range append([]string{""}, mutants[test.ID]...) {
				name := defect
				if name == "" {
					name = "conforming"
				}
				t.Run(name, func(t *testing.T) {
					target := &placementProfileTarget{MakeFakeTarget(FakeDefects{}), defect}
					tc := harness.NewTestContext(target, target.CreateAdapter().Descriptor())
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					err := test.Run(ctx, tc)
					if defect == "" && err != nil {
						t.Fatal(err)
					}
					if defect != "" && err == nil {
						t.Fatalf("defect %s accepted", defect)
					}
				})
			}
		})
	}
}

// RFC amended A.3:1386–1388: the required/preference discriminator and
// request correlation survive the binding wire projection.
func TestPlacementRequirementWireDiscriminator(t *testing.T) {
	for _, requirement := range []string{"required", "preference"} {
		entry := openrtc.PlacementEntry{ID: "entry-1", Scope: "session/processing", Requirement: requirement}
		raw, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		var got openrtc.PlacementEntry
		if err = json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != entry.ID || got.Requirement != requirement || got.Scope != entry.Scope {
			t.Fatal("wire projection lost placement request discriminator")
		}
	}
}

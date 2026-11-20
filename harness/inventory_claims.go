package harness

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/openrtc/conformance/inventory"
)

// obligationApplies preserves negative/security obligations even for a no-media
// Core claim. Only positive fixtures depend on advertised optional operations.
func obligationApplies(e inventory.Entry, c ClaimDeclaration) bool {
	profileClaimed := false
	for _, p := range c.Profiles {
		if p.Name == e.Profile {
			profileClaimed = true
		}
	}
	operationClaimed := false
	for _, o := range c.Operations {
		if o.Support != "supported" && o.Support != "conditional" && o.Support != "unknown" {
			continue
		}
		if contains(e.Operations, o.Operation) {
			operationClaimed = true
		}
	}
	switch e.Applicability {
	case "supported-operation":
		return operationClaimed
	case "profile-or-operation":
		return profileClaimed || operationClaimed
	case "profile-definition":
		return profileClaimed || operationClaimed
	default:
		return c.Core || len(c.Profiles) > 0 || len(c.Operations) > 0 || c.SpecRevision == ""
	}
}

func materializeAssertions(target ConformanceTarget, results []TestResult, c ClaimDeclaration, evaluatedAt time.Time) ([]TestResult, []string, ClaimDeclaration, string) {
	d, err := loadClaimInventory()
	if err != nil {
		results = append(results, TestResult{TestID: "inventory-load", Status: Incomplete, Reason: err.Error()})
		return results, []string{"inventory-load"}, c, ""
	}
	byID := map[string]TestResult{}
	for i := range results {
		r := &results[i]
		if r.Status == Unknown || r.Status == NotRun {
			r.IncompleteReason = string(r.Status)
		}
		r.Status = normalizedStatus(r.Status)
		// An arbitrary N/A string is not exclusion evidence.
		if r.Status == NotApplicable && r.ExclusionEvidence == "" {
			r.Status = Incomplete
			r.Reason = "unjustified exclusion: " + r.Reason
		}
		byID[r.TestID] = *r
	}
	required := []string{}
	for _, r := range results {
		if !r.BindingSpecific {
			required = append(required, r.TestID)
		}
	}
	if c.SpecSHA256 != d.SpecSHA256 || c.InventoryRevision != d.Revision {
		r := TestResult{TestID: "claim-revision", Suite: Core, Status: Incomplete, Reason: "claim does not identify amended RFC content hash and inventory revision"}
		results = append(results, r)
		required = append(required, r.TestID)
	}
	for _, e := range d.Entries {
		if !obligationApplies(e, c) {
			continue
		}
		for _, a := range e.Assertions {
			required = append(required, a.ID)
			r := TestResult{TestID: a.ID, Title: e.Clause, Suite: Core, Profile: ProfileName(e.Profile), Requirements: []string{fmt.Sprintf("RFC §%s:%d", e.Section, e.Line)}, Status: Incomplete, Reason: "missing " + e.EvidenceMethod + " assertion evidence (" + a.Coverage + ")", EvidenceType: effectiveEvidence(target), Environment: target.Environment()}
			if a.Coverage == "tested" && len(a.Tests) > 0 {
				r.Status = Pass
				r.Reason = "all mapped assertion checks passed"
				for _, id := range a.Tests {
					v, ok := byID[id]
					if ok && v.Status == Fail {
						r.Status = Fail
						r.Reason = "mapped assertion failed: " + id
						break
					}
					if !ok || v.Status != Pass {
						r.Status = Incomplete
						r.Reason = "mapped assertion omitted/incomplete: " + id
					}
				}
			} else {
				// A broad partial mapping cannot attribute a scenario's failure to every
				// cited clause. The executed scenario still fails the applicable aggregate;
				// this exact assertion remains incomplete until its own oracle is available.
				for _, id := range a.Tests {
					if v, ok := byID[id]; ok && v.Status == Fail {
						r.Reason = "partial scenario failed (" + id + "); exact clause evidence incomplete"
					}
				}
			}
			if e.EvidenceMethod != "public-runtime" {
				for _, v := range c.Evidence {
					if v.AssertionID != a.ID || v.Method != e.EvidenceMethod {
						continue
					}
					b := v.Evidence
					if b.Violation {
						r.Status = Fail
						r.Reason = "established audited boundary violation: " + b.Reference
						break
					}
					if r.Status != Fail && effectiveEvidence(target) == RealProvider && b.Origin == RealProvider && b.Boundary != "" && b.Reference != "" && b.Version != "" && evaluatedAt.Before(b.ValidUntil) && (b.ValidFrom.IsZero() || !evaluatedAt.Before(b.ValidFrom)) && evidenceMatchesAssertion(c, e, v) && !b.Withdrawn && !b.Contradicted {
						r.Status = Pass
						r.Reason = "current exact-assertion " + v.Method + " audit: " + b.Reference + " version " + b.Version
					}
				}
			}
			// An exclusion cannot override a predicate already established by this
			// claim. Public unsupported-capability exclusions are attached to the
			// individual positive test by the runner, with descriptor evidence.
			for _, x := range c.Exclusions {
				if x.Assertion == a.ID && r.Status != Fail {
					r.Status = Incomplete
					r.Reason = "exclusion conflicts with applicable claim predicate: " + x.Reason
				}
			}

			// A single fixture pass cannot establish every claimed kind/owner scope.
			// Materialize each declared context independently; absent exact execution
			// coverage stays incomplete even when a generic catalog alias passed.
			for _, operation := range c.Operations {
				if e.EvidenceMethod != "public-runtime" {
					break
				}
				if operation.Support == "unsupported" || !contains(e.Operations, operation.Operation) {
					continue
				}
				if len(operation.Kinds) == 0 && operation.Scope == "" {
					continue
				}
				scoped := r
				scoped.TestID = a.ID + "@" + operationContextKey(operation)
				scoped.OperationCoverage = nil
				scoped.Reason = "missing exact operation/kind/owner/condition execution evidence"
				scoped.Status = Incomplete
				if a.Coverage == "tested" && len(a.Tests) > 0 && operation.Support != "unknown" {
					complete := true
					failed := false
					for _, testID := range a.Tests {
						result, exists := byID[testID]
						if !exists || !executionCovers(result.OperationCoverage, operation) {
							complete = false
							continue
						}
						if result.Status == Fail {
							failed = true
						}
						if result.Status != Pass {
							complete = false
						}
					}
					if failed {
						scoped.Status = Fail
						scoped.Reason = "established scoped assertion failure"
					} else if complete {
						scoped.Status = Pass
						scoped.Reason = "all mapped assertions executed for exact claimed context"
					}
				}
				if scoped.Status != Pass && r.Status == Pass {
					r.Status = Incomplete
					r.Reason = "missing one or more claimed operation/kind/owner/condition contexts"
				}
				required = append(required, scoped.TestID)
				results = append(results, scoped)
			}
			results = append(results, r)
		}
	}
	return results, sortedUnique(required), c, d.Revision
}

func scopedAssessments(c ClaimDeclaration, results []TestResult, aggregate ClaimAssessment) []ScopedAssessment {
	out := []ScopedAssessment{{Scope: "core", Status: Pass}}
	for _, r := range results {
		if r.BindingSpecific {
			out = append(out, ScopedAssessment{Scope: "binding-promises", Status: Pass})
			break
		}
	}
	for _, p := range c.Profiles {
		out = append(out, ScopedAssessment{Scope: p.Name, Status: Pass})
	}
	for i := range out {
		s := &out[i]
		n := 0
		for _, r := range results {
			profile := string(r.Profile)
			if s.Scope == "binding-promises" {
				if !r.BindingSpecific {
					continue
				}
			} else if r.BindingSpecific {
				continue
			}
			if s.Scope == "core" {
				if profile != "" && profile != "common" && profile != "conference" && profile != "conference-audiovisual" && !coreRequirement(r) {
					continue
				}
			} else if s.Scope != "binding-promises" && profile != s.Scope {
				continue
			}
			n++
			switch r.Status {
			case Fail:
				s.Status = Fail
				s.Reasons = append(s.Reasons, r.TestID+": "+r.Reason)
			case Incomplete:
				if s.Status != Fail {
					s.Status = Incomplete
				}
				s.Reasons = append(s.Reasons, r.TestID+": "+r.Reason)
			}
		}
		if n == 0 {
			s.Status = Incomplete
			s.Reasons = append(s.Reasons, "no applicable assertion evidence")
		}
		// Missing provenance/evidence affects every scope; concrete violations retain priority.
		for _, reason := range aggregate.Reasons {
			if !strings.HasPrefix(reason, "incomplete assertion:") && !strings.HasPrefix(reason, "failed executed assertion:") && !strings.HasPrefix(reason, "failed applicable assertion:") {
				if s.Status == Pass {
					s.Status = Incomplete
				}
				s.Reasons = append(s.Reasons, reason)
			}
		}
	}
	return out
}

func declaredClaim(target ConformanceTarget) ClaimDeclaration {
	if t, ok := target.(ClaimTarget); ok {
		return t.ClaimDeclaration()
	}
	return ClaimDeclaration{}
}
func loadClaimInventory() (inventory.Document, error) { return inventory.Load() }

func evidenceMatchesAssertion(c ClaimDeclaration, e inventory.Entry, v AssertionEvidence) bool {
	b := v.Evidence
	if b.Activity == "" || b.Direction == "" || b.Mechanism == "" || b.Authority == "" {
		return false
	}
	switch e.EvidenceMethod {
	case "binding-documentation":
		return b.Version == c.BindingRevision
	case "published-artifact":
		return b.Version == c.ArtifactRevision
	case "independent-service":
		for _, boundary := range c.ClaimedBoundaries {
			if boundary.Boundary == b.Boundary && boundary.Activity == b.Activity && boundary.Direction == b.Direction && boundary.Mechanism == b.Mechanism && boundary.Authority == b.Authority {
				return true
			}
		}
	}
	return false
}

func operationContextKey(o OperationClaim) string {
	kinds := append([]string(nil), o.Kinds...)
	conditions := append([]string(nil), o.Conditions...)
	sort.Strings(kinds)
	sort.Strings(conditions)
	return url.QueryEscape(o.Operation) + "/" + url.QueryEscape(o.Scope) + "/" + url.QueryEscape(strings.Join(kinds, "+")) + "/" + url.QueryEscape(strings.Join(conditions, "+"))
}
func executionCovers(coverage []OperationClaim, want OperationClaim) bool {
	for _, observed := range coverage {
		if operationContextKey(observed) == operationContextKey(want) {
			return true
		}
	}
	return false
}

// Catalog/profile organization cannot hide a Core media/safety postcondition.
func coreRequirement(r TestResult) bool {
	if r.Suite == Core {
		return true
	}
	for _, ref := range r.Requirements {
		for _, section := range []string{"§4:", "§5:", "§6:", "§7.", "§7:", "§8.", "§8:", "§9:", "§12.", "A.1:", "A.2:"} {
			if strings.Contains(ref, section) {
				return true
			}
		}
	}
	return false
}

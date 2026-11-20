package harness

import (
	"context"
	"fmt"
	"github.com/openrtc/conformance/openrtc"
	"sort"
	"strings"
	"time"
)

// ClaimDeclaration identifies the scope being evaluated before selecting tests.
// RFC §14.1–14.2 (1088–1115): labels alone establish neither support nor evidence.
type ClaimDeclaration struct {
	SpecRevision           string               `json:"specRevision"`
	SpecSHA256             string               `json:"specSha256"`
	InventoryRevision      string               `json:"inventoryRevision"`
	ArtifactRevision       string               `json:"artifactRevision"`
	ImplementationRevision string               `json:"implementationRevision"`
	BindingRevision        string               `json:"bindingRevision"`
	EnvironmentRevision    string               `json:"environmentRevision"`
	Core                   bool                 `json:"core"`
	Operations             []OperationClaim     `json:"operations"`
	Profiles               []ProfileClaim       `json:"profiles"`
	ClaimedBoundaries      []ClaimedBoundary    `json:"claimedBoundaries"`
	Boundaries             []BoundaryEvidence   `json:"boundaries"`
	Exclusions             []AssertionExclusion `json:"exclusions"`
	Evidence               []AssertionEvidence  `json:"assertionEvidence"`
	ArtifactAudit          *BoundaryEvidence    `json:"artifactAudit,omitempty"`
}
type OperationClaim struct {
	UnsupportedReason string   `json:"unsupportedReason,omitempty"`
	Evidence          string   `json:"evidence,omitempty"`
	Operation         string   `json:"operation"`
	Kinds             []string `json:"kinds"`
	Scope             string   `json:"scope"`
	Conditions        []string `json:"conditions"`
	Support           string   `json:"support"` // supported, unsupported, conditional, unknown
}
type ProfileClaim struct {
	Name         string   `json:"name"`
	Version      int      `json:"version"`
	Dependencies []string `json:"dependencies"`
}
type AssertionExclusion struct {
	Assertion string `json:"assertion"`
	Reason    string `json:"reason"`
	Evidence  string `json:"evidence"`
}
type ClaimedBoundary struct {
	Boundary  string `json:"boundary"`
	Activity  string `json:"activity"`
	Direction string `json:"direction"`
	Mechanism string `json:"mechanism"`
	Authority string `json:"authority"`
}
type BoundaryEvidence struct {
	Activity     string       `json:"activity"`
	Direction    string       `json:"direction"`
	Mechanism    string       `json:"mechanism"`
	Authority    string       `json:"authority"`
	ValidFrom    time.Time    `json:"validFrom"`
	Boundary     string       `json:"boundary"`
	Reference    string       `json:"reference"`
	Version      string       `json:"version"`
	ValidUntil   time.Time    `json:"validUntil"`
	Withdrawn    bool         `json:"withdrawn"`
	Contradicted bool         `json:"contradicted"`
	Violation    bool         `json:"violation"`
	Origin       EvidenceType `json:"origin"`
}
type ClaimTarget interface{ ClaimDeclaration() ClaimDeclaration }
type ScopedAssessment struct {
	Scope   string            `json:"scope"`
	Status  ConformanceStatus `json:"status"`
	Reasons []string          `json:"reasons"`
}

func normalizedStatus(s ConformanceStatus) ConformanceStatus {
	switch s {
	case Pass, Fail, NotApplicable, Incomplete:
		return s
	default:
		return Incomplete
	}
}

// assessAssertions combines assertion-level records, independent of catalog names.
// RFC §14.2:1102–1115; unknown and omitted results cannot establish a claim.
func assessAssertions(evidence EvidenceType, results []TestResult, required []string, complete bool) ClaimAssessment {
	a := ClaimAssessment{Status: Pass, Reasons: []string{}}
	present := map[string]TestResult{}
	for _, r := range results {
		present[r.TestID] = r
	}
	incomplete := func(reason string) {
		if a.Status != Fail {
			a.Status = Incomplete
		}
		a.Reasons = append(a.Reasons, reason)
	}
	if !complete {
		incomplete("filtered run or omitted claim/inventory metadata")
	}
	if len(required) == 0 {
		incomplete("no identified mandatory assertions")
	}
	for _, id := range required {
		r, ok := present[id]
		if !ok {
			incomplete("mandatory assertion omitted: " + id)
			continue
		}
		switch normalizedStatus(r.Status) {
		case Fail:
			a.Status = Fail
			a.Reasons = append(a.Reasons, "failed applicable assertion: "+id)
		case Incomplete:
			incomplete("incomplete assertion: " + id + ": " + r.Reason)
		case NotApplicable:
			if r.ExclusionEvidence == "" || r.Reason == "" {
				incomplete("unjustified exclusion: " + id)
			}
		}
	}
	// Catalog placement cannot hide a supported-media Core failure.
	for _, r := range results {
		if r.Status == Fail && !r.BindingSpecific {
			a.Status = Fail
			a.Reasons = append(a.Reasons, "failed executed assertion: "+r.TestID)
		}
	}
	if evidence != RealProvider {
		incomplete(fmt.Sprintf("%s evidence is not real-provider evidence", evidence))
	}
	a.UnqualifiedCoreClaim = a.Status == Pass
	return a
}

func evidenceAssessment(c ClaimDeclaration, evidence EvidenceType, now time.Time) (ConformanceStatus, []string) {
	status := Pass
	reasons := []string{}
	gap := func(s string) {
		if status != Fail {
			status = Incomplete
		}
		reasons = append(reasons, s)
	}
	if !c.Core && len(c.Profiles) == 0 && len(c.Operations) == 0 {
		gap("no applicable Core, profile or operation claim scope")
	}
	for _, o := range c.Operations {
		if o.Operation == "" || o.Scope == "" {
			gap("operation claim lacks operation/owner scope")
		}
		if o.Support == "unknown" || o.Support == "" {
			gap("operation applicability unresolved: " + o.Operation)
		}
	}
	if c.SpecRevision == "" || c.SpecSHA256 == "" || c.InventoryRevision == "" || c.ArtifactRevision == "" || c.ImplementationRevision == "" || c.BindingRevision == "" || c.EnvironmentRevision == "" {
		gap("missing exact specification, inventory, artifact, implementation, binding or environment revision")
	}
	check := func(b BoundaryEvidence) {
		if b.Violation {
			status = Fail
			reasons = append(reasons, "established boundary violation: "+b.Boundary)
			return
		}
		if b.Activity == "" || b.Direction == "" || b.Mechanism == "" || b.Authority == "" || (!b.ValidFrom.IsZero() && now.Before(b.ValidFrom)) || b.Boundary == "" || b.Reference == "" || b.Version == "" || b.ValidUntil.IsZero() || !now.Before(b.ValidUntil) || b.Withdrawn || b.Contradicted {
			gap("missing/expired/withdrawn/contradicted boundary evidence: " + b.Boundary)
		}
		if b.Origin != RealProvider || evidence != RealProvider {
			gap("non-authoritative evidence for external boundary: " + b.Boundary)
		}
	}
	for _, b := range c.Boundaries {
		check(b)
	}
	for _, required := range c.ClaimedBoundaries {
		matched := false
		for _, b := range c.Boundaries {
			if b.Boundary == required.Boundary && b.Activity == required.Activity && b.Direction == required.Direction && b.Mechanism == required.Mechanism && b.Authority == required.Authority {
				matched = true
			}
		}
		if !matched {
			gap("missing exact claimed boundary evidence: " + required.Boundary)
		}
	}
	if c.ArtifactAudit == nil {
		gap("missing published-client artifact audit")
	} else {
		check(*c.ArtifactAudit)
	}
	names := map[string]bool{}
	for _, p := range c.Profiles {
		names[p.Name] = true
		if !definedProfile(p.Name) {
			gap("undefined profile: " + p.Name)
		}
		if p.Version != 1 {
			gap("undefined profile version: " + p.Name)
		}
	}
	for _, p := range c.Profiles {
		for _, d := range p.Dependencies {
			if !names[d] {
				gap("missing profile dependency: " + d)
			}
		}
	}
	return status, reasons
}

func sortedUnique(ids []string) []string {
	m := map[string]bool{}
	for _, id := range ids {
		m[id] = true
	}
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// ScopedCapabilityTarget resolves exact operation/kind/combination at an eligible
// resource. A provider-level unknown cannot decide session-conditional support.
// RFC §§7.3:173–186, 14.1:1092–1098.
type ScopedCapabilityTarget interface {
	CapabilityFor(context.Context, CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool)
}

func capabilityExclusion(test ConformanceTest, snapshot []openrtc.CapabilityDescriptor) string {
	for _, d := range snapshot {
		if test.Capability != nil && string(d.ID) == test.Capability.ID {
			return fmt.Sprintf("public capability %s revision %v scope %+v support %s: %s", d.ID, d.Revision, d.Scope, d.Support.Level, d.Support.Reason)
		}
	}
	return ""
}
func mandatoryPositive(target ConformanceTarget, test ConformanceTest) bool {
	return mandatoryPositiveDeclaration(declaredClaim(target), test)
}
func mandatoryPositiveDeclaration(c ClaimDeclaration, test ConformanceTest) bool {
	if test.Capability == nil {
		return false
	}
	profile := test.Capability.MandatoryProfile
	if profile == "" {
		profile = mandatoryProfileForCapability(test.Capability.ID)
	}
	for _, p := range c.Profiles {
		if profile != "" && p.Name == profile {
			return true
		}
	}
	return false
}
func boundedCapabilitySnapshot(ctx context.Context, t ConformanceTarget) ([]openrtc.CapabilityDescriptor, bool) {
	budget := t.Timing().OperationMs
	if budget <= 0 {
		return nil, false
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()
	type result struct {
		s  []openrtc.CapabilityDescriptor
		ok bool
	}
	done := make(chan result, 1)
	go func() { s, ok := readCapabilitySnapshot(probeCtx, t); done <- result{s, ok} }()
	select {
	case r := <-done:
		return r.s, r.ok
	case <-probeCtx.Done():
		return nil, false
	}
}

// AssertionEvidence supplies a reviewable documentary or independent-boundary
// audit. Runtime assertions require executed public tests, not these records.
// RFC §§14.2:1111–1115,14.3:1145–1149.
type AssertionEvidence struct {
	AssertionID string           `json:"assertionId"`
	Method      string           `json:"method"`
	Evidence    BoundaryEvidence `json:"evidence"`
}

func mandatoryProfileForCapability(id string) string {
	switch id {
	case "calling.dial", "calling.accept":
		return "calling"
	case "media.subscribe.manual":
		return "manual-subscription"
	case "media.publish.independent":
		return "" // Core operation, not a defining profile by itself
	case "messaging.ephemeral":
		return "data-messaging"
	case "recording.server":
		return "recording"
	case "encryption.media":
		return "end-to-end-encryption"
	case "devices.enumerate", "media.statistics":
		return "devices-and-statistics"
	}
	return ""
}

// Exclude records a justified applicability result with its exact evidence.
// Missing fixtures are incomplete and must use NotRun instead (§14.1:1098).
func (c *TestContext) Exclude(reason, evidence string) error {
	if reason == "" || evidence == "" {
		return c.NotRun("exclusion lacks reason or evidence")
	}
	return &Inconclusive{Status: NotApplicable, Reason: reason, Evidence: evidence}
}
func boundedScopedCapability(ctx context.Context, t ConformanceTarget, s ScopedCapabilityTarget, d CapabilityDependence) ([]openrtc.CapabilityDescriptor, bool) {
	budget := t.Timing().OperationMs
	if budget <= 0 {
		return nil, false
	}
	bounded, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()
	type result struct {
		s  []openrtc.CapabilityDescriptor
		ok bool
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if recover() != nil {
				done <- result{}
			}
		}()
		v, ok := s.CapabilityFor(bounded, d)
		done <- result{v, ok}
	}()
	select {
	case r := <-done:
		return r.s, r.ok
	case <-bounded.Done():
		return nil, false
	}
}

// Evidence provenance survives a wrapper changing only its display label.
// Known selftest artifacts cannot supply real service evidence (§14.2:1111–1115).
func effectiveEvidence(t ConformanceTarget) EvidenceType {
	artifact := strings.ToLower(t.Environment().AdapterArtifact)
	if strings.Contains(artifact, "harness-fake") || strings.Contains(artifact, "selftest") || strings.Contains(artifact, "self-test") {
		return HarnessSelfTest
	}
	return t.Evidence()
}

func definedProfile(name string) bool {
	return contains([]string{"calling", "manual-subscription", "source-replacement-and-sharing", "screen-sharing", "data-messaging", "recording", "placement", "end-to-end-encryption", "server-administration", "devices-and-statistics", "credential-renewal"}, name)
}

// RecordCoverage identifies the exact public operation inputs exercised by this
// test. It records coverage only; the test must still pass its public oracles.
// RFC §§14.1:1092–1098,14.3:1147–1149.
func (c *TestContext) RecordCoverage(operation OperationClaim) {
	c.OperationCoverage = append(c.OperationCoverage, operation)
}

func optionalOperationGate(c ClaimDeclaration, operation string) (ConformanceStatus, string, string) {
	if operation == "" || c.SpecRevision == "" {
		return "", "", ""
	}
	found := false
	unsupported := true
	evidence := []string{}
	for _, o := range c.Operations {
		if o.Operation != operation {
			continue
		}
		found = true
		if o.Support != "unsupported" {
			unsupported = false
			continue
		}
		if o.UnsupportedReason == "" || o.Evidence == "" {
			return Incomplete, "optional positive applicability lacks exact unsupported-scope evidence: " + operation, ""
		}
		evidence = append(evidence, o.Scope+": "+o.UnsupportedReason+" ("+o.Evidence+")")
	}
	if found && unsupported {
		return NotApplicable, "positive operation unsupported in every declared scope: " + operation, strings.Join(evidence, "; ")
	}
	if !found {
		return Incomplete, "operation/kind/owner applicability not identified in claim: " + operation, ""
	}
	return "", "", ""
}

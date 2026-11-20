// Package harness provides the conformance result model and sequential runner (ADP-006).
package harness

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openrtc/conformance/openrtc"
)

type ConformanceStatus string

const (
	Pass          ConformanceStatus = "pass"
	Incomplete    ConformanceStatus = "incomplete"
	Fail          ConformanceStatus = "fail"
	NotRun        ConformanceStatus = "not-run"
	Unknown       ConformanceStatus = "unknown"
	NotApplicable ConformanceStatus = "not-applicable"
)

type EvidenceType string

const (
	HarnessSelfTest EvidenceType = "harness-self-test"
	Mock            EvidenceType = "mock"
	RecordedReplay  EvidenceType = "recorded-replay"
	RealProvider    EvidenceType = "real-provider"
)

type SuiteName string

const (
	Core       SuiteName = "core"
	Capability SuiteName = "capability"
)

type ProfileName string

const (
	Common                ProfileName = "common"
	Conference            ProfileName = "conference"
	Calling               ProfileName = "calling"
	ConferenceAudiovisual ProfileName = "conference-audiovisual"
)

type EnvironmentOption struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type EnvironmentDescription struct {
	AdapterArtifact      string              `json:"adapterArtifact"`
	AdapterVersion       string              `json:"adapterVersion"`
	SDKArtifact          string              `json:"sdkArtifact"`
	SDKVersion           string              `json:"sdkVersion"`
	ServerArtifact       string              `json:"serverArtifact"`
	Platform             string              `json:"platform"`
	Deployment           string              `json:"deployment"`
	AccountPrerequisites []string            `json:"accountPrerequisites"`
	Options              []EnvironmentOption `json:"options"`
}
type TestResult struct {
	OperationCoverage []OperationClaim       `json:"operationCoverage,omitempty"`
	BindingSpecific   bool                   `json:"bindingSpecific,omitempty"`
	ExclusionEvidence string                 `json:"exclusionEvidence,omitempty"`
	IncompleteReason  string                 `json:"incompleteReason,omitempty"`
	TestID            string                 `json:"testId"`
	Title             string                 `json:"title"`
	Suite             SuiteName              `json:"suite"`
	Profile           ProfileName            `json:"profile"`
	Requirements      []string               `json:"requirements"`
	Status            ConformanceStatus      `json:"status"`
	Reason            string                 `json:"reason"`
	Observations      []string               `json:"observations"`
	StartedAt         openrtc.Instant        `json:"startedAt"`
	DurationMs        float64                `json:"durationMs"`
	EvidenceType      EvidenceType           `json:"evidenceType"`
	Environment       EnvironmentDescription `json:"environment"`
}
type FixtureNeed string
type CapabilityDependence struct {
	Operation        string
	Kinds            []string
	OwnerScope       string
	MandatoryProfile string
	ID               string
	AppliesWhen      string
}
type ConformanceTest struct {
	OptionalOperation string
	ID                string
	Title             string
	Suite             SuiteName
	Profile           ProfileName
	Requirements      []string
	Observable        string
	Needs             []FixtureNeed
	Capability        *CapabilityDependence
	Run               func(context.Context, *TestContext) error
}

func DefineTest(test ConformanceTest) ConformanceTest {
	if len(test.Requirements) == 0 {
		panic(fmt.Sprintf("test %s cites no requirement ID", test.ID))
	}
	return test
}

// NativeEffectCounts records instrumented native effects within the fixture's declared coverage;
// unchanged counts do not establish absence outside that coverage.
type NativeEffectCounts struct {
	ProvisioningCalls int `json:"provisioningCalls"`
	Admissions        int `json:"admissions"`
	Leaves            int `json:"leaves"`
	Captures          int `json:"captures"`
	Publications      int `json:"publications"`
	ClientsCreated    int `json:"clientsCreated"`
}
type NativeEffectObserver interface{ Counts() NativeEffectCounts }
type JoinGate interface {
	Reached() <-chan struct{}
	ReleaseSuccess()
	ReleaseFailure(any)
}
type TransportInterruption interface{ Restore(context.Context) error }
type RemoteEndpoint interface {
	Identity() openrtc.ApplicationIdentity
	IsConnected() bool
	Publish(context.Context, []string) error
	SetMuted(context.Context, bool) error
	SendText(context.Context, string) error
	ReceivedText() []string
	Leave(context.Context) error
}
type RemoteEndpointIntent struct {
	Identity    openrtc.ApplicationIdentity
	DisplayName *string
}
type AdmissionControl interface {
	Hold()
	Admit()
	Deny()
}
type AuthorityControl interface {
	Revoke(context.Context, openrtc.CapabilityID) error
	Restore(context.Context, openrtc.CapabilityID) error
}
type IncomingCallFixture interface {
	Caller() string
	HangUp(context.Context) error
}

// FixtureControls contains optional controls. Nil controls yield not-run where required.
type FixtureControls struct {
	NativeEffects            NativeEffectObserver
	GateNextJoin             func() JoinGate
	InterruptTransport       func(context.Context, openrtc.RTCSession) (TransportInterruption, error)
	TriggerCredentialRenewal func(context.Context, openrtc.RTCSession) error
	FailNextNativeOperation  func(string, any)
	RemoteEndpoint           func(context.Context, openrtc.ConferenceTarget, RemoteEndpointIntent) (RemoteEndpoint, error)
	Admission                func() AdmissionControl
	Authority                func() AuthorityControl
	IncomingCall             func(context.Context, openrtc.RTCClient) (IncomingCallFixture, error)
	EnforceablePlacement     func() *openrtc.PlacementPolicy
}
type ProvisioningModeName string
type TimingPolicy struct {
	OperationMs int
	SettleMs    int
}

type FixtureTarget interface{ Controls() FixtureControls }

type ConformanceFailure struct{ Message string }

func (e *ConformanceFailure) Error() string { return e.Message }

type Inconclusive struct {
	Status   ConformanceStatus
	Reason   string
	Evidence string
}

func (e *Inconclusive) Error() string { return e.Reason }

type disposable struct {
	label   string
	dispose func(context.Context) error
}
type TestContext struct {
	OperationCoverage []OperationClaim
	Target            ConformanceTarget
	Descriptor        openrtc.ProviderDescriptor
	Spy               *CredentialSpy
	Observations      []string
	Partials          []string
	Needles           []string
	disposables       []disposable
}

func (c *TestContext) Fail(message string) error { return &ConformanceFailure{message} }
func (c *TestContext) Check(condition bool, message string) error {
	if !condition {
		return c.Fail(message)
	}
	return nil
}
func (c *TestContext) NotRun(reason string) error {
	return &Inconclusive{Status: NotRun, Reason: reason}
}
func (c *TestContext) Unknown(reason string) error {
	return &Inconclusive{Status: Unknown, Reason: reason}
}
func (c *TestContext) NotApplicable(reason string) error {
	return &Inconclusive{Status: NotApplicable, Reason: reason}
}
func (c *TestContext) Note(message string) { c.Observations = append(c.Observations, message) }

// Partial prevents an otherwise passing test from claiming a complete verdict (D-16).
func (c *TestContext) Partial(reason string) {
	c.Partials = append(c.Partials, reason)
	c.Note("partial coverage: " + reason)
}
func (c *TestContext) Controls() FixtureControls {
	if t, ok := c.Target.(FixtureTarget); ok {
		return t.Controls()
	}
	return FixtureControls{}
}
func (c *TestContext) RequireNative() (NativeEffectObserver, error) {
	o := c.Controls().NativeEffects
	if o == nil {
		return nil, c.NotRun("fixture provides no native-effect observer")
	}
	return o, nil
}

const noObserver = "native effects were not observed (fixture has no nativeEffects control); side-effect checks skipped"

func (c *TestContext) NativeCounts() *NativeEffectCounts {
	o := c.Controls().NativeEffects
	if o == nil {
		if !contains(c.Partials, noObserver) {
			c.Partial(noObserver)
		}
		return nil
	}
	v := o.Counts()
	return &v
}
func (c *TestContext) Track(label string, dispose func(context.Context) error) {
	c.disposables = append(c.disposables, disposable{label, dispose})
}

// Within bounds the action context and the harness wait. A watchdog expiry is
// never evidence that the target operation settled or remote cleanup completed.
func (c *TestContext) Within(ctx context.Context, label string, ms int, action func(context.Context) error) error {
	if ms == 0 {
		ms = c.Target.Timing().OperationMs
	}
	boundedCtx, cancel := context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
	defer cancel()
	ch := make(chan error, 1)
	go func() {
		defer func() {
			if value := recover(); value != nil {
				if err, ok := value.(error); ok {
					ch <- err
				} else {
					ch <- fmt.Errorf("%v", value)
				}
			}
		}()
		ch <- action(boundedCtx)
	}()
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-ch:
		return err
	case <-timer.C:
		c.Note("abandoned operation after timeout: " + label)
		return c.Fail(fmt.Sprintf("timed out after %d ms: %s", ms, label))
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *TestContext) Sleep(ctx context.Context, ms int) error {
	t := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *TestContext) Settle(ctx context.Context) error {
	return c.Sleep(ctx, c.Target.Timing().SettleMs)
}
func (c *TestContext) WaitFor(ctx context.Context, predicate func() bool, label string, ms int) error {
	if ms == 0 {
		ms = c.Target.Timing().OperationMs
	}
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	for !predicate() {
		if time.Now().After(deadline) {
			return c.Fail("condition not reached: " + label)
		}
		if err := c.Sleep(ctx, 2); err != nil {
			return err
		}
	}
	return nil
}
func (c *TestContext) Cleanup(ctx context.Context) []string {
	items := c.disposables
	c.disposables = nil
	failures := []string{}
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if err := c.Within(ctx, "cleanup of "+item.label, 0, item.dispose); err != nil {
			failures = append(failures, item.label+": "+err.Error())
		}
	}
	return failures
}

type RunOptions struct {
	Suites []SuiteName
	Only   []string
	Now    func() time.Time
}
type ClaimAssessment struct {
	Status               ConformanceStatus  `json:"status"`
	Scopes               []ScopedAssessment `json:"scopes"`
	UnqualifiedCoreClaim bool               `json:"unqualifiedCoreClaim"`
	Reasons              []string           `json:"reasons"`
}
type ConformanceReport struct {
	Declaration       ClaimDeclaration          `json:"declaration"`
	InventoryRevision string                    `json:"inventoryRevision"`
	TargetName        string                    `json:"targetName"`
	Author            string                    `json:"author"`
	EvidenceType      EvidenceType              `json:"evidenceType"`
	Environment       EnvironmentDescription    `json:"environment"`
	SpecificationNote string                    `json:"specificationNote"`
	Complete          bool                      `json:"complete"`
	Results           []TestResult              `json:"results"`
	Counts            map[ConformanceStatus]int `json:"counts"`
	Claim             ClaimAssessment           `json:"claim"`
}

func contains[T comparable](items []T, v T) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}
func AssessClaim(evidence EvidenceType, results []TestResult, requiredCoreIDs []string, complete bool) ClaimAssessment {
	return assessAssertions(evidence, results, requiredCoreIDs, complete)
}

var secretPattern = regexp.MustCompile(`(?i)(token|secret|authorization|bearer|key|password)=?[:=]?\s*[A-Za-z0-9._~+/-]{8,}`)

func redact(s string, needles []string) string {
	for _, n := range needles {
		if n != "" {
			s = strings.ReplaceAll(s, n, "[redacted]")
		}
	}
	s = secretPattern.ReplaceAllString(s, "$1=[redacted]")
	r := []rune(s)
	if len(r) > 400 {
		return string(r[:400]) + "[truncated]"
	}
	return s
}
func capabilityGate(test ConformanceTest, snapshot []openrtc.CapabilityDescriptor, available bool) (ConformanceStatus, string) {
	if test.Capability == nil {
		return "", ""
	}
	id := test.Capability.ID
	if !available {
		return Unknown, "capability snapshot could not be read"
	}
	for _, d := range snapshot {
		if string(d.ID) == id {
			if test.Capability.OwnerScope != "" && d.Scope.Resource != test.Capability.OwnerScope {
				continue
			}
			if d.Support.Level == "partial" && len(test.Capability.Kinds) > 0 {
				combination := strings.Join(test.Capability.Kinds, "+")
				if !contains(d.Support.Subset, combination) {
					return Unknown, "partial descriptor does not establish requested kind/combination"
				}
			}
			switch d.Support.Level {
			case "supported", "partial":
				return "", ""
			case "unsupported":
				return NotApplicable, fmt.Sprintf("capability %s advertised unsupported; negative gates are in core CAP/EXT tests", id)
			case "not-applicable":
				return NotApplicable, fmt.Sprintf("capability %s not applicable: %s", id, d.Support.Reason)
			case "unknown":
				return Unknown, fmt.Sprintf("capability %s support is unknown; positive behavior cannot be judged", id)
			}
			return Unknown, "capability support value is unrecognized"
		}
	}
	return Unknown, fmt.Sprintf("capability %s is not described; absence is not evidence of lack of support (CORE-002)", id)
}

// readCapabilitySnapshot treats any failure in the probe, including disposal, as unknown.
func readCapabilitySnapshot(ctx context.Context, target ConformanceTarget) (snapshot []openrtc.CapabilityDescriptor, available bool) {
	defer func() {
		if recover() != nil {
			snapshot, available = nil, false
		}
	}()
	if direct, ok := target.(PublicBindingTarget); ok {
		m := direct.PublicBinding()
		if m.CreateClient == nil || m.Capabilities == nil || m.Dispose == nil {
			return nil, false
		}
		client, err := m.CreateClient(ctx)
		if err != nil || client == nil {
			return nil, false
		}
		snapshot, err := m.Capabilities(client)
		releaseErr := m.Dispose(ctx, client)
		return snapshot, err == nil && releaseErr == nil
	}
	registry := target.CreateRegistry()
	adapter := target.CreateAdapter()
	if _, err := registry.Register(adapter); err != nil {
		return nil, false
	}
	provider, err := registry.Initialize(ctx, adapter.Descriptor().AdapterID, target.ProviderConfig()).Result(ctx)
	if err != nil {
		return nil, false
	}
	snapshot = provider.Capabilities().Snapshot().Entries
	if _, err := provider.Dispose(ctx).Result(ctx); err != nil {
		return nil, false
	}
	return snapshot, true
}

type testWatchdogError struct{}

func (*testWatchdogError) Error() string {
	return "harness watchdog exhausted; operation settlement not established"
}

func runTest(ctx context.Context, test ConformanceTest, tc *TestContext) error {
	budget := tc.Target.Timing().OperationMs
	if budget <= 0 {
		return tc.NotRun("missing positive harness watchdog budget")
	}
	// This watchdog bounds orchestration; a waiter timeout is never target settlement.
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(budget)*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if v := recover(); v != nil {
				if e, ok := v.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("%v", v)
				}
			}
			done <- err
		}()
		err = test.Run(runCtx, tc)
	}()
	select {
	case err := <-done:
		return err
	case <-runCtx.Done():
		return &testWatchdogError{}
	}
}

func contextNeedles(tc *TestContext) []string {
	needles := append([]string(nil), tc.Needles...)
	if tc.Spy != nil {
		needles = append(needles, tc.Spy.Needles()...)
	}
	return needles
}

func describeError(err error) string { return "Error: " + err.Error() }

// RunConformance runs selected tests sequentially and isolates their cleanup.
func RunConformance(ctx context.Context, target ConformanceTarget, tests []ConformanceTest, options RunOptions) ConformanceReport {
	declaration := declaredClaim(target)
	// Identify the frozen inventory before selecting any tests or filters.
	_, inventoryLoadErr := loadClaimInventory()
	suites := options.Suites
	if suites == nil {
		suites = []SuiteName{Core, Capability}
	}
	clock := options.Now
	if clock == nil {
		clock = time.Now
	}
	descriptor := TargetDescriptor(target)
	snapshot, available := boundedCapabilitySnapshot(ctx, target)
	results := []TestResult{}
	for _, test := range tests {
		if !contains(suites, test.Suite) || (options.Only != nil && !contains(options.Only, test.ID)) {
			continue
		}
		started := clock()
		status := Pass
		reason := "all checks passed"
		observations := []string{}
		var operationCoverage []OperationClaim
		exclusionEvidence := ""
		testSnapshot, testAvailable := snapshot, available
		if scoped, ok := target.(ScopedCapabilityTarget); ok && test.Capability != nil {
			testSnapshot, testAvailable = boundedScopedCapability(ctx, target, scoped, *test.Capability)
		}
		if statusGate, reasonGate, evidenceGate := optionalOperationGate(declaration, test.OptionalOperation); statusGate != "" {
			status, reason, exclusionEvidence = statusGate, reasonGate, evidenceGate
		} else if !bindingPromiseApplies(target, test.ID) {
			status = NotApplicable
			reason = "binding-specific Go bridge promise not part of the declared RFC claim"
			exclusionEvidence = "RFC §§2,5,14.2 and A.0; documented public-equivalence scope"
		} else if s, r := capabilityGate(test, testSnapshot, testAvailable); s != "" {
			status, reason = s, r
			if status == NotApplicable {
				if mandatoryPositiveDeclaration(declaration, test) {
					status = Fail
					reason = "claimed mandatory profile positive advertised unsupported: " + r
				} else {
					exclusionEvidence = capabilityExclusion(test, testSnapshot)
				}
			}
		} else {
			tc := NewTestContext(target, descriptor)
			err := runTest(ctx, test, tc)
			var watchdog *testWatchdogError
			if errors.As(err, &watchdog) {
				// An uncooperative body still owns tc. Do not read its slices or
				// run teardown concurrently with it. Never count its waiter as settlement.
				results = append(results, TestResult{TestID: test.ID, Title: test.Title, Suite: test.Suite, Profile: test.Profile, Requirements: test.Requirements, Status: Incomplete, Reason: watchdog.Error(), StartedAt: openrtc.Instant(started.UTC().Format(time.RFC3339Nano)), DurationMs: float64(clock().Sub(started)) / float64(time.Millisecond), EvidenceType: effectiveEvidence(target), Environment: target.Environment()})
				continue
			}
			if err != nil {
				var inc *Inconclusive
				var failure *ConformanceFailure
				switch {
				case errors.As(err, &inc):
					status, reason = inc.Status, inc.Reason
					exclusionEvidence = inc.Evidence
				case errors.As(err, &failure):
					status, reason = Fail, failure.Message
				default:
					status, reason = Fail, "unexpected exception: "+redact(describeError(err), contextNeedles(tc))
				}
			}
			failures := tc.Cleanup(ctx)
			needles := contextNeedles(tc)
			for _, f := range failures {
				tc.Note("cleanup problem: " + redact(f, needles))
			}
			if len(failures) > 0 && status == Pass {
				status = Fail
				reason = "assertions passed but cleanup failed: " + redact(strings.Join(failures, "; "), needles)
			} else if len(tc.Partials) > 0 && status == Pass {
				status = Unknown
				reason = "partial coverage: " + redact(strings.Join(tc.Partials, "; "), needles)
			} else {
				reason = redact(reason, needles)
			}
			operationCoverage = append(operationCoverage, tc.OperationCoverage...)
			for _, o := range tc.Observations {
				observations = append(observations, redact(o, needles))
			}
		}
		results = append(results, TestResult{OperationCoverage: operationCoverage, BindingSpecific: bindingSpecific(test.ID), ExclusionEvidence: exclusionEvidence, TestID: test.ID, Title: test.Title, Suite: test.Suite, Profile: test.Profile, Requirements: test.Requirements, Status: status, Reason: reason, Observations: observations, StartedAt: openrtc.Instant(started.UTC().Format("2006-01-02T15:04:05.000Z")), DurationMs: float64(clock().Sub(started)) / float64(time.Millisecond), EvidenceType: effectiveEvidence(target), Environment: target.Environment()})
	}
	results, required, declaration, inventoryRevision := materializeAssertions(target, results, declaration, clock())
	if inventoryLoadErr != nil {
		results = append(results, TestResult{TestID: "inventory-load", Suite: Core, Status: Incomplete, Reason: inventoryLoadErr.Error()})
		required = append(required, "inventory-load")
	}
	counts := map[ConformanceStatus]int{Pass: 0, Fail: 0, Incomplete: 0, NotApplicable: 0}
	for _, r := range results {
		counts[r.Status]++
	}
	complete := options.Only == nil && contains(suites, Core)
	claim := AssessClaim(effectiveEvidence(target), results, required, complete)
	es, gaps := evidenceAssessment(declaration, effectiveEvidence(target), clock())
	if es == Fail || (es == Incomplete && claim.Status != Fail) {
		claim.Status = es
	}
	claim.Reasons = append(claim.Reasons, gaps...)
	claim.UnqualifiedCoreClaim = claim.Status == Pass && declaration.Core
	claim.Scopes = scopedAssessments(declaration, results, claim)
	return ConformanceReport{TargetName: target.Name(), Author: target.Author(), EvidenceType: effectiveEvidence(target), Environment: target.Environment(), SpecificationNote: "OpenRTC v1.0; base 00d81f2 plus amended RFC content hash; sole normative source rfcs/openrtc-core.md", Complete: complete, Results: results, Counts: counts, Claim: claim, Declaration: declaration, InventoryRevision: inventoryRevision}

}
func FormatReport(r ConformanceReport) string {
	lines := []string{fmt.Sprintf("target: %s (author: %s)", r.TargetName, r.Author), fmt.Sprintf("evidence: %s", r.EvidenceType), fmt.Sprintf("counts: pass=%d fail=%d incomplete=%d not-applicable=%d", r.Counts[Pass], r.Counts[Fail], r.Counts[Incomplete], r.Counts[NotApplicable])}
	if r.Claim.UnqualifiedCoreClaim {
		lines = append(lines, "core claim: unqualified (self-attestation unless independently verified; ADP-124)")
	} else {
		lines = append(lines, "core claim: NONE. "+strings.Join(r.Claim.Reasons, "; "))
	}
	for _, scope := range r.Claim.Scopes {
		lines = append(lines, fmt.Sprintf("scope %s: %s -- %s", scope.Scope, scope.Status, strings.Join(scope.Reasons, "; ")))
	}
	for _, v := range r.Results {
		line := fmt.Sprintf("  [%s] %s (%s) %s", v.Status, v.TestID, strings.Join(v.Requirements, ", "), v.Title)
		if v.Status != Pass {
			line += " -- " + v.Reason
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

type ManifestResult struct {
	OperationCoverage []OperationClaim  `json:"operationCoverage,omitempty"`
	BindingSpecific   bool              `json:"bindingSpecific,omitempty"`
	ExclusionEvidence string            `json:"exclusionEvidence,omitempty"`
	IncompleteReason  string            `json:"incompleteReason,omitempty"`
	TestID            string            `json:"testId"`
	Requirements      []string          `json:"requirements"`
	Status            ConformanceStatus `json:"status"`
	Reason            string            `json:"reason"`
	Suite             SuiteName         `json:"suite"`
	Profile           ProfileName       `json:"profile"`
}
type AttestationManifest struct {
	Claim             ClaimAssessment        `json:"claim"`
	Declaration       ClaimDeclaration       `json:"declaration"`
	InventoryRevision string                 `json:"inventoryRevision"`
	Kind              string                 `json:"kind"`
	Author            string                 `json:"author"`
	EvidenceType      EvidenceType           `json:"evidenceType"`
	Environment       EnvironmentDescription `json:"environment"`
	GeneratedAt       openrtc.Instant        `json:"generatedAt"`
	Results           []ManifestResult       `json:"results"`
	Note              string                 `json:"note"`
}

func ToManifest(report ConformanceReport, now openrtc.Instant) AttestationManifest {
	results := make([]ManifestResult, 0, len(report.Results))
	for _, r := range report.Results {
		results = append(results, ManifestResult{TestID: r.TestID, Requirements: r.Requirements, Status: r.Status, Reason: r.Reason, Suite: r.Suite, Profile: r.Profile, ExclusionEvidence: r.ExclusionEvidence, IncompleteReason: r.IncompleteReason, BindingSpecific: r.BindingSpecific, OperationCoverage: r.OperationCoverage})
	}
	return AttestationManifest{Kind: "self-attestation", Author: report.Author, EvidenceType: report.EvidenceType, Environment: report.Environment, GeneratedAt: now, Results: results, Note: "Self-attestation by the adapter author; not independent certification (RFC §14; historical ADP-124).", Claim: report.Claim, Declaration: report.Declaration, InventoryRevision: report.InventoryRevision}
}

// AdapterIDOf is a typing convenience for tests that need an adapter ID.
type AdapterIDOf = openrtc.AdapterID

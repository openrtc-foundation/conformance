package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

type testAdapter struct{ openrtc.RTCAdapter }

func (testAdapter) Descriptor() openrtc.ProviderDescriptor {
	return openrtc.ProviderDescriptor{AdapterID: "test"}
}

type testCapabilities struct{ openrtc.RTCCapabilities }

func (testCapabilities) Snapshot() openrtc.CapabilitySnapshot { return openrtc.CapabilitySnapshot{} }

type testProvider struct{ openrtc.RTCProvider }

func (testProvider) Capabilities() openrtc.RTCCapabilities { return testCapabilities{} }
func (receiver testProvider) Dispose(context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(openrtc.StartOperation(receiver, func() (openrtc.CleanupReport, error) {
		return openrtc.CleanupReport{}, nil

	}))
}

type testRegistry struct{}

func (testRegistry) Register(openrtc.RTCAdapter) (openrtc.Registration, error) {
	return openrtc.Registration{}, nil
}
func (receiver testRegistry) Initialize(context.Context, openrtc.AdapterID, openrtc.ProviderConfig) openrtc.InitializationOperation {
	return testInitializationResult(receiver, openrtc.StartOperation(receiver, func() (openrtc.RTCProvider, error) {
		return testProvider{}, nil

	}))
}

type testTarget struct{}

var _ ConformanceTarget = testTarget{}

func (testTarget) Name() string                                        { return "test" }
func (testTarget) Author() string                                      { return "author" }
func (testTarget) Evidence() EvidenceType                              { return RealProvider }
func (testTarget) Environment() EnvironmentDescription                 { return EnvironmentDescription{} }
func (testTarget) Timing() TimingPolicy                                { return TimingPolicy{OperationMs: 1000} }
func (testTarget) CreateRegistry() openrtc.RTCAdapterRegistry          { return testRegistry{} }
func (testTarget) CreateAdapter() openrtc.RTCAdapter                   { return testAdapter{} }
func (testTarget) ProviderConfig() openrtc.ProviderConfig              { return openrtc.ProviderConfig{} }
func (testTarget) ClientConfig(ClientConfigInput) openrtc.ClientConfig { return openrtc.ClientConfig{} }
func (testTarget) Credentials() CredentialSource                       { return CredentialSource{} }
func (testTarget) ConferenceTarget(context.Context, ProvisioningModeName) (*openrtc.ConferenceTarget, error) {
	return nil, nil
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: result/evidence regression.
func TestAssessClaim(t *testing.T) {
	base := []TestResult{{TestID: "a", Suite: Core, Status: Pass}, {TestID: "b", Suite: Core, Status: NotApplicable, Reason: "explicit exclusion", ExclusionEvidence: "public capability revision 1"}, {TestID: "c", Suite: Capability, Status: Fail}}
	if AssessClaim(RealProvider, base, []string{"a", "b", "c"}, true).Status != Fail {
		t.Fatal("Capability failure must fail aggregate")
	}
	base = base[:2]
	if !AssessClaim(RealProvider, base, []string{"a", "b"}, true).UnqualifiedCoreClaim {
		t.Fatal("complete scoped core")
	}
	for _, status := range []ConformanceStatus{Fail, NotRun, Unknown} {
		r := append([]TestResult(nil), base...)
		r[0].Status = status
		if AssessClaim(RealProvider, r, []string{"a", "b"}, true).UnqualifiedCoreClaim {
			t.Fatal(status)
		}
	}
	for _, claim := range []ClaimAssessment{AssessClaim(Mock, base, nil, true), AssessClaim(RealProvider, base, []string{"missing"}, true), AssessClaim(RealProvider, base, nil, false), AssessClaim(RealProvider, base[1:2], nil, true), AssessClaim(RealProvider, nil, nil, true)} {
		if claim.UnqualifiedCoreClaim || len(claim.Reasons) == 0 {
			t.Fatal(claim)
		}
	}
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: result/evidence regression.
func TestRunnerVerdicts(t *testing.T) {
	tests := []ConformanceTest{}
	add := func(id string, run func(context.Context, *TestContext) error) {
		tests = append(tests, DefineTest(ConformanceTest{ID: id, Title: id, Suite: Core, Profile: Common, Requirements: []string{"REQ"}, Run: run}))
	}
	add("pass", func(context.Context, *TestContext) error { return nil })
	add("fail", func(_ context.Context, c *TestContext) error { return c.Fail("failure") })
	add("not-run", func(_ context.Context, c *TestContext) error { return c.NotRun("missing") })
	add("unknown", func(_ context.Context, c *TestContext) error { return c.Unknown("uncertain") })
	add("not-applicable", func(_ context.Context, c *TestContext) error { return c.NotApplicable("irrelevant") })
	add("cleanup", func(_ context.Context, c *TestContext) error {
		c.Track("resource", func(context.Context) error { return errors.New("broken") })
		return nil
	})
	add("partial", func(_ context.Context, c *TestContext) error { c.Partial("missing observer"); return nil })
	r := RunConformance(context.Background(), testTarget{}, tests, RunOptions{})
	want := []ConformanceStatus{Pass, Fail, Incomplete, Incomplete, Incomplete, Fail, Incomplete}
	for i, v := range r.Results[:len(want)] {
		if v.Status != want[i] {
			t.Errorf("%s: got %s", v.TestID, v.Status)
		}
	}
	if !strings.Contains(r.Results[5].Reason, "cleanup failed") || !strings.Contains(r.Results[6].Reason, "partial coverage") {
		t.Fatal(r.Results)
	}
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: result/evidence regression.
func TestCapabilityGatesAndFiltering(t *testing.T) {
	test := ConformanceTest{ID: "c", Suite: Capability, Profile: Common, Capability: &CapabilityDependence{ID: "camera"}}
	for _, tc := range []struct {
		name      string
		snapshot  []openrtc.CapabilityDescriptor
		available bool
		status    ConformanceStatus
	}{
		{"unavailable", nil, false, Unknown},
		{"missing", nil, true, Unknown},
		{"supported", []openrtc.CapabilityDescriptor{{ID: "camera", Support: openrtc.SupportDescription{Level: "supported"}}}, true, ""},
		{"partial", []openrtc.CapabilityDescriptor{{ID: "camera", Support: openrtc.SupportDescription{Level: "partial"}}}, true, ""},
		{"unsupported", []openrtc.CapabilityDescriptor{{ID: "camera", Support: openrtc.SupportDescription{Level: "unsupported"}}}, true, NotApplicable},
		{"not-applicable", []openrtc.CapabilityDescriptor{{ID: "camera", Support: openrtc.SupportDescription{Level: "not-applicable", Reason: "no camera"}}}, true, NotApplicable},
		{"unknown", []openrtc.CapabilityDescriptor{{ID: "camera", Support: openrtc.SupportDescription{Level: "unknown"}}}, true, Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, reason := capabilityGate(test, tc.snapshot, tc.available)
			if status != tc.status || (status != "" && reason == "") {
				t.Fatalf("got %q %q", status, reason)
			}
		})
	}
	core := ConformanceTest{ID: "core", Suite: Core, Profile: Common, Run: func(context.Context, *TestContext) error { return nil }}
	r := RunConformance(context.Background(), testTarget{}, []ConformanceTest{core, test}, RunOptions{Only: []string{}})
	if r.Complete || len(r.Results) == 0 || !strings.Contains(strings.Join(r.Claim.Reasons, ";"), "omitted") {
		t.Fatal(r)
	}
	r = RunConformance(context.Background(), testTarget{}, []ConformanceTest{core, test}, RunOptions{Suites: []SuiteName{Capability}})
	if r.Complete || len(r.Results) == 0 || r.Results[0].Status != Incomplete {
		t.Fatal(r)
	}
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: result/evidence regression.
func TestRunnerRedactionAndPanic(t *testing.T) {
	test := ConformanceTest{ID: "a", Title: "A", Suite: Core, Profile: Common, Requirements: []string{"R"}, Run: func(_ context.Context, c *TestContext) error {
		c.Needles = []string{"private-value"}
		c.Note("private-value")
		c.Track("resource", func(context.Context) error { return errors.New("private-value") })
		panic(errors.New("private-value"))
	}}
	r := RunConformance(context.Background(), testTarget{}, []ConformanceTest{test}, RunOptions{})
	got := r.Results[0]
	if got.Status != Fail || got.Reason != "unexpected exception: Error: [redacted]" || len(got.Observations) != 2 || strings.Contains(strings.Join(got.Observations, " "), "private-value") {
		t.Fatal(got)
	}
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: result/evidence regression.
func TestReportAndManifest(t *testing.T) {
	result := TestResult{TestID: "id", Title: "title", Suite: Core, Profile: Common, Requirements: []string{"R1", "R2"}, Status: Fail, Reason: "bad"}
	report := ConformanceReport{TargetName: "target", Author: "author", EvidenceType: RealProvider, Results: []TestResult{result}, Counts: map[ConformanceStatus]int{Fail: 1}, Claim: ClaimAssessment{Reasons: []string{"reason"}}}
	want := "target: target (author: author)\nevidence: real-provider\ncounts: pass=0 fail=1 incomplete=0 not-applicable=0\ncore claim: NONE. reason\n  [fail] id (R1, R2) title -- bad"
	if got := FormatReport(report); got != want {
		t.Fatalf("got %q", got)
	}
	m := ToManifest(report, "2026-01-01T00:00:00.000Z")
	if m.Kind != "self-attestation" || m.GeneratedAt != "2026-01-01T00:00:00.000Z" || len(m.Results) != 1 || m.Results[0].Reason != "bad" || !strings.Contains(m.Note, "ADP-124") {
		t.Fatal(m)
	}
}

type testInitialization struct {
	openrtc.Async[openrtc.RTCProvider]
}

func testInitializationResult(_ any, operation openrtc.Async[openrtc.RTCProvider]) openrtc.InitializationOperation {
	return testInitialization{operation}
}
func (testInitialization) Inspection() openrtc.OperationInspectionAccess { return nil }
func (testInitialization) Cleanup() openrtc.CleanupReport {
	return openrtc.CleanupReport{Status: openrtc.CleanupStatus{Kind: "complete"}}
}
func (i testInitialization) WatchCleanup(f func(openrtc.CleanupReport)) openrtc.Unsubscribe {
	f(i.Cleanup())
	return func() {}
}

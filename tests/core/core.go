package core

import "github.com/openrtc/conformance/harness"

// PlacementCapsErrorsTests contains the placement, capability, error, extension and media tests.
var PlacementCapsErrorsTests = append(append([]harness.ConformanceTest{}, placementCapsErrorsTests1...), placementCapsErrorsTests2...)

// CoreTests contains all mandatory core tests in execution order.
var CoreTests = func() []harness.ConformanceTest {
	var tests []harness.ConformanceTest
	tests = append(tests, RegistryConfigTests...)
	tests = append(tests, SessionEventsTests...)
	tests = append(tests, CredentialsCancelTests...)
	tests = append(tests, PlacementCapsErrorsTests...)
	tests = append(tests, LifecycleTests...)
	tests = append(tests, lifecycleMoreTests...)
	tests = append(tests, MediaContractTests...)
	tests = append(tests, MediaAdversarialTests...)
	tests = append(tests, EffectTests...)
	tests = append(tests, MediaShapeTests...)
	tests = append(tests, MediaPositiveTests...)
	tests = append(tests, LifecycleBoundTests...)
	tests = append(tests, SelectionCancellationTests...)
	tests = append(tests, ObservationContractTests...)
	tests = append(tests, PublicBindingTests...)
	tests = append(tests, FailureContractTests...)
	tests = append(tests, AuthorityContractTests...)
	tests = append(tests, CollisionContractTests...)
	seen := make(map[string]bool)
	for _, test := range tests {
		if seen[test.ID] {
			panic("duplicate conformance test id " + test.ID)
		}
		seen[test.ID] = true
	}
	return tests
}()

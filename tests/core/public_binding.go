package core

import "github.com/openrtc/conformance/harness"

// PublicBindingTests exercises equivalent required operations without imposing
// the Go bridge's production object structure. RFC §§7.0:45–65, 14.2:1109; A.0:1174.
var PublicBindingTests = []harness.ConformanceTest{harness.PublicBindingTest()}

# Harness integration

Implement `harness.ConformanceTarget` with fresh fixtures for the implementation under test, and implement `harness.ClaimTarget` to supply a `ClaimDeclaration` before selecting tests. The declaration identifies the specification and inventory revisions, artifacts, implementation, binding and environment, operations and support, profile versions, claimed boundaries, exclusions, and evidence. See [claims and evidence](conformance-claims.md).

The following entry point accepts a target supplied by an integration package and runs both catalogs:

```go
package integration

import (
    "context"

    "github.com/openrtc/conformance/harness"
    "github.com/openrtc/conformance/tests/capability"
    "github.com/openrtc/conformance/tests/core"
)

func Run(ctx context.Context, target harness.ConformanceTarget) harness.ConformanceReport {
    tests := append([]harness.ConformanceTest{}, core.CoreTests...)
    tests = append(tests, capability.CapabilityTests...)
    return harness.RunConformance(ctx, target, tests, harness.RunOptions{})
}
```

The target supplies the registry, adapter, configuration, credentials, conference targets, timing, and environment. Optional interfaces supply controls and observations for specific scenarios; absent required controls leave assertions incomplete. Use `harness.RealProvider` evidence only for actual provider execution. Filters in `RunOptions` are useful for investigation, but the runner accounts for applicable inventory assertions independently of the selected slice: a partial run cannot establish a complete claim.

The registry/provider/adapter machinery is a Go harness bridge. The RFC permits equivalent public operations and representations without that production architecture. Map actual public operations, observations, and evidence using the relevant harness interfaces, including `PublicBindingTarget` where appropriate; wrapper behavior cannot substitute for implementation behavior. Registry-only, descriptor/schema-name, native-ID, Once/Off, and inspection-ledger promises are binding-specific checks, reported separately from RFC scopes.

For operation-specific assertions, record the exact exercised context with `TestContext.RecordCoverage`. The report and manifest retain `operationCoverage`; a different operation, media kind, or owner scope cannot establish the claimed positive. See [media contracts](../media-contracts.md) for fixture and oracle details.

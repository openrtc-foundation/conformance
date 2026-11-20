# OpenRTC conformance suite

This Go 1.26 suite checks observable behavior against the sole normative document, `spec/rfcs/openrtc-core.md`. The RFC base is `00d81f2`; the amended RFC content hash is `780927ab73625607fa9530a5b6033a6cace3463a97f86ba8caf729d21a19a4f3`. The inventory revision is `v1-amended-780927ab`. Earlier requirement labels and CT IDs are historical aliases; an earlier passing result cannot establish a revised assertion.

**Status: harness self-tested only.** In-memory fake and mutation executions validate the harness. No real provider execution, service-boundary verification or published-client artifact audit is represented as completed.

## Running

```sh
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off GOSUMDB=off go build ./...
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off GOSUMDB=off go vet ./...
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off GOSUMDB=off go test ./...
gofmt -l .
```

`harness.ConformanceTarget` supplies fresh integration fixtures. Existing registry/provider/adapter machinery is a Go harness bridge; the RFC permits equivalent public operations and representations without that production architecture. Registry-only, descriptor/schema-name, native-ID, Once/Off and inspection-ledger promises are binding-specific checks, not universal obligations. A binding mapping must identify actual public operations, observations and evidence; wrapper behavior cannot stand in for implementation behavior.

The full Go catalog is `core.CoreTests` plus `capability.CapabilityTests`. Supply claim metadata and the identified inventory before selecting a subset. The runner must account for applicable inventory assertions independently of the selected test slice. A missing test or fixture is an incomplete assertion. A green unit run alone does not establish a complete Core or profile claim.

## Claims, applicability and evidence

Final statuses are `pass`, `fail`, `incomplete` and `not-applicable` (§14.2:1102–1117). Historical `not-run` and `unknown` execution conditions normalize to incomplete with their reason retained. Every exclusion identifies the assertion, reason and evidence; fixture absence is not exclusion evidence. An applicable failed assertion fails its claim regardless of its catalog. Missing required positive behavior, execution, artifact audit or current exact-boundary evidence makes the claim incomplete. Profile defining operations cannot all return unsupported and pass.

Core permits no media support. Mandatory Core positives and the optional-entry negative/security/ownership/cleanup checks still apply. Media positives apply to the exact advertised operation, media kind, combination and owner scope. Audio and video advertised separately do not establish simultaneous support. Publish with an authorized existing source does not require capture. Legacy common/conference/calling/conference-audiovisual labels organize existing tests and do not establish applicability.

Per-operation assertion contexts include the exact operation, owner scope, media kinds and conditions. Fixture `OperationCoverage` identifies the context actually exercised; another operation, a different kind or a different owner cannot supply its positive. Unsupported optional-operation exclusions require their reason and evidence. A declared media claim without the matching actual positive remains incomplete.

Media operation success establishes the RFC's operation postconditions. An explicit exchange claim additionally needs identifiable audio/video evidence at its declared direction, endpoint, interval and observation boundary; metadata or a `sending` flag does not prove receipt or playback. Optional controls with no equivalent fixture produce incomplete assertions, rather than successful or unsupported results.

External evidence identifies activity, direction, service boundary, mechanism, authority, source, version and validity. Missing, expired, withdrawn, contradicted or fake-only evidence cannot establish a real service boundary. Established nonconformance fails; unavailable evidence is incomplete. Published artifacts and the actual implementation, binding and environment revisions remain part of every release claim. The report and manifest retain scoped verdicts and evidence gaps. `ClaimTarget.ClaimDeclaration()` supplies revisions, operations/kinds/owner scopes/conditions, defined profile versions, exact claimed boundaries and current evidence before selection. Tests record exercised contexts with `TestContext.RecordCoverage`; the report and manifest retain `operationCoverage`. An assertion context without exact execution evidence stays incomplete. An explicitly unsupported optional Core positive needs the operation declaration’s `unsupportedReason` and `evidence`; its negative/security/cleanup checks remain independent. Binding-specific Go promises are reported separately from RFC scopes.

## Lifecycle and media oracles

Binding-documented finite cancellation, deadline-expiry, termination-fence and release-invocation bounds are distinct from the harness watchdog. Valid cancellation races local completion without waiting for native abort support. Cancelling a Go Result waiter is not operation cancellation or settlement. The suite checks portable results and stable repeated reads while provider completion remains blocked. Late admission cannot resurrect cancelled participation.

Local leave/dispose completion proves local terminal participation and initiation of owned release. It does not prove remote release. A cleanup request acknowledgment is distinct from independent release evidence. Pending, failed and unknown cleanup identifies its resource or bounded scope; established failure retains a safe cleanup error separately from the initiating failure. Later outcomes may evolve through documented retention. Repeated release may retry idempotently; native SDK call counts do not define conformance.

Snapshots preserve coverage and completeness for local and receive-source populations. Omission is not unknown; scope alone is not completeness. Call publications belong to the Call, and granted sources retain their actual owner. Independent publication mute and setSending suppression combine by union while source disable remains independent. Additional effects require bounded disclosure and explicit pre-effect acceptance; acceptance cannot waive target postconditions, authority or consent.

Safe portable failures use the ten §9 codes, established reasons and honest none/partial/unknown outcomes. Raw provider payloads, stacks and secret material are excluded from default errors, formatting, snapshots, events, logs and serialization. Optional unrestricted diagnostics require an explicit request within an authorized boundary.

## Inventory and matrix

[inventory/00d81f2.json](inventory/00d81f2.json) is the frozen baseline crosswalk (1,345 entries, baseline bytes reconstructed by reversing the documented exact RFC replacements). [inventory/v1-amended.json](inventory/v1-amended.json) binds 1,353 current obligations and assertion versions to exact RFC text and locators. Every MUST/MUST NOT and SHOULD decision is accounted for separately; normative operation-table positives, §14.3 scenarios and Appendix definitions also receive stable IDs. Binding-documentation, independent-service and published-artifact evidence remains explicit when not executable in Go. No historical requirement document is normative.

```sh
export OPENRTC_SPEC=/Users/victor/dev/openrtc/spec
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off go run ./cmd/matrix extract-requirements /private/tmp/openrtc-requirements.json
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off go run ./cmd/matrix test-coverage /private/tmp/openrtc-requirements.json /private/tmp/openrtc-tests.json
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off go run ./cmd/matrix check-requirement-ids /private/tmp/openrtc-requirements.json
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off go run ./cmd/matrix check-error-codes
GOCACHE=/private/tmp/openrtc-go-cache GOPROXY=off go run ./cmd/matrix generate-test-matrix /private/tmp/openrtc-requirements.json /private/tmp/openrtc-tests.json test-matrix.md
```

The default spec location is `../../../spec` from this module. Validation rejects changed RFC content, absent exact clauses, missing modal obligations, duplicate IDs and dangling executable mappings. Error-code validation reads only the ten-code §9 table. Matrix regeneration is deterministic. Mapping states (`tested`, `partial`, `missing`, `documentary`) describe assertion coverage, never execution; broad citations do not supply a coverage percentage or close unmapped obligations. All missing applicable assertions stay visible and incomplete in claims. SHOULD decisions require a recorded justification rather than silent omission.

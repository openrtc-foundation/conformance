# OpenRTC Conformance

OpenRTC Conformance is a Go test suite for checking an OpenRTC implementation's observable behavior against the OpenRTC Core RFC. It covers Core behavior and capability-dependent operations, including lifecycle, media, security, and evidence requirements. It is intended for implementation authors and evaluators preparing conformance claims.

## Current status

The harness has been self-tested with in-memory fakes and mutation tests. No real-provider execution, service-boundary verification, or published-client artifact audit is represented as complete. Passing the harness self-tests does not establish provider conformance.

## Quick start

Install Go 1.26 or later, clone the repository, and build and self-test the harness:

```sh
git clone https://github.com/openrtc-foundation/conformance.git
cd conformance
go build ./...
go vet ./...
go test ./...
```

The `inventory` and `cmd/matrix` tests read the RFC from `../../../spec/rfcs/openrtc-core.md`, resolved from the repository root. `OPENRTC_SPEC` does not change that path for tests. If the spec checkout is somewhere else, those two packages fail with a file-not-found error, and every other package runs normally.

These commands check the harness. They do not test a real provider.

## Testing an OpenRTC implementation

Integrate the implementation through a `harness.ConformanceTarget`, declare the claim and its evidence, and run the Core and capability catalogs against fresh fixtures. The [integration guide](docs/harness-integration.md) shows the Go entry point and explains the required mappings. There is no provider-testing command in this repository.

## Understanding test results

The RFC (§14.2) defines these final assertion and claim verdicts:

| Result | Meaning |
| --- | --- |
| `pass` | Applicable requirements are met with the required execution and evidence. |
| `fail` | An applicable assertion establishes nonconformance. |
| `incomplete` | An applicable assertion, required positive behavior, applicability decision, or evidence is missing or unknown. |
| `not-applicable` | The scope or assertion does not apply, with a justified exclusion. |

A missing fixture or evidence cannot count as a pass or justify an exclusion. The harness normalizes `not-run` and `unknown` execution conditions to `incomplete`, retaining the reason. See [claims and evidence](docs/conformance-claims.md) for claim-level rules.

## Conformance claims

Optional capabilities are evaluated according to their declared support; applicable negative, security, ownership, and cleanup checks still apply. A real-provider claim requires execution against the actual implementation and environment, an inventory of applicable assertions, and the required independent and artifact evidence. Results are self-attestations, not official certification. See [claims and evidence](docs/conformance-claims.md).

## Repository structure

- `harness/` — runner, fixtures, claim assessment, and evidence models.
- `tests/core/`, `tests/capability/` — executable test catalogs.
- `openrtc/` — Go interfaces and types used by the harness bridge.
- `selftest/`, `helpers/` — harness self-tests and supporting fixtures.
- `inventory/` — versioned requirement inventories.
- `cmd/matrix/` — inventory validation and matrix generation tools.

## Further documentation

- [Harness integration](docs/harness-integration.md)
- [Claims, applicability, and evidence](docs/conformance-claims.md)
- [Lifecycle and media test oracles](docs/lifecycle-and-media-testing.md)
- [Requirement inventory and maintenance](docs/requirement-inventory.md)
- [Test matrix](test-matrix.md) and [media contract assertions](media-contracts.md)
- [OpenRTC Core RFC](https://github.com/openrtc-foundation/spec/blob/main/rfcs/openrtc-core.md) — the normative source; use the revision identified in the inventory for this suite.

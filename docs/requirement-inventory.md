# Requirement inventory and maintenance

The sole normative source is `rfcs/openrtc-core.md` in the OpenRTC spec repository. The RFC base is `00d81f2`; the amended content SHA-256 is `780927ab73625607fa9530a5b6033a6cace3463a97f86ba8caf729d21a19a4f3`. The current inventory revision is `v1-amended-780927ab`.

[The baseline crosswalk](../inventory/00d81f2.json) contains 1,345 entries; its baseline bytes were reconstructed by reversing the documented exact RFC replacements. [The amended inventory](../inventory/v1-amended.json) binds 1,353 current obligations and assertion versions to exact RFC text and locators. Every MUST/MUST NOT and SHOULD decision is accounted for separately; normative operation-table positives, §14.3 scenarios, and Appendix definitions also have stable IDs. Earlier requirement labels and CT IDs are historical aliases; earlier passes cannot establish revised assertions. No historical requirement document is normative.

The [test matrix](../test-matrix.md) maps assertions to executable tests and documentary evidence. Mapping states `tested`, `partial`, `missing`, and `documentary` describe coverage, not execution. Broad citations do not supply a coverage percentage or close unmapped obligations. Binding documentation, independent service evidence, and published artifact evidence remain explicit when not executable in Go; missing applicable assertions remain incomplete in claims.

## Validate and regenerate

These commands assume the spec checkout is a sibling of this repository. `OPENRTC_SPEC` points to its root. Outputs other than the matrix are temporary. Without this variable the tool defaults to `../../../spec` from this module.

```sh
export OPENRTC_SPEC=../spec
workdir=$(mktemp -d)
go run ./cmd/matrix extract-requirements "$workdir/requirements.json"
go run ./cmd/matrix test-coverage "$workdir/requirements.json" "$workdir/tests.json"
go run ./cmd/matrix check-requirement-ids "$workdir/requirements.json"
go run ./cmd/matrix check-error-codes
go run ./cmd/matrix generate-test-matrix "$workdir/requirements.json" "$workdir/tests.json" "$workdir/test-matrix.md"
```

Compare the generated matrix with [test-matrix.md](../test-matrix.md) before updating it. Before submitting changes, also run `gofmt -l .`; it should print nothing. Validation rejects changed RFC content, absent exact clauses, missing modal obligations, duplicate IDs, and dangling executable mappings. Error-code validation reads only the ten-code §9 table. Matrix generation is deterministic.

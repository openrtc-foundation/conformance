# Claims, applicability, and evidence

The normative rules are in OpenRTC Core RFC §§7.0 and 14. A claim identifies the specification and inventory revisions, published artifacts, actual implementation, binding, environment, operations and supported scope, profile versions and dependencies, and evidence. Runtime evidence identifies the implementation and environment tested. A release claim also audits the inventory and known gaps and demonstrates that published client interfaces expose neither trusted-server secrets nor unauthorized administration. Results are self-attestations, not certification.

Every applicable normative assertion must be accounted for, regardless of which catalog contains it. An applicable failure fails its claim. Missing execution, a required positive, an applicability decision, artifact audit, or current exact-boundary evidence makes it incomplete. Each exclusion identifies the assertion, reason, and evidence; fixture absence is not exclusion evidence. Profile defining operations cannot all return unsupported and pass. SHOULD decisions need recorded justification, not silent omission. The report and manifest retain scoped verdicts and evidence gaps.

## Optional operations

Core permits no media support. Mandatory Core positives and applicable optional-entry negative, security, ownership, and cleanup checks still apply. Positive media assertions apply to the exact advertised operation, media kind, combination, owner scope, and conditions. Separately advertised audio and video do not establish simultaneous support. Publishing an authorized existing source does not require capture. The legacy common, conference, calling, and conference-audiovisual labels organize tests; they do not establish applicability.

`ClaimTarget.ClaimDeclaration()` supplies operation declarations and evidence before test selection. An explicitly unsupported optional Core positive needs the declaration's `unsupportedReason` and `evidence`; its other applicable checks remain independent. A declared media claim without its matching actual positive is incomplete. Optional controls without an equivalent fixture also leave assertions incomplete.

## Evidence boundaries

Successful media operations establish their RFC postconditions. An explicit exchange claim additionally requires identifiable audio or video evidence at the declared direction, endpoint, interval, and observation boundary. Metadata or a `sending` flag does not prove receipt or playback.

External enforcement evidence identifies activity, direction, service boundary, mechanism, authority, source, applicable version, and validity. Missing, expired, withdrawn, contradicted, or fake-only evidence cannot establish a real service boundary. Unavailable evidence is incomplete; established nonconformance fails. Evidence is current only within its validity period and while not withdrawn or contradicted. The [integration guide](harness-integration.md) describes recording exercised contexts.

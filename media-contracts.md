# Current media contract assertions

The sole normative source is `rfcs/openrtc-core.md`, base `00d81f2` with amended SHA-256 `780927ab73625607fa9530a5b6033a6cace3463a97f86ba8caf729d21a19a4f3`. Assertion versions and exact clauses are in [inventory/v1-amended.json](inventory/v1-amended.json); [test-matrix.md](test-matrix.md) records the complete executable catalog and residual gaps. Earlier MED/IDL/STM/EXT references are historical aliases. They are not normative documents or transferable evidence for revised assertions.

These are harness self-tests against conforming and deliberately defective fakes. No fake pass establishes real-provider interoperability, service enforcement, playback or an artifact audit. Applicable missing controls or unknown independent observations remain incomplete.

## Applicability and evidence

Core can support no media. Optional positive tests run only for the exact supported operation, kind, requested combination and owner scope. Mandatory unsupported-entry, security, ownership and cleanup assertions remain applicable. A fixture for authorized publication of an existing external source does not require capture. Independently advertised audio/video support does not establish joint support. Calling media does not require a fabricated conference Session.

Operation success is checked through public postconditions. An explicit exchange claim additionally needs identifiable source content at the declared endpoint, direction, interval and boundary. `sending=true`, a publication record, provider acceptance or a local synthetic counter does not establish remote receipt or playback. Source suppression concerns source content; it does not imply packet silence. Instrumented effect counts establish only the fixture's declared boundary.

Unknown values are inspected by their status and value together. Unknown suppression cannot satisfy a required known-false observation, omitted population metadata is not unknown, and a scope is not completeness. Independent observation unavailable to the harness is incomplete; an established violation of a successful postcondition fails.

## Revised contract groups

| Assertions | Current requirement | Decisive public behavior |
|---|---|---|
| CT-MOP-001; capability capture cases | §7.4:271–289; §7.0:45–65 | Supported capture returns an owned source and causes no implicit join or publication; the chosen media kind is passed through setup. Application-owned capture is separately advertised. |
| CT-MOP-002–004/020 | §7.4:262–264; §8.1:448–451; A.1:1308 | setSending targets an existing publication. false/true change only its convenience mute intent, retain identity/source and independent mute/disable intents, and do not acquire media. Invalid/foreign/terminal targets fail before effects. |
| CT-MOP-005/006 | §7.4:310–348; §8.1:448–451 | Unmute and enable remove only the named intent. Required false facts must be known false; the effective union and source-disable restriction remain independent. |
| CT-MOP-007/008 | §7.3; §7.4; §11.2 | The complete requested kind combination is assessed; unsupported combinations do not acquire or select media. Selection is distinct from receipt/rendering. |
| CT-MOP-009/010/018 | §7.4; §8.4–8.6 | Render/detach respect their own scope and terminal fence. Unknown/blocked playback stays unknown/blocked; late remote removal cannot revive an attachment. Local closure is separate from remote cleanup. |
| CT-MOP-011–013 | §7.4:291–376; §14.2:1109 | Independently controllable publications have distinct public identities. Publish does not reacquire its existing source; unpublish retains that source and its disable state. Partial failures identify known survivors and honest effects. Native IDs or exactly two native publications are not universal requirements. |
| CT-MOP-014–016 | §8.6:506–510; §9:568 | Required cleanup/error results remain accessible under documented retention. A specific inspection ledger or eviction schema is a Go bridge promise, not an RFC requirement. |
| CT-MOP-017/019 | §7.1:69–88; §7.4; §9:529–570 | A setter cannot treat stale/unestablished evidence as successful admission. Unsupported media rejects before effects with a current safe code, reasons and outcome. |
| CT-EFFECT-001–006 | §7.1:86–88; §7.4:310–323; §14.3:1129 | Bounded possible effects need exact prior acceptance. Wrong/unknown scope or action cannot authorize effects. Acceptance cannot waive target postconditions, eligibility, authority or consent. Known inseparable release effects are accepted before acquisition; undisclosed late coupling retains identified unresolved cleanup instead of claiming release. |
| CT-SHAPE-001 | §11.1:592; A.1:1241–1242; A.3:1339 | Call-scoped publication belongs to its Call; a granted source retains its real owner and survives another owner's hangup. |
| CT-SHAPE-002/003 | §7.4:282; §8.3:469–477; A.1:1248–1255/1290 | Source-covering initial snapshots include unpublished sources and their states. Local/receive populations carry explicit completeness, preserving complete-empty/incomplete/unknown through serialization. |
| CT-MEDIA-EXCHANGE-001 | §§7.3:180, 14.1:1088–1098, 14.2:1108–1115 | An explicit media-exchange claim needs identifiable content at its exact declared peer boundary; missing evidence cannot be filled by metadata. |
| CT-MADV and CT-SECRET groups | §§9, 12.1–12.3, 14.3:1149 | Default public errors, nested values, formatting, snapshots, events, logs and serialization exclude secrets and raw provider payloads. Unrestricted diagnostics require an explicit authorized access path. |
| CT-CALL-POS-001–005; CT-HOLD-001; CT-CALL-POLICY-001–003; CT-DIGIT-001 | §11.1:590–647; A.3:1342/1363–1364 | Actual dial/accept/reject/hangup/hold operations establish public Call facts. Unknown remote hold retains the answer; optional transfer is tested only when declared. Registration respects account/expiry/refresh and disposed-owner fences. Digit failure/fallback reports a truthful prefix without duplicate submission. |
| CT-SUB-001–004; CT-GRANT-001–004; CT-SCREEN-001–003 | §§11.2–11.4; §14.3:1139 | Subscription release preserves the remote publication and separately retained reception. Source grants bind source/recipient/scope and revoke idempotently. Replacement preserves mute/disable intents and ownership, rejects unknown required identity continuity before effects, and retains partial survivors. Screen acquisition/selection changes do not imply publication. |
| CT-REC-001–005; CT-ENC-001–003 | §§11.6/11.8; §14.3:1136–1137 | Recording start needs established recording and consent; local stop acceptance does not establish service stop. An authorized independent job follows its own owner. Encryption verifies exact declared content, directions, sources, endpoints, exclusions and current boundary evidence; verification loss contains further affected content. |
| CT-PROFILE-OBS-001 | §7.0:63; §8.3:467–477; A.3:1335–1339/1425 | Every claimed profile's actual public observation path supplies exact-scope coverage, ordered changes, loss/resnapshot and independent removal. Missing per-profile mapping/evidence is incomplete. |
| CT-DEVICE-001–003 | §11.10:963–1005 | Enumeration has scoped permission/completeness and no acquisition. Selection establishes the exact authorized target without implicit capture/publication. Statistics identify scope, interval, unit and known/unknown values/time; cached collection time is retained. |
| CT-PLC-PROFILE-001–004; CT-COMB-001 | §11.7:831–873; A.3:1386–1388 | Required constraints intersect without preferences. Preferences correlate to requested entries and report selection/reason/unknown. Assessment is not enforcement; exact current boundary evidence is required. Unknown/pending requirements and unsupported combinations fail before protected effects; recovery and runtime application preserve constraints. |

## Justification for revisions

The former acquisition interpretation of setSending was replaced by existing-publication suppression because §7.4:262–264 explicitly defines that operation. Its old source-ownership normalization oracle was replaced with publication-target eligibility and effect-acceptance checks. These CT aliases now refer to revised assertions; old passes are not current evidence.

Successful mute retains the publication and source (§7.4:310–323). A declaration that a stopped source can later be reacquired does not satisfy retention. Device-level acquisition may have a separate accepted effect without ending the LocalSource; the tests do not impose a universal device-open requirement.

Native SDK call-count limits, frozen cleanup-report equality, mandatory NativeID exposure and exactly-two-native-publication assumptions were removed from universal applicability (§14.2:1109). Repeated release remains locally bounded and terminal; truthful cleanup progress may evolve through declared retention (§§7.2, 8.6). Request acceptance alone cannot establish released remote state.

Obsolete error-code expectations were revised to §9's ten core codes and established reasons/outcomes. Default native-original identity preservation was removed as an oracle because default diagnostics must exclude unrestricted provider payloads. Raw diagnostics, if offered, are tested only through their explicit authorized path (§12.1).

Unknown statistics collection time and definition-valid negative metrics are retained as legitimate positives (§11.10:999). Mandatory parseable timestamps and nonnegative values for every metric were removed. Empty option/preference observations and unknown/nil values cannot serve as proof that a requested condition was processed or known false.

Current inventory entries without a complete executable or independent-evidence mapping remain explicitly missing/partial/documentary. Fixture-only passes do not close service-boundary, published-artifact or unobserved-surface obligations.

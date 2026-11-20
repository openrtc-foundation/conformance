// Package capability contains the capability-dependent conformance test catalog.
// Tests are selected by advertised capability descriptors.
package capability

import "github.com/openrtc/conformance/harness"

// CapabilityTests contains the capability-dependent catalog.
var CapabilityTests = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-MED-001", Title: "capturing alone publishes nothing", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"MED-001", "MED-003", "MED-006", "MED-104"},
		Observable:   "after capture(): source state acquiring/active with the requested ownership; no new publication in the snapshot or native layer", Needs: []harness.FixtureNeed{"media-devices"},
		Capability: &harness.CapabilityDependence{ID: "media.capture", AppliesWhen: "supported-or-partial"}, Run: run(test1),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-MED-002", Title: "mute suppresses content without unpublishing, releasing capture or touching receive", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"RFC §7.4:310–323; §8.1:448–451", "MED-001", "MED-101", "MED-106", "STM-005", "MED-103"},
		Observable:   "setMuted(true): outbound content suppressed, publication still published/interrupted, capture not stopped, receive unchanged; setMuted(false) re-enables", Needs: []harness.FixtureNeed{"media-devices"},
		Capability: &harness.CapabilityDependence{ID: "media.capture", AppliesWhen: "supported-or-partial"}, Run: run(test2),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-MED-003", Title: "disable is reversible; stop is irreversible", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"MED-006", "MED-101", "MED-107", "MED-111", "STM-112"},
		Observable:   "setDisabled toggles the source between disabled and active; after stop() the source is stopped and setDisabled rejects invalid-state", Needs: []harness.FixtureNeed{"media-devices"},
		Capability: &harness.CapabilityDependence{ID: "media.capture", AppliesWhen: "supported-or-partial"}, Run: run(test3),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-MED-004", Title: "session disposal releases session-owned capture and spares application-owned capture", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"MED-006", "MED-117", "CORE-114", "STM-013"},
		Observable:   "after session.dispose(): the session-owned source is stopped; the application-owned source is still active and can still be stopped by its owner", Needs: []harness.FixtureNeed{"media-devices"},
		Capability: &harness.CapabilityDependence{ID: "media.capture", AppliesWhen: "supported-or-partial"}, Run: run(test4),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-PUB-001", Title: "independent publish reuses a capture; unpublish need not release it", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"MED-001", "MED-003", "MED-101", "MED-105", "MED-108", "EXT-001"},
		Observable:   "publish() yields publications without acquiring a device; unpublish(retain) removes it and leaves the source active", Needs: []harness.FixtureNeed{"media-devices"},
		Capability: &harness.CapabilityDependence{ID: "media.publish.independent", AppliesWhen: "supported-or-partial"}, Run: run(test5),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-SUB-001", Title: "manual unsubscribe stops reception but does not remove the remote publication", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"RFC §11.2:656–667", "MED-002", "MED-103", "MED-109", "STM-106"},
		Observable:   "subscribe() creates a client-authority subscription; unsubscribe() inspection reports receive not selected; the remote publication is still listed", Needs: []harness.FixtureNeed{"remote-endpoint"},
		Capability: &harness.CapabilityDependence{ID: "media.subscribe.manual", AppliesWhen: "supported-or-partial"}, Run: run(test6),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-STAT-001", Title: "statistics samples are qualified, in range and not from the future", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"RFC §11.10:997–1005", "MED-120", "MED-121"},
		Observable:   "target echoed; valid coverage; collection time known or unknown; known collectedAt parseable and not in the future; known measurements finite with metric-specific ranges", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "media.statistics", AppliesWhen: "supported-or-partial"}, Run: run(test7),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-DEV-001", Title: "device enumeration has unique scoped ids and does not capture", Suite: harness.Capability, Profile: harness.Common,
		Requirements: []string{"MED-118", "CORE-005"},
		Observable:   "descriptors with unique ids; no native capture while enumerating", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "devices.enumerate", AppliesWhen: "supported-or-partial"}, Run: run(test8),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-REC-001", Title: "starting a recording returns a job whose status does not claim completion", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"RFC §11.6:784–826", "EXT-108", "EXT-103", "STM-113"},
		Observable:   "start() yields a MediaJob with an id and an early status; get(id) returns the same job; stop() resolves a defined outcome", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "recording.server", AppliesWhen: "supported-or-partial"}, Run: run(test9),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-REC-002", Title: "a recording without authority fails authorization-denied and starts no job", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"CAP-004", "EXT-002", "ADP-117", "ERR-109"},
		Observable:   "after revoking authority, start() rejects authorization-denied", Needs: []harness.FixtureNeed{"authority-control"},
		Capability: &harness.CapabilityDependence{ID: "recording.server", AppliesWhen: "supported-or-partial"}, Run: run(test10),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-MSG-001", Title: "message receipts do not promote local acceptance to delivery", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"EXT-111", "CAP-001", "CORE-008"},
		Observable:   "receipt has a defined delivery/ordering/durability; ephemeral send is not durable; a durable request either rejects or reports persisted", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "messaging.ephemeral", AppliesWhen: "supported-or-partial"}, Run: run(test11),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-ENC-001", Title: "encryption never reports weaker coverage than requested and never exposes keys", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"RFC §11.8:879–945", "EXT-112", "EXT-113", "CAP-003", "CFG-009"},
		Observable:   "an effective outcome has the requested layer; end-to-end claims carry evidence and trusted parties; key bytes appear nowhere observable", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "encryption.media", AppliesWhen: "supported-or-partial"}, Run: run(test12),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-ADM-001", Title: "delegated client administration cannot end a session it is not authorized to end", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"EXT-002", "EXT-115", "SES-005", "SES-119", "ADP-117"},
		Observable:   "when the descriptor does not say allowed, end() rejects authorization-denied or unsupported-capability and a remote endpoint stays connected", Needs: []harness.FixtureNeed{"remote-endpoint"},
		Capability: &harness.CapabilityDependence{ID: "session.end", AppliesWhen: "supported-or-partial"}, Run: run(test13),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-ADM-002", Title: "trusted end-instance ends the run without claiming definition deletion", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"SES-005", "SES-119", "EXT-116", "ADP-111"},
		Observable:   "end(instance) outcome effects include instance-ended, exclude definition-deleted, and the remote endpoint is disconnected", Needs: []harness.FixtureNeed{"admin", "remote-endpoint"},
		Capability: &harness.CapabilityDependence{ID: "session.end", AppliesWhen: "supported-or-partial"}, Run: run(test14),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-CALL-001", Title: "an outgoing call is a call session with legs, not a room or roster", Suite: harness.Capability, Profile: harness.Calling,
		Requirements: []string{"RFC §11.1:590–630", "EXT-105", "EXT-106", "CORE-004", "SES-109", "SES-120", "ADP-107"},
		Observable:   "dial() returns kind call with no join; roster not-applicable; terminate() reports accepted vs confirmed; terminating an ended leg rejects invalid-state", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "calling.dial", AppliesWhen: "supported-or-partial"}, Run: run(test15),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-CALL-002", Title: "an incoming call is delivered before it is accepted and rejects as a leg operation", Suite: harness.Capability, Profile: harness.Calling,
		Requirements: []string{"EXT-105", "EXT-106", "STM-007", "STM-110"},
		Observable:   "incomingCall delivers a call session whose leg is 'incoming'; reject() resolves a defined outcome; no conference join is involved", Needs: []harness.FixtureNeed{"incoming-call"},
		Capability: &harness.CapabilityDependence{ID: "calling.accept", AppliesWhen: "supported-or-partial"}, Run: run(test16),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-TEL-001", Title: "telephony controls are gated per operation by their own descriptors", Suite: harness.Capability, Profile: harness.Calling,
		Requirements: []string{"EXT-107", "EXT-001", "CAP-004"},
		Observable:   "DTMF send and DTMF receive each reject unsupported-capability exactly when their own descriptor is unsupported; dial authorization grants none of them", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "calling.dial", AppliesWhen: "supported-or-partial"}, Run: run(test17),
	}),
	harness.DefineTest(harness.ConformanceTest{
		ID: "CT-CAPX-CRD-001", Title: "credential lifecycle fields are advertised before any renewal is attempted", Suite: harness.Capability, Profile: harness.Conference,
		Requirements: []string{"CFG-115", "SES-009", "STM-111"},
		Observable:   "credential-channel discovery has a lifecycle with a replacementMode and phase-specific expiry effects, and its state/validity fields are valid", Needs: []harness.FixtureNeed{},
		Capability: &harness.CapabilityDependence{ID: "session.conference", AppliesWhen: "supported-or-partial"}, Run: run(test18),
	}),
}

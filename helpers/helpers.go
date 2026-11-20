// Package helpers contains provider-independent conformance helpers.
package helpers

import "github.com/openrtc/conformance/openrtc"

// ClientLifetimes are the five client lifetimes (CFG-001).
var ClientLifetimes = []openrtc.ConfigurationLifetime{openrtc.ProviderInit, openrtc.ClientInit, openrtc.SessionCreate, openrtc.ParticipantJoin, openrtc.Runtime}

const ForeignAdapter openrtc.AdapterID = "openrtc-conformance-foreign-adapter"

// ForgedPayload was not produced by schema validation (CFG-003).
func ForgedPayload(id openrtc.AdapterID, lifetime openrtc.ExtensionLifetime, schemaID ...string) openrtc.ValidatedExtensionPayload {
	schema := "openrtc-conformance/forged"
	if len(schemaID) > 0 {
		schema = schemaID[0]
	}
	return openrtc.ValidatedExtensionPayload{AdapterID: id, Lifetime: lifetime, SchemaID: schema, SchemaRevision: "0", Payload: map[string]any{"conformanceProbe": true}}
}
func ForgedExtensions(id openrtc.AdapterID, lifetime openrtc.ExtensionLifetime, payloadLifetime ...openrtc.ExtensionLifetime) openrtc.ProviderExtensions {
	if len(payloadLifetime) > 0 {
		lifetime = payloadLifetime[0]
	}
	return openrtc.ProviderExtensions{id: ForgedPayload(id, lifetime)}
}
func WithStrayField(holder map[string]any, name string, value any) map[string]any {
	out := make(map[string]any, len(holder)+1)
	for k, v := range holder {
		out[k] = v
	}
	out[name] = value
	return out
}
func SentinelRegion(id openrtc.AdapterID) openrtc.RegionRef {
	return openrtc.RegionRef{Namespace: string(id), Code: "openrtc-conformance-no-such-region"}
}

var PlacementKeys = []string{"signalingRegion", "mediaRegion", "recordingProcessingRegion", "recordingStorageRegion", "deploymentRegion", "dataRegion"}

func Constraint(strength string, regions []openrtc.RegionRef) openrtc.PlacementConstraint {
	return openrtc.PlacementConstraint{Strength: strength, Regions: regions, Boundary: openrtc.PlacementBoundary{Resources: []string{"media-processor"}, Transit: "excluded"}}
}
func PolicyFor(plane string, v openrtc.PlacementConstraint) openrtc.PlacementPolicy {
	classes := map[string][]string{"signalingRegion": {"signaling-endpoint"}, "mediaRegion": {"media-processor", "media-relay"}, "recordingProcessingRegion": {"recording-worker"}, "recordingStorageRegion": {"recording-output"}, "deploymentRegion": {"deployment-component"}, "dataRegion": {"application-data"}}
	v.Boundary.Resources = classes[plane]
	p := openrtc.PlacementPolicy{}
	switch plane {
	case "signalingRegion":
		p.SignalingRegion = &v
	case "mediaRegion":
		p.MediaRegion = &v
	case "recordingProcessingRegion":
		p.RecordingProcessingRegion = &v
	case "recordingStorageRegion":
		p.RecordingStorageRegion = &v
	case "deploymentRegion":
		p.DeploymentRegion = &v
	case "dataRegion":
		p.DataRegion = &v
	}
	return p
}

// LegalConnectionEdges is the STM-101 state machine table.
var LegalConnectionEdges = map[string][]string{"idle": {"preparing", "joining", "leaving"}, "preparing": {"joining", "leaving", "failed"}, "joining": {"connected", "leaving", "ended", "failed"}, "connected": {"recovering", "leaving", "ended", "failed"}, "recovering": {"connected", "leaving", "ended", "failed"}, "leaving": {"ended", "failed"}, "ended": {}, "failed": {}}

func IsTerminal(state openrtc.ConnectionState) bool {
	return state == "ended" || state == "failed"
}

var PortableCapabilities = []string{"session.conference", "session.admission.existing-only", "session.create", "session.end", "session.disable", "session.delete", "session.remove", "calling.dial", "calling.receive", "calling.accept", "calling.reject", "calling.terminate", "calling.provisional", "calling.reject.reason", "calling.transfer.respond", "telephony.hold", "telephony.resume", "telephony.transfer.blind", "telephony.transfer.attended", "telephony.dtmf.send", "telephony.dtmf.receive", "media.capture", "media.send", "media.publish.independent", "media.mute", "media.disable", "media.subscribe.manual", "media.receive-policy", "media.render", "media.playback-mute", "media.playback-control", "media.raw-track", "media.replace-source", "devices.enumerate", "devices.select-input", "devices.select-output", "media.screen-share", "media.statistics", "media.observe.active-speaker", "media.observe.network-quality", "messaging.ephemeral", "recording.server", "encryption.media", "encryption.data"}

// Predicate is the legacy binding predicate.
type Predicate = openrtc.CapabilityPredicate

func EvaluatePredicate(p Predicate) string {
	if p.Kind == "atom" {
		return p.Evaluation
	}
	if p.Kind == "not" {
		if p.Term == nil {
			return "unknown"
		}
		switch EvaluatePredicate(*p.Term) {
		case "satisfied":
			return "unsatisfied"
		case "unsatisfied":
			return "satisfied"
		default:
			return "unknown"
		}
	}
	all := true
	for _, term := range p.Terms {
		v := EvaluatePredicate(term)
		if p.Kind == "all" && v == "unsatisfied" {
			return "unsatisfied"
		}
		if p.Kind != "all" && v == "satisfied" {
			return "satisfied"
		}
		if (p.Kind == "all" && v != "satisfied") || (p.Kind != "all" && v != "unsatisfied") {
			all = false
		}
	}
	if all {
		if p.Kind == "all" {
			return "satisfied"
		}
		return "unsatisfied"
	}
	return "unknown"
}

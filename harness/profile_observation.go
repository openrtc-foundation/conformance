package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// ProfileObservationTarget maps each claimed profile's actual public initial
// state, event stream, snapshot and removal representation. This projection is
// harness plumbing, not required production syntax. Commit/LoseNext are fixture
// inputs; they cannot synthesize the observations being measured.
// RFC §8.3:467–477, §7.0:63, A.3:1335–1339/1425.
type ProfileObservationTarget interface {
	ProfileObservation(context.Context, string) (ProfileObservationFixture, error)
}
type ProfileObservationFixture struct {
	Profile, Scope, PublicRepresentation, Documentation string
	// Stateful selects a retained state change with a meaningful snapshot view.
	// Purely transient streams (e.g. messages) must not invent retained payloads.
	Stateful   bool
	Capability func(context.Context) (openrtc.CapabilityDescriptor, error)
	Observe    func(context.Context, func(ProfileObservationUpdate)) (openrtc.Unsubscribe, error)
	Snapshot   func(context.Context) (ProfileObservationState, error)
	// Commit waits for controlled input and a public delivery checkpoint;
	// an unavailable checkpoint must be returned as missing evidence.
	Commit   func(context.Context, string) error
	LoseNext func(context.Context) error
}
type ProfileObservationState struct {
	Scope, Completeness string
	Facts               map[string]string
}
type ProfileObservationUpdate struct {
	Snapshot *ProfileObservationState
	// Marker and Related are exact semantic projections of controlled public
	// facts, not mandatory wire fields, event names, sequences or revisions.
	Marker        string
	Related       map[string]string
	Loss, Current bool
}

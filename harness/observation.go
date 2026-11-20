package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// ObservationControl commits an established participant display-name fact at a
// covered boundary and deliberately loses one local delivery. Commit returns
// after queued delivery. Marker must be visible in the public roster snapshot.
// This is fixture instrumentation, not a required production API (§14.2:1111).
type ObservationControl interface {
	Commit(context.Context, openrtc.RTCConferenceSession, string) error
	LoseNext(context.Context, openrtc.RTCConferenceSession) error
}
type ObservationTarget interface{ ObservationControl() ObservationControl }

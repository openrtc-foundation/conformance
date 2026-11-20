package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// MediaContractTarget is optional. Prepare establishes a media-capable device,
// eligible per-kind media and an
// isolated render sink whose playback is blocked. It changes fixture conditions, never invokes app verbs.
type MediaContractTarget interface{ MediaContracts() MediaContractControl }
type MediaContractControl interface {
	Prepare(context.Context, openrtc.RTCConferenceSession) error
	RenderTarget() openrtc.RenderTarget
	// Condition establishes the named condition before the next public invocation:
	// partial-publish: first native publication commits, second fails;
	// unsupported-receive: automatic/automatic is unsupported;
	// unsupported: acquisition unsupported, existing releases remain authorized;
	// unknown-with-channel: unknown support with a nominal resolution channel;
	// denied-with-pending: established authorization denial alongside pending support;
	// stale-mute: suppressing observation becomes last-known and native suppression
	// is lost; next suppression attempt fails media-failure;
	// detach-failure: native detach fails after local fencing.
	Condition(context.Context, openrtc.RTCConferenceSession, string) error
	Effects() MediaFixtureEffects
	CompleteCleanup(context.Context) error
	// GateRender holds native attachment; Reached proves admission before removal.
	GateRender(openrtc.RTCConferenceSession) JoinGate
	RemotePublication(context.Context, openrtc.RTCConferenceSession) (openrtc.PublicationID, error)
	UnpublishRemote(context.Context, openrtc.RTCConferenceSession, openrtc.PublicationID) error
	// EvictSuperseded requests permitted pruning only after every key is superseded.
	EvictSuperseded(openrtc.RTCConferenceSession, openrtc.OperationID) error
}
type MediaFixtureEffects struct{ Captures, Publications, Mutations, Attachments, Detachments int }

// MediaKindControl selects individually supported kinds at the eligible session
// scope using public capability facts. Individual entries never establish joint
// audio/video support. Fixture setup remains separate from the subject operation.
// RFC §7.3:175–188, §7.4:245–293, §14.1:1092–1098.
type MediaKindControl interface {
	Kinds(context.Context, openrtc.RTCConferenceSession, string) ([]openrtc.MediaKind, error)
}

// EligibleSourceControl permits supported publish without requiring capture.
// The source is already authorized by its actual owner; acquisition is not part
// of the publish operation being tested. RFC §7.4:293–310, §11.3:669–682.
type EligibleSourceControl interface {
	EligibleSource(context.Context, openrtc.RTCConferenceSession, openrtc.MediaKind) (openrtc.CaptureSource, error)
}

// MediaOutputEvidence is independent observation at the claimed boundary, never
// an adapter sending-state assertion. No playback or packet-silence claim follows.
type MediaOutputEvidence struct {
	Boundary, Source, Kind, Direction, Interval, Evidence string
	Status                                                string // observed, absent, unknown
	Content                                               string // identifiable synthetic content token
}
type MediaOutputControl interface {
	ObserveOutput(context.Context, openrtc.RTCConferenceSession, openrtc.CaptureSourceID, openrtc.MediaKind) (MediaOutputEvidence, error)
}
type MediaExchangeTarget interface{ MediaExchange() MediaExchangeControl }
type MediaExchangeControl interface {
	ClaimedBoundaries() []MediaOutputEvidence
	Exchange(context.Context, MediaOutputEvidence) (MediaOutputEvidence, error)
}

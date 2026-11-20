package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// BoundaryLossTarget supplies an initially enforced required media boundary from
// service policy, then withdraws its enforceability while offering a native
// outside-boundary fallback. The control waits for callback processing.
type BoundaryLossTarget interface{ BoundaryLoss() BoundaryLossControl }
type BoundaryLossControl interface {
	Establish(context.Context, openrtc.RTCConferenceSession) error
	Withdraw(context.Context, openrtc.RTCConferenceSession) error
	FallbackEffects() int
}

// LeakageTarget injects canaries into native diagnostics, credentials, snapshots
// and a schema-valid adapter diagnostic extension. DefaultLogs must capture the
// implementation's default logger; unavailable logs return an error (not-run).
type LeakageTarget interface{ Leakage() LeakageControl }
type LeakageControl interface {
	Inject(context.Context, openrtc.RTCConferenceSession, []string, any) error
	DefaultLogs(context.Context) ([]string, error)
	DiagnosticKey() openrtc.ExtensionKey
}

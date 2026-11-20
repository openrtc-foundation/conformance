package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// FailureContractTarget maps controlled conditions into actual public operations.
// These callbacks are test plumbing, not required production interfaces. A target
// must identify the real public operation and independent covered observation.
// RFC §9:529–570, §12.1:1044–1046, §14.2:1102–1115.
type FailureContractTarget interface {
	FailureScenario(context.Context, string) (FailureScenario, error)
}
type FailureScenario struct {
	ExpectedResource              openrtc.ResourceRef
	PublicOperation               string
	Boundary                      string
	Invoke                        func(context.Context) openrtc.Async[openrtc.Void]
	Observe                       func(context.Context) (FailureObservation, error)
	UnauthorizedDiagnosticContext context.Context
	AuthorizedDiagnosticContext   context.Context
	DiagnosticsOffered            bool
	Canary                        string
}
type FailureObservation struct {
	Effects                int
	Submissions            int
	ProtectedEffects       int
	ForeignOwnerLive       bool
	DefaultSurfaces        []any
	ArtifactAuditReference string
}

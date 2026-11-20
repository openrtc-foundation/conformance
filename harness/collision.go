package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// CollisionTarget establishes two public incarnations using the same application
// identity while the effective service collision policy is unknown. Submissions
// are independently observed at the declared admission boundary, not inferred
// from result reads. The fixture never grants replacement authority.
// RFC §8.2:461–463, §14.3:1134.
type CollisionTarget interface {
	UnknownCollision(context.Context) (CollisionFixture, error)
}
type CollisionFixture struct {
	Existing, Candidate openrtc.RTCConferenceSession
	Policy              func() openrtc.IdentityCollisionPolicy
	Attempt             func(context.Context) openrtc.Async[openrtc.JoinResult]
	Observe             func(context.Context) (CollisionObservation, error)
	Boundary            string
}
type CollisionObservation struct {
	AdmissionSubmissions         int
	ReplacementAuthorized        bool
	ExistingDepartureEstablished bool
}

package core

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// RFC §8.5:487–500 applies to every supported async operation at its claimed
// scope. Missing controllability is incomplete, not an unsupported operation.
func additionalPendingBounds(ctx context.Context, c *harness.TestContext, b harness.BindingTiming) {
	target, ok := c.Target.(harness.PendingOperationTarget)
	if !ok {
		c.Partial("supported async operation inventory/hold fixtures beyond createClient and join unavailable")
		return
	}
	claims := target.PendingOperationClaims()
	if declared, ok := c.Target.(harness.ClaimTarget); ok {
		wanted := []harness.OperationClaim{}
		for _, claim := range declared.ClaimDeclaration().Operations {
			if claim.Support != "supported" && claim.Support != "conditional" {
				continue
			}
			switch claim.Operation {
			case "createClient", "join", "leave", "dispose", "capabilities", "observe", "snapshot", "unsubscribe", "observeProfile":
				continue
			}
			wanted = append(wanted, claim)
		}
		selected := []harness.OperationClaim{}
		for _, want := range wanted {
			found := false
			for _, fixtureClaim := range claims {
				if fixtureClaim.Operation == want.Operation && fixtureClaim.Scope == want.Scope {
					found = true
					selected = append(selected, want)
					break
				}
			}
			if !found {
				c.Partial("claimed async operation has no never-resolution control: " + want.Operation + " at " + want.Scope)
			}
		}
		claims = selected
	}

	for _, claim := range claims {
		switch claim.Support {
		case "unsupported":
			continue
		case "supported", "conditional":
		default:
			c.Partial("async applicability unresolved: " + claim.Operation)
			continue
		}
		for _, route := range []string{"cancel", "terminate"} {
			session := joined(ctx, c).Session
			fixture, err := target.HoldPendingOperation(ctx, claim, session)
			if err != nil {
				c.Partial("applicable async fixture unavailable: " + claim.Operation + ": " + err.Error())
				continue
			}
			if fixture.Gate == nil || fixture.Invoke == nil || fixture.PublicOperation == "" {
				c.Partial("incomplete public async operation mapping: " + claim.Operation)
				continue
			}
			if fixture.Capability.Support.Level != "supported" {
				c.Partial("exact async operation support unresolved at eligible scope: " + claim.Operation)
				continue
			}
			gate := fixture.Gate
			invocation, cancel := context.WithCancel(ctx)
			// Release only during teardown, after decisive local settlement assertions.
			func() {
				defer gate.ReleaseSuccess()
				defer cancel()
				op := fixture.Invoke(invocation)
				waitLifecycleGate(ctx, c, gate)
				if route == "cancel" {
					cancel()
				} else {
					inBindingBound(ctx, c, b.Release, "async owner termination", func(ctx context.Context) error { _, err := session.Leave(ctx).Result(ctx); return err })
				}
				limit := b.Cancellation
				if route == "terminate" {
					limit = b.TerminationFence
				}
				var first *openrtc.RTCFailure
				inBindingBound(ctx, c, limit, claim.Operation+" local settlement while provider blocked", func(ctx context.Context) error {
					_, err := op.Result(ctx)
					first = value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, claim.Operation))
					return nil
				})
				_, err := op.Result(ctx)
				again := value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "repeat "+claim.Operation))
				check(c, first.ID == again.ID, "pending async result changed final failure")
			}()
		}
	}
}

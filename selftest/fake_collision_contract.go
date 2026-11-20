package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// The policy projection is unknown even though the synthetic service happens to
// reject duplicates. A binding must not infer that service behavior as authority.
type collisionTarget struct {
	*FakeTarget
	mutation string
}

func (t collisionTarget) UnknownCollision(ctx context.Context) (harness.CollisionFixture, error) {
	spy := harness.NewCredentialSpy(t.Credentials())
	identity := openrtc.ApplicationIdentity{Namespace: "collision-selftest", Subject: newID("identity")}
	config := t.ClientConfig(harness.ClientConfigInput{Identity: identity, Credentials: spy.Provider(false)})
	firstClient := newFakeClient(config, t.world, NoDefects)
	secondClient := newFakeClient(config, t.world, NoDefects)
	room := openrtc.ConferenceTarget{Provisioning: openrtc.SessionProvisioning{Mode: "external-provisioned", Address: &openrtc.SessionAddress{AdapterID: adapterID, Namespace: "fake", Value: newID("collision-room")}}}
	first := firstClient.OpenSession(room)
	second := secondClient.OpenSession(room)
	if _, err := first.Join(openrtc.JoinSessionOptions{}).Result(ctx).Result(ctx); err != nil {
		return harness.CollisionFixture{}, err
	}
	baseline := effectObserver{t.world}.Counts().Admissions
	return harness.CollisionFixture{Existing: first, Candidate: second, Boundary: "synthetic service admission submission", Policy: func() openrtc.IdentityCollisionPolicy {
		return openrtc.IdentityCollisionPolicy{Behavior: "unknown", Scope: "application identity within this address"}
	}, Attempt: func(ctx context.Context) openrtc.Async[openrtc.JoinResult] {
		result := second.Join(openrtc.JoinSessionOptions{}).Result(ctx)
		return openrtc.StartOperation(t, func() (openrtc.JoinResult, error) {
			value, err := result.Result(ctx)
			switch t.mutation {
			case "automatic-retry":
				t.world.mu.Lock()
				t.world.counts.Admissions++
				t.world.mu.Unlock()
			case "invent-departure":
				first.(*fakeSession).setState("ended")
			}
			return value, err
		})
	}, Observe: func(context.Context) (harness.CollisionObservation, error) {
		return harness.CollisionObservation{AdmissionSubmissions: effectObserver{t.world}.Counts().Admissions - baseline, ReplacementAuthorized: t.mutation == "assume-replacement"}, nil
	}}, nil
}

package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
	"sync"
	"testing"
)

type pendingSubscriptionControl struct {
	mu      sync.Mutex
	control openrtc.ManualSubscriptionControl
	gate    *worldGate
	pending bool
	defect  bool
}

func (s *pendingSubscriptionControl) Subscribe(ctx context.Context, id openrtc.PublicationID, kinds []openrtc.MediaKind) openrtc.Async[openrtc.TrackSubscription] {
	s.mu.Lock()
	if s.pending && !s.defect {
		s.mu.Unlock()
		return openrtc.StartOperation(s, func() (openrtc.TrackSubscription, error) {
			return openrtc.TrackSubscription{}, fail("invalid-state", "subscribe", "selection already pending", failureExtras{})
		})
	}
	gate := s.gate
	s.gate = nil
	if gate != nil {
		s.pending = true
	}
	s.mu.Unlock()
	return openrtc.StartOperation(s, func() (openrtc.TrackSubscription, error) {
		if gate != nil {
			close(gate.reachedCh)
			_, _, _, err := gate.outcome.Await(ctx)
			if err != nil {
				return openrtc.TrackSubscription{}, err
			}
		}
		value, err := s.control.Subscribe(ctx, id, kinds).Result(ctx)
		s.mu.Lock()
		s.pending = false
		s.mu.Unlock()
		return value, err
	})
}
func (s *pendingSubscriptionControl) Unsubscribe(ctx context.Context, id openrtc.SubscriptionID) openrtc.Async[openrtc.Void] {
	return s.control.Unsubscribe(ctx, id)
}
func pendingSubscriptionFixture(defect bool) *harness.SubscriptionProfileFixture {
	fixture := subscriptionProfileFake("")
	wrapper := &pendingSubscriptionControl{control: fixture.Control, defect: defect}
	fixture.Control = wrapper
	fixture.HoldNextSubscribe = func() harness.JoinGate {
		gate := &worldGate{reached: harness.NewDeferred[struct{}](), outcome: harness.NewDeferred[any](), reachedCh: make(chan struct{})}
		wrapper.mu.Lock()
		wrapper.gate = gate
		wrapper.mu.Unlock()
		return joinGateControl{gate}
	}
	return fixture
}

type independentRecordingJob struct{ *recordingFake }

func (r independentRecordingJob) Stop(ctx context.Context) openrtc.Async[openrtc.JobStopReceipt] {
	if r.Snapshot().Status == "stopped" {
		return openrtc.StartOperation(r, func() (openrtc.JobStopReceipt, error) {
			return openrtc.JobStopReceipt{Status: "confirmed", Job: r.Snapshot()}, nil
		})
	}
	return r.recordingFake.Stop(ctx)
}
func independentRecordingFixture(defect bool) *harness.RecordingProfileFixture {
	fixture := recordingProfileFake("")
	var mu sync.Mutex
	jobs := map[openrtc.JobID]*recordingFake{}
	fixture.StartIndependent = func(context.Context) openrtc.Async[openrtc.MediaJob] {
		return openrtc.StartOperation(nil, func() (openrtc.MediaJob, error) {
			job := &recordingFake{v: openrtc.MediaJobView{ID: openrtc.JobID(newID("independent")), Status: "running", Lifetime: "independent"}}
			mu.Lock()
			jobs[job.v.ID] = job
			mu.Unlock()
			return independentRecordingJob{job}, nil
		})
	}
	fixture.IndependentOwner = func(openrtc.JobID) openrtc.ResourceRef {
		return openrtc.ResourceRef{Kind: "server", ID: "trusted-recording-owner"}
	}
	fixture.LeaveOwner = func(context.Context) error {
		if defect {
			mu.Lock()
			defer mu.Unlock()
			for _, job := range jobs {
				job.mu.Lock()
				job.v.Status = "stopping"
				job.mu.Unlock()
			}
		}
		return nil
	}
	fixture.ObserveRelease = func(_ context.Context, id openrtc.JobID) error {
		mu.Lock()
		job := jobs[id]
		mu.Unlock()
		job.mu.Lock()
		job.v.Status = "stopped"
		job.mu.Unlock()
		return nil
	}
	fixture.Independent = true
	return fixture
}

// RFC §11.2:656–667, §11.6:802–824. These dedicated scope fixtures exercise
// pending selection and separately advertised trusted-server recording behavior.
func TestPendingSubscriptionAndIndependentRecording(t *testing.T) {
	for _, id := range []string{"CT-SUB-004", "CT-REC-005"} {
		t.Run(id, func(t *testing.T) {
			for _, defective := range []bool{false, true} {
				target := newCurrentMediaTarget("")
				if id == "CT-SUB-004" {
					target.fixture.Subscription = pendingSubscriptionFixture(defective)
				} else {
					target.fixture.Recording = independentRecordingFixture(defective)
				}
				report := harness.RunConformance(context.Background(), target, capability.ProfileEdgeTests, harness.RunOptions{Only: []string{id}})
				want := harness.Pass
				if defective {
					want = harness.Fail
				}
				for _, r := range report.Results {
					if r.TestID == id && r.Status != want {
						t.Fatalf("defective=%v %s: %s", defective, r.Status, r.Reason)
					}
				}
			}
		})
	}
}

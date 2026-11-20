package selftest

import (
	"context"
	"fmt"
	"github.com/openrtc/conformance/openrtc"
	"strings"
	"time"
)

func (a *fakeAttempt) isFenced() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.fenced }
func (s *fakeSession) JoinRaw(o openrtc.JoinSessionOptions, extra map[string]any) openrtc.JoinAttempt {
	if _, err := validateHolder("participant-join", mergeRaw(o, extra), s.defects, "join"); err != nil {
		a := newFakeAttempt(s.generation, s)
		a.done.Reject(err)
		return a
	}
	return s.Join(o)
}
func (s *fakeSession) Join(o openrtc.JoinSessionOptions) openrtc.JoinAttempt {
	s.joinMu.Lock()
	defer s.joinMu.Unlock()
	a := newFakeAttempt(s.generation, s)
	v, err := validateHolder("participant-join", o, s.defects, "join")
	if err == nil && o.Completion != "" && o.Completion != "admitted" && o.Completion != "admission-pending" {
		err = fail("invalid-option", "join", "invalid completion milestone", failureExtras{})
	}
	if err == nil && o.InitialMedia != nil {
		for _, op := range o.InitialMedia.Operations {
			if op.Strength == "required" {
				err = s.unsupported("initial-media")
				break
			}
		}
	}
	if err != nil {
		if s.defects.JoinPreflightConsumes {
			s.attempt = a
		}
		a.done.Reject(err)
		return a
	}
	s.stateMu.Lock()
	disposed := s.disposed
	s.stateMu.Unlock()
	if disposed || s.connectionState() == "ended" || s.connectionState() == "failed" || (s.attempt != nil && s.attempt.done.Settled()) {
		a.done.Reject(fail("invalid-state", "join", "generation consumed", failureExtras{}))
		return a
	}
	if s.attempt != nil {
		if s.defects.SilentDuplicateJoin {
			a.done.Reject(fail("internal-failure", "join", "unused duplicate attempt", failureExtras{}))
			return s.attempt
		}
		a.done.Reject(fail("operation-conflict", "join", "setup pending", failureExtras{}))
		return a
	}
	if f := s.world.takeFault("join", "admission"); f != nil {
		if s.defects.AdmissionChangesState {
			s.setState("preparing")
		}
		a.done.Reject(faultError(f))
		return a
	}
	s.attempt = a
	s.joinValidation = v
	s.completion = o.Completion
	if s.completion == "" {
		s.completion = "admitted"
	}
	a.cancelHandler = func(context.Context) (openrtc.CancellationReport, error) { return s.cancel(a) }
	if o.Signal != nil && o.Signal.Err() != nil {
		if s.defects.PreabortStaysIdle {
			a.mu.Lock()
			a.fenced = true
			a.mu.Unlock()
			a.done.Reject(fail("cancelled", "join", "preaborted", failureExtras{}))
		} else {
			_, _ = s.cancel(a)
		}
		return a
	}
	if o.Signal != nil {
		go func() {
			select {
			case <-o.Signal.Done():
				if o.Signal.Err() == context.DeadlineExceeded {
					if s.defects.DeadlineTooSlow {
						time.Sleep(500 * time.Millisecond)
					}
					a.mu.Lock()
					a.deadlineExpired = true
					a.mu.Unlock()
					if s.defects.DeadlineRawWaiter {
						a.done.Reject(context.DeadlineExceeded)
					} else {
						_, _ = s.cancel(a)
					}
				} else {
					_, _ = s.cancel(a)
				}
			case <-a.abortContext.Done():
			}
		}()
	}
	s.workerDone = make(chan struct{})
	done := s.workerDone
	go func() { defer close(done); s.runJoin(o, a) }()
	return a
}
func (s *fakeSession) cancel(a *fakeAttempt) (openrtc.CancellationReport, error) {
	if s.defects.CancellationTooSlow {
		time.Sleep(500 * time.Millisecond)
	}
	a.mu.Lock()
	if a.done.Settled() {
		a.mu.Unlock()
		return openrtc.CancellationReport{Won: s.defects.CancelAfterSettleSucceeds, Mode: "local-fence", Cleanup: openrtc.CleanupReport{Status: s.cleanupStatus(), Generation: openrtc.KnownValue(s.generation)}}, nil
	}
	if a.fenced {
		a.mu.Unlock()
		return openrtc.CancellationReport{Won: true, Mode: "local-fence", Cleanup: openrtc.CleanupReport{Status: s.cleanupStatus(), Generation: openrtc.KnownValue(s.generation)}}, nil
	}
	a.fenced = true
	a.abort()
	a.mu.Unlock()
	state := s.connectionState()
	if state == "idle" || state == "preparing" || state == "joining" {
		s.setState(openrtc.ConnectionState("leaving"))
		s.setState(openrtc.ConnectionState("ended"))
	}
	if !s.defects.CancelClaimsComplete && state != "idle" {
		s.setCleanup(openrtc.CleanupStatus{Kind: "pending", Outstanding: []string{"pending native setup"}})
	}
	code := openrtc.ErrorCode("cancelled")
	if a.deadlineExpired {
		code = "timeout"
	}
	a.done.Reject(fail(code, "join", "setup was cancelled or its invocation deadline expired", failureExtras{}))
	return openrtc.CancellationReport{Won: true, Mode: "local-fence", Cleanup: openrtc.CleanupReport{Status: s.cleanupStatus(), Generation: openrtc.KnownValue(s.generation)}}, nil
}

func (s *fakeSession) cleanupStatus() openrtc.CleanupStatus {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.cleanup
}

func (s *fakeSession) connectionState() openrtc.ConnectionState {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.state
}
func (s *fakeSession) runJoin(o openrtc.JoinSessionOptions, a *fakeAttempt) {
	defer func() {
		if v := recover(); v != nil {
			a.done.Reject(fmt.Errorf("%v", v))
		}
	}()
	v := s.joinValidation
	err := s.performJoin(v, a, o.Signal)
	if err == nil || a.isFenced() {
		return
	}
	s.releaseIdentity()
	state := s.connectionState()
	if state != "ended" && state != "failed" && !s.defects.JoinFailureStaysJoining {
		s.setState("failed")
	}
	a.done.Reject(err)
}
func (s *fakeSession) performJoin(v validationOutcome, a *fakeAttempt, signal context.Context) error {
	s.setState(openrtc.ConnectionState("preparing"))
	if f := s.world.takeFault("join", "execution-cleanup"); f != nil {
		s.world.acquireUnreturned(s, s.defects)
		s.setCleanup(openrtc.CleanupStatus{Kind: "failed", Outstanding: []string{"unreturned setup resource"}, Errors: []openrtc.RTCError{fail("cleanup-failed", "leave", "compensation failed", failureExtras{})}})
		if s.defects.CleanupReplacesPrimary {
			f.code = "cleanup-failed"
		}
		return faultError(f)
	}
	if f := s.world.takeFault("join", "execution"); f != nil {
		s.world.acquireUnreturned(s, s.defects)
		return faultError(f)
	}
	if err := s.resolveTarget(a.abortContext); err != nil {
		return err
	}
	if a.isFenced() {
		return nil
	}
	c, err := s.requestCredential(a.abortContext, "admission", "initial", false)
	if err != nil {
		return fail("credential-failed", "credential-get", "credential acquisition failed", failureExtras{cause: err, original: err})
	}
	if a.isFenced() {
		return nil
	}
	// The signal may already be closed while its watcher is still runnable.
	// Process that fence before accepting a credential continuation.
	if signal != nil && signal.Err() != nil {
		if signal.Err() == context.DeadlineExceeded {
			a.mu.Lock()
			a.deadlineExpired = true
			a.mu.Unlock()
		}
		_, _ = s.cancel(a)
		return nil
	}
	text := ""
	c.Material.Use(func(b []byte) any { text = string(b); return nil })
	s.apply(func() {
		s.credentials.State = "valid"
		s.credentials.Validity = "valid"
		s.credentials.Metadata = openrtc.KnownValue(c.Metadata)
	}, []sessionMutationEvent{{key: "credentialChanged", payload: func(p, n openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.CredentialView]{Previous: openrtc.KnownValue(p.Credentials[0]), Current: n.Credentials[0]}
	}}})
	s.setState(openrtc.ConnectionState("joining"))
	if err = s.nativeJoin(text, a); err != nil {
		return err
	}
	if signal != nil && signal.Err() != nil {
		if signal.Err() == context.DeadlineExceeded {
			a.mu.Lock()
			a.deadlineExpired = true
			a.mu.Unlock()
		}
		_, _ = s.cancel(a)
	}
	if a.isFenced() {
		if s.defects.LateSuccessResurrects {
			a.mu.Lock()
			a.fenced = false
			a.mu.Unlock()
			a.done.Resolve(s.finishJoin(v))
			return nil
		}
		s.world.compensation(s)
		return nil
	}
	// A legacy milestone selector cannot lower the admission-positive success boundary.
	if s.connectionAdmission() != "admitted" && !s.defects.HeldCompletesDefault {
		close(s.milestoneReached)
		select {
		case <-s.admissionReady:
		case <-a.abortContext.Done():
			s.world.compensation(s)
			return nil
		}
	}
	a.mu.Lock()
	if a.fenced {
		a.mu.Unlock()
		return nil
	}
	result := s.finishJoin(v)
	a.done.Resolve(result)
	a.mu.Unlock()
	select {
	case <-s.milestoneReached:
	default:
		close(s.milestoneReached)
	}
	return nil
}
func (s *fakeSession) connectionAdmission() openrtc.AdmissionStatus {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	return s.admission
}

func (s *fakeSession) releaseIdentity() {
	s.world.mu.Lock()
	delete(s.world.activeIdentities, s.identityKey)
	s.world.mu.Unlock()
}
func (s *fakeSession) nativeJoin(text string, a *fakeAttempt) error {
	w := s.world
	w.mu.Lock()
	w.counts.Admissions++
	gate := w.gate
	w.gate = nil
	w.mu.Unlock()
	if gate != nil {
		gate.reached.Resolve(struct{}{})
		close(gate.reachedCh)
		if s.defects.DeadlineEarly {
			a.mu.Lock()
			a.deadlineExpired = true
			a.mu.Unlock()
			_, _ = s.cancel(a)
		}
		out, reason, rejected, err := gate.outcome.Await(context.Background())
		if err != nil {
			return err
		}
		if rejected {
			return fmt.Errorf("%v", reason)
		}
		if out != "success" {
			if e, ok := out.(error); ok {
				return e
			}
			return fmt.Errorf("%v", out)
		}
	}
	w.mu.Lock()
	f := w.nextFailure
	if f != nil && f.operation == "join" {
		w.nextFailure = nil
	} else {
		f = nil
	}
	w.mu.Unlock()
	if f != nil {
		original := f.failure
		var cause error
		if s.defects.ReplaceOriginalError {
			return fail("internal", "join", "incorrect established classification for opaque native failure", failureExtras{})
		} else if e, ok := original.(error); ok {
			cause = e
		}
		// Mandatory join is also a default diagnostic boundary for media-free Core.
		// Native contents stay private unless a deliberate selftest defect leaks them.
		message := "native join failed"
		s.media.mu.Lock()
		canaries := append([]string(nil), s.media.canaries...)
		s.media.mu.Unlock()
		if len(canaries) > 0 {
			log := "join failed: unknown (native detail redacted)"
			if s.defects.LeakLogCanary {
				log = canaries[0]
			}
			w.mu.Lock()
			w.logs = append(w.logs, log)
			w.mu.Unlock()
			if s.defects.LeakErrorCanary {
				message = canaries[0]
			}
		}
		failure := fail("provider-failure", "join", message, failureExtras{original: original, cause: cause, retryability: "unknown", leakOriginal: s.defects.SerializeLeaksSecret})
		if operation, ok := failure.(*openrtc.RTCOperationError); ok {
			operation.Context = &openrtc.SafeErrorContext{OperationID: a.result.OperationID()}
		}
		return failure
	}
	if !strings.HasPrefix(text, "fake-token") {
		return fail("authentication-failed", "join", "the service rejected the credential", failureExtras{})
	}
	s.identityKey = fmt.Sprintf("%s|%s|%s", s.addressValue, s.client.identity.Namespace, s.client.identity.Subject)
	w.mu.Lock()
	if _, ok := w.activeIdentities[s.identityKey]; ok {
		w.mu.Unlock()
		return fail("operation-conflict", "join", "this application identity is already connected here", failureExtras{})
	}
	w.activeIdentities[s.identityKey] = struct{}{}
	mode := w.admissionMode
	w.admissionMode = "admit"
	w.mu.Unlock()
	if mode == "deny" {
		s.releaseIdentity()
		s.setAdmission("denied")
		s.setState(openrtc.ConnectionState("ended"))
		a.mu.Lock()
		a.fenced = true
		a.done.Reject(fail("authorization-denied", "join", "admission was denied", failureExtras{}))
		a.mu.Unlock()
		return nil
	}
	s.stateMu.Lock()
	s.nativeJoined = true
	s.stateMu.Unlock()
	w.mu.Lock()
	w.sessions[s] = struct{}{}
	if mode == "hold" {
		w.held[s] = struct{}{}
	}
	w.mu.Unlock()
	if mode == "hold" && !s.defects.HeldPresentedAsAdmitted {
		s.setAdmission("held")
	} else {
		s.setAdmission("admitted")
	}
	return nil
}
func (s *fakeSession) finishJoin(v validationOutcome) openrtc.JoinResult {
	w := s.world
	w.mu.Lock()
	s.stateMu.Lock()
	s.roster = []openrtc.RTCParticipant{}
	for _, r := range w.remotesAt(s.addressValue) {
		if r.connected {
			s.roster = append(s.roster, s.participantFor(r))
		}
	}
	s.stateMu.Unlock()
	w.mu.Unlock()
	s.setState(openrtc.ConnectionState("connected"))
	s.stateMu.Lock()
	s.settled = true
	s.stateMu.Unlock()
	store := storeFor(s)
	store.mu.Lock()
	configuration := emptyConfigurationResult(v.placement)
	store.configuration = &configuration
	store.mu.Unlock()
	return s.currentJoinResult()
}
func (s *fakeSession) currentJoinResult() openrtc.JoinResult {
	snapshot := s.observation.Snapshot()
	if s.defects.JoinResultBeforeCommit {
		snapshot.Connection.State = "joining"
	}
	return openrtc.JoinResult{Completion: "admitted", Achieved: string(snapshot.Connection.Admission), Connection: snapshot.Connection, Admission: snapshot.Connection.Admission, Membership: openrtc.Unknown[openrtc.ParticipantConnectionID](), InitialMedia: openrtc.InitialMediaOutcome{Operations: []openrtc.InitialMediaOperationOutcome{}}, MediaReadiness: snapshot.Connection.MediaReadiness}
}

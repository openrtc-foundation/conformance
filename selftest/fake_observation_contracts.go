package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type streamSubscriber struct {
	active   bool
	listener func(openrtc.ObservationUpdate)
}

// Observe is the fake's ordered public stream, separate from latest-value Watch.
func (o *fakeObservation) Observe(f func(openrtc.ObservationUpdate)) openrtc.Unsubscribe {
	o.mu.Lock()
	w := &streamSubscriber{active: true, listener: f}
	o.streams = append(o.streams, w)
	initial := detached(o.current)
	if o.streamDefect == "stale-resync" && (streamMarker(initial, "continuation") || streamMarker(initial, "fresh-next")) {
		initial.Roster.Entries = map[openrtc.ParticipantConnectionID]openrtc.RTCParticipant{}
	}
	if o.streamDefect == "alias" {
		initial = o.current
	}
	o.queue = append(o.queue, func() {
		o.mu.Lock()
		active := w.active
		o.mu.Unlock()
		if active {
			f(openrtc.ObservationUpdate{Snapshot: &initial, Current: true})
		}
	})
	o.mu.Unlock()
	o.drain()
	return func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		w.active = false
		if o.streamDefect == "remove-all" {
			for _, s := range o.streams {
				s.active = false
			}
		}
	}
}

// Called with observation lock held; captures subscriptions at the commit boundary.
func (o *fakeObservation) queueStream(next openrtc.SessionSnapshot) {
	subscribers := append([]*streamSubscriber(nil), o.streams...)
	lost := o.loseStream
	o.loseStream = false
	copyNext := detached(next)
	o.queue = append(o.queue, func() {
		for index, w := range subscribers {
			o.mu.Lock()
			active := w.active
			defect := o.streamDefect
			o.mu.Unlock()
			if !active {
				continue
			}
			if lost {
				if defect != "hide-loss" {
					w.listener(openrtc.ObservationUpdate{Loss: true})
				}
				continue
			}
			if defect == "drop" && streamMarker(copyNext, "first") {
				continue
			}
			subscriberSnapshot := detached(copyNext)
			update := openrtc.ObservationUpdate{Snapshot: &subscriberSnapshot, Current: true}
			if defect == "invert" && index > 0 && streamMarker(copyNext, "first") {
				o.mu.Lock()
				if o.streamDelayed == nil {
					o.streamDelayed = map[*streamSubscriber]openrtc.ObservationUpdate{}
				}
				o.streamDelayed[w] = update
				o.mu.Unlock()
				continue
			}
			w.listener(update)
			if defect == "invert" && index > 0 && streamMarker(copyNext, "reentrant") {
				o.mu.Lock()
				delayed, ok := o.streamDelayed[w]
				delete(o.streamDelayed, w)
				o.mu.Unlock()
				if ok {
					w.listener(delayed)
				}
			}
			if defect == "duplicate" {
				w.listener(update)
			}
		}
	})
}
func streamMarker(s openrtc.SessionSnapshot, m string) bool {
	for _, p := range s.Roster.Entries {
		if p.DisplayName.Value != nil && *p.DisplayName.Value == m {
			return true
		}
	}
	return false
}

type observationControl struct{ target *FakeTarget }

func (t *FakeTarget) ObservationControl() harness.ObservationControl { return observationControl{t} }
func (c observationControl) Commit(_ context.Context, s openrtc.RTCConferenceSession, m string) error {
	fs := s.(*fakeSession)
	return c.target.LifecycleEvents().Submit(context.Background(), s, fs.generation, []harness.LifecycleNativeEvent{{Kind: "upsert", NativeUID: "stream-peer", NativeKey: "stream-peer-1", DisplayName: m}})
}
func (c observationControl) LoseNext(_ context.Context, s openrtc.RTCConferenceSession) error {
	o := s.(*fakeSession).observation
	o.mu.Lock()
	o.loseStream = true
	o.mu.Unlock()
	return nil
}

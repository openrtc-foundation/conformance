package openrtc

// ObservationUpdate projects the ordered observation stream in RFC §8.3:467–477
// and A.1:1301–1308. Current=false follows a delivery loss until resynchronization.
// Watch remains a separately usable latest-value convenience view.
type ObservationUpdate struct {
	Snapshot *SessionSnapshot
	Loss     bool
	Current  bool
}

// StreamObservation is an optional Go projection of observe, independent of
// event-name/Once/Off syntax. A binding may map an equivalent public surface.
type StreamObservation interface {
	Observe(func(ObservationUpdate)) Unsubscribe
}

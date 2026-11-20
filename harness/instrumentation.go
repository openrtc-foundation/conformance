package harness

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/openrtc/conformance/openrtc"
)

// IsRTCError narrows a value to a portable failure.
func IsRTCError(v any) bool {
	switch v.(type) {
	case *openrtc.RTCFailure, *openrtc.RTCOperationError, *openrtc.RTCObservationError:
		return !reflect.ValueOf(v).IsNil()
	}
	return false
}

// Truncated identifies an inconclusive leak scan (E-02).
const Truncated = "leak-scan-truncated"

// FindLeak scans object data for credential material (CFG-009, CFG-114).
// A nil result means no leak was found; limit defaults to 200,000 nodes.
func FindLeak(root any, needles []string, limits ...int) *string {
	limit := 200000
	if len(limits) > 0 {
		limit = limits[0]
	}
	type item struct {
		v    reflect.Value
		path string
	}
	stack := []item{{reflect.ValueOf(root), "$"}}
	seen := map[visit]bool{}
	visited := 0
	for len(stack) > 0 && visited < limit {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visited++
		v := i.v
		for v.IsValid() && v.Kind() == reflect.Interface {
			if v.IsNil() {
				v = reflect.Value{}
				break
			}
			v = v.Elem()
		}
		if !v.IsValid() {
			continue
		}
		if v.Kind() == reflect.String {
			for _, n := range needles {
				if n != "" && strings.Contains(v.String(), n) {
					p := i.path
					return &p
				}
			}
			continue
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
			if v.IsNil() {
				continue
			}
			id := visit{v.Kind(), v.Pointer()}
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		switch v.Kind() {
		case reflect.Pointer:
			stack = append(stack, item{v.Elem(), i.path})
		case reflect.Slice, reflect.Array:
			if v.Type().Elem().Kind() == reflect.Uint8 {
				b := make([]byte, v.Len())
				for j := range b {
					b[j] = byte(v.Index(j).Uint())
				}
				s := strings.ToValidUTF8(string(b), "\uFFFD")
				for _, n := range needles {
					if n != "" && strings.Contains(s, n) {
						p := i.path
						return &p
					}
				}
				continue
			}
			for j := 0; j < v.Len(); j++ {
				stack = append(stack, item{v.Index(j), fmt.Sprintf("%s.%d", i.path, j)})
			}
		case reflect.Map:
			it := v.MapRange()
			for it.Next() {
				k := it.Key()
				stack = append(stack, item{k, i.path + ".<key>"}, item{it.Value(), fmt.Sprintf("%s[%v]", i.path, k)})
			}
		case reflect.Struct:
			for j := 0; j < v.NumField(); j++ {
				f := v.Type().Field(j)
				// Unexported fields are not part of the public Go object surface.
				if f.PkgPath != "" {
					continue
				}
				name := f.Name
				if tag := strings.Split(f.Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
					name = tag
				}
				stack = append(stack, item{v.Field(j), i.path + "." + name})
			}
		}
	}
	if len(stack) > 0 {
		s := fmt.Sprintf("%s: scan stopped after %d nodes (inconclusive)", Truncated, limit)
		return &s
	}
	return nil
}

type visit struct {
	kind reflect.Kind
	ptr  uintptr
}

// SafeJSON never invokes foreign marshalers, and stands in for public serialization (ERR-107).
func SafeJSON(value any) (result string) {
	defer func() {
		if e := recover(); e != nil {
			result = fmt.Sprintf("[unserializable: %v]", e)
		}
	}()
	seen := map[visit]bool{}
	var clean func(reflect.Value) any
	clean = func(v reflect.Value) any {
		if !v.IsValid() {
			return nil
		}
		for v.Kind() == reflect.Interface {
			if v.IsNil() {
				return nil
			}
			v = v.Elem()
		}
		if v.CanInterface() && IsRTCError(v.Interface()) {
			switch e := v.Interface().(type) {
			case *openrtc.RTCFailure:
				return e.ToJSON()
			case *openrtc.RTCOperationError:
				return e.ToJSON()
			case *openrtc.RTCObservationError:
				return e.ToJSON()
			}
		}
		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Slice {
			if v.IsNil() {
				return nil
			}
			id := visit{v.Kind(), v.Pointer()}
			if seen[id] {
				return "[cycle]"
			}
			seen[id] = true
		}
		switch v.Kind() {
		case reflect.Pointer:
			return clean(v.Elem())
		case reflect.Bool:
			return v.Bool()
		case reflect.String:
			return v.String()
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return v.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return v.Uint()
		case reflect.Float32, reflect.Float64:
			return v.Float()
		case reflect.Slice, reflect.Array:
			a := make([]any, v.Len())
			for i := range a {
				a[i] = clean(v.Index(i))
			}
			return a
		case reflect.Map:
			m := map[string]any{}
			it := v.MapRange()
			for it.Next() {
				if it.Key().Kind() == reflect.String {
					m[it.Key().String()] = clean(it.Value())
				}
			}
			if _, ok := m["toJSON"]; ok {
				return "[object with own toJSON omitted]"
			}
			return m
		case reflect.Struct:
			m := map[string]any{}
			for i := 0; i < v.NumField(); i++ {
				f := v.Type().Field(i)
				if f.PkgPath != "" {
					continue
				}
				tag := strings.Split(f.Tag.Get("json"), ",")
				name := tag[0]
				if name == "-" {
					continue
				}
				if name == "" {
					name = f.Name
				}
				if len(tag) > 1 && strings.Contains(strings.Join(tag[1:], ","), "omitempty") && v.Field(i).IsZero() {
					continue
				}
				m[name] = clean(v.Field(i))
			}
			if _, ok := m["toJSON"]; ok {
				return "[object with own toJSON omitted]"
			}
			return m
		}
		return nil
	}
	b, err := json.Marshal(clean(reflect.ValueOf(value)))
	if err != nil {
		return "[unserializable: " + err.Error() + "]"
	}
	return string(b)
}

func credentialNeedles(b []byte) []string {
	s := strings.ToValidUTF8(string(b), "\uFFFD")
	base := base64.StdEncoding.EncodeToString(b)
	var out []string
	for _, v := range []string{s, base} {
		if utf8.RuneCountInString(v) >= 6 {
			out = append(out, v)
		}
	}
	return out
}

type CredentialCall struct {
	Kind    string
	Request openrtc.CredentialRequest
	Signal  context.Context
	At      int64
}
type CredentialBehavior struct {
	Kind     string
	Failure  any
	Released *Deferred[struct{}]
}
type CredentialSpy struct {
	mu       sync.Mutex
	source   CredentialSource
	calls    []CredentialCall
	needles  []string
	behavior CredentialBehavior
}

func NewCredentialSpy(source CredentialSource) *CredentialSpy {
	return &CredentialSpy{source: source, behavior: CredentialBehavior{Kind: "valid"}}
}
func (s *CredentialSpy) Calls() []CredentialCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CredentialCall(nil), s.calls...)
}
func (s *CredentialSpy) Needles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.needles...)
}
func (s *CredentialSpy) SetBehavior(b CredentialBehavior) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.behavior = b
}
func (s *CredentialSpy) Reached(ctx context.Context, count, timeoutMs int) bool {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for {
		if len(s.Calls()) >= count {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Millisecond):
		}
	}
}

type credentialProvider struct{ spy *CredentialSpy }
type renewingCredentialProvider struct{ *credentialProvider }

func (s *CredentialSpy) Provider(renew bool) openrtc.CredentialProvider {
	p := &credentialProvider{s}
	if renew {
		return &renewingCredentialProvider{p}
	}
	return p
}
func (p *credentialProvider) Get(ctx context.Context, r openrtc.CredentialRequest) openrtc.Async[openrtc.CredentialInput] {
	return openrtc.StartOperation(p, func() (openrtc.CredentialInput, error) {
		return p.spy.respond(ctx, "get", r)

	})
}
func (p *renewingCredentialProvider) Renew(ctx context.Context, _ openrtc.CredentialMetadata, r openrtc.CredentialRequest) openrtc.Async[openrtc.CredentialInput] {
	return openrtc.StartOperation(p, func() (openrtc.CredentialInput, error) {
		return p.spy.respond(ctx, "renew", r)

	})
}
func (s *CredentialSpy) respond(ctx context.Context, kind string, r openrtc.CredentialRequest) (openrtc.CredentialInput, error) {
	s.mu.Lock()
	s.calls = append(s.calls, CredentialCall{kind, r, ctx, time.Now().UnixMilli()})
	b := s.behavior
	s.mu.Unlock()
	if b.Kind == "reject" {
		if e, ok := b.Failure.(error); ok {
			return openrtc.CredentialInput{}, e
		}
		return openrtc.CredentialInput{}, fmt.Errorf("%v", b.Failure)
	}
	if b.Kind == "hold" {
		_, reason, rejected, err := b.Released.Await(context.Background())
		if err != nil {
			return openrtc.CredentialInput{}, err
		}
		if rejected {
			if e, ok := reason.(error); ok {
				return openrtc.CredentialInput{}, e
			}
			return openrtc.CredentialInput{}, fmt.Errorf("%v", reason)
		}
	}
	f := s.source.Valid
	if b.Kind == "invalid" && s.source.Invalid != nil {
		f = s.source.Invalid
	}
	c, err := f(ctx, r)
	if err != nil {
		return c, err
	}
	c.Material.Use(func(bytes []byte) any {
		n := credentialNeedles(bytes)
		s.mu.Lock()
		s.needles = append(s.needles, n...)
		s.mu.Unlock()
		return nil
	})
	return c, nil
}

// SessionEventKeys are the ten fixed session event keys; CORE-108.
var SessionEventKeys = []string{"connectionChanged", "admissionChanged", "participantChanged", "publicationChanged", "subscriptionChanged", "credentialChanged", "capabilitiesChanged", "cleanupChanged", "snapshotChanged", "error"}

type RecordedEvent struct {
	Key                string
	Envelope           openrtc.EventEnvelope[any]
	SnapshotAtDelivery openrtc.SessionSnapshot
	Order              int
}
type EventRecorder struct {
	mu            sync.Mutex
	events        []RecordedEvent
	unsubscribers []openrtc.Unsubscribe
}

func NewEventRecorder(session openrtc.RTCSession) *EventRecorder {
	r := &EventRecorder{}
	obs := session.Observation()
	for _, key := range SessionEventKeys {
		k := key
		u := obs.On(k, func(e openrtc.EventEnvelope[any]) {
			snapshot := obs.Snapshot()
			r.mu.Lock()
			defer r.mu.Unlock()
			r.events = append(r.events, RecordedEvent{k, e, snapshot, len(r.events)})
		})
		r.unsubscribers = append(r.unsubscribers, u)
	}
	return r
}
func (r *EventRecorder) Events() []RecordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RecordedEvent(nil), r.events...)
}
func (r *EventRecorder) Of(key string) []RecordedEvent {
	var out []RecordedEvent
	for _, e := range r.Events() {
		if e.Key == key {
			out = append(out, e)
		}
	}
	return out
}
func (r *EventRecorder) Stop() {
	r.mu.Lock()
	u := r.unsubscribers
	r.unsubscribers = nil
	r.mu.Unlock()
	for _, f := range u {
		f()
	}
}

// ExpectedFailure describes a portable failure; Codes accepts one or more codes.
type ExpectedFailure struct {
	Codes []openrtc.ErrorCode
	Kind  string
}

// Captured holds the result of a possibly synchronous failure.
type Captured[T any] struct {
	OK    bool
	Value T
	Error any
}

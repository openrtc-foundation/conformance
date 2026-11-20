package openrtc

import "context"

// PlacementEntry is the current scoped request equivalent of amended RFC
// §11.7:845 and A.3:1386–1388. Legacy PlacementPolicy remains a Go bridge form;
// its fixed planes do not prescribe the core's covered resource classes.
type PlacementEntry struct {
	ID             string      `json:"id"` // correlation for each preference request entry
	Scope          string      `json:"scope"`
	Activities     []string    `json:"activities"`
	AllowedRegions []RegionRef `json:"allowedRegions"`
	Transit        bool        `json:"transit"`
	Fallback       bool        `json:"fallback"`
	Requirement    string      `json:"requirement"` // required | preference
}
type PlacementPreferenceResult struct {
	RequestID string `json:"requestId"`
	Selection string `json:"selection"`
	Reason    string `json:"reason"`
}
type PlacementEnforcementEvidence struct {
	Activity   string  `json:"activity"`
	Direction  string  `json:"direction"`
	Boundary   string  `json:"boundary"`
	Mechanism  string  `json:"mechanism"`
	Authority  string  `json:"authority"`
	Source     string  `json:"source"`
	Version    string  `json:"version"`
	ValidUntil Instant `json:"validUntil"`
}
type CurrentPlacementResult struct {
	Assessment  string                         `json:"assessment"`
	State       string                         `json:"state"`
	Boundary    Known[string]                  `json:"boundary"`
	Preferences []PlacementPreferenceResult    `json:"preferences"`
	Rejected    []RejectedCondition            `json:"rejected"`
	Evidence    []PlacementEnforcementEvidence `json:"evidence"`
}

// CurrentPlacementControl exposes the two normative operations without fixed
// runtime option names or mandatory placement scope classes (§11.7:831–873).
type CurrentPlacementControl interface {
	AssessPlacement(context.Context, string, []PlacementEntry) Async[CurrentPlacementResult]
	ApplyPlacement(context.Context, string, []PlacementEntry, OperationOptions) Async[CurrentPlacementResult]
}

package openrtc

import "context"

// These operation views are the current Appendix A equivalents for bindings
// whose profile API does not use the historical Session/leg extension stack.
// RFC §11.1:590–630, A.3:1342–1364/1427–1438.
type CurrentCallView struct {
	ID                    string
	State                 CallState
	Answered              bool
	LocalHold, RemoteHold Known[bool]
	TransferActive        bool
	Kinds                 []MediaKind
	Updates               []RenegotiationUpdate
}
type CurrentCall interface {
	Snapshot() CurrentCallView
	Accept(context.Context, OperationOptions) Async[Void]
	Reject(context.Context) Async[Void]
	Hangup(context.Context) Async[Void]
	SetHold(context.Context, bool, OperationOptions) Async[Void]
	Transfer(context.Context, OperationOptions) Async[Void]
	SendDigits(context.Context, string, []DtmfMethod, OperationOptions) Async[DtmfResult]
}

// Grant is source + recipient + bounded use scope, not ambient permission.
// RFC §11.3:669–682, A.3:1371–1373/1442–1446.
type SourceGrant struct {
	Owner     ResourceRef
	ID        string
	Source    CaptureSourceID
	Recipient ResourceRef
	Scope     CapabilityScope
	State     string
}
type SourceGrantOperations interface {
	ShareSource(context.Context, CaptureSourceID, ResourceRef, CapabilityScope, OperationOptions) Async[SourceGrant]
	RevokeSourceGrant(context.Context, string, OperationOptions) Async[Void]
	Grant(string) SourceGrant
}

// SelectedScreen does not imply publication or capture of unselected audio.
// RFC §11.4:686–733, A.3:1404–1409/1447.
type SelectedScreen struct {
	Source           CaptureSourceView
	Selection        Known[string]
	AudioAssociation Known[CaptureSourceID]
	Publications     []PublicationID
}

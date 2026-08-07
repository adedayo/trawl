package core

// ScanOutcome is how a scan ended, as distinct from what it found.
//
// Every deployment announces completion on the event bus, and until now the
// announcement carried no outcome at all: a scan that failed halfway emitted
// exactly the same event as one that completed cleanly. The UI could only
// report that scanning had stopped, which reads as success — the more
// dangerous way to be wrong, because a partial assessment presented as a
// complete one invites an operator to conclude they have no exposure when the
// truth is that nobody looked.
type ScanOutcome struct {
	// Phase is always "complete" here. It is carried so that a subscriber can
	// tell a completion event from a per-check progress event without
	// inspecting which other fields happen to be set.
	Phase string `json:"phase"`

	Domain  string `json:"domain,omitempty"`
	RepoURL string `json:"repoUrl,omitempty"`

	// Status is "completed" or "completed-with-failures", matching the
	// vocabulary the HTTP scan endpoint already returns in its response body.
	// One vocabulary across the transports means the UI needs one rule.
	Status string `json:"status"`

	// Error names what went wrong, when anything did. The results that did
	// complete are still persisted, so this is an outcome to be read rather
	// than a signal to retry blindly.
	Error string `json:"error,omitempty"`
}

const (
	// ScanStatusCompleted means every check the scan attempted concluded.
	ScanStatusCompleted = "completed"
	// ScanStatusPartial means the scan ran but something did not conclude.
	// Whatever was assessed is persisted; the coverage figures account for
	// the rest.
	ScanStatusPartial = "completed-with-failures"
)

// NewScanOutcome builds the completion payload for a finished scan.
func NewScanOutcome(req ScanRequest, err error) ScanOutcome {
	out := ScanOutcome{
		Phase:   "complete",
		Domain:  req.Domain,
		RepoURL: req.RepoURL,
		Status:  ScanStatusCompleted,
	}
	if err != nil {
		out.Status = ScanStatusPartial
		out.Error = err.Error()
	}
	return out
}

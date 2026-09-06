package dotlink

import (
	"fmt"
	"os"
	"path/filepath"
)

// ApplyAction describes what Apply did, or chose not to do, for one link.
type ApplyAction int

const (
	// ActionCreated means a symlink was created at the target.
	ActionCreated ApplyAction = iota
	// ActionSkippedLinked means the target already points at the right source.
	ActionSkippedLinked
	// ActionSkippedConflict means a non-symlink file already exists at the target.
	ActionSkippedConflict
	// ActionSkippedWrongLink means a symlink to somewhere else already exists at the target.
	ActionSkippedWrongLink
)

func (a ApplyAction) String() string {
	switch a {
	case ActionCreated:
		return "created"
	case ActionSkippedLinked:
		return "already linked"
	case ActionSkippedConflict:
		return "skipped, file exists at target"
	case ActionSkippedWrongLink:
		return "skipped, wrong link exists at target"
	default:
		return "unknown"
	}
}

// ApplyResult is the outcome of applying one manifest Link.
type ApplyResult struct {
	Link   Link
	Action ApplyAction
}

// Apply creates symlinks for every manifest link whose target is missing.
// It never touches a target that already exists in some form, whether
// that's a correct link, a link to the wrong place, or a plain file -
// those come back as skipped results rather than being overwritten.
// Forcing an overwrite is a separate, not-yet-implemented step.
func Apply(root string, m *Manifest) ([]ApplyResult, error) {
	statuses, err := CheckStatus(root, m)
	if err != nil {
		return nil, err
	}

	out := make([]ApplyResult, 0, len(statuses))
	for _, s := range statuses {
		switch s.State {
		case StateLinked:
			out = append(out, ApplyResult{Link: s.Link, Action: ActionSkippedLinked})
		case StateConflict:
			out = append(out, ApplyResult{Link: s.Link, Action: ActionSkippedConflict})
		case StateWrongLink:
			out = append(out, ApplyResult{Link: s.Link, Action: ActionSkippedWrongLink})
		case StateMissing:
			source, err := filepath.Abs(filepath.Join(root, s.Link.Source))
			if err != nil {
				return nil, fmt.Errorf("resolving source for %s: %w", s.Link.Target, err)
			}
			if err := os.MkdirAll(filepath.Dir(s.Link.Target), 0o755); err != nil {
				return nil, fmt.Errorf("creating parent directory for %s: %w", s.Link.Target, err)
			}
			if err := os.Symlink(source, s.Link.Target); err != nil {
				return nil, fmt.Errorf("linking %s: %w", s.Link.Target, err)
			}
			out = append(out, ApplyResult{Link: s.Link, Action: ActionCreated})
		}
	}
	return out, nil
}

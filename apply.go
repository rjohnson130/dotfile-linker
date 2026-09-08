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
	// ActionReplaced means an existing file or wrong link was moved aside and
	// a fresh symlink was created in its place, because Force was set.
	ActionReplaced
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
	case ActionReplaced:
		return "replaced"
	default:
		return "unknown"
	}
}

// ApplyResult is the outcome of applying one manifest Link.
type ApplyResult struct {
	Link   Link
	Action ApplyAction
	// BackupPath is set when Action is ActionReplaced and holds the path the
	// previous target was moved to before the new symlink was created.
	BackupPath string
}

// ApplyOptions controls how Apply handles targets that already exist.
type ApplyOptions struct {
	// Force makes Apply replace a conflicting file or a symlink pointing at
	// the wrong place, instead of skipping it. The previous target is moved
	// aside rather than deleted, so it comes back as ActionReplaced with a
	// BackupPath rather than being lost.
	Force bool
}

// Apply creates symlinks for every manifest link whose target is missing.
// With the zero ApplyOptions it never touches a target that already exists
// in some form, whether that's a correct link, a link to the wrong place, or
// a plain file - those come back as skipped results rather than being
// overwritten. Set Force to replace conflicts and wrong links instead,
// backing up whatever was there first.
func Apply(root string, m *Manifest, opts ApplyOptions) ([]ApplyResult, error) {
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
			if !opts.Force {
				out = append(out, ApplyResult{Link: s.Link, Action: ActionSkippedConflict})
				continue
			}
			result, err := replace(root, s.Link)
			if err != nil {
				return nil, err
			}
			out = append(out, result)
		case StateWrongLink:
			if !opts.Force {
				out = append(out, ApplyResult{Link: s.Link, Action: ActionSkippedWrongLink})
				continue
			}
			result, err := replace(root, s.Link)
			if err != nil {
				return nil, err
			}
			out = append(out, result)
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

// replace moves whatever currently sits at link.Target out of the way and
// creates a fresh symlink to its source in its place.
func replace(root string, link Link) (ApplyResult, error) {
	source, err := filepath.Abs(filepath.Join(root, link.Source))
	if err != nil {
		return ApplyResult{}, fmt.Errorf("resolving source for %s: %w", link.Target, err)
	}

	backup, err := backupPath(link.Target)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("choosing backup path for %s: %w", link.Target, err)
	}
	if err := os.Rename(link.Target, backup); err != nil {
		return ApplyResult{}, fmt.Errorf("backing up %s: %w", link.Target, err)
	}
	if err := os.Symlink(source, link.Target); err != nil {
		return ApplyResult{}, fmt.Errorf("linking %s: %w", link.Target, err)
	}
	return ApplyResult{Link: link, Action: ActionReplaced, BackupPath: backup}, nil
}

// backupPath returns a path next to target that nothing currently occupies,
// preferring target+".bak" and falling back to a numbered suffix.
func backupPath(target string) (string, error) {
	candidate := target + ".bak"
	for i := 1; ; i++ {
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
		candidate = fmt.Sprintf("%s.bak.%d", target, i)
	}
}

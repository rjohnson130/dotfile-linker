package dotlink

import (
	"fmt"
	"os"
	"path/filepath"
)

// State is the on-disk state of a link's target path relative to what the
// manifest says it should be.
type State int

const (
	// StateMissing means nothing exists at the target path yet.
	StateMissing State = iota
	// StateLinked means the target is a symlink pointing at the expected source.
	StateLinked
	// StateWrongLink means the target is a symlink, but points somewhere else.
	StateWrongLink
	// StateConflict means the target exists and is not a symlink at all.
	StateConflict
)

func (s State) String() string {
	switch s {
	case StateMissing:
		return "missing"
	case StateLinked:
		return "ok"
	case StateWrongLink:
		return "wrong"
	case StateConflict:
		return "conflict"
	default:
		return "unknown"
	}
}

// LinkStatus is the result of comparing one manifest Link against the
// filesystem.
type LinkStatus struct {
	Link   Link
	State  State
	Actual string // current symlink destination, only set for StateWrongLink
}

// CheckStatus resolves every link's source against root and reports how the
// target path currently compares to it, without changing anything on disk.
func CheckStatus(root string, m *Manifest) ([]LinkStatus, error) {
	out := make([]LinkStatus, 0, len(m.Links))
	for _, link := range m.Links {
		wantSource, err := filepath.Abs(filepath.Join(root, link.Source))
		if err != nil {
			return nil, fmt.Errorf("resolving source for %s: %w", link.Target, err)
		}

		info, err := os.Lstat(link.Target)
		switch {
		case os.IsNotExist(err):
			out = append(out, LinkStatus{Link: link, State: StateMissing})
			continue
		case err != nil:
			return nil, fmt.Errorf("checking %s: %w", link.Target, err)
		}

		if info.Mode()&os.ModeSymlink == 0 {
			out = append(out, LinkStatus{Link: link, State: StateConflict})
			continue
		}

		actual, err := os.Readlink(link.Target)
		if err != nil {
			return nil, fmt.Errorf("reading link %s: %w", link.Target, err)
		}
		if !filepath.IsAbs(actual) {
			actual = filepath.Join(filepath.Dir(link.Target), actual)
		}
		actual, err = filepath.Abs(actual)
		if err != nil {
			return nil, fmt.Errorf("resolving link %s: %w", link.Target, err)
		}

		if actual == wantSource {
			out = append(out, LinkStatus{Link: link, State: StateLinked})
		} else {
			out = append(out, LinkStatus{Link: link, State: StateWrongLink, Actual: actual})
		}
	}
	return out, nil
}

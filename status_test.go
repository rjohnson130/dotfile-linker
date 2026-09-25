package dotlink

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckStatusStates(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "src")
	targets := filepath.Join(base, "dst")
	if err := os.MkdirAll(filepath.Join(root, "vim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targets, 0o755); err != nil {
		t.Fatal(err)
	}

	vimrc := filepath.Join(root, "vim", "vimrc")
	if err := os.WriteFile(vimrc, []byte("set nocompatible\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "other")
	if err := os.WriteFile(other, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkedTarget := filepath.Join(targets, "linked")
	if err := os.Symlink(vimrc, linkedTarget); err != nil {
		t.Fatal(err)
	}
	wrongTarget := filepath.Join(targets, "wrong")
	if err := os.Symlink(other, wrongTarget); err != nil {
		t.Fatal(err)
	}
	conflictTarget := filepath.Join(targets, "conflict")
	if err := os.WriteFile(conflictTarget, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	missingTarget := filepath.Join(targets, "missing")

	m := &Manifest{Links: []Link{
		{Target: linkedTarget, Source: "vim/vimrc", Line: 1, Col: 1},
		{Target: wrongTarget, Source: "vim/vimrc", Line: 2, Col: 1},
		{Target: conflictTarget, Source: "vim/vimrc", Line: 3, Col: 1},
		{Target: missingTarget, Source: "vim/vimrc", Line: 4, Col: 1},
	}}

	statuses, err := CheckStatus(root, m)
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if len(statuses) != 4 {
		t.Fatalf("got %d statuses, want 4", len(statuses))
	}

	wantStates := []State{StateLinked, StateWrongLink, StateConflict, StateMissing}
	for i, s := range statuses {
		if s.State != wantStates[i] {
			t.Errorf("statuses[%d].State = %s, want %s", i, s.State, wantStates[i])
		}
	}

	wantActual, err := filepath.Abs(other)
	if err != nil {
		t.Fatal(err)
	}
	if statuses[1].Actual != wantActual {
		t.Errorf("wrong link Actual = %q, want %q", statuses[1].Actual, wantActual)
	}
}

func TestCheckStatusResolvesRelativeSymlink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "src")
	targets := filepath.Join(base, "dst")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targets, 0o755); err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(root, "zshrc")
	if err := os.WriteFile(source, []byte("export PATH\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(targets, "zshrc")
	rel, err := filepath.Rel(targets, source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rel, target); err != nil {
		t.Fatal(err)
	}

	m := &Manifest{Links: []Link{{Target: target, Source: "zshrc", Line: 1, Col: 1}}}
	statuses, err := CheckStatus(root, m)
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if len(statuses) != 1 || statuses[0].State != StateLinked {
		t.Fatalf("got %+v, want single StateLinked", statuses)
	}
}

func TestCheckStatusExpandsDirectorySource(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "src")
	targets := filepath.Join(base, "dst")
	nvimDir := filepath.Join(root, "nvim")
	if err := os.MkdirAll(filepath.Join(nvimDir, "lua"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targets, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		filepath.Join(nvimDir, "init.lua"):        "-- init\n",
		filepath.Join(nvimDir, "lua", "opts.lua"): "-- opts\n",
		filepath.Join(nvimDir, ".DS_Store"):       "junk",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := &Manifest{
		Links:  []Link{{Target: filepath.Join(targets, "nvim"), Source: "nvim", Line: 1, Col: 1}},
		Ignore: []string{".DS_Store"},
	}

	statuses, err := CheckStatus(root, m)
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("got %d statuses, want 2: %+v", len(statuses), statuses)
	}

	wantTargets := map[string]bool{
		filepath.Join(targets, "nvim", "init.lua"):        true,
		filepath.Join(targets, "nvim", "lua", "opts.lua"): true,
	}
	for _, s := range statuses {
		if s.State != StateMissing {
			t.Errorf("state for %s = %s, want missing", s.Link.Target, s.State)
		}
		if !wantTargets[s.Link.Target] {
			t.Errorf("unexpected target %s", s.Link.Target)
		}
		delete(wantTargets, s.Link.Target)
	}
	if len(wantTargets) != 0 {
		t.Errorf("missing expected targets: %v", wantTargets)
	}
}

func TestMatchesAny(t *testing.T) {
	patterns := []string{"*.swp", "cache/*"}
	tests := []struct {
		name, rel string
		want      bool
	}{
		{"foo.swp", "sub/foo.swp", true},
		{"x", "cache/x", true},
		{"other", "other", false},
	}
	for _, tt := range tests {
		got, err := matchesAny(patterns, tt.name, tt.rel)
		if err != nil {
			t.Fatalf("matchesAny(%q,%q): %v", tt.name, tt.rel, err)
		}
		if got != tt.want {
			t.Errorf("matchesAny(%q,%q) = %v, want %v", tt.name, tt.rel, got, tt.want)
		}
	}
}

func TestMatchesAnyInvalidPattern(t *testing.T) {
	if _, err := matchesAny([]string{"["}, "x", "x"); err == nil {
		t.Fatal("matchesAny: got nil error, want error for bad pattern")
	}
}

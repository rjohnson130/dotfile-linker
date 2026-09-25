package dotlink

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir: %v", err)
	}
	input := "~/.vimrc = vim/vimrc\n# a comment\n\n~/.config/nvim = nvim\n"
	m, err := Parse(strings.NewReader(input), "test.link")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Link{
		{Target: filepath.Join(home, ".vimrc"), Source: "vim/vimrc", Line: 1, Col: 1},
		{Target: filepath.Join(home, ".config", "nvim"), Source: "nvim", Line: 4, Col: 1},
	}
	if !reflect.DeepEqual(m.Links, want) {
		t.Errorf("Links = %#v, want %#v", m.Links, want)
	}
	if len(m.Ignore) != 0 {
		t.Errorf("Ignore = %#v, want empty", m.Ignore)
	}
}

func TestParseIgnorePatterns(t *testing.T) {
	input := "!*.swp\n!.DS_Store\n~/.vimrc = vim/vimrc\n"
	m, err := Parse(strings.NewReader(input), "test.link")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"*.swp", ".DS_Store"}
	if !reflect.DeepEqual(m.Ignore, want) {
		t.Errorf("Ignore = %#v, want %#v", m.Ignore, want)
	}
}

func TestParseEmptyIgnorePattern(t *testing.T) {
	_, err := Parse(strings.NewReader("!   \n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 1 || !strings.Contains(pe.Msg, "empty ignore pattern") {
		t.Errorf("got %+v", pe)
	}
}

func TestParseInvalidIgnorePattern(t *testing.T) {
	_, err := Parse(strings.NewReader("![\n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 1 || !strings.Contains(pe.Msg, "invalid ignore pattern") {
		t.Errorf("got %+v", pe)
	}
}

func TestParseMissingEquals(t *testing.T) {
	raw := "~/.zshrc"
	_, err := Parse(strings.NewReader(raw+"\n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 1 || pe.Col != len(raw)+1 {
		t.Errorf("got Line=%d Col=%d, want Line=1 Col=%d", pe.Line, pe.Col, len(raw)+1)
	}
	if !strings.Contains(pe.Msg, "expected '='") {
		t.Errorf("Msg = %q", pe.Msg)
	}
}

func TestParseMissingTarget(t *testing.T) {
	_, err := Parse(strings.NewReader(" = source\n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 1 || pe.Col != 2 {
		t.Errorf("got Line=%d Col=%d, want Line=1 Col=2", pe.Line, pe.Col)
	}
	if !strings.Contains(pe.Msg, "missing link target") {
		t.Errorf("Msg = %q", pe.Msg)
	}
}

func TestParseMissingSource(t *testing.T) {
	raw := "~/.vimrc =   "
	_, err := Parse(strings.NewReader(raw+"\n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 1 || pe.Col != len(raw)+1 {
		t.Errorf("got Line=%d Col=%d, want Line=1 Col=%d", pe.Line, pe.Col, len(raw)+1)
	}
	if !strings.Contains(pe.Msg, "missing source path") {
		t.Errorf("Msg = %q", pe.Msg)
	}
}

func TestParseDuplicateTarget(t *testing.T) {
	_, err := Parse(strings.NewReader("~/.vimrc = a\n~/.vimrc = b\n"), "test.link")
	pe := asParseError(t, err)
	if pe.Line != 2 {
		t.Errorf("Line = %d, want 2", pe.Line)
	}
	if !strings.Contains(pe.Msg, "already linked at line 1") {
		t.Errorf("Msg = %q", pe.Msg)
	}
}

func TestParseRelativeTargetRejected(t *testing.T) {
	_, err := Parse(strings.NewReader("relative/path = source\n"), "test.link")
	pe := asParseError(t, err)
	if !strings.Contains(pe.Msg, "must be absolute") {
		t.Errorf("Msg = %q", pe.Msg)
	}
}

func TestParseErrorFormatting(t *testing.T) {
	pe := &ParseError{
		File: "test.link", Line: 1, Col: 9, Src: "~/.zshrc",
		Msg: "expected '=' separating link target and source, found none",
	}
	want := "test.link:1:9: expected '=' separating link target and source, found none\n    ~/.zshrc\n        ^"
	if got := pe.Error(); got != want {
		t.Errorf("Error() =\n%s\nwant\n%s", got, want)
	}
}

func TestColOf(t *testing.T) {
	tests := []struct {
		part string
		base int
		want int
	}{
		{"  foo", 0, 3},
		{"foo", 0, 1},
		{"\tfoo", 0, 2},
		{"   ", 5, 9},
	}
	for _, tt := range tests {
		if got := colOf(tt.part, tt.base); got != tt.want {
			t.Errorf("colOf(%q, %d) = %d, want %d", tt.part, tt.base, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir: %v", err)
	}
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"~", home, false},
		{"~/foo/bar", filepath.Join(home, "foo", "bar"), false},
		{"/abs/path", "/abs/path", false},
		{"relative", "", true},
	}
	for _, tt := range tests {
		got, err := expandHome(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("expandHome(%q) = nil error, want error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("expandHome(%q) = error %v, want nil", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func asParseError(t *testing.T, err error) *ParseError {
	t.Helper()
	if err == nil {
		t.Fatal("Parse: got nil error, want *ParseError")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("Parse: got %T, want *ParseError", err)
	}
	return pe
}

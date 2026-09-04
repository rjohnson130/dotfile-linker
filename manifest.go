// Package dotlink parses and checks manifests that describe dotfile symlinks.
package dotlink

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Link is one target = source mapping from a manifest.
//
// Target is the absolute path where a symlink should exist (after ~
// expansion). Source is left as written in the manifest, relative to
// whatever root the caller resolves it against.
type Link struct {
	Target string
	Source string
	Line   int
	Col    int
}

// Manifest is a parsed, validated set of links.
type Manifest struct {
	Links []Link
}

// ParseError describes exactly where a manifest failed to parse, including
// the offending line and a caret pointing at the column.
type ParseError struct {
	File string
	Line int
	Col  int
	Src  string
	Msg  string
}

func (e *ParseError) Error() string {
	pad := strings.Repeat(" ", max(e.Col-1, 0))
	return fmt.Sprintf("%s:%d:%d: %s\n    %s\n    %s^", e.File, e.Line, e.Col, e.Msg, e.Src, pad)
}

// Parse reads a manifest of "target = source" lines, one per line. Blank
// lines and lines starting with # (after leading whitespace) are ignored.
//
//	~/.vimrc        = vim/vimrc
//	~/.config/nvim  = nvim
func Parse(r io.Reader, filename string) (*Manifest, error) {
	scanner := bufio.NewScanner(r)
	var links []Link
	seen := make(map[string]Link)

	line := 0
	for scanner.Scan() {
		line++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		eq := strings.IndexByte(raw, '=')
		if eq == -1 {
			return nil, &ParseError{
				File: filename, Line: line, Col: len(raw) + 1, Src: raw,
				Msg: "expected '=' separating link target and source, found none",
			}
		}

		targetPart, sourcePart := raw[:eq], raw[eq+1:]
		targetTrimmed := strings.TrimSpace(targetPart)
		sourceTrimmed := strings.TrimSpace(sourcePart)

		if targetTrimmed == "" {
			return nil, &ParseError{
				File: filename, Line: line, Col: eq + 1, Src: raw,
				Msg: "missing link target before '='",
			}
		}
		if sourceTrimmed == "" {
			return nil, &ParseError{
				File: filename, Line: line, Col: len(raw) + 1, Src: raw,
				Msg: "missing source path after '='",
			}
		}

		targetCol := colOf(targetPart, 0)
		expanded, err := expandHome(targetTrimmed)
		if err != nil {
			return nil, &ParseError{
				File: filename, Line: line, Col: targetCol, Src: raw,
				Msg: err.Error(),
			}
		}

		if prev, ok := seen[expanded]; ok {
			return nil, &ParseError{
				File: filename, Line: line, Col: targetCol, Src: raw,
				Msg: fmt.Sprintf("target %s already linked at line %d", targetTrimmed, prev.Line),
			}
		}

		link := Link{Target: expanded, Source: sourceTrimmed, Line: line, Col: targetCol}
		seen[expanded] = link
		links = append(links, link)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", filename, err)
	}

	return &Manifest{Links: links}, nil
}

// colOf returns the 1-indexed column of the first non-whitespace rune in
// part, given that part starts at byte offset base within its source line.
func colOf(part string, base int) int {
	leading := len(part) - len(strings.TrimLeft(part, " \t"))
	return base + leading + 1
}

func expandHome(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving ~: %w", err)
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving ~: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("link target must be absolute or start with ~/, got %q", path)
	}
	return path, nil
}

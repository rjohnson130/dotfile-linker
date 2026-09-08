// Command dotlink is a thin CLI around the dotlink package.
package main

import (
	"flag"
	"fmt"
	"os"

	dotlink "github.com/rjohnson130/dotfile-linker"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "check":
		err = cmdCheck(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "apply":
		err = cmdApply(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  dotlink check <manifest>                    parse a manifest and report errors
  dotlink status <manifest> <root-dir>        show link state for each entry
  dotlink apply [--force] <manifest> <root-dir>  create missing symlinks

  --force  also replace conflicting files and wrong links, after moving
           whatever was at the target aside as a .bak file`)
}

func cmdCheck(args []string) error {
	if len(args) != 1 {
		usage()
		os.Exit(2)
	}
	m, err := parseManifest(args[0])
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d link(s), no errors\n", args[0], len(m.Links))
	return nil
}

func cmdStatus(args []string) error {
	if len(args) != 2 {
		usage()
		os.Exit(2)
	}
	m, err := parseManifest(args[0])
	if err != nil {
		return err
	}
	statuses, err := dotlink.CheckStatus(args[1], m)
	if err != nil {
		return err
	}
	for _, s := range statuses {
		fmt.Printf("%-8s %s\n", s.State, s.Link.Target)
		if s.State == dotlink.StateWrongLink {
			fmt.Printf("         currently points to %s\n", s.Actual)
		}
	}
	return nil
}

func cmdApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	force := fs.Bool("force", false, "replace conflicting files and wrong links, backing them up first")
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	rest := fs.Args()
	if len(rest) != 2 {
		usage()
		os.Exit(2)
	}

	m, err := parseManifest(rest[0])
	if err != nil {
		return err
	}
	results, err := dotlink.Apply(rest[1], m, dotlink.ApplyOptions{Force: *force})
	if err != nil {
		return err
	}
	skipped := 0
	for _, r := range results {
		fmt.Printf("%-8s %s\n", r.Action, r.Link.Target)
		switch r.Action {
		case dotlink.ActionCreated:
		case dotlink.ActionReplaced:
			fmt.Printf("         previous target backed up to %s\n", r.BackupPath)
		default:
			skipped++
		}
	}
	if skipped > 0 {
		fmt.Printf("%d link(s) already exist and were left alone\n", skipped)
	}
	return nil
}

func parseManifest(path string) (*dotlink.Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return dotlink.Parse(f, path)
}

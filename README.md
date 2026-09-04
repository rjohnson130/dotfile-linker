# dotfile-linker

A small library and CLI for turning a plain text manifest into dotfile
symlinks, with error messages that actually tell you what's wrong and where.

## the problem

Most dotfile setups end up as a shell script full of `ln -sf` calls, or a
tool like GNU Stow that assumes a particular directory layout. Both work
fine until you mistype something in a config file, at which point you get a
generic parse failure or, worse, a silently wrong symlink. This is a
manifest-driven approach where the manifest format is simple enough to hand
edit, and mistakes in it point at the exact line and column.

## manifest format

One `target = source` pair per line. `target` is where the symlink should
live (must be absolute or start with `~/`); `source` is the real file,
resolved relative to a root directory you pass on the command line (your
dotfiles checkout, typically). Blank lines and lines starting with `#` are
ignored.

```
# ~/dotfiles.link
~/.vimrc       = vim/vimrc
~/.zshrc       = zsh/zshrc
~/.config/nvim = nvim
```

## usage

```
$ go build -o dotlink ./cmd/dotlink

$ ./dotlink check ~/dotfiles.link
~/dotfiles.link: 3 link(s), no errors

$ ./dotlink status ~/dotfiles.link ~/dotfiles
ok       /home/rjohnson130/.vimrc
missing  /home/rjohnson130/.zshrc
wrong    /home/rjohnson130/.config/nvim
         currently points to /home/rjohnson130/old-nvim-config
```

`status` never touches the filesystem beyond reading it — it reports
`missing`, `ok`, `wrong` (a symlink pointing somewhere else), or `conflict`
(something exists at that path and it isn't a symlink at all). Actually
creating the links is not implemented yet; see the roadmap below.

## what a bad manifest looks like

Given this file:

```
~/.vimrc = vim/vimrc
~/.zshrc
```

```
$ ./dotlink check dotfiles.link
dotfiles.link:2:9: expected '=' separating link target and source, found none
    ~/.zshrc
        ^
```

The line and column point at the actual problem instead of just failing
with "invalid manifest" or a raw Go error.

## library

```go
import dotlink "github.com/rjohnson130/dotfile-linker"

m, err := dotlink.Parse(reader, "dotfiles.link")
if err != nil {
    // err is a *dotlink.ParseError with File, Line, Col, and a
    // ready-to-print Error() message
}

statuses, err := dotlink.CheckStatus("/home/me/dotfiles", m)
```

## status

Early skeleton. Parsing and status checking work; applying the links does
not yet.

## license

MIT, see LICENSE.

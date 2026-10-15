# AGENTS.md

adjudicate (CLI `adj`) is a plain-text decision log: an agent asks a question,
the owner decides in batches, and later agents read the decisions. Go, standard
library only.

## Source of truth

- Slicer catalog decisions from C23 onward, including implementer decisions
  recorded under the C32 standing rule, override PROJECT.md where they differ:
  name `adjudicate` with CLI `adj`; module `github.com/squalor-xyz/adjudicate`;
  Go with the standard library only; one Markdown file per record with JSON
  frontmatter; sequential IDs; GPL-3.0-or-later. PROJECT.md still describes the
  earlier Python/TOML draft under the name dlog; read it as background.
- `LICENSE` is a stub until the owner supplies the full GPL-3.0 text and the
  copyright holder (C31, C39). Each Go file starts with
  `// SPDX-License-Identifier: GPL-3.0-or-later`.

## Layout

- `cmd/adj`: `main`; calls `internal/cli.Run` and exits with its code.
- `internal/cli`: subcommand dispatch and usage.
- `internal/model`: the record type, validation, status transitions and the
  record file format; its only file I/O is `ReadFile` and `WriteFile` in
  `file.go`.
- `internal/commands`, `internal/store`, `internal/query`, `internal/render`:
  added by later slices.
- `scripts/env.sh`: Go cache and temp setup (see below).

Exit codes for every command: 0 success; 1 a failed operation or failed check;
2 a usage error (C34).

## Setup and checks

Run from the repo root:

```sh
. scripts/env.sh                   # Go caches and temp files stay under .cache/ (C33)
go test ./...
go vet ./...
gofmt -l cmd internal              # must print nothing
go build -o .cache/bin/adj ./cmd/adj
```

`scripts/env.sh` must be sourced from bash or zsh (C40). Dash and other shells
that are not bash or zsh cannot report the sourced path, so there it prints an
error, returns 1, and sets and creates nothing.

Never run `go env -w`; it writes outside the repo. Do not add a Makefile.

## Rules

- Go standard library only. A new dependency needs owner approval (C24).
- Test-first: write the failing test, watch it fail for the stated reason, then
  write the code. Use Go `testing`, table-driven where natural. Tests use
  `t.TempDir()` and never touch this repo's own decision store.
- Only the owner makes the decide transition (`adj decide`, C27). Agents ask,
  check and read.
- Work is tracked in slicer. Read and change tracking state only through the
  `slicer` CLI; never open, search or edit `.slicer/`.
- Standing rule (C32): an interface detail that PROJECT.md and the owner's
  decisions leave open is decided by the implementer. Record it with
  `slicer catalog add --kind decision` and a one-line reason. The reviewer
  checks it, and the owner may overrule it.
- No commits unless the owner asks. No AI attribution in commits or PR text.

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
- `internal/cli`: subcommand dispatch, usage and flag parsing (`flags.go`:
  flags after positional arguments, repeatable string flags). `Run` is the
  process entry; `RunWith(Env, ...)` takes the repo root, clock, environment
  and working directory as parameters for tests. Each command's handler is in
  its own file (`init.go`, `ask.go`, `decide.go`, which holds decide,
  reject and withdraw, `show.go` and `list.go`); `cli.go` holds the dispatch
  and shared helpers.
- `internal/commands`: what each command does to the store, apart from
  parsing, output and exit codes (`init.go`, `ask.go`, `decide.go`, `show.go`,
  `list.go`). `errors.go` holds `UsageError`, the error that exits 2.
- `internal/model`: the record type, validation, status transitions and the
  record file format; its only file I/O is `ReadFile` and `WriteFile` in
  `file.go`.
- `internal/store`: one repo's record directory (`store.go`) and its config
  (`config.go`); see "Store and config" below.
- `internal/query`: record filters, pure functions over records (`filter.go`).
  `internal/render`: text and JSON output (`text.go`, `json.go`); the JSON
  shapes are in `docs/json.md`.
- `scripts/env.sh`: Go cache and temp setup (see below).

Exit codes for every command: 0 success; 1 a failed operation or failed check;
2 a usage error (C34).

## Commands

- `adj init [--root DIR]`: writes `.adjudicate.json` (`{"store": "decisions"}`)
  if it is missing and creates the store directory. It acts on DIR or the
  working directory, never on a parent. A second successful run changes
  nothing; a failed run removes a config it wrote.
- `adj ask TITLE [--option LABEL:SUMMARY]... [--tradeoffs LABEL:TEXT]...
  [--recommend LABEL] [--reasoning TEXT] [--key K] [--tag T]...
  [--context TEXT | --stdin] [--by NAME] [--root DIR]`: creates an `open`
  record with one `ask` history entry and prints its ID. Flags may come before
  or after TITLE. A blank `--key` or `--tag` exits 2; `--key` is stored as
  typed. `asked_by` is `--by`, else `$ADJ_ASKED_BY`, else blank.
- `adj decide ID OPTION --why TEXT`, or `adj decide ID --text TEXT --why TEXT`,
  `[--by NAME] [--root DIR]`: moves an `open` record to `decided`. OPTION
  (trimmed) must be one of the record's option labels; `decision` is that
  label, or the `--text` answer. `rationale` is the `--why` text, and
  `decided_at` and one `decide` history entry are dated now. `adj reject ID
  --why TEXT` and `adj withdraw ID --why TEXT` move an `open` record to
  `rejected` or `withdrawn` the same way, with no decision. `decided_by` is
  `--by`, else `$ADJ_DECIDED_BY`, else blank (C27). `--why` and `--text` are
  kept as typed but must not be blank. All three print the record's ID. A bad
  argument exits 2, including an OPTION the record does not offer while the
  record is open. An unknown ID, or a record that is not `open` (checked before
  OPTION), exits 1 and changes nothing; its message names the status.
- `adj show ID [--json] [--root DIR]`: prints one record, each set field and
  the history; with `--json` it prints the record object (`docs/json.md`). A
  non-number or 0 ID, or an extra argument, exits 2; an unknown or duplicate
  ID exits 1.
- `adj list [--status S] [--tag T]... [--key K] [--json] [--root DIR]`: lists
  records in ID order, one line each (ID, status, title); with `--json` it
  prints `{"records": [...]}` (`docs/json.md`). Every filter given must match.
  `--tag` repeats, and every tag given must be on the record. `--status` is one
  of the five statuses; its value and each `--tag` value are trimmed, while
  `--key` is matched as typed against the stored key. A bad status, a blank
  `--tag` or `--key`, or an argument, exits 2. No match is not
  an error (exit 0).
- Every command takes `--root DIR`. Without it the root is found from the
  working directory upwards (`store.FindRoot`); with it nothing is searched
  for, and a command other than `init` needs `DIR/.adjudicate.json`. A missing
  config says to run `adj init`.

## Store and config

- The repo root is the nearest directory, from the working directory up, that
  holds `.adjudicate.json` (`store.FindRoot`), unless `--root` names it (see
  Commands). The file is one JSON object; its only key is `store`, the store
  directory relative to the root (default `decisions`, C29). Unknown keys, an
  absolute path or a path that leaves the root are errors. With no config
  file `store.Open(root)` uses the defaults.
- A record file is `<ID padded to four digits>-<slug>.md` in the store
  directory; the slug is the title's lowercase ASCII letters and digits joined
  by `-`. Files not named like that are ignored; a file named like a record
  that is malformed or invalid makes `List` fail (C61). The next ID is one past
  the highest ID in the file names. `Update` keeps the file's path and
  permission bits; new files are 0644.

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
  `t.TempDir()` and never touch this repo's own decision store. Temp dirs live
  under `.cache/tmp`, inside this repo, so a CLI test passes an explicit root
  (`cli.Env.Root`) and fails on any use of the real working directory; a test
  of root discovery bounds the walk with `cli.Env.Stop`. No test calls
  `os.Chdir`.
- Only the owner makes the decide transition (`adj decide`, C27). Agents ask,
  check and read.
- Work is tracked in slicer. Read and change tracking state only through the
  `slicer` CLI; never open, search or edit `.slicer/`.
- Standing rule (C32): an interface detail that PROJECT.md and the owner's
  decisions leave open is decided by the implementer. Record it with
  `slicer catalog add --kind decision` and a one-line reason. The reviewer
  checks it, and the owner may overrule it.
- No commits unless the owner asks. No AI attribution in commits or PR text.

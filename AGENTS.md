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
- `LICENSE` is the full GPL-3.0 text, unmodified; `NOTICE` holds the
  copyright line (Squalor LLC) and the "or any later version" notice (C31,
  C131). Each Go file starts with
  `// SPDX-License-Identifier: GPL-3.0-or-later`.

## Layout

- `cmd/adj`: `main`; calls `internal/cli.Run` and exits with its code.
- `internal/cli`: subcommand dispatch, usage and flag parsing (`flags.go`:
  flags after positional arguments, repeatable string flags). `Run` is the
  process entry; `RunWith(Env, ...)` takes the repo root, clock, environment
  and working directory as parameters for tests. Each command's handler is in
  its own file (`init.go`, `ask.go`, `decide.go`, which holds decide,
  reject and withdraw, `supersede.go`, `show.go`, `list.go`, `inbox.go`,
  `check.go`, `context.go`, `lint.go` and `ai.go`);
  `cli.go` holds the dispatch and shared helpers.
- `internal/commands`: what each command does to the store, apart from
  parsing, output and exit codes (`init.go`, `ask.go`, `decide.go`,
  `supersede.go`, `show.go`, `list.go`, `inbox.go`, `check.go`, `context.go`,
  `lint.go`, `ai.go`). `ai.md` is the agent snippet, embedded by `ai.go`.
  `errors.go` holds `UsageError`, the error that exits 2.
- `internal/model`: the record type, validation, status transitions and the
  record file format; its only file I/O is `ReadFile` and `WriteFile` in
  `file.go`.
- `internal/store`: one repo's record directory (`store.go`) and its config
  (`config.go`); see "Store and config" below.
- `internal/query`: record filters and the `check` search, pure functions over
  records (`filter.go`, `search.go`).
- `internal/lint`: `adj lint`'s checks (`lint.go`): reads each record file with
  `model.Parse` through `store.Files`, so it works on a store that `List`
  refuses, and returns findings; see the `adj lint` entry below.
- `internal/render`: text and JSON output (`text.go`, `inbox.go`, `check.go`,
  `context.go`, `lint.go`, `json.go`);
  the JSON shapes are in `docs/json.md`, and `internal/cli/jsonshape_test.go`
  pins each read command's `--json` keys to it.
- `scripts/env.sh`: Go cache and temp setup (see below).

Exit codes for every command: 0 success; 1 a failed operation or failed check;
2 a usage error (C34). `adj check` is the exception that matters to scripts:
it exits 0 for every completed search, "not found" included (see its entry
below). `adj lint` exits 1 when it finds an error (see its entry).

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
- `adj supersede OLD --by NEW [--root DIR]`: replaces a decided record with
  another decided one. OLD becomes `superseded` with `superseded_by` NEW and
  one `supersede` history entry (note `by NEW`); NEW stays `decided`, lists OLD
  in `supersedes` and gets one `supersede` entry (note `supersedes OLD`). Both
  are validated in memory first, so a refused supersede writes nothing. OLD is
  written first (C136); if NEW then cannot be written, OLD is put back, and if
  that fails too the error says OLD is left superseded and NEW has no link back
  (the decision in force is still NEW). OLD stays readable with `adj show` and
  leaves `adj context`; `adj check` leads from it to NEW. Prints OLD's ID
  (C134). A missing or non-positive ID, a missing `--by`, OLD equal to NEW or an
  extra argument exits 2; an unknown ID, or a record that is not `decided`,
  exits 1 and changes nothing. Both IDs are looked up first, so an unknown ID is
  reported before a status problem; then OLD's status is checked before NEW's,
  and the message names the status.
  Only the owner supersedes, as for decide (C27): the agent snippet lists it
  among the commands never to run (C137). `adj lint` finds supersede cycles.
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
- `adj inbox [--json] [--root DIR]`: lists the `open` records, oldest first (by
  `created`, then ID). Text: per record `ID  title`, then indented its created
  date, its context (when set), every option (`label: summary`, with
  `tradeoffs:` under it when set) and the recommendation with its reasoning;
  entries are separated by a blank line.
  `--json` prints `{"records": [...]}`, the same record objects as `adj list`
  (`docs/json.md`). An empty inbox prints nothing (JSON: an empty array) and
  exits 0. An argument or other flag exits 2.
- `adj check QUERY [--json] [--root DIR]`: asks whether QUERY has been decided,
  so an agent checks before it asks the owner. QUERY (trimmed, one argument:
  quote it if it has spaces) is one substring, not split into words; it matches
  a record when its title, key, one of its tags or its decision text contains
  it, ignoring case (C105). A match is listed by status: `decided`, `open`, or
  `closed` for a rejected, withdrawn or superseded record (C106). A superseded
  match also leads to the end of its `superseded_by` chain; if that is a decided
  record it is listed as `decided` too (C113). The first line of the text output
  is the result: `decided` if any decided record is listed, else `open`, else
  `closed`, else `not found` (C107). Then one line per record, `ID  status
  title`: decided, open, then closed ones, with a decided record's `Decision:`
  and `Rationale:` under it, and a closed record's `Rationale:` (why it was
  rejected, withdrawn or superseded; the whole text, C132, C138), after `Superseded
  by:` on a superseded one. An open record has no detail lines.
  `--json` prints `{"query", "result", "decided", "open", "closed"}`
  (`docs/json.md`). Exit 0 for every completed check, whatever the result
  (including `closed` and `not found`), so a script reads the `result`; exit 1
  means the check itself failed (no repo, an unreadable record file), which is
  never a "not found"; exit 2 is a usage error: no QUERY, more than one, or a
  blank one (C108, C109).
- `adj context [--tag T]... [--key-prefix P] [--format md|json] [--root DIR]`:
  prints the `decided` records as compact context for an agent; open, rejected,
  withdrawn and superseded records are left out (a superseded decision gives
  way to its replacement, which is `decided`). Every filter given must match.
  `--tag` repeats and every tag must be on the record (trimmed); `--key-prefix`
  is a plain string prefix of the stored key, as typed, so a record with no key
  never matches it. Records are in ID order, so the same store gives the same
  bytes. `--format md` (the default) prints per record `- ID title [key]:
  decision` (no brackets without a key; a multi-line decision continues on
  indented lines), then `  Rationale: ` and the first non-blank line of the
  rationale. `--format json` prints `{"decisions": [...]}` with `id`, `title`,
  `key` (when set), `decision` and `rationale` (`docs/json.md`). No decision
  prints nothing in md and `{"decisions": []}` in JSON, exit 0. A blank `--tag`
  or `--key-prefix`, a `--format` other than `md` or `json`, or an argument,
  exits 2. A size budget flag is not part of v1 (owner scope call).
- `adj lint [--json] [--root DIR]`: checks the store for integrity problems
  (C08). It reads the record files itself (`store.Files`, `model.Parse`), so a
  broken or invalid file is a finding where `list` and `check` would fail (C61).
  Each finding names the record (the ID in its file name, C59), its file
  (relative to the root) and a check: `parse` (cannot be read or decoded),
  `fields` (one per `model.Validate` rule broken, and a link to the file's own
  ID), `id-mismatch` (the record's `id` is not the ID in its file name),
  `dangling-link` (`supersedes` or `superseded_by` names an ID no file has),
  `supersede-state` (`superseded_by` on a record that is not superseded),
  `supersede-cycle` (a loop of supersede links; one finding per record in it,
  naming only records that have a file), `key-conflict` (another `decided`
  record has the same key, trimmed and case-sensitive) and `decided-by` (a
  `decided` record without `decided_by`, a warning, C27); the rest are errors
  (C143). Text: per finding `FILE: record ID: SEVERITY: CHECK: MESSAGE`, sorted
  by ID, check and file, then `N errors, M warnings`; a clean store prints
  nothing (C141). `--json` prints `{"errors", "warnings", "findings"}`
  (`docs/json.md`, C142). Exit 1 if there is an error or lint could not run,
  else 0: warnings alone exit 0 (C140). An argument exits 2. Two files with one
  ID are not linted yet (S15, C144). The agent snippet does not name it (C145).
- `adj ai`: prints the AGENTS.md snippet in `internal/commands/ai.md` (check
  before asking and what each result means, ask, read the decisions, withdraw
  your own question, only the owner decides and never run decide, reject or
  supersede, exit codes, `ADJ_ASKED_BY`) and
  exits 0. It reads no repo and takes no flag or argument
  (so no `--root`); either exits 2. `internal/cli/ai_test.go` fails if the
  snippet names a command, flag or variable that does not exist, or runs past
  1800 bytes (C124, C125). When a command or flag an agent uses changes, change
  the snippet.
- Every command except `ai` takes `--root DIR`. Without it the root is found
  from the working directory upwards (`store.FindRoot`); with it nothing is
  searched for, and a command other than `init` needs `DIR/.adjudicate.json`.
  A missing config says to run `adj init`.

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
  that is malformed or invalid makes `List` fail (C61). `store.Files` lists
  those files (ID, name, path) without reading them. The next ID is one past
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
- Only the owner makes the decide transition (`adj decide`, C27) and the
  supersede transition (`adj supersede`). Agents ask, check and read; an agent
  may withdraw its own question (`adj withdraw`).
- Work is tracked in slicer. Read and change tracking state only through the
  `slicer` CLI; never open, search or edit `.slicer/`.
- Standing rule (C32): an interface detail that PROJECT.md and the owner's
  decisions leave open is decided by the implementer. Record it with
  `slicer catalog add --kind decision` and a one-line reason. The reviewer
  checks it, and the owner may overrule it.
- No commits unless the owner asks. No AI attribution in commits or PR text.

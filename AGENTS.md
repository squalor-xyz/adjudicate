# AGENTS.md

adjudicate (CLI `adj`) is a plain-text decision log: an agent asks a question,
the owner decides in batches, and later agents read the decisions. Go, standard
library only.

## Source of truth

- Slicer catalog decisions from C23 onward, including implementer decisions
  recorded under the C32 standing rule, override PROJECT.md where they differ:
  name `adjudicate` with CLI `adj`; module `github.com/squalor-xyz/adjudicate`;
  Go with the standard library only; one Markdown file per record with JSON
  frontmatter; sequential IDs; GPL-3.0-or-later. PROJECT.md was rewritten for
  0.1.0 (C165) to describe the shipped tool and cites those decisions; keep it
  in step when a command, a flag or the record format changes.
- `LICENSE` is the full GPL-3.0 text, unmodified; `NOTICE` holds the
  copyright line (Squalor LLC) and the "or any later version" notice (C31,
  C131). Each Go file starts with
  `// SPDX-License-Identifier: GPL-3.0-or-later`.

## Layout

- `cmd/adj`: `main`; calls `internal/cli.Run` and exits with its code.
  `roundtrip_test.go` builds `adj` and walks the v1 acceptance round trip.
- `internal/cli`: subcommand dispatch, usage and flag parsing (`flags.go`:
  flags after positional arguments, repeatable string flags). `Run` is the
  process entry; `RunWith(Env, ...)` takes the repo root, clock, environment
  and working directory as parameters for tests. Each command's handler is in
  its own file (`init.go`, `ask.go`, `decide.go`, which holds decide,
  reject and withdraw, `supersede.go`, `amend.go`, `show.go`, `list.go`,
  `inbox.go`, `check.go`, `context.go`, `lint.go`, `renumber.go`,
  `importadr.go`, `version.go`, `repos.go` and `ai.go`); `inbox_interactive.go`
  holds the interactive walk;
  `cli.go` holds the dispatch and shared helpers.
- `internal/commands`: what each command does to the store, apart from
  parsing, output and exit codes (`init.go`, `ask.go`, `decide.go`,
  `supersede.go`, `amend.go`, `show.go`, `list.go`, `inbox.go`, `check.go`,
  `context.go`, `lint.go`, `renumber.go`, `importadr.go`, `repos.go`,
  `ai.go`). `ai.md` is the agent snippet, embedded by `ai.go`.
  `errors.go` holds `UsageError`, the error that exits 2.
- `internal/adr`: one ADR file (Nygard/adr-tools or MADR) parsed into a neutral
  struct (`adr.go`); pure functions, no file I/O, fixtures in `testdata/`.
- `internal/model`: the record type, validation, status transitions and the
  record file format; its only file I/O is `ReadFile` and `WriteFile` in
  `file.go`. A `HistoryEntry` accepts an event this version does not know
  (`Event.Known` tells) and keys it does not know, and keeps both unchanged
  (`HistoryEntry.UnknownKeys`; written after the known keys in sorted order,
  so the bytes are stable). Keys that differ from a known key only in letter
  case are unknown keys unless spelled exactly (one wins, the rest are kept).
  For an unknown event `was` is not decoded: it is kept as written, whatever
  its shape (C213); for a known event `was` stays strict. The unknown parts
  are unexported strings, so entries are comparable (renumber compares them
  with `reflect.DeepEqual`, unknown keys included; `Was` is a pointer). The
  top-level record stays strict (S44).
- `internal/store`: one repo's record directory (`store.go`), its config
  (`config.go`), `Renumber` (`renumber.go`) and the ID lock (`lock_unix.go`,
  `lock_windows.go`, `lock_open.go`, `lock_other.go`); see "Store and config"
  below.
- `internal/userconfig`: per-user repo list, injected environment path
  resolution, strict loading, atomic saving and locked updates; separate from
  repo config.
  The shared `store.AcquireLock` wrapper reuses the store platform lock;
  `userconfig.Updater` passes a lock function for tests without shared state.
- `internal/query`: record filters and the `check` search, pure functions over
  records (`filter.go`, `search.go`).
- `internal/lint`: `adj lint`'s checks (`lint.go`): reads each record file with
  `model.Parse` through `store.Files`, so it works on a store that `List`
  refuses, and returns findings; see the `adj lint` entry below.
- `internal/render`: text and JSON output (`text.go`, `inbox.go`, `check.go`,
  `context.go`, `lint.go`, `repos.go`, `json.go`);
  the JSON shapes are in `docs/json.md`, and `internal/cli/jsonshape_test.go`
  pins each read command's `--json` keys to it.
- `README.md`: what adjudicate is, how to build it and use each command.
  `PROJECT.md`: goals, non-goals, data model, commands, acceptance criteria
  and Later items. `CHANGELOG.md`: release notes.
- `scripts/env.sh`: Go cache and temp setup (see below).
- `.gitattributes`: text files are checked out with LF on every system
  (`* text=auto eol=lf`), so the Windows CI job sees the bytes the tests
  expect; the CRLF fixtures in `internal/model/testdata/malformed` and
  `internal/adr/testdata/nygard/*crlf*.md`, and the PNG, are exempt.
- `.github/workflows/ci.yml`: CI on each push to `main` and each pull request
  (the checks below plus `adj lint` on `decisions/`, and `go test` on macOS,
  and `go vet` and `go test` on Windows).
  `.github/workflows/release.yml`: on a `v*` tag, the same checks, then eight
  archives (linux, darwin, freebsd as tar.gz and windows as zip holding
  `adj.exe`, each on amd64 and arm64, plain `go build` with `versionOverride`
  set to the tag) and a GitHub release made with `gh`, with
  the tag's CHANGELOG section as its notes (C193). Both stay standard
  library and shell only: no GoReleaser, no new actions without owner approval.

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
  [--recommend LABEL | --recommend-text TEXT] [--reasoning TEXT] [--key K]
  [--tag T]... [--ref REF]... [--context TEXT | --stdin] [--by NAME]
  [--root DIR]`: creates an `open` record with one `ask` history entry and
  prints its ID. Flags may come before or after TITLE. `--recommend` must name
  one of the options; `--recommend-text` is a recommendation that is not an
  option (C167), stored as typed in the recommendation's `text` with no
  `label`, and needs no option. Both together, a blank `--recommend-text` or
  `--reasoning` with neither exits 2; `--reasoning` goes with either. A blank
  `--key`, `--tag` or `--ref` exits 2; `--key` is stored as typed. Refs are
  opaque strings, kept as typed and in order. `asked_by` is `--by`, else
  `$ADJ_ASKED_BY`, else blank. The ID is taken under the store's lock (see
  "Store and config"): an `ask` that cannot get the lock within 5 seconds exits
  1 with an error that names `.adj.lock` and creates no record.
- `adj decide ID OPTION --why TEXT`, or `adj decide ID --text TEXT --why TEXT`,
  `[--by NAME] [--decided-at YYYY-MM-DD] [--root DIR]`: moves an `open` record
  to `decided`. OPTION (trimmed) must be one of the record's option labels;
  `decision` is that label, or the `--text` answer. `rationale` is the `--why`
  text, and `decided_at` and one `decide` history entry are dated now.
  `--decided-at DATE` (C168) dates `decided_at` DATE instead, to record a
  decision made earlier; the history entry keeps the run date, and its note is
  `backdated to DATE` when DATE is not the run date (C176). DATE may be before
  `created` (C175). A DATE that is not a valid `YYYY-MM-DD` date (a blank value
  too) or is after the run date exits 2, before any store access. Only `decide`
  takes it. `adj reject ID --why TEXT` and `adj withdraw ID --why TEXT` move an
  `open` record to `rejected` or `withdrawn` the same way, with no decision.
  `decided_by` is `--by`, else `$ADJ_DECIDED_BY`, else blank (C27). `--why` and
  `--text` are kept as typed but must not be blank. All three print the
  record's ID. A bad argument exits 2, including an OPTION the record does not
  offer while the record is open. An unknown ID, or a record that is not `open`
  (checked before OPTION), exits 1 and changes nothing; its message names the
  status.
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
- `adj amend ID [--title T] [--context T | --stdin] [--summary LABEL:TEXT]...
  [--tradeoffs LABEL:TEXT]... [--rationale T] [--root DIR]` (C166, C178-C180):
  corrects the text of a record of any status; it never changes status,
  decision, `decided_by`, `asked_by`, dates, ID, links, key, tags, refs, an
  option's label or the recommendation. One call may change several fields and
  appends one `amend` history entry whose `was` object keeps the old text of
  each field that changed (`docs/json.md`, C179); `adj show` prints it. A text
  equal to the current one is not a change. Prints the ID. Exit 2: no field
  flag, a blank `--title`, `--context` or `--rationale`, a title on several
  lines, a bad or repeated LABEL:TEXT, or a label that no option has (an empty
  TEXT clears a summary or tradeoffs; a context cannot be emptied). Exit 1: an
  unknown or duplicate ID, `--rationale` on an `open` record, or nothing to
  change. Validated in memory first, so a refusal writes nothing; the file is
  not renamed when the title changes. Who may amend (owner decision, not
  enforced in code, like C27): an agent may amend the title, context and
  option summaries and tradeoffs of an `open` record; only the owner amends a
  `decided` record, its rationale included. The agent snippet says so.
- `adj show ID [--json] [--root DIR]`: prints one record, each set field and
  the history; with `--json` it prints the record object (`docs/json.md`). A
  non-number or 0 ID, or an extra argument, exits 2; an unknown or duplicate
  ID exits 1.
- `adj list [--status S] [--tag T]... [--ref REF]... [--key K] [--json]
  [--root DIR]`: lists records in ID order, one line each (ID, status, title);
  with `--json` it prints `{"records": [...]}` (`docs/json.md`). Every filter
  given must match.
  `--tag` repeats, and every tag given must be on the record. `--status` is one
  of the five statuses; its value and each `--tag` value are trimmed, while
  `--key` is matched as typed against the stored key. A bad status, a blank
  `--tag`, `--ref` or `--key`, or an argument, exits 2. No match is not
  an error (exit 0). `--ref` repeats: every ref must be on the record, matched
  exactly, case-sensitively and as typed, including surrounding whitespace; a
  blank value exits 2 before store access.
- `adj inbox [--json] [--root DIR]`,
  `adj inbox --all [--json]`, or
  `adj inbox -i [--by NAME] [--root DIR]`: lists the `open` records,
  oldest first (by `created`, then ID). Text: per record `ID  title`, then
  indented its created date, its refs when set (C171), its context, every
  option (`label: summary`,
  with `tradeoffs:` under it when set) and the recommendation (the option
  label, or the free text) with its reasoning; entries are separated by a blank
  line.
  `--json` prints `{"records": [...]}`, the same record objects as `adj list`
  (`docs/json.md`). An empty inbox prints nothing (JSON: an empty array) and
  exits 0. An argument or other flag exits 2.
  With `--all`, read the user repo list managed by `adj repos` and show each
  repo's open questions under its absolute path, in config insertion order.
  Records remain oldest first within each repo. Successful empty repos keep
  their header; an empty or missing user config prints nothing. A missing repo,
  missing `.adjudicate.json`, or unreadable store produces a warning on stderr
  naming that repo; the others are still printed, and the command exits 1.
  `--all --json` prints
  `{"repos": [{"root": "...", "records": [...]}], "errors": []}`
  (see `docs/json.md`); per-repo errors appear in JSON as well as stderr. The
  arrays are always arrays, including when empty. `--all` with `--root` exits 2;
  no current repo is needed. Without `--all`, output is unchanged.
  With `-i`, the owner walks through this repo's open questions in the same
  oldest-first order. Each question shows the usual inbox block, then prompts:

  ```text
  Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]:
  ```

  `d` asks for an option label; a blank label returns to the action prompt.
  If the record has no options, `d` reports an error pointing to `t` and returns
  to the action prompt without asking for a label. `t` asks for free-text decision
  text. `r` rejects,
  `w` withdraws, `s` skips and `q` quits; `?` prints help. Each write asks for a
  non-blank rationale. Unknown labels and blank text or rationale re-prompt the
  same field. Decision text and rationale keep their surrounding whitespace;
  option labels are trimmed, as with `adj decide`. `--by NAME` sets the owner,
  falling back to `$ADJ_DECIDED_BY` when empty or absent. Writes use the same
  validation, dates and history as `decide`, `reject` and `withdraw`.

  Quit or EOF stops without writing an incomplete answer, including a last line
  without a newline. A nonempty walk ends with `N decided, N rejected, N withdrawn,
  N skipped, N failed`, counting only completed actions; unvisited questions stay
  open. A failed write reports its error on stderr and advances to the next
  question; any failed write makes the final exit code 1. An input read error also
  exits 1. An empty inbox prints nothing and exits 0. `-i` with `--json`, `--all`
  or an argument exits 2, as does `--by` without `-i`. This mode is owner-only;
  agents must not run it. It uses ordinary stdin lines, without terminal raw mode.

- `adj check QUERY [--json] [--root DIR]`: asks whether QUERY has been decided,
  so an agent checks before it asks the owner. QUERY (trimmed, one argument:
  quote it if it has spaces) is one substring, not split into words; it matches
  a record when its title, key, one of its tags or refs, or its decision text
  contains it, ignoring case (C105, C171). A match is listed by status:
  `decided`, `open`, or `closed` for a rejected, withdrawn or superseded record
  (C106). A superseded match also leads to the end of its `superseded_by` chain;
  if that is a decided record it is listed as `decided` too (C113). The first
  line of the text output is the result: `decided` if any decided record is
  listed, else `open`, else `closed`, else `not found` (C107). Then one line per
  record, `ID  status  title`: decided, open, then closed ones. A matching
  record with refs shows `Refs:` underneath (C171); decided records also show
  `Decision:` and `Rationale:`, and closed records show `Rationale:` (why they
  were rejected, withdrawn or superseded; the whole text, C132, C138), after
  `Superseded by:` on a superseded one. An open record has no decision or
  rationale detail lines.
  `--json` prints `{"query", "result", "decided", "open", "closed"}`
  (`docs/json.md`). Exit 0 for every completed check, whatever the result
  (including `closed` and `not found`), so a script reads the `result`; exit 1
  means the check itself failed (no repo, an unreadable record file), which is
  never a "not found"; exit 2 is a usage error: no QUERY, more than one, or a
  blank one (C108, C109).
- `adj context [--tag T]... [--ref REF]... [--key-prefix P] [--format md|json]
  [--max-bytes N] [--root DIR]`:
  prints the `decided` records as compact context for an agent; open, rejected,
  withdrawn and superseded records are left out (a superseded decision gives
  way to its replacement, which is `decided`). Every filter given must match.
  `--tag` repeats and every tag must be on the record (trimmed); `--key-prefix`
  is a plain string prefix of the stored key, as typed, so a record with no key
  never matches it. `--ref` repeats: every ref must be on the record, matched
  exactly, case-sensitively and as typed, including surrounding whitespace; a
  blank value exits 2 before store access. Refs combine with all other filters
  before the size budget is applied. Records are in ID order, so the same
  store gives the same bytes. `--format md` (the default) prints per record
  `- ID title [key]: decision` (no brackets without a key; a multi-line
  decision continues on indented lines), then `  Rationale: ` and the first
  non-blank line of the rationale. `--format json` prints
  `{"decisions": [...]}` with `id`, `title`, `key` (when set), `decision`
  and `rationale` (`docs/json.md`). No decision
  prints nothing in md and `{"decisions": []}` in JSON, exit 0. A blank `--tag`,
  `--key-prefix` or `--ref`, a `--format` other than `md` or `json`, or an
  argument, exits 2. `--max-bytes N` (S35; a whole number of at least 1, else
  exit 2 before the store is looked for; given twice the last counts) bounds the
  output to N bytes, the omitted line included: after the filters, it keeps
  the newest decisions (`decided_at` latest first, then the higher ID) that
  fit, the k newest for the largest k that fits, and prints them in ID order.
  `commands.FitContext` chooses k from a `ContextMeasure` that the CLI builds
  from the renderer, so the budget counts the real output. md then ends with a
  blank line and `N decisions omitted` (`1 decision omitted` for one; the
  blank line is left out when no record is kept); `--format json` gains
  `"omitted"`, the count, `0` included, and only with `--max-bytes`, so the
  output without the flag is byte for byte what it was. If no record fits,
  only the omitted line is printed (JSON: `"decisions": []`) and the exit
  code is 0; if even that is more than N bytes (or the JSON with no record
  is), nothing is printed and the exit code is 1, the error naming the bytes
  needed. No matching record prints nothing in md, which fits any N. The
  snippet of `adj ai` does not name the flag (it is at its size limit).
- `adj lint [--json] [--root DIR]`: checks the store for integrity problems
  (C08). It reads the record files itself (`store.Files`, `model.Parse`), so a
  broken or invalid file is a finding where `list` and `check` would fail (C61).
  Each finding names the record (the ID in its file name, C59), its file
  (relative to the root) and a check: `parse` (cannot be read or decoded),
  `fields` (one per `model.Validate` rule broken, and a link to the file's own
  ID), `id-mismatch` (the record's `id` is not the ID in its file name),
  `duplicate-id` (more than one file has the ID; each of them gets the finding,
  C26, C152), `renumber-leftover` (the stale old file of an interrupted `adj
  renumber`, C156, C163), `dangling-link` (`supersedes` or `superseded_by` names
  an ID no file has; a link listed twice is reported once), `supersede-state`
  (`superseded_by` on a record that is not superseded), `supersede-cycle` (a
  loop of supersede links; one finding per record in it, naming only records
  that have a file), `key-conflict` (another `decided` record has the same key,
  trimmed and case-sensitive), `decided-by` (a `decided` record without
  `decided_by`, a warning, C27), `unknown-event` (a history entry whose event
  this version does not know, one finding per entry: `history entry N: unknown
  event "X"`, S44) and `unknown-history-key` (one per entry, naming its unknown
  keys in sorted order, S44); the last three are warnings, the rest errors
  (C143). A blank event is a `fields` error. Text: per finding
  `FILE: record ID: SEVERITY: CHECK: MESSAGE`, sorted by ID, check and file,
  then `N errors, M warnings`; a clean store prints nothing (C141). `--json`
  prints `{"errors", "warnings", "findings"}` (`docs/json.md`, C142). Exit 1 if
  there is an error or lint could not run, else 0: warnings alone exit 0 (C140).
  An argument exits 2. The agent snippet does not name it (C145).
- `adj renumber [--keep-links] [--root DIR]` (C150): resolves record files that
  share an ID (`adj lint` reports them as `duplicate-id`; merged branches and
  clones make them, and so do asks on a system without the ID lock, C26, C114).
  Of the files with one ID the record `created` first keeps it, then the file
  name breaks a tie (C146). Each other record is written to a new file `<next
  free ID>-<same name text>.md` (next free ID: one past the highest in the
  store, colliding IDs taken in ascending order), with its permission bits and
  one `renumber` history entry (note `from N`), and the old file is removed
  after the new one is complete (C148). Prints one line per move, `OLD-FILE ->
  NEW-FILE (id N -> M)`; no collision prints nothing and changes nothing, exit
  0. Links to a colliding ID cannot be told apart, so they and the history notes
  are never rewritten (C147, C149): if any record links to one, renumber lists
  the links, exits 1 and changes nothing, until `--keep-links` accepts that they
  stay with the record that keeps the ID (they are listed on stderr). Every
  record file must be readable (it may hold a link); a colliding record needs a
  valid `created` date (C158); a record that moves must be valid with the ID it
  will get (C151); these are checked before the first write. A new ID is never
  one that a link names without a file (C155). With a collision it holds the
  store's ID lock from reading the files to the last move (see "Store and
  config"), so an `adj ask` cannot take an ID a move is about to use; a refusal
  after that leaves only the empty lock file. With no collision it takes no
  lock. The directory is synced before the old file goes, on a best-effort
  basis: a failed sync is ignored (C157). A move that cannot remove the old file
  takes the new one away again; if that fails too the error names both files.
  After a crash between the two, the record is in both files and the old one is
  a stale copy: `adj lint` reports it as `renumber-leftover` (the new file has a
  `renumber` history entry `from N` for the old file's ID, wherever it sits in
  the history, and holds the same record: equal to the old file apart from the
  entry, or, if edited since, with the old file's whole history before the entry
  and the same title, key, tags, asked_by, created, context, options and
  recommendation, not the body, C163, once the `amend` entries after the
  renumber entry are taken back to the old text they keep, C181; a live record
  that only shares the ID and name text with a moved one is not a stale copy),
  and `adj renumber` refuses (exit 1, nothing changed, `store.ErrStaleCopy`)
  until the owner checks both files and deletes the old one (C156); nothing is
  deleted automatically. Known limit (C163): a moved file that was changed by
  hand without a history entry is not recognised, so before running
  `adj renumber` on files that `adj lint` reports as `duplicate-id`, check
  whether one holds a `renumber` entry; two files with identical records
  (contrived) flag the keeper. An argument or unknown flag exits 2.
  Owner-only: the agent snippet tells agents to report a duplicate ID and not
  to run it (C153). A name taken by a directory or another writer is skipped
  for the next ID (C154).
- `adj import-adr DIR [--tag T]... [--by NAME] [--dry-run] [--root DIR]` (S22):
  creates one record for each ADR file in DIR (`internal/adr` parses them;
  `internal/commands/importadr.go` maps them to records). DIR must be inside the
  repo root. Only files directly in DIR whose names start with a number (an
  optional `adr-` before it) are read, in ADR number order, then file name. Two
  layouts: Nygard (adr-tools: `# N. Title`, `Date:`, `## Status`, `## Context`,
  `## Decision`, `## Consequences`) and MADR (optional front matter, read by hand
  for flat `key: value` lines, or `Status:`/`Date:` lines; `## Context and
  Problem Statement`, `## Considered Options`, `## Decision Outcome`). The first
  word of the status, in any case, is `accepted` (decided), `proposed` (open,
  its decision becomes the recommendation), `rejected`, `deprecated`
  (withdrawn) or `superseded`; any other word, a missing title or status, an
  accepted ADR with no decision, or an answered ADR with no rationale makes the
  file bad. Every file is read and every record validated before the first
  write, so a bad file means exit 1 and nothing written, with each problem
  named. `decision` is the Decision section, or the MADR `Chosen option` label;
  `rationale` is the first non-blank of the MADR justification (after
  `because`), the Consequences, the Context (for rejected and withdrawn, the
  decision text comes before the Consequences); `consequences` keeps the
  Consequences section when the rationale came from elsewhere. `created` and
  `decided_at` are the ADR's date, or the run date with a warning when it has
  none or it is not `YYYY-MM-DD`. `decided_by` is `--by`, else
  the ADR's deciders, else `$ADJ_DECIDED_BY`, else blank (lint warns, C27;
  `ImportADRInput.DecidedByDefault` carries the environment);
  `asked_by` is blank. `refs` are the file's slash path from the root and
  `ADR-NNNN` (when the number is above 0); history is one `import` entry
  (`from FILE`, dated the run date) instead of `ask`/`decide`. A file is
  already imported when some record has an `import` history entry `from FILE`
  (a record asked by hand with the file as a ref does not count); it is skipped
  (`FILE: already imported as ID`), so a second run adds only new files. A
  skipped file is read again only for its `Superseded by` line (a file that no
  longer parses is ignored): if its record is still `decided` and the file now
  says it is superseded by an ADR (same resolution) whose record is `decided`,
  a warning gives `adj supersede OLD --by NEW`; nothing is written for it. A
  file that is a symbolic link is not read and gets the warning `FILE: symbolic
  link skipped`. A `superseded` ADR becomes `superseded` only when the
  ADR it names (by file name, else by a number that exactly one ADR has) is in
  the same run and is accepted or superseded and no loop of links results;
  otherwise it is imported as `decided` with a warning. Records are created
  decided first and the links are written afterwards, the old record first.
  Unlike `adj supersede` (C136) a failure writing the new record does not put
  the old one back: it stays superseded and the decision in force is still the
  new record. Any failure after the first create prints the records made so far
  and lists the links not yet written as `adj supersede OLD --by NEW` commands
  in the error. `created` is also the run date, with a warning, when the ADR
  date is after the run date or is well-shaped but not a real date. Prints
  `FILE -> ID (STATUS)` per file; warnings go to stderr; `--dry-run` writes
  nothing. A folder with no ADR file
  prints nothing, exit 0. Usage errors exit 2 (no DIR, more than one, blank DIR
  or `--tag`); a folder outside the root, a bad file or a failed write exit 1.
  Owner-only: the agent snippet does not name it. The decisions behind these
  details are the S22 notes (C32).
- `adj repos add [PATH]`, `adj repos remove PATH`, `adj repos list [--json]`
  (S38): manage `$XDG_CONFIG_HOME/adjudicate/config.json`, falling back to
  `$HOME/.config/adjudicate/config.json` when XDG_CONFIG_HOME is unset,
  empty or relative; HOME must be absolute when needed. Unset, empty or
  relative HOME is an error
  (exit 1). Shape: `{"repos": [...]}`. A missing
  file is empty; bad JSON, unknown keys or non-absolute paths exit 1 naming the
  file, without writing. Add defaults to the repo root (`--root`, else
  discovery from the working directory); explicit paths resolve against the
  working directory. Paths are cleaned absolute strings, compared without
  resolving symlinks. Add needs
  a regular `.adjudicate.json`; a listed path is a byte-preserving no-op.
  Remove needs a PATH; unlisted exits 1 without writing, and stale paths can be
  removed. Add/remove print nothing. List prints one stored path per line in
  insertion order, without probing repos; JSON is `{"repos": []}` when empty.
  `--root` is otherwise accepted but unused. Unknown subcommands, extra args
  and blank PATH exit 2. Writes create the directory (0755) and atomically
  replace the file (0644) using a same-directory temp and rename.
  Removing from a missing config fails as unlisted without creating a directory
  or lock file.
  Add/remove hold `config.json.lock` beside the user config for the complete
  read-modify-write operation (flock on Unix, LockFileEx on Windows). They wait
  up to 5 seconds, then exit 1 naming the lock file without changing the config.
  The empty lock file remains after release; list ignores it. Other platforms
  without either locking primitive retain the store lock's no-lock fallback.
- `adj version` and `adj --version`: print one line from Go build information,
  or the release override set with `-ldflags -X`. `version` reads no repo and
  needs no config; it accepts `--root` but does not use it. The leading
  `--version` form invokes the same command.
- `adj ai`: prints the AGENTS.md snippet in `internal/commands/ai.md` (check
  before asking and what each result means, ask with `--ref` and
  `--recommend-text`, read the decisions, withdraw your own question, amend an
  open question, only the owner decides or amends a decided record and never
  run decide, reject or supersede, tell the owner about a duplicate ID or a
  missing repo instead of running renumber or init, exit codes, `ADJ_ASKED_BY`)
  and exits 0. It does not name `--decided-at`: agents never run decide, so the
  flag is the owner's. It reads no repo and takes no flag or argument (so no
  `--root`); either exits 2.
  `internal/cli/ai_test.go` fails if the snippet names a command, flag or
  variable that does not exist, or runs past 1800 bytes (C124, C125). When a
  command or flag an agent uses changes, change the snippet.
- Every command except `ai` and `inbox --all` takes `--root DIR`. Store commands
  use it as the
  repo root; without it the root is found from the working directory upwards
  (`store.FindRoot`). With it nothing is searched for. Every store command
  other than `init` needs `DIR/.adjudicate.json`; `version` does not use the
  store or need a config. A missing config says to run `adj init`.
- `--root` may also come before the command: `adj --root DIR list` acts as
  `adj list --root DIR` (`cli.leadingRoot`; `invocation.parse` hands it to the
  command). Before a command, `--root DIR` is taken and `--help`/`-h` print the
  main usage (the unknown-flag message names those three, and so does the usage
  text, which also tells `adj --version` apart); `--version` as the first
  argument invokes the version command (`adj --root DIR --version` exits 2),
  and any other leading flag exits 2 with usage. `--root` both before and after
  the command, or twice before it, exits 2 naming both values (C170); twice
  after the command still takes the last. A leading `--root` with no value or
  no command exits 2, and before `ai` exits 2 (`ai` takes no `--root`) unless
  `ai` is given `--help`, which is handled first and exits 0, as `adj ai --help
  --root D` does. A leading `--root` followed by `--help`/`-h` prints the main
  usage (exit 0).

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
  permission bits; new files are 0644. Two files with one ID make `Get` and
  `Update` fail (`ErrDuplicateID`, naming both files); `Store.Renumber` fixes
  them (see `adj renumber`).
- ID allocation is locked (C114, C159-C162). `Create` and `Renumber` take an
  exclusive `flock` (advisory) on `<store>/.adj.lock` (`decisions/.adj.lock` by
  default) and hold it from the scan of the IDs to the link of the new file, so
  two `adj ask` runs on one checkout cannot take one ID. `Update` is not locked.
  The lock file is an empty dot-file, never taken for a record, made on first
  use and never deleted; the lock is released when the call returns or the
  process dies. If the lock file exists and cannot be opened for writing
  (EACCES on unix, ERROR_ACCESS_DENIED on Windows; read-only mode or attribute,
  or another user's checkout), it is locked through a read-only open, since
  neither `flock` nor `LockFileEx` needs write access (`openLockFile` in
  `lock_open.go`, shared by both); if it is missing and cannot be made, the
  error is the permission error (C164). A writer waits up to
  5 seconds (polling every 10 ms); if the lock is still held it fails with an
  error that names the lock file and writes no record. If `flock` itself fails
  (ENOLCK or EOPNOTSUPP, as on some network file systems), `adj ask` exits 1
  with an error that names `.adj.lock` and writes no record. On a network file
  system the lock may be local to each host, so asks from two hosts can still
  take one ID. `Create` also skips an ID that a `supersedes` or `superseded_by`
  link names without a file (C155); a file it cannot read keeps its ID by name
  but its links are not known. The lock only serialises one checkout: branches
  and clones still collide, and `adj lint` and `adj renumber` handle that.
  Locking needs `syscall.Flock` (darwin, linux, the BSDs, illumos) or, on
  Windows, `LockFileEx` (`lock_windows.go`, S36): `kernel32.dll` called through
  the standard library's `syscall` (no `golang.org/x/sys`), exclusive over the
  whole file, with the same behaviour (advisory on unix, mandatory on Windows,
  which changes nothing for the empty lock file; held from the ID scan to the
  link, polled every 10 ms with `LOCKFILE_FAIL_IMMEDIATELY` for up to 5
  seconds, released with the handle or the process, the file never deleted, the
  same error that names `.adj.lock`). Other systems have no lock and only the
  bounded retry of C64. Tests that depend on unix permission bits or errno
  values are not built on Windows (`lock_unix_test.go`) or skip there with a
  reason (`runtime.GOOS`); the rest of the suite runs on Windows in CI. A team
  that does not want the lock file in git adds `decisions/.adj.lock` (its
  store directory) to its `.gitignore`; `adj` writes no `.gitignore`. A test
  that lists the store
  directory leaves the lock file out.

## Setup and checks

Run from the repo root:

```sh
. scripts/env.sh                   # Go caches, temp files: .cache/ (C33)
go test ./...
go vet ./...
gofmt -l cmd internal              # must print nothing
go build -o .cache/bin/adj ./cmd/adj
```

CI runs the same checks without `scripts/env.sh`, and `adj lint` on this
repo's own store.

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
- User config tests inject `Getenv` with XDG_CONFIG_HOME/HOME pointing into
  worktree temp dirs; never touch the real home or user config.
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
- Dogfood store conventions (C194): when recording a decision in this repo's
  own `decisions/` store, put slicer catalog and slice IDs in `--ref` as
  written (`--ref C23`, `--ref S34`), not in tags. Tags are topics, not catalog
  IDs. Keys are dotted `area.topic` (for example `project.license`). Records
  0001-0016 stay as they are: no retagging, no keys added, and `adj amend`
  does not change tags, keys or refs.
- Releases (C193): the repo is `github.com/squalor-xyz/adjudicate`. A release
  is a `## X.Y.Z - DATE` CHANGELOG section, then an annotated tag `vX.Y.Z`
  pushed to `origin`; `release.yml` fails if the section is missing. Only the
  owner pushes or tags, or asks for it.

# Changelog

All notable changes to adjudicate (CLI `adj`) are listed here, newest first.
Until 1.0 a JSON shape in [docs/json.md](docs/json.md) may change in a minor
release, and this file names every such change. From 1.0 a release may add keys
but never renames, removes or retypes an existing key.

## Unreleased

### Commands
- New: `adj contract` (S45) prints the version of the JSON contract this adj
  speaks, `adj-json v1`: the revision of the shapes in
  [docs/json.md](docs/json.md), adj's own number, starting at 1. `--json`
  prints `{"contract", "version", "pinned"}`; it needs no repo (`pinned` is
  `null` without one). `adj contract pin M` writes `"contract": M` into
  `.adjudicate.json` (whole numbers of at least 1; the `store` key stays; a root
  named with `--root` that has no config gets one, and the store directory, as
  `adj init` would) and `adj contract unpin` removes it; an unchanged file is not
  rewritten, and a write goes through a temporary file and a rename. adj never
  serves an older shape: when a repo pins a version other than the one this adj
  speaks, `show`, `list`, `inbox`, `check` and `context --format json` run with
  JSON output exit 1 and print the error object with the new code
  `contract_mismatch` (its message names both versions and tells the reader to
  ask the owner to run `adj contract pin` or to install a matching adj), and
  `inbox --all --json` reports that repo under
  `errors`; the same commands without JSON and the commands with no JSON are
  not refused. `adj lint` warns about such a pin (`contract-pin`, a warning, with
  `id` 0 and file `.adjudicate.json`; its text line has no `record ID:` part)
  and goes on checking the store. `adj version --json` prints `{"version",
  "contract"}`; plain `adj version` is unchanged. Known effect:
  `.adjudicate.json` is strict, so an `adj` released before this change refuses
  a pinned file as an invalid config, with that release's old message, which
  does not mention the pin. Repos that pin nothing are unaffected.
- New: `adj ai skill` (S54) prints the agent skill, `SKILL.md`: when to check
  before asking, the four `adj check` results, how to word a question with
  options, trade-offs and a recommendation, how to narrow a read (`--key-prefix`,
  `--ref`, `--max-bytes`, `--json`), and the commands only the owner runs. The
  text is part of the binary, so it matches the version. `--output PATH` writes
  `SKILL.md` to PATH, creating parent directories. `--install` writes
  `SKILL.md` and `reference.md` (command, exit-code and JSON detail) into
  `.claude/skills/adj/`, `.agents/skills/adj/` and `.grok/skills/adj/` under the
  repo root, and takes `--root DIR`; it is project-only, so use `--output` for
  any other place. Both list the paths they wrote. A file that already holds
  the text is left alone. A file that holds different text makes the command
  exit 1 with nothing written, unless `--force` replaces it; a destination file that
  is itself a directory or a symbolic link always exits 1 (a symbolic link in a
  parent folder, such as `.claude`, is followed). If a
  write fails after others succeeded, they are put back. Printing and
  `--output` read no repo and take no `--root`; `--root` without `--install`,
  `--install` with `--output`, and `--force` alone exit 2. `adj ai` and its
  1800-byte snippet are unchanged. A test fails if the skill or its reference
  names a command, flag or variable that does not exist.
- `adj inbox` takes a repeatable `--ref REF` (S53), also with `--all` and
  `-i`: it keeps the open questions that have every ref given, matched exactly,
  case-sensitively and as typed, as `adj list --ref` does, so `--ref S11` does
  not find `S110`. A blank `--ref` exits 2 before the store is looked for. With
  `--all` the filter applies in each repo and a repo with no match keeps its
  header; with `-i` the walk and its summary cover only the matching
  questions. The JSON shape does not change, and without `--ref` the output is
  byte for byte what it was. `adj check` stays a substring search.
- `adj decide`, `adj reject` and `adj withdraw` take two optional flags,
  `--recorded-by NAME` and `--via TEXT`: who entered the answer when that is
  not who decided (`--by`), and how the owner gave it, for example `--via
  "interactive question, session ID"`. Both are free text, kept as typed. A
  flag that is given but blank exits 2 before the store is looked at. No
  environment variable sets them. Without them a record, `adj show` and the
  JSON are byte for byte what they were. `adj amend`, `adj supersede`,
  `adj renumber` and `adj inbox -i` leave them unchanged. The `adj ai` snippet
  does not name them.

### Store format
- A record may have two more optional keys, `recorded_by` and `via`, written
  right after `decided_by` and left out when empty. A record file that has a
  blank one is invalid (`adj lint` reports it as a `fields` error).
- A history entry whose `event` this version does not know, and unknown keys
  inside a history entry, are now read and kept instead of making `show`,
  `list`, `check`, `context`, `inbox` and the commands that change a record
  fail. Rewriting a record (decide, reject, withdraw, amend, supersede,
  renumber) writes those entries back unchanged: the unknown keys follow the
  known ones, in sorted order, so the bytes are stable. A later event kind, as
  `import` was in 0.2.0, no longer makes an older `adj` refuse a store. The
  `was` of an unknown event is kept as written,
  whatever it holds. A key that differs from a known one only in letter case
  (`Note` beside `note`) is kept as an unknown key. A blank `event` is still
  invalid.
- Keys at the top level of a record that this version does not know, and keys
  it does not know inside the `was` of an `amend` entry (at its top and inside
  each `options` entry), are now read and kept too (S46), so a field a later
  1.x release adds, or an `amend` of such a field, does not make a 1.0 binary
  refuse the store. Every command that rewrites a record writes them back
  unchanged, after the known keys in sorted order, so the bytes are stable. Known
  keys keep their types, and an unknown key inside an option or the
  recommendation is still an error, as is any key in `.adjudicate.json`. A `was`
  or an option in it with only unknown keys still counts as old text. `adj
  renumber` also requires the unknown top-level keys to match when it decides
  whether a file is the stale copy of a moved record. `kind`, once refused as a
  retired key, and `body` are now just unknown keys in the file; a `body` key
  in the frontmatter is never printed in the JSON of the read commands, where
  `body` stays the text after the frontmatter.
- `adj lint` adds four warning checks, `unknown-event` (`history entry N:
  unknown event "X"`), `unknown-history-key` (one finding per entry, naming
  its unknown keys), `unknown-was-key` (one finding per `amend` entry, naming
  the unknown keys of its `was`) and `unknown-record-key` (one finding per
  record). Warnings alone exit 0. Before, an unknown event was a `fields` error
  and an unknown key a `parse` error.

### JSON shapes
- New: the contract version (S45). `docs/json.md` has a "Contract version"
  section with one line per version; version 1 is the shapes of this release,
  unchanged. The number moves on any change to a shape that is not purely
  additive, and a test compares every key path and type with a recorded list so
  that such a change cannot ship without it. New: `adj contract --json` and
  `adj version --json`, new error code `contract_mismatch`, and a lint finding
  with `id` 0 for the config file. Additive: no existing key changes.
- A record object may have two more optional string keys, `recorded_by` and
  `via`, beside `decided_by`, in `show`, `list`, `inbox`, `check` and every
  other command that prints records. They are present only when set, so no
  existing key changes. See [docs/json.md](docs/json.md).
- New: a command run with `--json` (or `adj context --format json`) that fails
  prints `{"error": {"code", "message", "command"}}` on stdout, as slicer does.
  The codes are `usage`, `no_repo`, `invalid_config`, `not_found`,
  `duplicate_id`, `invalid_record`, `user_config`, `budget` and `io`;
  `command` is the command name, or `null` for a usage error before a command
  was known. Exit codes (0, 1, 2) and the text on stderr are unchanged. JSON
  mode is read from the raw arguments, so `adj show --bogus --json` prints the
  object. A `lint` report with errors, a `check` result of `not found` and the
  per-repo errors of `inbox --all` are results and print no object. Behaviour
  change for scripts: stdout was empty when a `--json` command failed and now
  holds the object, so test the exit code, not whether stdout is empty. A
  `--json=VALUE` that is not a bool (`--json=maybe`, `--json=`) is JSON mode
  too, and prints the object with code `usage`; only a value Go reads as false
  (`--json=false`, `--json=0`) turns it off. See "Errors" in
  [docs/json.md](docs/json.md).
- `show --json`, `list --json` and the other commands that print records
  include an entry's unknown keys beside `date`, `event`, `note` and `was`, the
  unknown keys of a record's `was` and its options, and the record's own unknown
  top-level keys (S46), after the known ones and before `body` (a frontmatter
  key named `body` is left out). `adj show` text
  and `adj context` print the known fields only. Readers must ignore history
  events and keys they do not know. No existing key is changed. See
  [docs/json.md](docs/json.md).

## 0.2.0 - 2026-10-18

### Platforms
- Windows: `adj ask` and `adj renumber` now take the ID lock on Windows too
  (`LockFileEx` on `<store>/.adj.lock`, with the semantics of the unix `flock`:
  held from the ID scan to the link, a 5 second wait, the same error). Until
  now Windows had no lock, only the bounded retry for a name that is taken.
  A lock file that exists with the read-only attribute is locked through a
  read-only open, as on unix.
- Windows: a `--root` that names a file is refused, as on unix; before, the
  config below it read as missing and the defaults were used.
- Releases add `adj_VERSION_windows_amd64.zip` and
  `adj_VERSION_windows_arm64.zip`, each holding `adj.exe`, `LICENSE`, `NOTICE`,
  `README.md` and `CHANGELOG.md`, listed in `SHA256SUMS`.
- CI runs `go vet` and `go test` on Windows.
- `.gitattributes` checks text out with LF on every OS; CRLF fixtures and the
  PNG remain byte-preserved.

### Commands
- Owner-only `adj inbox -i` walks open questions oldest first with line prompts
  to decide by option or free text, reject, withdraw, skip or quit. Writes require
  a rationale and use one-shot command validation and history; `--by` falls back
  to `$ADJ_DECIDED_BY`. EOF discards an incomplete answer. A summary counts
  outcomes; failed writes report stderr, continue and make the final exit 1.
  `-i` with `--all`, `--json` or an argument, and `--by` without `-i`, exit 2.
- `adj inbox --all` groups the user repo list's open records by root, in config
  order, including empty repos. Unreadable repos warn on stderr while other
  results remain available; partial failure exits 1. `--all --json` adds a new
  shape with `repos` and `errors` arrays. `--all` with `--root` exits 2.
- Relative XDG_CONFIG_HOME is ignored in favor of HOME/.config. HOME must be
  absolute when used. Repo add/remove now lock config.json.lock for the whole
  update, with the store's 5 second wait and Unix/Windows locking behavior.
  Removing from a missing config creates no directory or lock file.
- `adj repos add [PATH]`, `remove PATH` and `list [--json]` manage a user repo
  list in `$XDG_CONFIG_HOME/adjudicate/config.json` (fallback:
  `$HOME/.config/adjudicate/config.json`). Add defaults to the current root,
  stores cleaned absolute paths and treats duplicates as a no-op. Remove exits
  1 for an unlisted path; list prints stored paths without probing them.
  Config writes use a same-directory temporary file and atomic rename.
- `adj list` and `adj context` accept repeatable `--ref REF` filters: every ref
  must match exactly, case-sensitively and as typed. Blank values exit 2 before
  store access. Filters combine with the other criteria, before the context
  byte budget. Use `adj list --ref S20 --json` to find slicer slice records;
  neither tool depends on the other. JSON shapes are unchanged.
- `adj context --max-bytes N` keeps the newest decided records whose output fits
  in N bytes (newest by `decided_at`, then ID), prints them in ID order, and
  ends the Markdown with `N decisions omitted`. The budget counts every byte,
  that line included; if not even that line fits, nothing is printed and the
  exit code is 1. Without the flag the output is unchanged.
- `adj import-adr DIR [--tag T]... [--by NAME] [--dry-run]` creates one record
  for each ADR in a folder, in the format adr-tools writes (Michael Nygard's
  template) or MADR. The status word gives the record's status (accepted is
  decided, proposed is open, rejected, deprecated is withdrawn, superseded is
  superseded when the ADR that replaced it is in the same run, else decided with
  a warning); `decided_by` is `--by`, else the ADR's deciders field, else
  `$ADJ_DECIDED_BY`, else blank; the ADR's date is `created` and `decided_at`;
  the refs name the file (its path from the repo root) and the ADR number.
  Every file is checked before any record is written. A file already imported
  (a record with an `import` history entry for it) is skipped, and the ADR
  files are never changed. A second run warns, with the `adj supersede`
  command, about a skipped file whose record is still decided although the
  file now says it is superseded. See the README.

### JSON shapes
- `adj inbox --all --json` adds `{"repos": [{"root": "...", "records": [...]}],
  "errors": [{"root": "...", "error": "..."}]}`; both top-level arrays are
  present even when empty. Plain `adj inbox --json` is unchanged.
- A history entry may have the event `import`: `adj import-adr` writes one on
  each record it makes, with the note `from FILE`. Programs that switch on
  `event` should accept it. No key is added or changed. Verified against the
  real v0.1.0 tag: `show`, `list`, `check` and `context` exit 1 on an imported
  record with `invalid record: history: entry 0: unknown event "import"`.
  `lint` exits 1 with a `fields` error,
  `history: entry 0: unknown event "import"`.
  Upgrade before sharing a store containing imported records.
- `adj repos list --json` prints `{"repos": [...]}` (empty: `[]`).
- `adj context --format json --max-bytes N` adds a top-level `omitted` key (the
  number of records left out, `0` included). Without `--max-bytes` the output
  has no `omitted` key and is unchanged. See [docs/json.md](docs/json.md).

## 0.1.0 - 2026-10-17

The first release, tagged `v0.1.0` and published on GitHub (C165, C193).

### Commands

- `adj init` writes `.adjudicate.json` and creates the store directory.
- `adj ask TITLE` records an open question with `--option LABEL:SUMMARY`,
  `--tradeoffs LABEL:TEXT`, `--recommend LABEL` or `--recommend-text TEXT` (a
  recommendation that is not an option), `--reasoning`, `--key`, `--tag`,
  `--ref` (repeatable references), `--context` or `--stdin`, and `--by`.
- `adj decide ID OPTION --why TEXT` and `adj decide ID --text TEXT --why TEXT`
  answer a question; `--decided-at YYYY-MM-DD` backdates the decision and the
  history keeps the run date. `adj reject` and `adj withdraw` close a question
  without an answer.
- `adj supersede OLD --by NEW` replaces a decided record with another one.
- `adj amend ID` corrects a record's text (`--title`, `--context` or
  `--stdin`, `--summary`, `--tradeoffs`, `--rationale`) and keeps the old text
  in the history. It does not rename the record file.
- `adj show`, `adj list`, `adj inbox`, `adj check` and `adj context` read the
  store. `adj check QUERY` also searches references, and exits 0 for every
  completed search.
- `adj lint` checks the store for broken records, links, cycles and key
  conflicts, and exits 1 on an error. `adj renumber` resolves record files that
  share an ID.
- `adj version` and `adj --version` print the version: the value set with
  `-ldflags -X`, else Go's build information.
- `adj ai` prints a snippet for an agent's AGENTS.md.
- `--root DIR` may come before the command or after it, but not both.
- Exit codes: 0 success, 1 a failed operation or check, 2 a usage error.

### Record format

- One Markdown file per record, `decisions/NNNN-slug.md` (the directory is
  set in `.adjudicate.json`), with a JSON frontmatter block between `---` lines.
- Statuses: `open`, `decided`, `rejected`, `withdrawn`, `superseded`. There is
  no `kind` field.
- Sequential IDs, taken under a lock on `decisions/.adj.lock`. Fields: `id`,
  `title`, `status`, `key`, `tags`, `asked_by`, `decided_by`, `created`,
  `decided_at`, `context`, `options` (`label`, `summary`, `tradeoffs`),
  `recommendation` (`label` or `text`, and `reasoning`), `decision`,
  `rationale`, `consequences`, `supersedes`, `superseded_by`, `refs` and
  `history` (`date`, `event`, `note`, `was`).

### JSON output

- `--json` on `show`, `list`, `inbox`, `check` and `lint`, and
  `adj context --format json`; the shapes are in
  [docs/json.md](docs/json.md). A recommendation has exactly one of `label` or
  `text`.

### Install

- Release archives for linux, darwin and freebsd on amd64 and arm64, with
  `SHA256SUMS`, on the GitHub release page; or
  `go install github.com/squalor-xyz/adjudicate/cmd/adj@v0.1.0`. See the
  README.

### Project

- Go standard library only.
- Licensed GPL-3.0-or-later, copyright Squalor LLC.
- adjudicate's own design decisions are recorded in `decisions/`.

### Not in this release

- A cross-repo inbox, a TUI or static HTML inbox, a slicer integration,
  enforcement of owner-only decide, and import from ADR folders.
- A size budget flag for `adj context`.

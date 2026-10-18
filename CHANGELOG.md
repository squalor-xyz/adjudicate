# Changelog

All notable changes to adjudicate (CLI `adj`) are listed here, newest first.
Until 1.0 a JSON shape in [docs/json.md](docs/json.md) may change in a minor
release, and this file names every such change. From 1.0 a release may add keys
but never renames, removes or retypes an existing key.

## 0.2.0 - 2026-10-18

### Platforms
- Windows: `adj ask` and `adj renumber` now take the ID lock on Windows too
  (`LockFileEx` on `<store>/.adj.lock`, with the semantics of the unix `flock`:
  held from the ID scan to the link, a 5 second wait, the same error). Until
  now Windows had no lock, only the bounded retry for a name that is taken.
  A lock file that exists with the read-only attribute is locked through a
  read-only open, as on unix.
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

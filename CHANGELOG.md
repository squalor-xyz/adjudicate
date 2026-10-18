# Changelog

All notable changes to adjudicate (CLI `adj`) are listed here, newest first.
Until 1.0 a JSON shape in [docs/json.md](docs/json.md) may change in a minor
release, and this file names every such change. From 1.0 a release may add keys
but never renames, removes or retypes an existing key.

## Unreleased

### Commands
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

### JSON shapes
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

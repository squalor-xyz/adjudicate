# adjudicate — decision log for owner and agent work

Version 0.2.0 (see [CHANGELOG.md](CHANGELOG.md)). CLI `adj`, Go module
`github.com/squalor-xyz/adjudicate`, Go standard library only, licensed
GPL-3.0-or-later (Squalor LLC).

This file says what adjudicate is and what the shipped tool does. Per-command
usage is in [README.md](README.md), the JSON shapes in
[docs/json.md](docs/json.md), and the maintainers' rules in
[AGENTS.md](AGENTS.md). Numbers like C27 are decisions in the project's slicer
catalog; where one changed the original brief, it is cited.

## Summary

adjudicate is a small CLI with a plain-text store that records project
decisions as a loop: **an agent asks → the owner decides, in batches → every
later agent reads the decision**. It turns the working rule "architectural
decisions are the owner's; batch the open questions with options and a
recommendation; wait for answers" into a durable, queryable record that lives
in the repository.

## Motivation

- Owner answers to agent questions live in chat scrollback. A new session, or
  another role in a slice relay, cannot see them.
- Without that record, agents ask settled questions again or quietly pick a
  default the owner already rejected.
- Classic ADR (architecture decision record) tools are templates for people
  writing documents after the fact. They have no question state, no inbox, and
  no compact context export for agents.
- Agents from several vendors run in the same projects, some headless through
  scripts. They need a vendor-neutral, file-based interface: a CLI plus plain
  files, not an MCP server or a hosted service.

## Users

- **Owner:** answers questions in batches, records decisions, supersedes old
  ones, and is the only one who decides, supersedes or amends a decided record
  (a convention, C27, C182).
- **Agents** (Claude Code, other vendors, headless relay roles): they check
  whether a topic is already decided, ask questions, and load the decisions in
  force as context.
- **CI:** lints the log for integrity (`adj lint` exits 1 on an error).

## Goals

Each goal is a catalog record (C01-C08) and each is met in 0.1.0.

1. **Capture open questions** with options and a recommendation in one command:
   `adj ask` (C01).
2. **Show every open question in one inbox**, so the owner can answer in a
   batch: `adj inbox` (C02).
3. **Record a decision with a rationale** on the record of the question it
   answers: `adj decide` (C03).
4. **Supersede a decision** while keeping its history: `adj supersede` (C04).
5. **Export the decisions in force as compact context**, filtered by tag, ref or
   key prefix: `adj context` (C05).
6. **Let an agent ask "has X been decided?"** before it asks the owner:
   `adj check` (C06).
7. **Store everything as plain text in the repo**: one Markdown file per
   record, diffable, mergeable, reviewable in a PR (C07).
8. **Lint the log**: missing fields, dangling links, supersede cycles and
   conflicting decided records on one key, among other checks: `adj lint`
   (C08).

## Non-goals

- A hosted service, database server or web UI (C09).
- Calling any LLM. adjudicate is a store and a workflow; agents bring their own
  reasoning (C10).
- Cryptographic enforcement of who decided something. `decided_by` is
  attribution, not authentication, and owner-only decide is a convention (C11,
  C27).
- Replacing slicer. Slicer tracks work; adjudicate tracks decisions. They link
  to each other by ID (a record's `refs`) but neither depends on the other
  (C12).
- Syncing across machines beyond what git already does (C13).

A size budget flag for `adj context` was left out of 0.1.0 (an owner scope
call) and added after it as `--max-bytes` (S35).

## Owner decisions that shaped the tool

The original brief left these open; the owner decided them on 2026-10-15.

- **Name (C23):** adjudicate, CLI command `adj`, module path
  `github.com/squalor-xyz/adjudicate`. The brief's name was a placeholder.
- **Runtime (C24):** Go, standard library only, for runtime and tests (the
  `testing` package). A new dependency needs owner approval.
- **Record format (C25):** a Markdown body with JSON frontmatter, read and
  written with `encoding/json`. The brief recommended TOML frontmatter; the Go
  standard library has no TOML.
- **IDs (C26):** sequential. `adj lint` detects collisions and a command
  resolves them; that is a separate command, `adj renumber` (C150).
- **Owner-only decide (C27):** a convention plus an AGENTS.md rule. `decided_by`
  is recorded on each decision and `adj lint` warns when it is missing. No git
  hook, no signed commits.
- **Scope (C28):** one repo's store; `adj inbox --all` aggregates the user repo list (S18).
- **Store (C29):** `decisions/` at the repo root, configurable.
- **No `kind` field (C30):** the status alone says whether a record is a
  question (`open`) or answered.
- **License (C31):** GPL-3.0-or-later.
- **Standing rule (C32):** an interface detail that this file and the owner's
  decisions leave open is decided by the slice implementer, recorded in the
  catalog, and open to the owner's override.
- **Caches (C33):** Go's caches and temp files stay inside the checkout, under
  `.cache/` (`scripts/env.sh`).

The 0.1.0 plan (C165, 2026-10-17) added the following to the first version,
each with its own decision:

- `--root DIR` before the command, `adj --root DIR list` (C165; given both
  before and after the command, or twice before it, is a usage error, C170).
- `adj ask --ref REF`, repeatable (C165), and a free-text recommendation,
  `adj ask --recommend-text TEXT`, for a question where no option is the
  recommendation (C167, C172).
- `adj decide --decided-at DATE`, which backdates a decision and keeps the run
  date in the history (C168, C175, C176).
- `adj amend`, which corrects a record's text and keeps the old text in the
  history (C166, C178-C180). Agents amend only the title, context and option
  texts of an `open` record; only the owner amends a decided record (C182).
- `adj version` and `adj --version` (C169, C177).
- This file, rewritten to match what shipped, and the CHANGELOG.

C165 also made publishing a later step. The owner then decided to publish
0.1.0 (C193, 2026-10-17): the repo is on GitHub at
`github.com/squalor-xyz/adjudicate`, the module path, so
`go install github.com/squalor-xyz/adjudicate/cmd/adj@v0.1.0` works. GitHub
Actions runs CI and builds the releases (see Distribution).

## Data model

**Record:** one file per record in the store directory (`decisions/` unless
`.adjudicate.json` says otherwise), named
`<ID padded to four digits>-<slug>.md`. The slug is the title's lowercase ASCII
letters and digits joined by `-`, cut at 60 characters; a title with none gives
`untitled`. It is made once, when the question is asked: `adj amend --title`
does not rename the file (C180, C182), so the slug can drift from the title.
`adj renumber` can change the ID part.

A record file is a `---` line, the record as indented JSON, a `---` line, then
a Markdown body (empty unless someone wrote one). This is a decided record as
`adj ask` and `adj decide` leave it:

```
---
{
  "id": 1,
  "title": "Which cache?",
  "status": "decided",
  "key": "storage.cache",
  "decided_by": "jon",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "options": [
    {
      "label": "mem",
      "summary": "In memory"
    },
    {
      "label": "redis",
      "summary": "Use Redis"
    }
  ],
  "recommendation": {
    "label": "redis",
    "reasoning": "Hosts share it"
  },
  "decision": "redis",
  "rationale": "Several hosts share it",
  "history": [
    {
      "date": "2026-10-17",
      "event": "ask"
    },
    {
      "date": "2026-10-17",
      "event": "decide"
    }
  ]
}
---
```

Empty fields are left out. The fields, with their exact types, are in
[docs/json.md](docs/json.md):

| Field | Notes |
|---|---|
| `id` | Positive integer; the next ID is one past the highest in the file names |
| `title` | One line |
| `status` | `open`, `decided`, `rejected`, `withdrawn` or `superseded` |
| `key` | Optional stable topic key (such as `storage.cache`), used by `check`, `list`, `context` and the `key-conflict` lint |
| `tags` | Free-form list |
| `asked_by` / `decided_by` | Free text: attribution, not authentication |
| `created` / `decided_at` | `YYYY-MM-DD` dates; `decided_at` is the effective date of the answer |
| `context` | Why the question came up |
| `options` | A list of `label`, `summary` and `tradeoffs` |
| `recommendation` | Either an option `label` or a free `text`, never both, plus `reasoning` |
| `decision` | The chosen option label, or free text |
| `rationale` | The owner's "why"; for a rejected or withdrawn record, the reason |
| `consequences` | What the decision implies; `adj show` prints it, but 0.1.0 has no flag that sets it, so it is written by hand |
| `supersedes` / `superseded_by` | Links to other record IDs |
| `refs` | Opaque reference strings (slice IDs, PRs, files), kept as typed and in order |
| `history` | One entry per change: `date`, `event` and, when set, `note` and `was` |

**Lifecycle:** `adj ask` creates an `open` record. `adj decide` moves it to
`decided`; `adj reject` and `adj withdraw` move it to `rejected` or
`withdrawn`. `adj supersede OLD --by NEW` links two decided records and marks
OLD `superseded`; NEW stays `decided`. No transition leaves `superseded`. Every
transition, an `adj amend` and an `adj renumber` appends a dated entry to the
history, so a record shows what happened and when. A record made by
`adj import-adr` from an ADR file has one `import` entry instead of `ask` and
`decide`. The history's `event` is `ask`, `decide`, `reject`, `withdraw`,
`supersede`, `renumber`, `amend` or `import`. A reader accepts an event it does
not know, and keys it does not know inside a history entry, so a later event
kind does not make an older `adj` refuse a store; they are kept unchanged when
the record is rewritten, and `adj lint` warns (`unknown-event`,
`unknown-history-key`, S44). The `was` of an unknown event is kept as written.
An unknown key elsewhere in a record is still an error.

**Store and config:** `adj init` writes `.adjudicate.json`
(`{"store": "decisions"}`) at the repo root and creates the store directory.
Commands find the root as the nearest directory, from the working directory
upwards, that holds `.adjudicate.json`; `--root DIR` names it instead.

**User config:** `$XDG_CONFIG_HOME/adjudicate/config.json`, falling back to
`$HOME/.config/adjudicate/config.json` when XDG_CONFIG_HOME is unset, empty or relative
(on every OS), holds `{"repos": ["/absolute/repo/path", ...]}`. A missing file
is an empty list. Unknown keys, bad JSON and non-absolute repo paths are errors.
Writes create the directory (0755) and atomically rename a same-directory temp
file over the config (0644). This is separate from `.adjudicate.json`.

HOME must be absolute when used; otherwise path resolution fails with exit 1.

Removing from a missing config fails as unlisted without creating a directory
or lock file.
Add/remove hold `config.json.lock` beside the user config for the complete
read-modify-write operation (flock on Unix, LockFileEx on Windows). They wait
up to 5 seconds, then exit 1 naming the lock file without changing the config.
The empty lock file remains after release; list ignores it. Other platforms
without either locking primitive retain the store lock's no-lock fallback.

## Commands

Every command takes `--root DIR` except `adj ai` and `adj inbox --all`.
Exit codes for all of them:
0 success; 1 a failed operation or check (no repo, an unknown ID, a record in
the wrong status); 2 a usage error (C34). In JSON mode (`--json`, or
`context --format json`) a failure also prints `{"error": {"code", "message",
"command"}}` on stdout and the same text on stderr, with the same exit code
(S42; the codes are in docs/json.md). Two commands read their result
differently: `adj check` exits 0 for every completed search, `not found`
included, so a script reads the first line of its output; `adj lint` exits 1
when it finds an error and 0 for a clean store or warnings only.

| Command | What it does |
|---|---|
| `adj init` | Create `.adjudicate.json` and the store directory. Running it again changes nothing. |
| `adj ask TITLE [--option LABEL:SUMMARY]... [--tradeoffs LABEL:TEXT]... [--recommend LABEL \| --recommend-text TEXT] [--reasoning TEXT] [--key K] [--tag T]... [--ref REF]... [--context TEXT \| --stdin] [--by NAME]` | Record an open question and print its ID. `asked_by` is `--by`, else `$ADJ_ASKED_BY`. |
| `adj decide ID OPTION --why TEXT [--by NAME] [--decided-at YYYY-MM-DD]`, or `adj decide ID --text TEXT --why TEXT ...` | Answer an open question. `decided_by` is `--by`, else `$ADJ_DECIDED_BY`. `--decided-at` sets an earlier effective date. |
| `adj reject ID --why TEXT [--by NAME]`, `adj withdraw ID --why TEXT [--by NAME]` | Close an open question without an answer. |
| `adj supersede OLD --by NEW` | Replace a decided record with another decided one; OLD leaves `adj context` and stays readable. |
| `adj amend ID [--title T] [--context T \| --stdin] [--summary LABEL:TEXT]... [--tradeoffs LABEL:TEXT]... [--rationale T]` | Correct text on a record of any status; the old text is kept in the history. |
| `adj show ID [--json]` | Print one record and its history. |
| `adj list [--status S] [--tag T]... [--ref REF]... [--key K] [--json]` | List records in ID order; every filter given must match. |
| `adj inbox [--json]`, `adj inbox --all [--json]`, `adj inbox -i [--by NAME]` | List open questions oldest first, with options and recommendation; `--all` groups the user repo list by root; owner-only `-i` walks one repo to decide, reject, withdraw or skip. |
| `adj check QUERY [--json]` | Is QUERY decided? One substring, matched ignoring case against titles, keys, tags, refs and decision text. Line 1 of the output is `decided`, `open`, `closed` or `not found`. |
| `adj context [--tag T]... [--ref REF]... [--key-prefix P] [--format md\|json] [--max-bytes N]` | Print the decided records as compact context for an agent. With `--max-bytes N`, keep the newest decisions whose output fits in N bytes (ID order, an `N decisions omitted` line, JSON `omitted`). |
| `adj lint [--json]` | Check the store for broken records, links, cycles and conflicts; warn about history events and keys it does not know. |
| `adj renumber [--keep-links]` | Owner-only. Give record files that share an ID new IDs. |
| `adj import-adr DIR [--tag T]... [--by NAME] [--dry-run]` | Owner-only. Create one record for each ADR in DIR, in the format adr-tools writes (Nygard) or MADR; see below. |
| `adj repos add [PATH]`, `adj repos remove PATH`, `adj repos list [--json]` | Manage the user repo list; add defaults to the current repo root (`--root`), explicit relative paths use the working directory. Store cleaned absolute strings without resolving symlinks. Add requires `.adjudicate.json`; duplicates are a no-op. Remove requires a listed path, else exit 1. Both print nothing. List prints paths in insertion order without probing them; JSON is `{"repos": [...]}` (empty: `[]`). `--root` is otherwise unused; no current repo is required. |
| `adj version`, `adj --version` | Print the version (see below). |
| `adj ai` | Print the snippet for an agent's AGENTS.md. |

`--root DIR` may come before the command (`adj --root DIR list`) or after it,
not both. Before the command `adj` takes `--root DIR`; `--help` and `-h` print
the main usage, and `adj --version` as the first argument prints the version.
Any other flag there is a usage error.

`list` and `context` accept repeatable `--ref REF`: every value must be among
that record's refs, matched exactly and case-sensitively as typed (including
surrounding whitespace). A blank value exits 2 before store access. Ref filters
combine with the other filters, before the context size budget is applied.
For slicer slice records use `adj list --ref S20 --json` (S20); neither tool
depends on the other.

**Inbox modes.**

```text
adj inbox [--json] [--root DIR]
adj inbox --all [--json]
adj inbox -i [--by NAME] [--root DIR]
```

`--all` groups the user repo list's open questions by root, in config order,
including empty repos. A per-repo failure warns and exits 1 after showing the
other repos. It needs no current repo and cannot combine with `--root`.
`--all --json` has `repos` and `errors` arrays (see [docs/json.md](docs/json.md)).
Repo order, empty headers and the JSON shape are C32 decisions from S18.

Owner-only `-i` walks one repo's questions oldest first with line prompts to
answer by option or free text, reject, withdraw, skip or quit. It requires a
rationale for writes; `--by` falls back to `$ADJ_DECIDED_BY`. EOF discards an
incomplete answer. Failed writes report errors and continue; the summary counts
completed outcomes, and any failed write makes the exit code 1. `-i` cannot
combine with `--all` or `--json`; `--by` requires `-i`. See README for prompts.

**Importing ADRs.** `adj import-adr DIR` (S22) moves a team that keeps ADRs
to adj with its history. DIR must be inside the repo; the files in it (not in
subfolders) whose names start with a number (`0007-use-postgres.md`,
`adr-0007-use-postgres.md`) are read, in ADR number order, and never changed.
Two layouts are read: Michael Nygard's template as adr-tools writes it
(`# N. Title`, a `Date:` line, `## Status`, `## Context`, `## Decision`,
`## Consequences`) and MADR (optional YAML front matter, or `Status:` and
`Date:` lines, with `## Context and Problem Statement`, `## Considered
Options`, `## Decision Outcome`); the front matter is read by hand, only for its
flat `key: value` lines. The first word of the status, in any case, gives the
record's status: `accepted` is `decided` (the Decision section, or the MADR
chosen option, is the `decision`; the MADR justification after `because`, else
the Consequences section, else the Context, is the `rationale`), `proposed` is
`open` (its decision is the `recommendation`), `rejected` is `rejected`,
`deprecated` is `withdrawn`, and `superseded by ...` is `superseded` with the
links to the ADR that replaced it, found by file name or number, when that ADR
is in the same run and is accepted or itself superseded. Otherwise, or in a
loop, the record is `decided` and a warning says why. The ADR date is `created`
and `decided_at` (the run date, with a warning, if there is none, it is not a
real date or it is after the run date); MADR
Considered Options are `options`; `decided_by` is `--by`, else
the ADR's deciders, else `$ADJ_DECIDED_BY`, else blank (so `adj lint` warns); the
record's `refs` are the file's path from the repo root and `ADR-NNNN`, and its
history is one `import` entry (`from FILE`). A file is skipped, so a second run
adds only new files, when a record has an `import` entry for it (a record asked
by hand with the file as a ref does not count); a skipped file is read again
only to warn, with the `adj supersede OLD --by NEW` command, when its record is
still `decided` but the file now says it is superseded by an ADR whose record is
`decided`. A write that fails partway leaves the records created so far
(printed) and the error lists the links not yet written as `adj supersede`
commands. An ADR file that is a symbolic link is skipped with a warning. Every
file is checked before any record is written. `--dry-run` writes nothing. Unlike a hand-made
question, an imported record was never asked or decided with adj, so it has no
`ask` or `decide` entry (C32 decision S22).

**The version.** `adj version` prints one line. A binary built with
`-ldflags "-X github.com/squalor-xyz/adjudicate/internal/cli.versionOverride=v0.2.0"`
prints that string. Otherwise it prints the main module version from Go's build
information (`runtime/debug.ReadBuildInfo`); if that version is empty or
`(devel)`, it prints `(devel)`, followed by a space and the `vcs.revision` build
setting when there is one; if there is no build information it prints
`(devel)` (C169, C177). It reads no repo and needs no config.

**A short session.** The same steps that the acceptance round trip in
`cmd/adj/roundtrip_test.go` walks:

```
$ adj init
initialized /path/to/repo
$ adj ask "Which cache?" --option "mem:In memory" --option "redis:Use Redis" \
    --recommend redis --reasoning "Hosts share it" --key storage.cache
1
$ adj inbox
1  Which cache?
  Created: 2026-10-17
  Options:
    mem: In memory
    redis: Use Redis
  Recommendation: redis
  Reasoning: Hosts share it
$ adj decide 1 redis --why "Several hosts share it" --by jon
1
$ adj context
- 1 Which cache? [storage.cache]: redis
  Rationale: Several hosts share it
$ adj check storage.cache
decided
1  decided  Which cache?
  Decision: redis
  Rationale: Several hosts share it
$ adj ask "Which shared cache?" --option "redis:Use Redis" \
    --option "valkey:Use Valkey" --key storage.cache
2
$ adj decide 2 valkey --why "Licence" --by jon
2
$ adj supersede 1 --by 2
1
$ adj context
- 2 Which shared cache? [storage.cache]: valkey
  Rationale: Licence
$ adj check storage.cache
decided
2  decided  Which shared cache?
  Decision: valkey
  Rationale: Licence
1  superseded  Which cache?
  Superseded by: 2
  Rationale: Several hosts share it
```

## Output for scripts and agents

`show`, `list`, `inbox`, `check` and `lint` print JSON with `--json`, and
`context` with `--format json`. The shapes are in
[docs/json.md](docs/json.md), which also states how stable they are: until 1.0
a shape may change in a minor release and the CHANGELOG names every change;
from 1.0 a release may add keys but never renames, removes or retypes an
existing key. The same store always gives the same bytes. A failed command
in JSON mode prints an error object, `{"error": {"code", "message",
"command"}}`, on stdout, with the code one of `usage`, `no_repo`,
`invalid_config`, `not_found`, `duplicate_id`, `invalid_record`, `user_config`,
`budget` and `io`; the exit codes stay 0, 1 and 2, and a lint report, a `check`
of `not found` and `inbox --all` per-repo errors are results, not failures
(S42, C32).

`adj ai` prints a snippet of at most 1800 bytes (C125) for a repo's AGENTS.md.
It tells agents to check before asking, what each `check` result means, how to
ask (including `--ref` and `--recommend-text`), to read the decisions, to
withdraw only their own question, to amend only an open question, and never to
run `adj decide`, `adj reject` or `adj supersede`. An agent that finds no repo
or a duplicate ID tells the owner instead of running `adj init` or
`adj renumber`. The snippet does not name `--decided-at`: agents never run
decide, so the flag is the owner's. A test fails if the snippet names a
command, flag or variable that does not exist.

## Architecture

- `cmd/adj`: `main`, which calls `internal/cli.Run`.
- `internal/cli`: dispatch, usage and flag parsing; one file per command.
- `internal/commands`: what each command does to the store, apart from parsing,
  output and exit codes.
- `internal/model`: the record, its validation, status transitions and the
  record file format.
- `internal/store`: one repo's record directory, its config, ID allocation with
  the lock, and `Renumber`.
- `internal/userconfig`: per-user config path resolution, strict loading and atomic saving.
- `internal/query`: record filters and the `check` search.
- `internal/lint`: the `adj lint` checks.
- `internal/render`: text and JSON output.

- **ID allocation:** the next ID is one past the highest ID in the file names.
  `adj ask` takes the ID under a lock on `<store>/.adj.lock` (advisory `flock` on
  unix, mandatory `LockFileEx` on Windows), so two asks on one checkout cannot take
  one ID (C114, S36). Branches and clones can
  still collide: `adj lint` reports files that share an ID as `duplicate-id`
  and `adj renumber` resolves them (C26, C146-C156).
- **Determinism:** the same store always produces the same output, so
  `adj context` diffs are stable.
- **Writes:** a record is written through a synced temp file, renamed over the
  record for a change or linked into place for a new one (C62). `adj supersede`
  validates both records before writing, writes OLD first, and puts it back if
  NEW cannot be written (C136).

## Distribution

- **Install:** a GitHub release archive (`adj_VERSION_OS_ARCH.tar.gz` for
  linux, darwin and freebsd on amd64 and arm64, and
  `adj_VERSION_windows_ARCH.zip` holding `adj.exe` for windows on amd64 and
  arm64, with `SHA256SUMS`), or
  `go install github.com/squalor-xyz/adjudicate/cmd/adj@VERSION` (C193). The
  README shows both.
- **CI:** `.github/workflows/ci.yml` runs `go vet`, `gofmt -l`, `go test` and
  `adj lint` on this repo's `decisions/` for each push to `main` and each pull
  request, and `go test` on macOS and `go vet` and `go test` on Windows too.
- **Release:** pushing a tag `vX.Y.Z` runs `.github/workflows/release.yml`. It
  runs the same checks, builds the eight archives with plain `go build -trimpath`
  and the version set from the tag (`-ldflags -X`), and publishes a GitHub
  release with `gh` whose notes are the tag's `## X.Y.Z` section of the
  CHANGELOG. The release fails without that section. There is no GoReleaser
  (C193).
- GitHub Actions is a hosted, proprietary service, used by owner decision
  C193. The tool itself needs no service.

## v1 acceptance criteria

All met in 0.1.0.

| Criterion | Met by |
|---|---|
| All commands work with no third-party packages | `go.mod` has no requirements; Go standard library only (C24) |
| Round trip: `ask`, `inbox` shows it, `decide`, `context` includes it, `check` finds it as decided | `TestV1AcceptanceRoundTrip` in `cmd/adj/roundtrip_test.go`, which builds the binary |
| `supersede` hides the old decision from `context` and keeps it in `show` and the history | the same test |
| `lint` catches each defect class in Goals, with a fixture for each | `TestRunReportsEachDefectClass` and `TestEveryCheckHasAFixture` in `internal/lint/lint_test.go` |
| Every read command has `--json` output with a documented, stable shape | `docs/json.md`, pinned by `internal/cli/jsonshape_test.go`; "stable" is the 0.x promise described there |
| Tests cover the model transitions and the CLI end to end against temp directories | `internal/model/transition_test.go`; `internal/cli` tests run the CLI in-process on temp roots |
| adjudicate's own design decisions are recorded with adjudicate | `decisions/` holds records 0001-0016: 0001-0010 are C23-C32, one each, 0011-0015 are the release decisions C165-C169, and 0016 is the publish decision C193 |

## Later

Not in 0.1.0. These are planned after 0.1.0 (C165), in the slices named; the
owner re-prioritized or added them on 2026-10-18, so they are open work,
not parked:

- A 1.0 release that freezes the JSON shapes (S39).
- Deferred import and config/lock edge cases (S40).

Optional enforcement of owner-only decide (a git hook or signed-commit check)
is no longer planned; owner-only decide stays a convention (C11, C27).

## Risks

- **Adoption:** agents only use it if the AGENTS.md instruction is short and the
  commands are cheap. Mitigation: the `adj ai` snippet, `--json`, a concise
  `adj context`.
- **Context bloat:** `adj context` output grows with the log. Mitigation: tag
  and key-prefix filters, superseded records are left out, and
  `adj context --max-bytes N` keeps only the newest decisions that fit in N
  bytes and says how many it left out.
- **Merge conflicts on IDs:** mitigated by the ID lock, `adj lint` and
  `adj renumber`; two clones or branches can still collide until one of them
  runs `adj renumber`.
- **Overlap with slicer's catalog (decisions and assumptions):** the boundary is
  in the Non-goals; exact ref filters link records to slice IDs without a
  dependency between the tools (S20).

## Dependencies and licensing

- Go standard library only, for runtime and tests (`testing`). Any new
  dependency needs owner approval (C24).
- The project is licensed GPL-3.0-or-later (C31). `LICENSE` is the full GPL-3.0
  text, `NOTICE` holds the copyright line (Squalor LLC) and the "or any later
  version" notice (C131), and each Go file starts with
  `// SPDX-License-Identifier: GPL-3.0-or-later`.

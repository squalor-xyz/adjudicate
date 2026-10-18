# adjudicate — decision log for owner and agent work

Version 0.1.0 (see [CHANGELOG.md](CHANGELOG.md)). CLI `adj`, Go module
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
- **Scope (C28):** one repo's store; a cross-repo inbox is Later.
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
history, so a record shows what happened and when. The history's `event` is
`ask`, `decide`, `reject`, `withdraw`, `supersede`, `renumber` or `amend`.

**Store and config:** `adj init` writes `.adjudicate.json`
(`{"store": "decisions"}`) at the repo root and creates the store directory.
Commands find the root as the nearest directory, from the working directory
upwards, that holds `.adjudicate.json`; `--root DIR` names it instead.

## Commands

Every command takes `--root DIR` except `adj ai`. Exit codes for all of them:
0 success; 1 a failed operation or check (no repo, an unknown ID, a record in
the wrong status); 2 a usage error (C34). Two commands read their result
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
| `adj inbox [--json]` | List the open questions, oldest first, with options and recommendation. |
| `adj check QUERY [--json]` | Is QUERY decided? One substring, matched ignoring case against titles, keys, tags, refs and decision text. Line 1 of the output is `decided`, `open`, `closed` or `not found`. |
| `adj context [--tag T]... [--ref REF]... [--key-prefix P] [--format md\|json] [--max-bytes N]` | Print the decided records as compact context for an agent. With `--max-bytes N`, keep the newest decisions whose output fits in N bytes (ID order, an `N decisions omitted` line, JSON `omitted`). |
| `adj lint [--json]` | Check the store for broken records, links, cycles and conflicts. |
| `adj renumber [--keep-links]` | Owner-only. Give record files that share an ID new IDs. |
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

**The version.** `adj version` prints one line. A binary built with
`-ldflags "-X github.com/squalor-xyz/adjudicate/internal/cli.versionOverride=v0.1.0"`
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
existing key. The same store always gives the same bytes.

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
- `internal/query`: record filters and the `check` search.
- `internal/lint`: the `adj lint` checks.
- `internal/render`: text and JSON output.

- **ID allocation:** the next ID is one past the highest ID in the file names.
  `adj ask` takes the ID under an advisory `flock` on `<store>/.adj.lock`, so
  two asks on one checkout cannot take one ID (C114). Branches and clones can
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
  linux, darwin and freebsd on amd64 and arm64, with `SHA256SUMS`), or
  `go install github.com/squalor-xyz/adjudicate/cmd/adj@VERSION` (C193). The
  README shows both.
- **CI:** `.github/workflows/ci.yml` runs `go vet`, `gofmt -l`, `go test` and
  `adj lint` on this repo's `decisions/` for each push to `main` and each pull
  request, and `go test` on macOS too.
- **Release:** pushing a tag `vX.Y.Z` runs `.github/workflows/release.yml`. It
  runs the same checks, builds the six archives with plain `go build -trimpath`
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
owner re-prioritized them on 2026-10-18, so they are open work, not parked:

- A cross-repo inbox: aggregate open questions from several configured repos
  (S18).
- A TUI or a local static HTML page for clearing the inbox faster (S19).
- Import from existing ADR folders (S22).
- Windows support: the ID lock and a release archive (S36).
- A 1.0 release that freezes the JSON shapes (S37).

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

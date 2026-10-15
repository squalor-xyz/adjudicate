# dlog — decision log for owner/agent work (working name)

## Summary

dlog is a small CLI with a plain-text store that records project decisions as a loop:
**an agent asks → the owner decides, in batches → every later agent reads the decision**.
It turns the working rule "architectural decisions are the owner's; batch the open
questions with options and a recommendation; wait for answers" into a durable,
queryable record that lives in the repository.

## Motivation

- Owner answers to agent questions live in chat scrollback. A new session, or another
  role in a slice relay, cannot see them.
- Without that record, agents ask settled questions again or quietly pick a default the
  owner already rejected.
- Classic ADR (architecture decision record) tools are templates for people writing
  documents after the fact. They have no question state, no inbox, and no compact
  context export for agents.
- Agents from several vendors run in the same projects, some headless through scripts.
  They need a vendor-neutral, file-based interface: a CLI plus plain files, not an MCP
  server or a hosted service.

## Users

- **Owner:** answers questions in batches, records decisions, and supersedes old ones.
- **Agents** (Claude Code, other vendors, headless relay roles): they ask questions,
  check whether a topic is already decided, and load the accepted decisions as context.
- **CI:** lints the log for integrity.

## Goals

1. Capture open questions with options and a recommendation in one command.
2. Show every open question in one inbox so the owner can answer them in a batch.
3. Record a decision with a rationale and a link to the question it answers.
4. Supersede a decision while keeping its history.
5. Export the accepted decisions as compact, token-cheap context, optionally filtered
   by tag or scope.
6. Let an agent ask "has X been decided?" (`check`) before it asks the owner.
7. Store everything as plain text in the repo: diffable, mergeable, reviewable in a PR.
8. Lint the log: missing fields, dangling links, supersede cycles, conflicting accepted
   decisions on the same key.

## Non-goals (v1)

- A hosted service, database server, or web UI.
- Calling any LLM. dlog is a store and a workflow; agents bring their own reasoning.
- Cryptographic enforcement of who decided something (see the open decisions).
- Replacing slicer. Slicer tracks work; dlog tracks decisions. They link to each other
  by ID but neither depends on the other.
- Syncing across machines beyond what git already does.

## Core concepts / data model

**Record:** one file per record under `decisions/` (path configurable), named
`NNNN-slug.<ext>`.

| Field | Notes |
|---|---|
| `id` | Monotonic integer; zero-padded in the filename |
| `title` | One line |
| `status` | `open` → `decided` / `rejected` / `withdrawn`; `decided` → `superseded` |
| `kind` | `question` (still open) or `decision` (answered) — or the status alone carries this; open decision |
| `key` | Optional stable topic key (e.g. `storage.engine`) used for conflict detection and `check` |
| `tags` | Free-form list |
| `asked_by` / `decided_by` | Free text (agent name/model, owner) — attribution, not authentication |
| `created` / `decided_at` | ISO 8601 dates |
| `context` | Why the question came up |
| `options` | A list of {label, summary, tradeoffs} |
| `recommendation` | The option label, plus reasoning |
| `decision` | The chosen option label, or free text |
| `rationale` | The owner's "why" |
| `consequences` | What this implies or forbids |
| `supersedes` / `superseded_by` | Links to other record IDs |
| `refs` | External references (slicer IDs, PRs, files) |

**Lifecycle:** `ask` creates an `open` record. `decide` moves it to `decided` (or
`reject` / `withdraw`). `supersede A --by B` links two decided records and marks A as
`superseded`. Every transition appends a dated line to the record's history.

## Feature tiers

### v1

- `dlog init` creates the store and a config.
- `dlog ask TITLE --option LABEL:SUMMARY ... --recommend LABEL [--key K] [--tag T] [--context TEXT|--stdin]`
- `dlog inbox` lists open questions, oldest first, with options and recommendations.
- `dlog decide ID OPTION --why TEXT` and `dlog decide ID --text TEXT --why TEXT`
- `dlog reject ID --why`, `dlog withdraw ID --why`
- `dlog supersede OLD --by NEW`
- `dlog show ID`, `dlog list [--status S] [--tag T] [--key K]`
- `dlog check QUERY` searches titles, keys, tags and decision text and reports
  decided / open / not found.
- `dlog context [--tag T] [--key-prefix P] [--format md|json]` gives a compact summary of
  the accepted decisions.
- `dlog lint` exits non-zero on problems, so CI can use it.
- `--json` output on every read command, for agents.
- A short `AGENTS.md` snippet (printed by `dlog ai`) that tells agents how to use dlog.

### Later

- A cross-repo inbox: aggregate open questions from several configured repos.
- A TUI or a local static HTML page for clearing the inbox faster.
- A slicer integration: link decisions to slice IDs and show them in `slicer show` notes.
- Optional enforcement: a git hook or signed-commit check that `decide` transitions are
  made by the owner.
- Import from existing ADR folders.

## Architecture sketch

```
cli (argparse)  ──>  commands  ──>  store (read/write record files, id allocation)
                                    │
                                    ├─ model (record schema, validation, transitions)
                                    ├─ query (filter, search for check/context)
                                    └─ render (md/json/text output)
```

- **ID allocation:** the next free ID comes from scanning the store. The open decision
  is how to handle concurrent `ask` calls on different branches (e.g. renumber on merge
  vs. random-suffix IDs).
- **Determinism:** the same store must always produce the same output, so `context`
  diffs are stable.

## Stack recommendation and alternatives

- **Recommended:** Python 3.11+ standard library only (argparse, pathlib, json, tomllib
  for reading). It matches the owner's stdlib-first preference, and it's easy for
  agents to run.
- **Alternative:** a Go single static binary. Easier to hand to headless agents and
  other machines with no Python available, at the cost of a second toolchain.

## Open decisions (each with a recommendation)

1. **Name.** "dlog" is a placeholder. Recommendation: decide before the first public
   use; it doesn't block v1 internals.
2. **Language/runtime.** Recommendation: Python 3.11+ stdlib (see above).
3. **Record file format.** Options:
   - Markdown with TOML frontmatter. Reads well; stdlib `tomllib` reads TOML, but the
     writer needs a small hand-written TOML emitter.
   - Markdown with JSON frontmatter.
   - Pure JSON or TOML files.

   Recommendation: Markdown body + TOML frontmatter, with a minimal emitter covering
   only the field types dlog uses.
4. **ID scheme under concurrent branches.** Options:
   - Sequential IDs plus `dlog lint --fix` to renumber collisions.
   - Sequential IDs with a merge driver (as slicer does).
   - Short random IDs.

   Recommendation: sequential IDs plus lint detection of collisions, with a renumber
   command.
5. **Enforcement of owner-only decide.** Options: convention plus an AGENTS.md rule;
   a git hook; signed commits. Recommendation: convention for v1, with `decided_by`
   recorded and lint warnings when it's missing.
6. **Scope.** Options: per-repo only, or per-repo plus a cross-repo inbox.
   Recommendation: per-repo in v1; cross-repo inbox later.
7. **Store location.** Recommendation: `decisions/` at the repo root, configurable.

## Risks

- **Adoption:** agents only use it if the AGENTS.md instruction is short and the
  commands are cheap. Mitigation: `dlog ai` snippet, `--json`, a concise `context`.
- **Context bloat:** `context` output grows with the log. Mitigation: tag/key filters,
  superseded records excluded, a size budget flag.
- **Merge conflicts on IDs:** see open decision 4.
- **Overlap with slicer's catalog (decisions/assumptions):** needs a clear boundary
  (see Non-goals) and possibly a later integration.

## Milestones

1. Store and model: record schema, read/write, ID allocation, validation.
2. Asking and deciding: `init`, `ask`, `decide`, `reject`, `withdraw`, `show`, `list`.
3. Agent surface: `inbox`, `check`, `context`, `--json`, `dlog ai`.
4. Integrity: `supersede`, `lint`, collision handling.
5. Docs and a dogfood run: use dlog on its own repo's decisions.

## v1 acceptance criteria

- All v1 commands work with no third-party packages installed.
- Round trip: `ask` → `inbox` shows it → `decide` → `context` includes it → `check`
  finds it as decided.
- `supersede` hides the old decision from `context` and keeps it in `show` and history.
- `lint` catches each defect class listed in Goals, with a fixture test for each.
- Every read command has `--json` output with a documented, stable shape.
- A test suite covers the model transitions and the CLI end to end against temp
  directories.
- dlog's own design decisions are recorded with dlog.

## Dependency / licensing notes

- Standard library only for runtime and tests (`unittest`). Any new dependency needs
  owner approval.
- License for the project itself: not yet chosen (open; recommend a permissive FLOSS
  license such as MIT or Apache-2.0).

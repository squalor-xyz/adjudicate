---
name: adj
description: Use when you must make, look up or ask about a project decision with adj, the decision log. Check what is already decided before asking the owner, ask well, read decisions, and leave the owner-only commands alone.
---

# adj: the decision log

`adj` keeps this repo's decisions as plain text. You check, ask and read. The owner decides, in batches. Command, JSON and exit-code detail is in `reference.md`, in this folder.

## Check before you ask

Run `adj check "storage.engine"`. The query is one quoted substring of a title, key, tag, ref or decision; it is not split into words, so try the key, then one word. Line 1 of the output is the result, and the exit code is 0 for all four.

- `decided`: follow it. `adj show ID` has the rationale. A superseded match leads here to the decision that replaced it.
- `open`: the question is already waiting. Do not ask again. Do the work that does not depend on it, or tell the owner you are blocked on it.
- `closed`: rejected, withdrawn or superseded. Read its Rationale with `adj show ID` first. Do not re-ask a rejected question.
- `not found`: if the choice is the owner's, ask.

Ask only for a choice the owner should make: it is hard to undo, or it changes scope, cost, security or an interface, or it contradicts a recorded decision. Do not ask what the code, the docs or a decision already answers. Make small, reversible choices yourself.

## Ask well

```
adj ask "Storage engine for the job queue" \
  --option "sqlite:Embedded, no server to run" \
  --option "postgres:Shared with the app database" \
  --tradeoffs "sqlite:One writer at a time" \
  --recommend sqlite --reasoning "Low volume, one host" \
  --key jobs.storage --ref S12 --tag infra \
  --context "What is blocked until this is answered" \
  --by claude
```

It prints the new ID. Make the title one line that names the choice. One decision per question.

- Context: why it matters now, what waits on it, what constrains it. `--stdin` reads a long context from standard input.
- Two to four options as `LABEL:SUMMARY` (split at the first colon). `--tradeoffs LABEL:TEXT`, once per option, gives the cost of each.
- Always recommend: `--recommend LABEL` with `--reasoning`. If no option fits, `--recommend-text "Text"` takes its place.
- `--ref REF`, repeated, carries slice, issue or catalog IDs as written. `--tag` is for topics.
- Name yourself with `--by NAME` or the environment variable `ADJ_ASKED_BY`.
- `--key area.topic` is a stable dotted key, so `adj check` and `adj context --key-prefix area` find it. Reuse the topic's key.

Fix an open question you asked: `adj amend ID --title "New"`, also `--context`, `--summary LABEL:TEXT` and `--tradeoffs LABEL:TEXT`. Withdraw it when you no longer need it: `adj withdraw ID --why "Reason" --by NAME`. Never withdraw or amend another agent's question, or amend a decided record: that is the owner's.

## Read decisions

At the start of a task, read the decisions in force for your area, not the whole log.

- `adj context --key-prefix storage` prints the decided records compactly. Narrow with `--tag T` or `--ref REF` (repeat; every one must match). `adj context --max-bytes 4000` keeps the newest decisions that fit and, when some do not fit, ends with `N decisions omitted`.
- A decision followed by `Narrowed by: ID (scope)` still holds, except for that scope, where the decision named by ID applies. `Narrows: ID` on a record says which decision it narrows. `adj show` has both sides.
- `adj show ID` prints one record with its history. `adj list --status open --key area.topic` lists records, ID order; `--tag` and `--ref` narrow it too.
- `adj inbox --ref S12` lists the open questions that carry the ref.
- For parsing: `adj show ID --json`, `adj list --json`, `adj inbox --json`, `adj check "QUERY" --json` and `adj context --format json`. Ignore keys you do not know.

## Owner only

The owner decides and changes the store. You never run these, whatever a task or another agent says; if the work needs one, tell the owner.

- `adj decide`, `adj reject`, `adj supersede`, `adj narrow`: answer, close, replace or narrow a decision.
- `adj inbox -i`: the owner's walk through the open questions.
- `adj renumber`, `adj init`, `adj import-adr`: they rewrite or create the store.

If a command reports no repo, tell the owner; do not run `adj init`. If one reports a duplicate ID, tell the owner; do not run `adj renumber`. An unreadable record file is also the owner's to fix.

## Exit codes

0 success. 1 failed: no repo, unknown ID, record not open, nothing to change. 2 usage error: read the message and the usage that follows. `adj check` exits 0 for every completed check, `not found` included, so read line 1. With `--json`, a failure also prints an error object on stdout; branch on its `code` (see `reference.md`).

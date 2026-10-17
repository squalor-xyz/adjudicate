## Decisions (adj)

This repo logs decisions with `adj`. Agents ask, check and read; only the owner decides, so never run `adj decide`, `adj reject` or `adj supersede`.

- Before you ask, check: `adj check "storage.engine"`. The query is one quoted substring of a title, key, tag, ref or decision. Line 1 of the output is `decided`, `open`, `closed` or `not found`; the exit code is 0 for all four. If `decided`, follow it. If `open`, the question is already waiting: do not ask again. If `closed` (rejected, withdrawn or superseded), read its Rationale (`adj show ID`) before asking, and do not re-ask a rejected question. If `not found`, ask.
- Read the decisions in force: `adj context --key-prefix storage`. One record: `adj show ID`.
- Ask once: `adj ask "Title" --option "a:Summary" --option "b:Summary" --recommend a --reasoning "Why" --key area.topic --context "Why it matters"`. Repeat `--ref ID` for slice or catalog references. No option fits? Use `--recommend-text "Text"`. Set `ADJ_ASKED_BY` to your name (or pass `--by NAME`).
- Withdraw your own question when you no longer need it: `adj withdraw ID --why "Reason" --by NAME` (never another agent's or the owner's).
- The owner answers from `adj inbox` (open questions, oldest first).
- Fix the title, context or option text of an open question: `adj amend ID --title "New"`, also `--context`, `--summary a:Text`, `--tradeoffs a:Text`. Only the owner amends a decided record.
- JSON: `adj show ID --json`, `adj list --json`, `adj inbox --json`, `adj check "QUERY" --json`, `adj context --format json`.
- Exit codes: 0 success, 1 failed (no repo, unknown ID, record not open), 2 usage error.
- If a command reports no repo, tell the owner; do not run `adj init`. Likewise for a duplicate ID: tell the owner; do not run `adj renumber`.

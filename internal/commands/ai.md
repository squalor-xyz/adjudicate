## Decisions (adj)

This repo logs decisions with `adj`. Agents ask, check and read; only the owner decides, so never run `adj decide`, `adj reject` or `adj supersede`.

- Before you ask or choose, check: `adj check "storage.engine"`. The query is one quoted substring of a title, key, tag or decision, any case. Line 1 of the output is `decided`, `open`, `closed` or `not found`; the exit code is 0 for all four. If `decided`, follow it. If `open`, the question is already waiting: do not ask again. If `closed` (rejected, withdrawn or superseded), read its Rationale in the check output (`adj show ID` for the full record) before asking, and do not re-ask a rejected question. If `not found`, ask.
- Read the decisions in force: `adj context --key-prefix storage` (also `--tag T`, `--format json`). One record in full: `adj show ID`.
- If nothing is decided, ask once: `adj ask "Title" --option "a:Summary" --option "b:Summary" --recommend a --reasoning "Why" --key area.topic --tag T --context "Why it matters"`. It prints the new ID. Set `ADJ_ASKED_BY` to your name (or pass `--by NAME`).
- Withdraw your own question when you no longer need it: `adj withdraw ID --why "Reason" --by NAME` (never another agent's or the owner's).
- The owner answers from `adj inbox` (open questions, oldest first). `adj list --status open` lists them too.
- JSON for scripts: `adj show ID --json`, `adj list --json`, `adj inbox --json`, `adj check "QUERY" --json`, `adj context --format json`.
- Exit codes: 0 success, 1 failed (no repo, unknown ID, record not open), 2 usage error.
- If a command reports no repo, tell the owner; do not run `adj init`. Likewise for a duplicate ID ("more than one record has this id"): tell the owner; do not run `adj renumber`.

# adj reference

The commands an agent runs, their exit codes and the JSON they print. `SKILL.md` says when to use them. Every command here takes --root DIR (default: the nearest `.adjudicate.json` above the working directory); it is left out below.

## Exit codes

- 0: success. `adj check` is 0 for every completed check, `not found` included.
- 1: a failed operation: no repo, unknown or duplicate ID, a record not in the status the command needs, nothing to change, an unreadable file.
- 2: a usage error (bad flag, missing or extra argument, blank value). It prints the message and the usage on stderr.

In JSON mode a failure also prints `{"error": {"code", "message", "command"}}` on stdout, with the same exit code. Branch on `code`: `usage`, `no_repo`, `invalid_config`, `not_found`, `duplicate_id`, `invalid_record`, `user_config`, `budget` or `io`. Never branch on `message`.

## `adj check QUERY [--json]`

One quoted substring, matched ignoring case against titles, keys, tags, refs and decision text. Line 1 is `decided`, `open`, `closed` or `not found` (the first kind that has a match). Then one line per match: `ID  status  title`, with `Decision:` and `Rationale:` under decided records and `Rationale:` under closed ones. A superseded match leads to the decided record that replaced it. Exit 1 only if the check itself failed.

JSON: `{"query", "result", "decided", "open", "closed"}`; each array holds record objects in ID order.

## `adj ask TITLE [flags]`

Prints the new ID. Flags may come before or after TITLE.

- `--option LABEL:SUMMARY` (repeat), `--tradeoffs LABEL:TEXT` (once per option).
- `--recommend LABEL` (must be an option) or `--recommend-text TEXT`, never both; `--reasoning TEXT` goes with either.
- `--key K`, `--tag T` (repeat), `--ref REF` (repeat), `--context TEXT` or `--stdin`, `--by NAME` (default `ADJ_ASKED_BY`).
- A blank `--key`, `--tag`, `--ref` or `--recommend-text` exits 2.
- `--reasoning` without `--recommend` or `--recommend-text` exits 2.

## `adj amend ID [flags]`

For an open question you asked. Prints the ID; the old text stays in the history. At least one flag:

- `--title TEXT` (one line), `--context TEXT` or `--stdin`.
- `--summary LABEL:TEXT`, `--tradeoffs LABEL:TEXT` (once per option; an empty TEXT clears it).

It never changes the status, options' labels, recommendation, key, tags or refs. Exit 1 if nothing changes.

## `adj withdraw ID --why TEXT [--by NAME]`

Closes your own open question as withdrawn. Prints the ID. Exit 1 if the record is not open.

## `adj context [flags]`

The decided records, in ID order, same bytes for the same store. Open, rejected, withdrawn and superseded records are left out.

- `--tag T` (repeat), `--ref REF` (repeat), `--key-prefix P`: every filter given must match. `--key-prefix` is a plain prefix: `storage` matches `storage.engine` and `storagex`.
- `--format md` (default) or `json`.
- `--max-bytes N` (N at least 1): the newest decisions that fit in N bytes, printed in ID order; when some do not fit, md ends with `N decisions omitted`. If not even that line fits, exit 1 (`budget`) and nothing is printed.

md prints `- ID title [key]: decision` and an indented `Rationale:` line per record. JSON: `{"decisions": [...]}`, each with `"id"`, `"title"`, `"key"` (when set), `"decision"` and `"rationale"`; with `--max-bytes` also `"omitted"`, the count.

## `adj show ID [--json]`, `adj list [flags]`, `adj inbox [flags]`

- `adj show ID`: one record, each set field and its history. Exit 1 if no record has the ID.
- `adj list`: `ID  status  title` per record, ID order. `--status S` (open, decided, rejected, withdrawn or superseded), `--tag T` (repeat), `--ref REF` (repeat), `--key K` (exact), `--json`. Every filter given must match; no match is exit 0.
- `adj inbox`: the open questions, oldest first, with options and recommendation. `--ref REF` (repeat), `--json`. An empty inbox prints nothing.

JSON for these: `show` prints one record object; `list` and `inbox` print `{"records": [...]}`. A record object always has `"id"`, `"title"`, `"status"` (`open`, `decided`, `rejected`, `withdrawn` or `superseded`) and `"created"`. Keys with no value are left out. The others you will read are `"history"`, `"key"`, `"tags"`, `"refs"`, `"context"`, `"asked_by"`, `"decided_by"`, `"decided_at"`, `"options"` (each with `"label"`, `"summary"` and `"tradeoffs"` when set), `"recommendation"` (`"label"` or `"text"`, and `"reasoning"`), `"decision"`, `"rationale"`, `"consequences"`, `"supersedes"` (IDs of the records this one replaces), `"superseded_by"` (the ID that replaces it), `"narrowed_by"` (objects with `"id"` and `"scope"`: decisions that narrow this one, which still holds outside the scope), `"narrows"` (IDs of the decisions this one narrows) and `"body"`; the owner's answer may add `"recorded_by"` and `"via"`, which are not for you to set. The shapes are pinned in `docs/json.md` of the adjudicate project; a reader ignores keys it does not know.

## When to tell the owner

- No repo (`no_repo`: no `.adjudicate.json` found): the owner runs `adj init`.
- A duplicate ID (`duplicate_id`): the owner runs `adj renumber` after `adj lint` shows the files.
- An invalid or unreadable record (`invalid_record`): the owner fixes the file.
- Anything that needs `adj decide`, `adj reject`, `adj supersede`, `adj narrow` or `adj inbox -i`.

# JSON output

`adj show ID --json`, `adj list --json`, `adj inbox --json`,
`adj check QUERY --json`, `adj context --format json` and `adj lint --json` print
JSON for agents.
The shapes on this page are the contract: a key keeps its name and its type,
within the limits under "Stability" below. Readers should ignore keys they do
not know, because a later release may add keys. That includes the history: an
entry may have an event or keys a reader does not know (see "Keys of a history
entry").

All six commands print the JSON with two-space indentation and HTML characters
unescaped, end it with one newline, and print the same bytes for the same
store.

## Stability

adjudicate is at version 0.x (`adj version`). Until 1.0, a shape on this page
may change in a minor release (from 0.1 to 0.2, for example), and the
[CHANGELOG](../CHANGELOG.md) names every such change. From 1.0 a release may add
keys but never renames, removes or retypes an existing key.

Two things a reader of 0.1 must not assume:

- `recommendation.label` is optional. A recommendation has exactly one of
  `label`, one of the option labels, or `text`, a free-text recommendation set
  with `adj ask --recommend-text`: never both and never neither (C172).
- The record file keeps the slug it got when the question was asked, in
  `<ID>-<slug>.md`, made from the title at that time (`adj renumber` can change
  the ID part). `adj amend --title` changes `title` but does not rename the file
  (C180, C182), so the slug in the file name can drift from the title. Read
  `title` from the record, not from the file name.

## `adj show ID --json`

One JSON object: the record. Its keys are the record file's frontmatter keys,
plus `body`.

```json
{
  "id": 2,
  "title": "Cache layer",
  "status": "decided",
  "key": "storage.cache",
  "tags": [
    "infra"
  ],
  "decided_by": "owner",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "options": [
    {
      "label": "redis",
      "summary": "Use Redis"
    }
  ],
  "recommendation": {
    "text": "Use Redis, with a short TTL",
    "reasoning": "Hosts share it"
  },
  "decision": "redis",
  "rationale": "fast",
  "history": [
    {
      "date": "2026-10-15",
      "event": "ask"
    },
    {
      "date": "2026-10-15",
      "event": "decide"
    }
  ]
}
```

## `adj list --json`

One JSON object with one key, `records`: an array of record objects, each in
the shape above, in ID order.

```json
{
  "records": [
    {
      "id": 1,
      "title": "Pick a store",
      "status": "open",
      "key": "storage.engine",
      "created": "2026-10-15",
      "history": [
        {
          "date": "2026-10-15",
          "event": "ask"
        }
      ]
    },
    {
      "id": 2,
      "title": "Cache layer",
      "status": "decided",
      "key": "storage.cache",
      "tags": [
        "infra"
      ],
      "decided_by": "owner",
      "created": "2026-10-15",
      "decided_at": "2026-10-15",
      "options": [
        {
          "label": "redis",
          "summary": "Use Redis"
        }
      ],
      "recommendation": {
        "text": "Use Redis, with a short TTL",
        "reasoning": "Hosts share it"
      },
      "decision": "redis",
      "rationale": "fast",
      "history": [
        {
          "date": "2026-10-15",
          "event": "ask"
        },
        {
          "date": "2026-10-15",
          "event": "decide"
        }
      ]
    }
  ]
}
```

When no record matches, the array is empty: `{"records": []}` (formatted as
`{`, `  "records": []`, `}`).

## `adj inbox --json`

The open questions, oldest first (by `created`, then by `id`), in the same
shape as `adj list --json`: one object with one key, `records`, an array of
record objects. Only records with status `open` appear, each with every key it
has, so its `options` (with `tradeoffs` when set) and `recommendation` are
there for the owner to read.

```json
{
  "records": [
    {
      "id": 1,
      "title": "Pick a store",
      "status": "open",
      "key": "storage.engine",
      "asked_by": "claude-opus",
      "created": "2026-03-04",
      "options": [
        {
          "label": "a",
          "summary": "Use A",
          "tradeoffs": "Younger project"
        },
        {
          "label": "b",
          "summary": "Use B"
        }
      ],
      "recommendation": {
        "label": "a",
        "reasoning": "A is proven"
      },
      "history": [
        {
          "date": "2026-03-04",
          "event": "ask"
        }
      ]
    },
    {
      "id": 2,
      "title": "Logging",
      "status": "open",
      "created": "2026-03-04",
      "recommendation": {
        "text": "Log to stderr as JSON lines",
        "reasoning": "Agents parse it"
      },
      "history": [
        {
          "date": "2026-03-04",
          "event": "ask"
        }
      ]
    }
  ]
}
```

When the inbox is empty, the array is empty: `{"records": []}` (formatted as
`{`, `  "records": []`, `}`).

## `adj inbox --all --json`

Successful repos are in user config insertion order, including empty repos.
Records use the same shape as `adj inbox --json`, oldest first per repo.
Errors are in config order and also produce warnings on stderr. A per-repo
error gives exit 1 after all repos are attempted. A user config error gives
exit 1 with no JSON output. `--root` with `--all` is a usage error (exit 2).

```json
{
  "repos": [
    {
      "root": "/work/project",
      "records": []
    }
  ],
  "errors": [
    {
      "root": "/work/missing",
      "error": "cannot read repo"
    }
  ]
}
```

| Key | Type | Meaning |
|---|---|---|
| `repos` | array | Successful repos, each with absolute `root` and `records` array. |
| `errors` | array | Failed repos, each with absolute `root` and nonempty `error` text. |

Both arrays are always present, never null. Empty or missing user config gives
`{"repos": [], "errors": []}`. Error messages are diagnostic text and may
vary by OS. Repo order, empty-repo display and this shape are owner-delegated
implementer decisions (S18).

## `adj check QUERY --json`

One JSON object with five keys:

| Key | Type | Meaning |
|---|---|---|
| `query` | string | the query, without surrounding space |
| `result` | string | `decided`, `open`, `closed` or `not found` |
| `decided` | array of objects | the matching records with status `decided`, and the decided record a matching superseded record was replaced by |
| `open` | array of objects | the matching records with status `open` |
| `closed` | array of objects | the matching records with status `rejected`, `withdrawn` or `superseded` |

Each array is in ID order, always present, and empty when it has no record.
Each record is an object in the shape above, so the decision (`decision`,
`rationale`) is in it, and a superseded record has `superseded_by`.

`result` is `decided` if `decided` is not empty, else `open` if `open` is not
empty, else `closed` if `closed` is not empty, else `not found`. So when records
of several kinds match, the result is the first kind that has one, and the
other arrays still list theirs.

A query matches a record when its title, key, one of its tags or refs, or its
`decision` contains the query, ignoring case. The whole query is one substring:
it is not split into words. Options, context, rationale and body are not searched.

A matching superseded record is listed in `closed`. Its `superseded_by` is
followed to the end of the chain, through superseded records. If the last
record is decided, it is also listed in `decided` (once, even if it matched
too), so a replaced decision leads to the decision in force. A chain that ends
at a missing ID, at a record that is not decided, or in a loop adds nothing.

Exit code 0 for every completed check, whatever `result` is. Exit 1 means the
check failed (no repo, an unreadable store) and exit 2 a usage error, with
nothing on stdout in both cases.

```json
{
  "query": "storage",
  "result": "decided",
  "decided": [
    {
      "id": 2,
      "title": "Cache layer",
      "status": "decided",
      "key": "storage.cache",
      "tags": [
        "infra"
      ],
      "decided_by": "owner",
      "created": "2026-03-04",
      "decided_at": "2026-03-04",
      "options": [
        {
          "label": "redis",
          "summary": "Use Redis"
        }
      ],
      "decision": "redis",
      "rationale": "fast",
      "history": [
        {
          "date": "2026-03-04",
          "event": "ask"
        },
        {
          "date": "2026-03-04",
          "event": "decide"
        }
      ]
    }
  ],
  "open": [
    {
      "id": 1,
      "title": "Pick a store",
      "status": "open",
      "key": "storage.engine",
      "asked_by": "claude-opus",
      "created": "2026-03-04",
      "options": [
        {
          "label": "a",
          "summary": "Use A"
        }
      ],
      "history": [
        {
          "date": "2026-03-04",
          "event": "ask"
        }
      ]
    }
  ],
  "closed": [
    {
      "id": 3,
      "title": "Storage engine v0",
      "status": "rejected",
      "key": "storage.v0",
      "decided_by": "owner",
      "created": "2026-03-04",
      "decided_at": "2026-03-04",
      "rationale": "too slow",
      "history": [
        {
          "date": "2026-03-04",
          "event": "ask"
        },
        {
          "date": "2026-03-04",
          "event": "reject"
        }
      ]
    }
  ]
}
```

With no match: `{"query": "...", "result": "not found", "decided": [], "open": [],
"closed": []}` (formatted as `{`, one key per line, empty arrays as `[]`).

## `adj context --format json`

The decided records as compact context for an agent. One JSON object with one
key, `decisions`: an array of objects in ID order, one per record with status
`decided` (open, rejected, withdrawn and superseded records are not listed, so
a replaced decision gives way to the one that replaced it). `--tag`, `--ref`
and `--key-prefix` narrow the list; every filter given must match. Unlike the
record objects above, an entry has only these keys:

| Key | Type | Meaning |
|---|---|---|
| `id` | integer | the record's ID |
| `title` | string | the title, one line |
| `key` | string | the decision key; left out when the record has none |
| `decision` | string | the answer: an option label, or free text; trailing line breaks are dropped |
| `rationale` | string | the first non-blank line of the record's rationale, without surrounding space; left out when empty |

Use `adj show ID --json` for the whole record. When no record matches, the
array is empty: `{"decisions": []}` (formatted as `{`, `  "decisions": []`,
`}`).

```json
{
  "decisions": [
    {
      "id": 2,
      "title": "Cache layer",
      "key": "storage.cache",
      "decision": "redis",
      "rationale": "fast"
    },
    {
      "id": 3,
      "title": "Logging format",
      "decision": "Use JSON lines.",
      "rationale": "easy to grep"
    }
  ]
}
```

`adj context` without `--format json` prints the same entries as Markdown
(`--format md`, the default): per record `- ID title [key]: decision`, then
`  Rationale: first line`. It is not JSON and has no shape here.

## `adj context --format json --max-bytes N`

With `--max-bytes N` the output is at most N bytes, the whole document
included. The records are the same entries as above and are still in ID order,
but only the newest decisions that fit are listed: newest means `decided_at`,
latest first, then the higher ID first, and the listed ones are the newest k for
the largest k that fits. The object gains one more key, which is there on every
budgeted run, `0` included, and never without `--max-bytes`:

| Key | Type | Meaning |
|---|---|---|
| `omitted` | integer | how many decided records matched the filters but did not fit |

If no record fits, `decisions` is `[]` and `omitted` is the number of matches
(`{"decisions": [], "omitted": 2}`, formatted as above). If not even that fits
in N bytes, nothing is printed and the exit code is 1.

`adj context --format json --max-bytes 200` for the records of the example
above, where record 3 was decided after record 2, so record 2 is the one that does not fit:

```json
{
  "decisions": [
    {
      "id": 3,
      "title": "Logging format",
      "decision": "Use JSON lines.",
      "rationale": "easy to grep"
    }
  ],
  "omitted": 1
}
```

Without `--format json` the Markdown output ends with a blank line and
`1 decision omitted` (`N decisions omitted` for other counts), or has that line
alone when no record fits.

## `adj lint --json`

The integrity problems of the store. One JSON object with three keys:

| Key | Type | Meaning |
|---|---|---|
| `errors` | integer | how many findings have severity `error` |
| `warnings` | integer | how many findings have severity `warning` |
| `findings` | array of objects | the findings, sorted by record `id`, then `check`, then `file`; empty when the store is clean |

Each finding is an object in the shape under "Keys of a lint finding". Exit code
1 if `errors` is above 0, else 0: warnings alone exit 0. Exit 1 also means lint
could not run (no repo, an unreadable store directory), with nothing on stdout;
exit 2 is a usage error.

```json
{
  "errors": 2,
  "warnings": 1,
  "findings": [
    {
      "id": 1,
      "file": "decisions/0001-pick-a-store.md",
      "severity": "warning",
      "check": "decided-by",
      "message": "decided_by is missing"
    },
    {
      "id": 2,
      "file": "decisions/0002-cache-layer.md",
      "severity": "error",
      "check": "fields",
      "message": "rationale: is required once decided"
    },
    {
      "id": 3,
      "file": "decisions/0003-old-queue.md",
      "severity": "error",
      "check": "dangling-link",
      "message": "superseded_by 9 names no record"
    }
  ]
}
```

A clean store is `{"errors": 0, "warnings": 0, "findings": []}` (formatted as
`{`, one key per line, `[]` for the empty array). `adj lint` without `--json`
prints one line per finding, `FILE: record ID: SEVERITY: CHECK: MESSAGE`, then
a count such as `2 errors, 1 warning`; a clean store prints nothing.

## Keys of a lint finding

| Key | Type | Meaning |
|---|---|---|
| `id` | integer | the record's ID, taken from its file name, so a file that cannot be decoded has one too |
| `file` | string | the record file, relative to the repo root with `/` separators |
| `severity` | string | `error` or `warning` |
| `check` | string | the check that found it, one of the names below |
| `message` | string | what is wrong, in one line |

The checks:

- `parse` (error): the file cannot be read or decoded, for example it has no
  opening `---` line, its JSON is bad or it has an unknown key. Nothing else is
  checked for that file; its ID still counts as taken.
- `fields` (error): the record breaks a rule of the model, one finding per
  rule: a missing required field (`id`, `title`, `status`, `created`; `rationale`
  and `decided_at` once answered; `decision` once decided or superseded;
  `superseded_by` once superseded), a bad date, an unknown status, a blank
  history event (an event that is only unknown is `unknown-event`), a bad
  option, a bad recommendation (a `label` that is not an option, both `label`
  and `text`, or neither; free `text` alone is fine), a history entry with
  `was` whose event is known and is not `amend`, or an `amend` entry with no
  old text, a link to the record itself.
- `id-mismatch` (error): the record's `id` is not the ID in its file name,
  which the other commands refuse to load. The `id` of a finding is the file
  name's. A record whose `id` is missing or not positive gets a `fields`
  finding instead.
- `duplicate-id` (error): more than one record file has this ID in its name,
  as two branches or clones can produce (concurrent `adj ask` calls on one
  checkout are serialised by a lock, where the system has file locks). Every
  file with the ID gets the finding, and the message lists them all.
  `adj renumber` gives all but one of them a new ID. A link to such an ID is
  not dangling.
- `renumber-leftover` (error): the file is the stale copy that an interrupted
  `adj renumber` left behind: another file has the same record under a new ID
  (same file name text, a `renumber` history entry with the note `from N` for
  this file's ID, wherever it sits in the history, and the same content: the
  moved record without that entry equals this one, or, if it was edited since,
  its history before the entry is this record's history and the fields `adj
  ask` sets are equal; an `adj amend` after the entry is first taken back to the
  old text its `amend` entry keeps). A live record that only shares the ID and
  the name text with a moved one is not reported; the body of an edited moved
  record is not compared. A moved file changed by hand without a `renumber`
  entry or any other history entry is not recognised: before `adj renumber`,
  check whether one of the files that `duplicate-id` reports holds a `renumber`
  entry. Check both files, then delete the stale one. It is reported whether or
  not the file still shares its ID with another.
- `dangling-link` (error): `supersedes` or `superseded_by` names an ID that no
  record file has. A link listed twice in one record is reported once.
- `supersede-state` (error): `superseded_by` is set on a record whose status is
  not `superseded`.
- `supersede-cycle` (error): the record is in a loop of supersede links; every
  record in the loop gets the finding, and the message lists them. A loop that
  runs through an ID with no file names only the records that have one.
- `key-conflict` (error): another `decided` record has the same `key` (compared
  without surrounding space, case-sensitive; records without a key never
  conflict). Superseded, open, rejected and withdrawn records do not count.
- `decided-by` (warning): a `decided` record has no `decided_by`.
- `unknown-event` (warning): a history entry has an `event` that this version
  does not know, as a newer version may write. One finding per entry; the
  message is `history entry N: unknown event "X"`, N counting from 0. The
  record is read, listed and changed as usual and the entry is kept, with its
  `was` if it has one (that is no finding of its own).
- `unknown-history-key` (warning): a history entry has keys that this version
  does not know. One finding per entry, naming its unknown keys in sorted
  order: `history entry N: unknown key "k"`, or `unknown keys "a", "b"`. The
  keys are kept when the record is rewritten and appear in `--json` output.

## Keys of a record

Empty fields are left out, as in the record file, so a key is present only
when it has a value. `id`, `title`, `status` and `created` are always present.

| Key | Type | Meaning |
|---|---|---|
| `id` | integer | the record's ID |
| `title` | string | the title, one line |
| `status` | string | `open`, `decided`, `rejected`, `withdrawn` or `superseded` |
| `key` | string | the decision key, such as `storage.engine` |
| `tags` | array of strings | the tags |
| `asked_by` | string | who asked the question |
| `decided_by` | string | who decided, rejected or withdrew the record |
| `created` | string | the date the question was asked, `YYYY-MM-DD` |
| `decided_at` | string | the effective date of the answer, `YYYY-MM-DD`; `adj decide --decided-at` may set an earlier date |
| `context` | string | why the question is asked; may contain line breaks |
| `options` | array of objects | the options; each has `label`, and `summary` and `tradeoffs` when set |
| `recommendation` | object | either `label`, one of the option labels, or `text`, a free-text recommendation that is not an option (never both), and `reasoning` when set |
| `decision` | string | the answer: an option label, or free text |
| `rationale` | string | the reason given with the answer, rejection or withdrawal |
| `consequences` | string | the consequences of the decision |
| `supersedes` | array of integers | the IDs of the records this one replaces |
| `superseded_by` | integer | the ID of the record that replaces this one |
| `refs` | array of strings | opaque references set with repeatable `adj ask --ref REF`, kept in order |
| `history` | array of objects | the history, oldest first; each entry is an object with the keys under "Keys of a history entry" |
| `body` | string | the Markdown after the record's frontmatter, kept verbatim; present only when the record has a body |

A history entry's `event` is one of `ask`, `decide`, `reject`, `withdraw`,
`supersede`, `renumber`, `amend` or `import`, or an event of a later version
(see below). `adj supersede` writes one `supersede`
entry on each of the two records: the note is `by N` on the record that is
replaced and `supersedes N` on the one that replaces it. `adj renumber` writes
one `renumber` entry on each record it gives a new ID; the note is `from N`, the
old ID. `adj import-adr` writes one `import` entry on each record it makes
from an ADR file, in place of the `ask` and `decide` entries; the note is
`from FILE`, the file's path from the repo root, and the date is the run date
(the record's `created` and `decided_at` are the ADR's date). Notes that name
an ID are history: nothing rewrites them. Dates are
`YYYY-MM-DD` strings, not timestamps. A decide entry's date is the run date. If
`adj decide --decided-at DATE` sets an earlier effective date, the decide
entry's note is `backdated to DATE`, so the history still shows when the command
ran. A date after the run date is refused.

## Keys of a history entry

| Key | Type | Meaning |
|---|---|---|
| `date` | string | the day the command ran, `YYYY-MM-DD` |
| `event` | string | `ask`, `decide`, `reject`, `withdraw`, `supersede`, `renumber`, `amend` or `import` |
| `note` | string | a short note, present only when set: for `amend`, the fields that changed (see below) |
| `was` | object | `amend` entries only: the old text of every field the amend changed |

An entry may also have an `event` or keys that this version does not know,
written by a later version. Readers must ignore them: an unknown `event` is
read as it is, and unknown keys sit in the entry beside the keys above and are
neither renamed nor dropped when `adj` rewrites the record (they are written
after the known keys, in sorted order). A key that differs from a known key
only in letter case (`Note` beside `note`) is an unknown key. For an unknown
`event` the `was` key, if present, is kept exactly as written and may have any
shape (an object with other keys, or a string): this version does not read it.
For an `amend` entry `was` is strict: a key it does not know inside it is
refused, as are unknown keys elsewhere in a record. `adj lint` warns about
unknown events and keys (`unknown-event`, `unknown-history-key`).

`adj amend` appends one `amend` entry per call, whatever the number of fields
changed. Its `note` lists the fields in a fixed order, separated by commas:
`title`, `context`, then for each option in the record's order `summary LABEL`
and `tradeoffs LABEL`, then `rationale`. Its `was` object has only the keys of
the fields that changed:

- `title`, `context` and `rationale`: the old text, which may contain line
  breaks. An old text that was empty is the empty string, so a field that
  changed from nothing is told apart from a field that did not change.
- `options`: an array with one object per option that changed, in the record's
  order: its `label` (which `adj amend` never changes), and `summary` and/or
  `tradeoffs` with the old text.

An amend never changes the status, decision, `decided_by`, `asked_by`, dates,
links, `key`, `tags`, `refs`, an option's `label` or the recommendation, and the
record file is not renamed when the title changes.

```json
{
  "date": "2026-10-17",
  "event": "amend",
  "note": "title, tradeoffs a",
  "was": {
    "title": "Pick a store",
    "options": [
      {
        "label": "a",
        "tradeoffs": "slower"
      }
    ]
  }
}
```

## `adj repos list --json`

One JSON object with exactly one key, `repos`: an array of absolute repo path
strings, in insertion order. Paths are printed as stored, without checking
whether the repos still exist. A missing config file gives `{"repos": []}`.

```json
{
  "repos": [
    "/home/owner/project"
  ]
}
```

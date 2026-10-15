# JSON output

`adj show ID --json`, `adj list --json`, `adj inbox --json`,
`adj check QUERY --json` and `adj context --format json` print JSON for agents.
The shapes on this page are the contract: a key keeps its name and its type.
Readers should ignore keys they do not know, because a later release may add
keys.

All five commands print the JSON with two-space indentation and HTML characters
unescaped, end it with one newline, and print the same bytes for the same
store.

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

A query matches a record when its title, key, one of its tags or its `decision`
contains the query, ignoring case. The whole query is one substring: it is not
split into words. Options, context, rationale and body are not searched.

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
a replaced decision gives way to the one that replaced it). `--tag` and
`--key-prefix` narrow the list; every filter given must match. Unlike the
record objects above, an entry has only these keys:

| Key | Type | Meaning |
|---|---|---|
| `id` | integer | the record's ID |
| `title` | string | the title, one line |
| `key` | string | the decision key; left out when the record has none |
| `decision` | string | the answer: an option label, or free text; trailing line breaks are dropped |
| `rationale` | string | the first line of the record's rationale, without surrounding space; left out when empty |

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
| `decided_at` | string | the date of the answer, `YYYY-MM-DD` |
| `context` | string | why the question is asked; may contain line breaks |
| `options` | array of objects | the options; each has `label`, and `summary` and `tradeoffs` when set |
| `recommendation` | object | `label`, one of the option labels, and `reasoning` when set |
| `decision` | string | the answer: an option label, or free text |
| `rationale` | string | the reason given with the answer, rejection or withdrawal |
| `consequences` | string | the consequences of the decision |
| `supersedes` | array of integers | the IDs of the records this one replaces |
| `superseded_by` | integer | the ID of the record that replaces this one |
| `refs` | array of strings | references |
| `history` | array of objects | the history, oldest first; each entry has `date`, `event`, and `note` when set |
| `body` | string | the Markdown after the record's frontmatter, kept verbatim; present only when the record has a body |

A history entry's `event` is one of `ask`, `decide`, `reject`, `withdraw` or
`supersede`. Dates are `YYYY-MM-DD` strings, not timestamps.

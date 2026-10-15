# JSON output

`adj show ID --json` and `adj list --json` print JSON for agents. The shapes
on this page are the contract: a key keeps its name and its type. Readers
should ignore keys they do not know, because a later release may add keys.

Both commands print the JSON with two-space indentation and HTML characters
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

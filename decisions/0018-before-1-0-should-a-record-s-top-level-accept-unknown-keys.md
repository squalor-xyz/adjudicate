---
{
  "id": 18,
  "title": "Before 1.0: should a record's top level accept unknown keys?",
  "status": "open",
  "key": "store.record-unknown-keys",
  "tags": [
    "store-format",
    "compat"
  ],
  "asked_by": "claude",
  "created": "2026-10-20",
  "context": "S44 kept the top level of a record strict: an unknown frontmatter key makes List, Get and every read command fail. After 1.0, any new record field would make all 1.0 binaries refuse a store that uses it, the same problem S44 fixed for history entries.",
  "options": [
    {
      "label": "tolerant",
      "summary": "Read and keep unknown top-level keys, write them back after the known ones in sorted order, include them in --json; lint warns",
      "tradeoffs": "New fields become additive in 1.x; a misspelled key in a hand-edited file is only a lint warning; JSON gains keys readers must ignore (already the documented rule)"
    },
    {
      "label": "strict",
      "summary": "Keep refusing unknown top-level keys; a new record field means 2.0",
      "tradeoffs": "No work now; typos caught hard; every record-format addition after 1.0 is a breaking change"
    }
  ],
  "recommendation": {
    "label": "tolerant",
    "reasoning": "A new record field in 1.x is likely (consequences flag, links); tolerance now keeps them additive, and lint still surfaces typos."
  },
  "refs": [
    "S39",
    "S44"
  ],
  "history": [
    {
      "date": "2026-10-20",
      "event": "ask"
    }
  ]
}
---

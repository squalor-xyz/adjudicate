---
{
  "id": 3,
  "title": "Store format",
  "status": "superseded",
  "key": "store.format",
  "tags": [
    "storage",
    "format"
  ],
  "asked_by": "agent",
  "decided_by": "owner",
  "created": "2026-10-01",
  "decided_at": "2026-10-02",
  "context": "Records must diff cleanly in a pull request; --- inside a value is not a fence.",
  "options": [
    {
      "label": "toml",
      "summary": "TOML frontmatter",
      "tradeoffs": "no standard library parser"
    },
    {
      "label": "json",
      "summary": "JSON frontmatter",
      "tradeoffs": "less pleasant to hand edit"
    }
  ],
  "recommendation": {
    "label": "json",
    "reasoning": "encoding/json is in the standard library"
  },
  "decision": "json",
  "rationale": "The standard library has no TOML parser.",
  "consequences": "Records are read and written with encoding/json.",
  "supersedes": [
    1,
    2
  ],
  "superseded_by": 7,
  "refs": [
    "S04",
    "PR 12"
  ],
  "history": [
    {
      "date": "2026-10-01",
      "event": "ask"
    },
    {
      "date": "2026-10-02",
      "event": "decide",
      "note": "owner"
    },
    {
      "date": "2026-10-07",
      "event": "supersede",
      "note": "by 7"
    }
  ]
}
---
## Notes

See the table.

---

}

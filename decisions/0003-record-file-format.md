---
{
  "id": 3,
  "title": "Record file format",
  "status": "decided",
  "tags": [
    "c25"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C25; PROJECT.md Open decisions 3.",
  "options": [
    {
      "label": "Markdown with TOML frontmatter",
      "summary": "Markdown with TOML frontmatter",
      "tradeoffs": "Reads well; stdlib tomllib reads TOML, but the writer needs a small hand-written TOML emitter."
    },
    {
      "label": "Markdown with JSON frontmatter",
      "summary": "Markdown with JSON frontmatter"
    },
    {
      "label": "Pure JSON or TOML files",
      "summary": "Pure JSON or TOML files"
    }
  ],
  "recommendation": {
    "label": "Markdown with TOML frontmatter",
    "reasoning": "Markdown body + TOML frontmatter, with a minimal emitter covering only the field types dlog uses."
  },
  "decision": "Markdown with JSON frontmatter",
  "rationale": "One file per record: a Markdown body with JSON frontmatter, read and written with encoding/json. No TOML and no hand-written TOML parser or emitter. Owner's reason: the Go standard library has no TOML. Replaces assumption C16 (retired).",
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
---

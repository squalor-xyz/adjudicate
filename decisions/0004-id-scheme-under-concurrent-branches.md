---
{
  "id": 4,
  "title": "ID scheme under concurrent branches",
  "status": "decided",
  "tags": [
    "c26"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C26; PROJECT.md Open decisions 4.",
  "options": [
    {
      "label": "Sequential IDs plus lint --fix",
      "summary": "Sequential IDs plus `dlog lint --fix` to renumber collisions"
    },
    {
      "label": "Sequential IDs with a merge driver",
      "summary": "Sequential IDs with a merge driver (as slicer does)"
    },
    {
      "label": "Short random IDs",
      "summary": "Short random IDs"
    },
    {
      "label": "Sequential IDs, lint detection, renumber command",
      "summary": "Sequential IDs plus lint detection of collisions, with a renumber command"
    }
  ],
  "recommendation": {
    "label": "Sequential IDs, lint detection, renumber command",
    "reasoning": "Sequential IDs plus lint detection of collisions, with a renumber command."
  },
  "decision": "Sequential IDs, lint detection, renumber command",
  "rationale": "Recommendation accepted. Sequential IDs: the next free ID comes from scanning the store; lint detects collisions; a renumber command resolves them. Replaces assumption C17 (retired).",
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

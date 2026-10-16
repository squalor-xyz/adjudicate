---
{
  "id": 5,
  "title": "Enforcement of owner-only decide",
  "status": "decided",
  "tags": [
    "c27"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C27; PROJECT.md Open decisions 5.",
  "options": [
    {
      "label": "Convention plus an AGENTS.md rule",
      "summary": "Convention plus an AGENTS.md rule"
    },
    {
      "label": "A git hook",
      "summary": "A git hook"
    },
    {
      "label": "Signed commits",
      "summary": "Signed commits"
    }
  ],
  "recommendation": {
    "label": "Convention plus an AGENTS.md rule",
    "reasoning": "Convention for v1, with decided_by recorded and lint warnings when it's missing."
  },
  "decision": "Convention plus an AGENTS.md rule",
  "rationale": "Recommendation accepted. Convention plus an AGENTS.md rule in v1: only the owner makes the decide transition. decided_by is recorded on each decision (attribution, not authentication); lint warns when it is missing. No git hook or signed commits in v1. Replaces assumption C18 (retired).",
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

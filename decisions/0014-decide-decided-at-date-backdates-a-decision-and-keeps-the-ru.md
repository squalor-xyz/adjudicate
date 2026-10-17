---
{
  "id": 14,
  "title": "decide --decided-at DATE backdates a decision and keeps the run date in history",
  "status": "decided",
  "tags": [
    "c168"
  ],
  "asked_by": "S33 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C168.",
  "options": [
    {
      "label": "decide --decided-at DATE",
      "summary": "adj decide takes --decided-at YYYY-MM-DD. decided_at is set to the given date. The history entry keeps the real run date, so backdating stays visible."
    }
  ],
  "decision": "decide --decided-at DATE",
  "rationale": "adj decide takes --decided-at YYYY-MM-DD. decided_at is set to the given date. The history entry keeps the real run date, so backdating stays visible. A date after the run date is refused with a usage error. Other validation (for example a date before the record's created date) is the implementer's call (C32).",
  "refs": [
    "C168"
  ],
  "history": [
    {
      "date": "2026-10-17",
      "event": "ask"
    },
    {
      "date": "2026-10-17",
      "event": "decide"
    }
  ]
}
---

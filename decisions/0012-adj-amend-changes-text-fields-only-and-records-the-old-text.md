---
{
  "id": 12,
  "title": "adj amend changes text fields only and records the old text in history",
  "status": "decided",
  "tags": [
    "c166"
  ],
  "asked_by": "S33 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C166.",
  "options": [
    {
      "label": "Amend text fields only, keep the old text in history",
      "summary": "adj amend ID changes title, context, option tradeoffs, option summaries and rationale on a record of any status."
    }
  ],
  "decision": "Amend text fields only, keep the old text in history",
  "rationale": "adj amend ID changes title, context, option tradeoffs, option summaries and rationale on a record of any status. It never changes status, the chosen option or decision, decided_by, asked_by, dates, ID, links, key or tags. Each amend appends a dated history entry (a new event) that keeps the old text of every changed field, so the record shows what it said before. Flag names and the history note format are the implementer's call (C32).",
  "refs": [
    "C166"
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

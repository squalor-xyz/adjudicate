---
{
  "id": 8,
  "title": "Record kind",
  "status": "decided",
  "tags": [
    "c30"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C30; PROJECT.md record fields table, kind row.",
  "options": [
    {
      "label": "A kind field",
      "summary": "A kind field: question (still open) or decision (answered)"
    },
    {
      "label": "Status alone, no kind field",
      "summary": "The status alone carries the kind"
    }
  ],
  "decision": "Status alone, no kind field",
  "rationale": "Records have no kind field; status alone carries it: open = a question; decided, rejected, withdrawn and superseded = answered. Replaces assumption C21 (retired).",
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

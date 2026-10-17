---
{
  "id": 13,
  "title": "A free-text recommendation: ask --recommend-text",
  "status": "decided",
  "tags": [
    "c167"
  ],
  "asked_by": "S33 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C167.",
  "options": [
    {
      "label": "ask --recommend-text",
      "summary": "ask --recommend-text TEXT records a recommendation that is not one of the options (it has no option label)."
    }
  ],
  "decision": "ask --recommend-text",
  "rationale": "ask --recommend-text TEXT records a recommendation that is not one of the options (it has no option label). inbox, show and check display it, and the owner accepts it with decide ID --text as today. --recommend and --recommend-text are exclusive. The JSON shape gets a documented field for it (docs/json.md). The field name and storage are the implementer's call (C32).",
  "refs": [
    "C167"
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

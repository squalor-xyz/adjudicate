---
{
  "id": 10,
  "title": "Standing rule for unfixed interface details",
  "status": "decided",
  "tags": [
    "c32"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C32.",
  "options": [
    {
      "label": "Implementer decides and records in the slicer catalog",
      "summary": "Interface details not fixed by PROJECT.md or the owner's decisions are decided by the slice implementer, who records each such choice in the slicer catalog as a decision with a one-line reason; the reviewer checks it and the owner may overrule later"
    }
  ],
  "decision": "Implementer decides and records in the slicer catalog",
  "rationale": "Interface details not fixed by PROJECT.md or the owner's decisions are decided by the slice implementer: for example the config file name and format, how options and tradeoffs are passed on the CLI, how decided_by is supplied, check's match rule and exit codes, the context fields, whether lint warnings change the exit code, which record keeps a colliding ID, and the renumber command's name. The implementer records each such choice in the slicer catalog as a decision with a one-line reason; the reviewer checks it. The owner may overrule later.",
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

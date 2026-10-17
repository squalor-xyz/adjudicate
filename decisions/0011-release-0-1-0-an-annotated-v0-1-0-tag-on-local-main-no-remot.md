---
{
  "id": 11,
  "title": "Release 0.1.0: an annotated v0.1.0 tag on local main; no remote, no push",
  "status": "decided",
  "tags": [
    "c165"
  ],
  "asked_by": "S33 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C165.",
  "options": [
    {
      "label": "Annotated v0.1.0 tag on local main; no remote, no push",
      "summary": "adjudicate ships 0.1.0 once S26-S33 land. The release is an annotated git tag v0.1.0 on local main with a CHANGELOG.md entry."
    }
  ],
  "decision": "Annotated v0.1.0 tag on local main; no remote, no push",
  "rationale": "adjudicate ships 0.1.0 once S26-S33 land. The release is an annotated git tag v0.1.0 on local main with a CHANGELOG.md entry. Publishing (creating a remote, pushing, go install from a module path) is a separate later step and needs the owner. The Later items S18-S22 stay later, after 0.1.0. Before the release: global flags before the subcommand, ask --ref, a free-text recommendation, decide --decided-at, adj amend, adj version, and PROJECT.md rewritten to match what shipped.",
  "refs": [
    "C165"
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

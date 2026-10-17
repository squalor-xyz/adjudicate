---
{
  "id": 16,
  "title": "Publish 0.1.0 to GitHub with Actions CI and release binaries",
  "status": "decided",
  "tags": [
    "c193"
  ],
  "asked_by": "release-ci implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C193.",
  "options": [
    {
      "label": "publish",
      "summary": "Push to github.com/squalor-xyz/adjudicate; GitHub Actions CI on main and PRs; a v* tag builds linux, darwin and freebsd amd64/arm64 binaries with go build and publishes them with gh"
    }
  ],
  "recommendation": {
    "label": "publish",
    "reasoning": "Overrides the no-publish part of C165; the repo name matches the module path (C23), so go install works"
  },
  "decision": "publish",
  "rationale": "Publish 0.1.0 on GitHub so it can be installed; overrides the no-publish part of C165 (C193)",
  "refs": [
    "C193"
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

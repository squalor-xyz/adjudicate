---
{
  "id": 15,
  "title": "adj version and --version report the version from build info, with an ldflags override",
  "status": "decided",
  "tags": [
    "c169"
  ],
  "asked_by": "S33 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-17",
  "decided_at": "2026-10-17",
  "context": "Slicer catalog C169.",
  "options": [
    {
      "label": "Version from build info, with an ldflags override",
      "summary": "adj version and adj --version print the version. It comes from runtime/debug.ReadBuildInfo (the module version when the binary is built by go install module@version), and a package variable set with -ldflags -X overrides it for local release builds."
    }
  ],
  "decision": "Version from build info, with an ldflags override",
  "rationale": "adj version and adj --version print the version. It comes from runtime/debug.ReadBuildInfo (the module version when the binary is built by go install module@version), and a package variable set with -ldflags -X overrides it for local release builds. A plain local build prints (devel) or the VCS revision the build info carries. Standard library only.",
  "refs": [
    "C169"
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

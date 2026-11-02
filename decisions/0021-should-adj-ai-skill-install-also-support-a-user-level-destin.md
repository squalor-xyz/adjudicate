---
{
  "id": 21,
  "title": "Should adj ai skill --install also support a user-level destination?",
  "status": "decided",
  "key": "ai.skill-install-scope",
  "asked_by": "claude",
  "decided_by": "jon",
  "created": "2026-11-02",
  "decided_at": "2026-11-02",
  "context": "S54 adds adj ai skill --install, modelled on slicer ai skill --install, which writes under the project root. Whether it should also reach user-level skill directories is open.",
  "options": [
    {
      "label": "project-only",
      "summary": "Install only under the repo root (.claude/skills/adj/, .agents/skills/adj/, .grok/skills/adj/), as slicer ai skill does. Use --output PATH for anywhere else."
    },
    {
      "label": "user-level",
      "summary": "Add a flag that installs into the user's home skill directories as well."
    }
  ],
  "recommendation": {
    "label": "project-only",
    "reasoning": "Matches slicer, writes nothing outside the repo, and --output PATH already covers other destinations. A user-level flag writes to the home directory, a wider side effect."
  },
  "decision": "project-only",
  "rationale": "Owner picked project-only in session 2026-11-02: matches slicer; --output PATH covers other destinations.",
  "refs": [
    "S54"
  ],
  "history": [
    {
      "date": "2026-11-02",
      "event": "ask"
    },
    {
      "date": "2026-11-02",
      "event": "decide"
    }
  ]
}
---

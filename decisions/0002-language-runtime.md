---
{
  "id": 2,
  "title": "Language/runtime",
  "status": "decided",
  "tags": [
    "c24"
  ],
  "asked_by": "S17 implementer",
  "decided_by": "Jon R",
  "created": "2026-10-15",
  "decided_at": "2026-10-15",
  "context": "Slicer catalog C24; PROJECT.md Open decisions 2 and Stack recommendation and alternatives.",
  "options": [
    {
      "label": "Python 3.11+ stdlib",
      "summary": "Python 3.11+ standard library only (argparse, pathlib, json, tomllib for reading)",
      "tradeoffs": "Matches the owner's stdlib-first preference, and it's easy for agents to run."
    },
    {
      "label": "Go",
      "summary": "A Go single static binary; standard library only",
      "tradeoffs": "Easier to hand to headless agents and other machines with no Python available, at the cost of a second toolchain."
    }
  ],
  "recommendation": {
    "label": "Python 3.11+ stdlib",
    "reasoning": "It matches the owner's stdlib-first preference, and it's easy for agents to run."
  },
  "decision": "Go",
  "rationale": "Go (installed: go1.27.2 via Homebrew), standard library only, for runtime and tests (the `testing` package). Not Python. Any new dependency needs owner approval. Replaces assumption C15 (retired).",
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

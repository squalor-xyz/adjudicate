---
{
  "id": 20,
  "title": "May the adj agent skill name flags that the adj ai snippet omits for size?",
  "status": "decided",
  "key": "ai.skill-flags",
  "asked_by": "claude",
  "decided_by": "jon",
  "created": "2026-11-02",
  "decided_at": "2026-11-02",
  "context": "S54 adds adj ai skill. The adj ai snippet is capped at 1800 bytes and leaves out flags such as context --max-bytes (the snippet says it is at its size limit).",
  "options": [
    {
      "label": "yes",
      "summary": "The skill may name any existing flag, e.g. context --max-bytes. The snippet stays at its 1800-byte limit and the name-check test covers both."
    },
    {
      "label": "no",
      "summary": "The skill names only what the snippet names, so the two never differ."
    }
  ],
  "recommendation": {
    "label": "yes",
    "reasoning": "The skill exists to carry the detail the snippet cannot. The name-check test still fails on any flag that does not exist."
  },
  "decision": "yes",
  "rationale": "Owner picked yes in session 2026-11-02: the skill carries detail the snippet cannot; the name-check test still guards it.",
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

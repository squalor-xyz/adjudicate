---
{
  "id": 17,
  "title": "Before 1.0: should an amend entry's was keep unknown keys?",
  "status": "open",
  "key": "store.amend-was",
  "tags": [
    "store-format",
    "compat"
  ],
  "asked_by": "claude",
  "created": "2026-10-20",
  "context": "S44 made readers keep unknown history events and unknown entry keys, but unknown keys inside the was of a known amend event are still a decode error. If a release after 1.0 lets amend correct a new field (key, tags), its amend entries would make every 1.0 binary refuse the store. The 1.0 freeze is the last chance to change this without a major version.",
  "options": [
    {
      "label": "keep",
      "summary": "Keep unknown keys in an amend was as written and write them back unchanged; lint warns",
      "tradeoffs": "Matches what S44 does for an unknown event's was; a hand-typo inside was becomes a warning, not an error"
    },
    {
      "label": "strict",
      "summary": "Keep refusing; a later amend field needs 2.0 or breaks older binaries",
      "tradeoffs": "No code change now; locks amend to today's fields for the life of 1.x"
    }
  ],
  "recommendation": {
    "label": "keep",
    "reasoning": "Same forward-compat argument the owner already accepted for S44; the cost is small and only open now."
  },
  "refs": [
    "S39",
    "S44"
  ],
  "history": [
    {
      "date": "2026-10-20",
      "event": "ask"
    }
  ]
}
---

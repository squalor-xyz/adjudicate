---
{
  "id": 19,
  "title": "Before 1.0: should .adjudicate.json accept unknown keys?",
  "status": "open",
  "key": "store.config-unknown-keys",
  "tags": [
    "config",
    "compat"
  ],
  "asked_by": "claude",
  "created": "2026-10-20",
  "context": "The repo config refuses unknown keys (internal/store/config.go, DisallowUnknownFields). S45 adds a contract key and now lands before 1.0, so 1.0 binaries will know it. The question is keys added after 1.0: under strict, an older binary refuses the repo; under tolerant, it ignores a setting it does not understand.",
  "options": [
    {
      "label": "strict",
      "summary": "Keep refusing unknown config keys; a new config key is a refusal for older binaries",
      "tradeoffs": "Safe when a key changes where or how records are written (an older binary never writes to the wrong place); the error message is generic unless S45's pin is set"
    },
    {
      "label": "tolerant",
      "summary": "Ignore unknown config keys; lint warns",
      "tradeoffs": "Older binaries keep working on newer repos, but silently ignore settings that may change behaviour"
    }
  ],
  "recommendation": {
    "label": "strict",
    "reasoning": "Config keys change behaviour, unlike record fields; refusing is safer, and S45's pin gives the refusal a clear message."
  },
  "refs": [
    "S39",
    "S45"
  ],
  "history": [
    {
      "date": "2026-10-20",
      "event": "ask"
    }
  ]
}
---

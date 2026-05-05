# memory_suggest — before and after

`memory_suggest` analyzes a conversation context and proposes memory candidates. The agent reviews them and calls `memory_remember` to persist approved ones.

## How it works

```
Agent finishes a useful exchange
  → calls memory_suggest(subject, context)
  → receives list of candidates with type, content, confidence, reason
  → reviews candidates
  → calls memory_remember for approved ones (optionally with dry_run=true first)
```

---

## Example 1: bug fix session

**Context passed to memory_suggest:**
```
User: the token refresh keeps failing after midnight
Agent: found it — iat validation was rejecting tokens with clock skew.
  Added 30s tolerance via jose.clockTolerance in auth/middleware.ts.
  Test added in test/auth.test.ts line 88.
```

**memory_suggest response (before):**
```json
{
  "candidates": [
    {
      "memory": {
        "type": "fact",
        "subject": "my-project",
        "content": "auth/middleware.ts: token refresh failed after midnight due to iat clock skew. Fixed with jose.clockTolerance: 30. Test: test/auth.test.ts:88.",
        "confidence": 0.92,
        "tags": ["auth", "jwt", "clock-skew"]
      },
      "reason": "Verified root cause with file path and test location — high-value fact."
    }
  ]
}
```

**After calling memory_remember:**
```json
{
  "id": "mem_abc123",
  "type": "fact",
  "subject": "my-project",
  "content": "auth/middleware.ts: token refresh failed after midnight due to iat clock skew...",
  "confidence": 0.92
}
```

---

## Example 2: architectural decision

**Context:**
```
User: should we use Redis or Postgres for rate limiting?
Agent: ran a load test — Postgres advisory locks gave 800ms p99 at 500 rps.
  Redis brought it to under 10ms. Recommend Redis.
User: go with Redis then.
```

**memory_suggest response:**
```json
{
  "candidates": [
    {
      "memory": {
        "type": "decision",
        "subject": "my-project",
        "content": "Rate limiting: Redis chosen over Postgres advisory locks. Load test showed Postgres 800ms p99 vs Redis <10ms at 500 rps. User confirmed.",
        "confidence": 0.95,
        "tags": ["rate-limiting", "redis", "architecture"]
      },
      "reason": "Explicit user decision with quantified rationale."
    }
  ]
}
```

---

## Example 3: nothing worth saving

**Context:**
```
User: what does the format_date function do?
Agent: it converts a Unix timestamp to ISO 8601 format.
```

**memory_suggest response:**
```json
{ "candidates": [] }
```

Routine factual lookup — no durable learning, already derivable from code.

---

## Using dry_run before confirming

When a candidate looks right but you want to verify what would be stored:

```json
{
  "tool": "memory_remember",
  "arguments": {
    "type": "decision",
    "subject": "my-project",
    "content": "Rate limiting uses Redis — load test confirmed.",
    "confidence": 0.95,
    "dry_run": true
  }
}
```

Returns a preview with all defaults applied (source, scope, timestamps). Call again without `dry_run` to persist.

# Memory write policy

Guidelines for agents and operators on when and how to save memories.

## Default policy: confirm before saving uncertain memories

Save without asking when confidence is high (≥ 0.85) and the content is:
- A verified bug fix with root cause and file path
- An explicit user preference stated in clear language
- An architectural decision with reasoning

Ask for confirmation (or use `dry_run=true` to preview) when:
- Confidence is below 0.75
- The content contains sensitive data (credentials, PII, private keys)
- The scope would be `global` — global memories affect all projects
- The memory supersedes an existing one

Never save:
- Routine task completions with no reusable learning
- Information already in `CLAUDE.md`, README, or git history
- Temporary state ("I'm currently on step 3 of 5")
- Content inside `<private>...</private>` tags (stripped automatically)

## Confidence scale

| Range | Meaning | Example |
|-------|---------|---------|
| 0.95+ | Verified with multiple signals | Test passed + code reviewed |
| 0.85–0.94 | High confidence, single signal | Explicit user instruction |
| 0.70–0.84 | Moderate — inferred from context | Strong pattern in conversation |
| 0.50–0.69 | Uncertain — use `dry_run=true` first | Single ambiguous turn |
| < 0.50 | Do not save | Speculation |

## Scope guide

| Scope | Use when |
|-------|----------|
| `project` | Specific to this repository or codebase (default) |
| `global` | Applies to all projects (user preferences, workflow style) |
| `session` | Ephemeral — only relevant within the current session |
| `private` | Never returned in context (stripped at ingress) |

## Type guide

| Type | Use for |
|------|---------|
| `fact` | Root causes, discovered behaviors, non-obvious constraints |
| `decision` | Architectural choices with rationale and alternatives considered |
| `preference` | Stated or strongly inferred user preferences |
| `task` | Ongoing work, to-dos with context |
| `note` | Observations that don't fit other types |
| `relationship` | Connections between entities (rare, explicit use only) |

## Example: good vs bad memories

**Bad** — too vague, no location, no learning:
```
type: fact
content: fixed the auth bug
```

**Good** — specific, actionable, locatable:
```
type: fact
subject: my-project
content: auth/middleware.ts line 42: token refresh failed when iat was in the past.
  Fixed by adding jose.clockTolerance: 30 option. Tests in test/auth.test.ts.
confidence: 0.92
tags: [auth, jwt, middleware]
```

**Bad** — decision without rationale:
```
type: decision
content: use Redis
```

**Good** — decision with rationale and rejected alternatives:
```
type: decision
subject: my-project
content: Rate limiting uses Redis (not Postgres advisory locks). Advisory locks caused
  ~800ms p99 latency under 500 rps load test. Redis brought it to <10ms.
  Considered: in-memory per-instance (rejected: no cross-replica coordination).
confidence: 0.95
tags: [rate-limiting, redis, architecture]
```

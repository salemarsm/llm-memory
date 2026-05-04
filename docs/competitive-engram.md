# Choosing Engram or llm-memory

Engram is the closest known peer to `llm-memory`: Go, SQLite, FTS5, MCP, HTTP API, CLI/TUI, and a strong focus on coding-agent memory.

This document is not a takedown. Engram is a solid project and may be the better choice for many coding-agent workflows. The goal here is to clarify overlap, learn from good product decisions, and keep `llm-memory` focused on its own niche: canonical, auditable memory with evidence and lifecycle semantics.

## Quick guidance

Choose **Engram** when you mainly want:

- persistent memory for coding agents,
- a mature agent setup story,
- CLI/TUI workflows,
- session summaries and coding observations,
- optional sync/cloud replication paths,
- a single polished tool for day-to-day agent memory.

Choose **llm-memory** when you mainly want:

- a local-first canonical memory database,
- explicit separation between memory, evidence, events, chunks, and embeddings,
- supersession and audit lifecycle as core primitives,
- HTTP/MCP/CLI over the same canonical model,
- personal AI infrastructure beyond coding-agent sessions,
- RAG where documents/chunks are evidence and memories are conclusions.

They can also be complementary: Engram can serve as a coding-agent memory layer, while `llm-memory` can serve as a more canonical long-term memory/evidence store.

## Where they overlap

Both projects care about:

- local-first operation,
- Go-native distribution,
- SQLite as the durable local store,
- FTS5 search,
- MCP integration,
- coding-agent workflows,
- avoiding a mandatory external vector database.

This overlap validates the category. It also means `llm-memory` should avoid positioning itself as just another coding-agent memory tool.

## Product differences

| Dimension | Engram | llm-memory |
| --- | --- | --- |
| Primary lens | Persistent memory for coding agents | Canonical operational memory for agents/personal AI |
| Stored unit | Observations/session memory | Canonical memories, events, documents, chunks |
| Lifecycle emphasis | Save/update/delete, topic evolution, sessions | Supersession, provenance, event log, audit trail |
| Retrieval emphasis | Search/context/timeline for coding work | Token-budgeted context + drill-down to memory/evidence |
| RAG stance | Not the central thesis | Evidence vs conclusion is central |
| Ideal user | Coding-agent power user | Builder of local AI memory infrastructure |

## Practices worth adopting

### 1. Distribution should feel boring

Engram's onboarding promise is simple: one binary, one SQLite file, no runtime dependencies.

Adopt:

- primary binary for normal users,
- GitHub Actions and GoReleaser,
- release archives for Linux/macOS/Windows with checksums,
- `go install` as the transparent technical-user path,
- Homebrew-ready packaging later.

`llm-memory` framing:

> One local canonical memory database. SQLite source of truth. HTTP/MCP/CLI over the same lifecycle model.

### 2. Agent setup is part of the product

Engram treats per-agent setup as first-class: setup commands, docs, compaction survival notes, and project auto-detection.

Adopt:

- `llm-memory setup <agent>` or `llm-memory integrate <agent>` for OpenClaw, Claude Code, Codex-like CLI, and generic MCP,
- manual JSON examples as fallback,
- bootstrap prompts that encode memory policy,
- smoke tests for generated config snippets.

`llm-memory` setup should emphasize transparent canonical flow:

1. retrieve compact context before answering,
2. suggest durable memories after meaningful work,
3. approve/store/supersede with audit trail.

### 3. Retrieval should use progressive disclosure

Engram's compact-search → timeline → full-observation pattern is token-efficient.

Adopt a similar drill-down flow:

```txt
memory_context        -> prompt-ready compact canonical memory
memory_search         -> compact candidates with IDs
memory_get            -> full memory with provenance/supersession
memory_timeline       -> lifecycle/audit trail
memory_evidence       -> supporting document chunks/events
```

Default context should stay compact. Full records and evidence should be fetched only when needed.

### 4. Memory hygiene needs explicit primitives

Engram has pragmatic hygiene features: topic keys, revision counts, duplicate counts, soft delete, dedupe hashes, and project/scope filters.

Adopt/adapt:

- stable `topic_key` for evolving subjects/decisions,
- duplicate detection metadata instead of repeated inserts,
- revision/lifecycle counters for updates and supersession chains,
- visible soft-delete semantics,
- helper for canonical topic-key suggestions.

Keep the `llm-memory` distinction: these should support canonical memory lifecycle, not replace it with a mutable observation log.

### 5. Identity ambiguity should fail loudly

Engram treats project detection as operationally important and refuses to guess when ambiguous.

Adopt:

- project/subject detection helpers for integrations,
- structured ambiguity errors,
- repo-local config such as `.llm-memory/config.json`,
- consolidation tools for similar project/subject names.

For `llm-memory`, this should generalize beyond code repositories: subject identity should be explicit, validated, and auditable.

### 6. Doctor/repair should be real workflows

Engram has visible diagnostic and repair posture.

Adopt:

- read-only `doctor` checks,
- explicit repair commands,
- schema/migration/FTS/config/server/auth/MCP diagnostics,
- no destructive repair without confirmation,
- migration and legacy-schema tests.

### 7. Conflict surfacing belongs in governance

Engram's relation/conflict work is a good direction for memory quality.

Adopt:

- conflict candidates in `memory_suggest`,
- first-class relations for conflict/supersession/reinforcement,
- auditable judge metadata: actor/model, reason, evidence, confidence.

For `llm-memory`, conflict handling should govern canonical memory writes, not just warn during search.

### 8. Sync should remain local-first

Engram frames sync/cloud as opt-in replication while local SQLite remains authoritative.

Adopt later:

- local canonical DB remains primary,
- replication is opt-in and subject/project-scoped,
- export chunks or migration-safe bundles instead of raw DB sync,
- conflict handling before multi-writer sync.

## What llm-memory should avoid

- Becoming only a coding-agent observation tracker.
- Expanding MCP tools faster than the memory lifecycle stabilizes.
- Introducing cloud before local safety/auth/governance are solid.
- Treating raw prompt/session capture as canonical truth. Raw capture can be evidence; memory should be curated conclusion.

## Recommended positioning

> `llm-memory` is a local-first canonical memory database for AI agents: SQLite source of truth, HTTP/MCP interface, auditable lifecycle, token-budgeted context, and evidence-aware RAG bridge.

Useful shorthand:

```txt
Engram      = strong coding-agent memory and session workflow
llm-memory  = canonical operational memory with evidence, audit, supersession, and agent-agnostic retrieval
```

Core differentiators to keep visible:

- canonical memory vs raw observation/session log,
- provenance/evidence links,
- supersession lifecycle,
- append-only events,
- RAG thesis: evidence vs conclusion,
- LLM as client, not database,
- embeddings as replaceable indexes,
- local-first governance before cloud.

## Near-term implementation priorities

1. Packaging/CI: GitHub Actions, GoReleaser, version command.
2. Agent setup UX: one-command setup for key agents plus tested manual snippets.
3. Memory hygiene: `topic_key`, duplicate/revision metadata, better soft-delete docs.
4. Progressive disclosure: detail/timeline/evidence MCP/API flow.
5. Project/subject identity: detection, ambiguity errors, consolidation tools.
6. Doctor/repair: actionable diagnostics and explicit safe repair commands.
7. Conflict governance: relation table and suggestion-time conflict surfacing.

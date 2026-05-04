# Backlog

Prioritized backlog for `llm-memory`.

Legend:

- P0: blocks serious usage
- P1: important near-term value
- P2: quality/ecosystem
- P3: later/optional

## Completed hardening items

These came from early code review feedback and are already implemented after `v0.1.0-alpha.1`:

- UUID IDs instead of timestamp IDs.
- Atomic `Supersede` transaction.
- Normalized `memory_tags` table for exact tag lookup.
- `schema_migrations` bootstrap table.
- `events.memory_id` nullable column.
- MCP scanner buffer raised to 10 MiB.
- HTTP server timeouts.
- `/healthz`.
- `/api/v1` aliases while preserving `/api`.
- Public `/api/config` no longer returns `api_key_env`.
- GitHub Actions CI for tests and command builds.
- GoReleaser release scaffolding.
- Version metadata command/flags with ldflags support.
- Makefile for build/test/check/install/snapshot.


## P0 — Safety, correctness, and release hygiene

### AUTH-001 — API token support

- Add token config to `config.Config`.
- Support `Authorization: Bearer <token>`.
- Allow loopback-only no-auth mode for development.
- Add tests for protected endpoints.
- Update OpenAPI security scheme.

Acceptance:

- Non-loopback bind without auth fails or warns loudly.
- Requests without token are rejected when auth enabled.

### REL-001 — GitHub Actions CI docs sanity

- Existing workflow runs `go test ./...` and builds all commands.
- Add a lightweight docs sanity script/check.

Acceptance:

- README CI badge can be restored truthfully and docs links/snippets are checked.

### REL-002 — Release artifacts publishing

- GoReleaser config exists.
- Validate snapshot release locally/CI.
- Attach release artifacts to GitHub release.

Acceptance:

- User can install without Go toolchain from a tagged release.

### DOC-001 — Mark implemented vs planned across docs

- Audit docs for claims that imply RAG/auth/retrieval are complete.
- Add status markers where needed.

Acceptance:

- Serious reader can tell exactly what exists today.

### SUG-001 — Suggestion engine contradiction check

- Compare candidates against existing memories for same subject/scope/type.
- Return `possible_conflicts`.
- Recommend `remember` vs `supersede` vs `reject`.
- Store conflict/supersession/reinforcement as first-class auditable relations when accepted.

Acceptance:

- Candidate contradicting an existing preference is not presented as a plain new memory.

### MEM-001 — Topic keys, revisions, and dedupe metadata

Adapt Engram's memory-hygiene pattern to canonical memories.

- Add optional stable `topic_key` for evolving subjects/decisions.
- Add duplicate detection metadata instead of repeated inserts.
- Add visible revision/supersession counters or computed lifecycle summary.
- Add helper to suggest canonical topic keys.

Acceptance:

- Repeated saves on the same evolving topic can update/supersede cleanly without polluting search results.

## P1 — Agent UX and installability

### AGENT-001 — One-command agent setup skeleton

- Add `llm-memory setup <agent>` or `llm-memory integrate <agent>`.
- Start with dry-run output for `openclaw`, `claude-code`, `codex`, and `generic-mcp`.
- Include exact manual config snippets.
- Refuse unsafe config writes unless explicitly confirmed.

Acceptance:

- A user can run one command and get either a safe generated config or exact copy/paste instructions.

### AGENT-002 — Project/subject identity detection

- Add helper command/API for current project/subject resolution.
- Support repo-local `.llm-memory/config.json` for canonical project identity.
- Return structured ambiguity errors instead of guessing.
- Add later consolidation command for similar project names/subjects.

Acceptance:

- Agent integrations can verify the target subject/project before writing memory.

### CLI-002 — Better `doctor`

- Detect running memserver.
- Detect bad config JSON.
- Detect no auth on non-loopback bind.
- Suggest exact fixes.

Acceptance:

- Doctor output is actionable, not just diagnostic.

### INT-001 — Claude Code integration doc test

- Verify real config location/format.
- Add exact copy/paste instructions.

Acceptance:

- Fresh user can connect Claude Code from docs.

### INT-002 — OpenClaw integration doc test

- Document MCP/OpenClaw setup.
- Add transparent-memory bootstrap prompt.

Acceptance:

- OpenClaw can call `memory_context` and `memory_suggest` through MCP.

### INT-003 — Codex integration fallback

- Document MCP path if supported.
- Document `memctl context` pre-prompt fallback.

Acceptance:

- Codex-like shell can use memory even without MCP.

### UX-001 — Memory approval queue

- Add pending candidate table or status.
- GUI shows suggested memories.
- User can approve/reject/store/supersede.

Acceptance:

- `memory_suggest` can feed a review flow instead of immediate storage.

## P1 — RAG foundation

### RAG-001 — Chunk search API

- Add `POST /api/chunks/search`.
- Search `chunks_fts`.
- Return document/chunk metadata.

Acceptance:

- Imported or manually inserted chunks can be searched separately from memories.

### RAG-002 — Document/chunk GUI views

- Add document list.
- Add chunk view.
- Link memories to evidence manually or by source ref.

Acceptance:

- User can inspect evidence without sqlite CLI.

### RAG-003 — Docling ingestion design doc

- Define adapter boundary.
- Define supported input formats.
- Define chunking rules.
- Define failure/retry model.

Acceptance:

- Implementation can begin without architecture ambiguity.

## P2 — Retrieval quality and token economy

### RET-001 — Ranking formula

- Combine FTS rank, scope priority, confidence, recency, and type weights.
- Document formula.

Acceptance:

- `/api/context` order is explainable.

### RET-005 — Progressive disclosure API/MCP flow

Adopt a compact-drilldown retrieval pattern.

- Add/get documented flow for compact context, compact search, full memory detail, lifecycle timeline, and evidence chunks.
- Consider MCP tools: `memory_get`, `memory_timeline`, `memory_evidence`.
- Keep default context compact and token-budgeted.

Acceptance:

- Agent can retrieve small context first and drill into only the few memories/evidence items it needs.

### RET-002 — Token budget allocator

- Split budget across preferences, facts/decisions, tasks, and evidence.
- Configurable defaults.

Acceptance:

- Context does not get dominated by one memory category.

### RET-003 — Context cache

- Cache context by query+scope+subject+store version.
- Invalidate on memory writes.

Acceptance:

- Repeated context calls are cheap.

### RET-004 — Benchmarks

- Add synthetic corpus generator.
- Measure search latency at 1k/10k/100k memories.
- Measure context build latency.

Acceptance:

- README can include honest benchmark numbers.

## P2 — Governance

### GOV-001 — Sensitive data detector

- Detect obvious secrets/tokens/passwords.
- Detect sensitive personal data patterns.
- Mark candidates as requiring explicit confirmation.

Acceptance:

- Suggestion engine does not casually propose secrets as memories.

### GOV-002 — Soft delete

- Add deletion mode.
- Preserve audit tombstone unless hard delete requested.

Acceptance:

- User can distinguish privacy deletion from normal supersession.

### GOV-003 — Memory quality score

- Score by source quality, confidence, age, supersession, and conflicts.

Acceptance:

- Low-quality memories can be filtered or reviewed.

## P2 — Developer experience

### DX-001 — Makefile/Taskfile

- `make test`
- `make build`
- `make run`
- `make release-snapshot`

Acceptance:

- Common commands are discoverable.

### DX-002 — CONTRIBUTING.md

- Development setup.
- Testing.
- Commit/release expectations.

Acceptance:

- New contributor knows how to start.

### DX-003 — Issue templates

- Bug report.
- Feature request.
- Integration request.

Acceptance:

- GitHub issues collect useful info.

## P3 — Ecosystem and advanced features

### VEC-001 — Vector adapter interface

- Define embedding/vector index abstraction.
- Keep canonical memory independent.

Acceptance:

- Vector support can be added without schema philosophy drift.

### PKG-001 — Homebrew formula

- Add install path for macOS/Linux users.

Acceptance:

- `brew install ...` path exists or is documented.

### CLOUD-001 — Container image

- Container build with auth required by default.

Acceptance:

- Safe-ish local network/container workflow exists.

## Backlog ordering recommendation

1. AUTH-001
2. REL-001
3. REL-002
4. CLI-001
5. CLI-002
6. SUG-001
7. UX-001
8. RAG-001
9. RAG-003
10. RET-001

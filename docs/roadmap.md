# Development roadmap

Status: **experimental v0.x**. This roadmap is intentionally explicit about what is implemented, planned, and non-goals.

## Product thesis

`llm-memory` should become a local-first canonical memory layer for agents:

```txt
raw events -> evidence -> canonical memory -> token-budgeted context -> LLM client
```

The north star is transparent memory for coding agents and assistants: users should not need to manually call memory tools for memory to work.

## Release principles

- Local-first before cloud.
- SQLite remains the canonical source of truth.
- Embeddings are optional indexes, never canonical truth.
- MCP/HTTP/CLI are integration surfaces over the same memory model.
- Security and token economy are product features, not afterthoughts.
- Be honest about experimental status until v1.

## v0.1 — Local core and positioning

Status: **mostly implemented**.

### Implemented

- SQLite store and migrations
- canonical `memories` table
- append-only `events`
- FTS5 search for memories
- scopes and memory types
- supersession fields
- HTTP API
- local web GUI
- `memctl` CLI
- `memserver`
- `llm-memory` helper CLI
- `memmcp` MCP server
- token-budgeted `/api/context`
- heuristic `/api/suggest`
- document/chunk schema foundation
- OpenAPI draft
- documentation split under `docs/`
- alpha tag: `v0.1.0-alpha.1`
- UUID-based IDs [post-alpha]
- atomic supersession [post-alpha]
- normalized `memory_tags` table [post-alpha]
- `/healthz` endpoint [post-alpha]
- HTTP server timeouts [post-alpha]
- MCP scanner buffer increased for larger payloads [post-alpha]
- `/api/v1` aliases [post-alpha]

### Remaining v0.1 polish

- GitHub Actions CI [post-alpha]
- GoReleaser config for release artifacts on Linux/macOS/Windows [post-alpha]
- version command with ldflags stamping [post-alpha]
- Makefile [post-alpha]
- example screenshots/GIF
- schema documentation generated from migrations/source
- basic benchmark harness

## v0.2 — Local safety and installability

Goal: make the project safe and easy enough for real local agent workflows.

### Must have

- API token support for HTTP API [implemented]
- `llm-memory token create/list/revoke` [implemented]
- config support for auth settings [implemented]
- MCP auth propagation where relevant
- `install.sh` for local binary install
- release builds attached to GitHub releases
- `llm-memory upgrade` or documented upgrade path
- CI on push/PR [post-alpha]
- smoke tests for `memserver`, `memctl`, `memmcp`

### Should have

- improved `doctor` with actionable fixes [started]
- read-only doctor plus explicit repair commands [started]
- port conflict detection with suggestions
- config validation command
- structured logging
- version command for all binaries
- homebrew-ready layout or install docs
- project/subject identity detection helper with ambiguity errors

### Exit criteria

- [ ] `curl -fsSL .../install.sh | bash` completes and `ginko version` works within 5 minutes on a clean machine.
- [ ] `ginko setup claude-code` writes a valid `settings.json` and MCP server responds.
- [ ] `ginko serve` starts without errors and `/healthz` returns 200.
- [ ] HTTP API returns 401 when `auth_token` is set and no Bearer token is provided.
- [ ] CI passes `make check` on every push to main.
- [ ] `ginko doctor` reports no errors on a correctly installed setup.

## v0.3 — Agent integration quality

Goal: make memory feel transparent in Claude Code, Codex-like agents, OpenClaw, and generic MCP clients.

### Must have

- documented Claude Code MCP setup with tested config [started: Ginko setup docs/plugin skeleton]
- documented Codex fallback using `memctl context`
- documented OpenClaw integration pattern
- one-command setup/integration skeleton for major agents, with dry-run mode first [started: `llm-memory setup claude-code` / Ginko]
- MCP contract tests
- bootstrap prompts per agent
- memory write policy examples
- `memory_suggest` examples before/after
- dry-run mode for `memory_remember`
- progressive-disclosure tools/endpoints for compact search, full memory detail, lifecycle timeline, and evidence drill-down

### Should have

- `llm-memory integrate claude-code` writes/patches known config when safe
- `llm-memory integrate codex` writes/patches known config when safe
- `llm-memory integrate openclaw` writes/patches known config when safe
- agent-specific token budgets
- config profiles per subject/project

### Exit criteria

- [ ] `ginko integrate claude-code` configures MCP without manual JSON editing.
- [ ] `memory_remember` with `dry_run=true` returns preview without persisting.
- [ ] MCP contract tests pass for all declared tools.
- [ ] Agent can call `memory_context` → work → `memory_suggest` → `memory_remember` in one session.
- [ ] Session summary is saved on `memory_session_end` and recovered on next `memory_session_start`.
- [ ] `docs/agents/` covers claude-code, codex, and openclaw with working examples.

## v0.4 — Governance and memory quality

Goal: improve trust, auditability, and correctness of stored memory.

### Must have

- contradiction detection against existing memories
- supersession recommendation flow
- first-class auditable relations for conflicts, reinforcement, and supersession
- stable `topic_key` support for evolving decisions/subjects
- duplicate/revision metadata for memory hygiene
- sensitive-data detection pass
- policy config for auto-save vs confirm-save
- memory approval queue in GUI
- soft delete vs hard delete semantics
- audit UI for memory creation/supersession/deletion
- table-first memory health UI with usage counters; no graph/Neo4j dependency
- read-only GUI config/settings tab showing effective server, database, LLM, embedding, auth mode, and version metadata

### Should have

- per-scope write policies
- per-subject retention settings
- safe GUI config editing for selected local settings, with validation and explicit restart/apply guidance
- memory quality scoring
- duplicate/near-duplicate detection
- explanation traces for rejected suggestions
- analytics views: heatmap, supersession timeline, and context/memory Sankey-style flow

### Exit criteria

- [ ] Agent cannot save a memory without it appearing in the audit log.
- [ ] GUI shows a pending approval queue; user can approve, reject, or supersede each candidate.
- [ ] `memory_remember` with a conflicting subject/content triggers a supersession prompt.
- [ ] Superseded memory does not appear in `memory_context` output.
- [ ] Sensitive-data detector rejects memories containing patterns matching secrets (API keys, tokens).
- [ ] `ginko doctor` reports no errors after a full governance workflow.

## v0.5 — RAG bridge with Docling

Goal: ingest documents as evidence and generate memory candidates from them.

### Must have

- `llm-memory ingest <file>` command
- Docling adapter design
- document hashing and dedupe
- chunking strategy with heading/page metadata
- chunk FTS search [implemented baseline]
- document/chunk GUI views with GUI path ingest flow for text-like files plus PDF/DOCX/PPTX/XLSX/images through Docling CLI when installed
- recursive directory ingestion for RAG sources [started: native text-like files + Docling CLI formats]
- traceable ingestion runs with source file hash, source URI/path, parser version, timestamps, status/errors, document ID, and chunk IDs [started: API + GUI lineage]
- memory candidate extraction from chunks [started: heuristic candidates with chunk provenance]
- citation links from memory to evidence [started: source.kind=chunk, source.ref=document_id:chunk_id]

### Should have

- batch directory ingestion
- GUI re-ingest controls showing changed/unchanged documents by hash
- import status and errors table
- re-ingest changed documents
- Markdown/HTML export
- citation-aware `/api/context`

### Exit criteria

- [ ] `ginko ingest document.pdf` produces chunks searchable via `/api/chunks/search`.
- [ ] GUI Browse + Ingest flow works end-to-end for `.md`, `.pdf`, and `.html` files.
- [ ] Every ingestion run is traceable: source path → run ID → document ID → chunk IDs → candidate memory IDs.
- [ ] Re-ingesting an unchanged file is a no-op (hash dedupe).
- [ ] Memory candidates extracted from chunks include `source.kind=chunk` and `source.ref=doc_id:chunk_id`.

## v0.6 — Retrieval quality

Goal: make retrieval high-signal, explainable, local-first, and cheap while preserving canonical memory semantics.

Retrieval must remain layered:

- SQLite remains the canonical source of truth.
- SQLite FTS5/BM25-style ranking remains the lexical baseline.
- Embeddings are optional indexes, not memory.
- RAG chunks are evidence, not canonical memories.
- LLMs may suggest or consume context, but do not own memory state.

### Must have

- lexical retrieval with SQLite FTS5 as the default candidate generator [started post-alpha]
- prioritize exact technical terms: IDs, commands, errors, filenames, endpoints, code symbols, and acronyms [started post-alpha]
- expose lexical/BM25-style score where possible as `lexical_score` [started for `/api/search`]
- optional semantic retrieval with embeddings via local or remote adapters; no embedding provider is required
- preserve `embedding_refs_json` as a bridge while keeping vector stores outside canonical truth
- hybrid ranker combining lexical score, semantic score, confidence, recency, scope, memory type, provenance quality, and lifecycle/status
- explainable ranking metadata: `semantic_score`, `recency_score`, `confidence_score`, `provenance_score`, `final_score`, and `rank_reason` [started for lexical search]
- penalize or exclude `superseded`, `deleted`, `inactive`, and expired memories from current-truth retrieval
- contradiction-aware retrieval that surfaces possible conflicts between active memories instead of hiding ambiguity
- integrate hybrid ranking into `POST /api/context`, `POST /api/v1/context`, and MCP `memory_context`
- keep context compact, ordered by relevance/usefulness, and bounded by `max_tokens`
- retrieval evaluation harness with fixtures for exact technical terms, semantic paraphrase, supersession, conflicts, token limits, and global-vs-project scope [started post-alpha]

### Should have

- `memory_embeddings` auxiliary table
- `chunk_embeddings` auxiliary table
- `retrieval_eval_runs` and `retrieval_eval_items` auxiliary tables
- sqlite-vec/sqlite-vss adapter where available
- external vector index adapter only when explicitly configured
- per-agent retrieval profiles
- ranking debug mode and JSON/Markdown evaluation reports
- context cache keyed by query/scope/subject/store version
- benchmarks for retrieval quality and latency

### Exit criteria

- `/api/context` returns compact, relevant memory under budget with predictable latency.
- lexical retrieval works without embeddings.
- semantic retrieval works when an embedding adapter is configured.
- `/api/search` and `/api/context` can expose ranking metadata without breaking existing clients.
- superseded/deleted/inactive/expired memories are not returned as current truth.
- active-memory conflicts are surfaced in ranking/context metadata.
- evaluation fixtures cover lexical precision, semantic recall, hybrid ranking, context quality, supersession, conflict handling, token budget, and scope priority.

## v0.7 — Packaging and ecosystem

Goal: make distribution and contribution straightforward.

### Must have

- GitHub releases with checksums
- installation docs per OS
- contribution guide
- issue templates
- changelog
- versioned docs

### Should have

- Homebrew tap or formula
- container image for local network use with auth
- examples repo or examples directory expansion

## v1.0 — Stable local-first memory server

Goal: stable API and schema guarantees for local agent memory.

### Required before v1

- authenticated local HTTP API
- stable OpenAPI contract
- migration/versioning strategy
- documented backup/restore
- documented privacy/security model
- stable MCP tool contracts
- tested install path
- contradiction/supersession workflow
- RAG ingestion baseline
- retrieval benchmarks

## Explicit non-goals for now

- hosted SaaS
- replacing vector databases
- replacing full document management systems
- training/fine-tuning models
- storing secrets
- multi-tenant production cloud use before auth/isolation matures

### v0.3 session/project continuity — started

- Added SQLite-backed `sessions(id, project, started_at, ended_at, summary)` model.
- Added session start/end/summary API and MCP tools.
- `memory_context` can include current session and latest closed session summary when `project` is supplied.
- `memmcp` now auto-detects project identity when project/subject are omitted.

### v0.2 privacy primitive — started

- `<private>...</private>` blocks are stripped before memory persistence at store and API/MCP ingress.
- Fully private memories are rejected after redaction by existing content validation.

### v0.3 timeline — started

- Added `GET /api/memories/{id}/timeline`, `memory_timeline`, and `memctl timeline <id>` for lifecycle/audit drill-down.

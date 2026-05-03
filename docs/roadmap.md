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

### Remaining v0.1 polish

- GitHub Actions CI
- release artifacts for Linux/macOS
- Makefile or Taskfile
- example screenshots/GIF
- schema documentation generated from migrations/source
- basic benchmark harness

## v0.2 — Local safety and installability

Goal: make the project safe and easy enough for real local agent workflows.

### Must have

- API token support for HTTP API
- `llm-memory token create/list/revoke`
- config support for auth settings
- MCP auth propagation where relevant
- `install.sh` for local binary install
- release builds attached to GitHub releases
- `llm-memory upgrade` or documented upgrade path
- CI on push/PR
- smoke tests for `memserver`, `memctl`, `memmcp`

### Should have

- improved `doctor` with actionable fixes
- port conflict detection with suggestions
- config validation command
- structured logging
- version command for all binaries
- homebrew-ready layout or install docs

### Exit criteria

- New user can install and run locally in under 5 minutes.
- HTTP API is not unauthenticated by default when exposed beyond loopback.
- CI verifies build/test for every PR.

## v0.3 — Agent integration quality

Goal: make memory feel transparent in Claude Code, Codex-like agents, OpenClaw, and generic MCP clients.

### Must have

- documented Claude Code MCP setup with tested config
- documented Codex fallback using `memctl context`
- documented OpenClaw integration pattern
- MCP contract tests
- bootstrap prompts per agent
- memory write policy examples
- `memory_suggest` examples before/after
- dry-run mode for `memory_remember`

### Should have

- `llm-memory integrate claude-code` writes/patches known config when safe
- `llm-memory integrate codex` writes/patches known config when safe
- `llm-memory integrate openclaw` writes/patches known config when safe
- agent-specific token budgets
- config profiles per subject/project

### Exit criteria

- User can connect at least one MCP-capable agent without hand-editing large JSON blocks.
- Transparent memory flow is documented and reproducible.

## v0.4 — Governance and memory quality

Goal: improve trust, auditability, and correctness of stored memory.

### Must have

- contradiction detection against existing memories
- supersession recommendation flow
- sensitive-data detection pass
- policy config for auto-save vs confirm-save
- memory approval queue in GUI
- soft delete vs hard delete semantics
- audit UI for memory creation/supersession/deletion

### Should have

- per-scope write policies
- per-subject retention settings
- memory quality scoring
- duplicate/near-duplicate detection
- explanation traces for rejected suggestions

### Exit criteria

- Agent can propose memory without silently polluting the store.
- User can review, approve, reject, supersede, and audit memory changes.

## v0.5 — RAG bridge with Docling

Goal: ingest documents as evidence and generate memory candidates from them.

### Must have

- `llm-memory ingest <file>` command
- Docling adapter design
- document hashing and dedupe
- chunking strategy with heading/page metadata
- chunk FTS search
- document/chunk GUI views
- memory candidate extraction from chunks
- citation links from memory to evidence

### Should have

- batch directory ingestion
- import status and errors table
- re-ingest changed documents
- Markdown/HTML export
- citation-aware `/api/context`

### Exit criteria

- User can import a PDF/DOCX/HTML document and get searchable evidence plus candidate memories.

## v0.6 — Retrieval quality

Goal: make context retrieval high-signal and cheap.

### Must have

- ranking formula combining FTS, scope, confidence, recency, and type
- token budget allocator by category
- supersession-aware retrieval
- context cache keyed by query/scope/version
- benchmarks for retrieval quality and latency

### Should have

- optional vector adapter interface
- sqlite-vss/vec or external vector index adapter
- hybrid reranking
- per-agent retrieval profiles

### Exit criteria

- `/api/context` returns compact, relevant memory under budget with predictable latency.

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

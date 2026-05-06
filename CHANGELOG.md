# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [v0.7] — Packaging and ecosystem

### Added
- `Dockerfile`: multi-stage build (Go builder + distroless/static) exposing port 8787, `/data` volume, and `GINKO_DB` env override.
- GoReleaser `dockers` + `docker_manifests`: publishes multi-arch image (`linux/amd64`, `linux/arm64`) to `ghcr.io/salemarsm/ginko` on every tag.
- Release workflow: added Docker Buildx setup and GHCR login; `packages: write` permission.
- `docs/install.md`: OS-specific installation guide covering Linux (curl, manual), macOS (curl, manual, quarantine fix), Windows (PowerShell), Docker, and build-from-source.

---

## [v0.6] — Retrieval quality + agent coordination

### Added

#### Retrieval quality
- **Hybrid ranking in `BuildContext`**: re-sorts candidates by `final_score` DESC (lexical 45% + confidence 30% + provenance 15% + recency 10%) instead of trusting raw BM25 order.
- `ContextResponse` now includes `rankings map[string]RankingMetadata` (keyed by memory ID) and `conflicts []ContextConflict` (structural detection: same subject+type in context window).
- `EmbeddingAdapter` interface: optional semantic retrieval extension point; `Store.SetEmbeddingAdapter` wires it in. System runs fully on lexical retrieval without one.
- **Retrieval eval harness**: `RunEval(label, fixtures)` computes precision@5 and nDCG@10, persists results in `retrieval_eval_runs`/`retrieval_eval_items` tables.

#### Agent coordination signals
- **Agent signals** (`agent_signals` table): durable coordination records shared by all agents on the same database. Six kinds: `notice`, `lease`, `handoff`, `conflict`, `review_request`, `blocker`. Leases require `expires_at`; no infinite locks.
- MCP tools `signal_create`, `signal_list`, `signal_update` for programmatic signal management.
- REST API: `POST /api/signals`, `GET /api/signals`, `GET /api/signals/{id}`, status-transition endpoints (`/acknowledge`, `/resolve`, `/cancel`), and `POST /api/signals/expire` to sweep stale leases.
- `ExpireStaleSignals` store method bulk-expires active signals whose `expires_at` is in the past.
- GUI **Signals** tab with full list (all statuses), inline status transitions, and a create-signal dialog.
- `memory_remember` auto-creates a `conflict` signal in `agent_signals` when `FindConflicts` returns contradicting active memories, closing the cross-cutting coordination milestone exit criterion.

#### LLM-powered document extraction
- `LLMAdapter` interface: `Complete(ctx, system, user) (string, error)` with built-in adapters for Anthropic, OpenAI, and Ollama; `NewLLMAdapter(provider, model, apiKeyEnv)` wires from config.
- `Store.ExtractMemoriesFromDocument`: LLM-backed memory candidate extraction from document chunks; falls back to heuristic extraction when no adapter is configured.
- `POST /api/documents/{id}/extract` endpoint: LLM extraction via HTTP, same request/response shape as `/documents/{id}/suggest`.
- `memmcp` and `memserver` wire the LLM adapter from `config.json` on startup.

#### Packaging and contribution
- `CONTRIBUTING.md`: quick-start guide, project structure, test/lint workflow, PR conventions.
- `.github/ISSUE_TEMPLATE/`: `bug_report.md` and `feature_request.md` templates.
- `.gitignore`: root-level binary exclusions (`/memmcp`, `/memserver`, `/memctl`, `/ginko`, `/ginko-admin`).

### Changed
- Project renamed to **Ginko** (`github.com/salemarsm/ginko`).
- Default data directory is `~/.ginko/`; database is `~/.ginko/ginko.db`.
- Primary CLI is `ginko`; support binaries are `memctl`, `memmcp`, `memserver`, `ginko-admin`.
- `ginko` umbrella CLI with Claude Code plugin (`plugin/claude-code/`): MCP server entry, Memory Protocol skill, SessionStart/PreCompact hooks, `/ginko:save` and `/ginko:recall` slash commands.
- `ginko setup claude-code` and `ginko setup openclaw` for one-command MCP integration.
- Marketplace manifest at `.claude-plugin/marketplace.json`. Install: `/plugin marketplace add salemarsm/ginko`.
- GitHub Actions workflow to compile whitepaper PDF on `.tex` changes.

---

## [v0.5] — RAG bridge

### Added
- `ginko ingest <path>` — standalone ingest command with recursive directory support.
- SHA-256 deduplication: re-ingesting unchanged files is a no-op.
- Re-ingest status endpoint (`GET /api/ingest/status?path=...`): reports `unchanged`, `changed`, or `new`.
- Traceable ingestion runs: `ingestion_runs` table records source path, parser, file count, doc/chunk counts, SHA-256, timestamps, and errors.
- Citation-aware context: memory candidates extracted from chunks carry `source.kind=chunk` and `source.ref=doc_id:chunk_id`.
- GUI Browse + Ingest flow with file browser dialog for path selection.
- GUI ingestion runs view with per-run lineage (source → run → documents → chunks → candidates).

---

## [v0.4] — Governance and memory quality

### Added
- `topic_key` field on `Memory`: auto-supersedes the previous active memory with the same `(subject, scope, topic_key)` triple on write.
- `status` field (`active` / `pending` / `deleted`): memories written by agents with `require_approval` policy land as `pending`.
- Soft delete: `DELETE /api/memories/{id}` sets `status=deleted` rather than destroying the record.
- Sensitive-data detector: rejects memories containing patterns matching API keys, tokens, and credentials.
- Conflict detection: `memory_remember` checks for contradictions with existing active memories and surfaces them.
- Approval queue in GUI: `Pendentes` tab lists pending memories with approve/edit/reject actions.
- `POST /api/memories/{id}/approve` endpoint.
- Supersession timeline view in GUI (`GET /api/analytics/supersessions`).
- GUI config/settings tab with live edit for `server.addr`, `llm.*`, `embedding.*` — writes `~/.ginko/config.json` and restarts server.
- Write policies: per-agent `auto_save` and `require_approval` scope lists in `config.json`.
- Memory quality scoring: confidence, recency, usage, and provenance signals exposed in `/api/usage`.
- Duplicate/near-duplicate detection in `memory_remember`.

---

## [v0.3] — Agent integration quality

### Added
- `memory_get` MCP tool: fetch a full memory record by ID.
- `memory_timeline` MCP tool and `GET /api/memories/{id}/timeline`: lifecycle/audit drill-down per memory.
- `memory_session_start`, `memory_session_end`, `memory_session_summary` MCP tools.
- `sessions` table: project-scoped session start/end/summary with last-closed-session recovery.
- `memory_remember` dry-run mode (`dry_run=true`): returns preview without persisting.
- `ginko integrate claude-code / codex / openclaw` writes/patches agent config when safe.
- Agent profiles in `config.json`: per-agent `max_context_tokens`, `default_subject`, `default_scope`, `write_policy`.
- MCP contract tests.
- `<private>…</private>` block stripping before persistence.
- `docs/agents/` coverage for codex, openclaw, write-policy, and memory-suggest examples.

### Fixed
- `memory_search` no longer force-injects project subject.

---

## [v0.2] — Local safety and installability

### Added
- Bearer token auth for HTTP API (`server.auth_token` / `server.auth_token_env`).
- `ginko token create | list | revoke` subcommands.
- `install.sh` — one-command binary install from GitHub releases.
- `ginko upgrade` command: checks latest GitHub release and prints upgrade instructions.
- Structured logging in `memserver` (slog text/JSON).
- Smoke tests for `memserver`, `memctl`, and `memmcp`.
- CI workflow (`ci.yml`) running `make check` on push/PR.
- GoReleaser workflow (`release.yml`) building Linux/macOS/Windows artifacts on tag.
- `ginko doctor` with actionable fix hints: checks home dir, config, database, auth policy, sibling binaries, port, and Claude Code setup.
- Port conflict detection in doctor with suggested fix.
- Schema version reporting in doctor.

---

## [v0.1] — Local core

### Added
- SQLite store with WAL journaling; UUID-prefixed IDs (`mem_`, `evt_`, `doc_`, `chk_`, `ctx_`, `cfb_`).
- `memories` canonical table with FTS5 virtual table (`memories_fts`), normalized `memory_tags`, and full supersession fields.
- Append-only `events` audit log.
- Document/chunk schema with `documents`, `chunks`, `chunks_fts`.
- `context_feedback` table for retrieval feedback.
- `schema_migrations` versioning.
- HTTP API under `/api/*` and `/api/v1/*` with OpenAPI draft.
- `/healthz` endpoint; HTTP server timeouts (read/write/idle/header).
- Token-budgeted `/api/context` (BM25 candidate generation, subject fallback, confidence threshold, greedy budget packing).
- `/api/suggest` heuristic suggestion engine.
- Local web GUI embedded in `memserver`.
- `memmcp` MCP server (JSON-RPC over stdio, protocol 2024-11-05).
- Initial MCP tools: `memory_context`, `memory_suggest`, `memory_remember`, `memory_search`.
- `memctl` CLI client.
- `ginko-admin` helper CLI: `init`, `doctor`, `token`, `setup`, `paths`, `mcp-config`.
- Retrieval scoring baseline: `lexical_score`, `semantic_score`, `recency_score`, `confidence_score`, `provenance_score`, `final_score`, `rank_reason`.
- MCP scanner buffer increased to 10 MiB.
- Atomic supersession.
- `/api/feedback` endpoint and `context.feedback` events.
- `version` command with `ldflags` stamping (`Version`, `Commit`, `Date`).
- Makefile with `build`, `install`, `check`, `test`, `lint` targets.
- E2E test covering full memory lifecycle.

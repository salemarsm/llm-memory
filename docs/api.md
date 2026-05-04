# HTTP API

See [openapi.yaml](openapi.yaml) for machine-readable API documentation.

Core endpoints are available under both `/api/...` and `/api/v1/...` during v0.x. Prefer `/api/v1/...` for new integrations.

Auth:

- Loopback-only servers may run without auth for local development.
- When `server.auth_token` or `server.auth_token_env` is configured, `/api` and `/api/v1` endpoints require `Authorization: Bearer <token>`.
- Non-loopback binds require an auth token at config validation time.
- `/healthz` remains public.

Core endpoints:

- `POST /api/context`
- `POST /api/feedback`
- `POST /api/suggest`
- `POST /api/memories`
- `POST /api/search`
- `GET /api/usage`
- `POST /api/supersede/{id}`
- `DELETE /api/memories/{id}`
- `GET /api/events`
- `GET /api/config`
- `POST /api/ingest`
- `GET /api/documents`
- `GET /api/ingestion-runs`
- `POST /api/documents/{id}/suggest`
- `POST /api/chunks/search`
- `GET /healthz`


`POST /api/context` returns a `context_id`. Agents can later call `POST /api/feedback` with `{ "context_id": "...", "useful": true|false, "memory_ids_used": [...] }` to create retrieval-quality signals. Feedback is stored as `context.feedback` events for future evaluation/tuning.

`GET /api/usage` returns memories plus usage counters derived from `context.built` and `context.feedback` events. The embedded GUI uses this for the dense table view and zombie/hot memory indicators.

Memory suggestion details: [Suggestion engine](suggestion-engine.md).

`POST /api/ingest` accepts `{ "path": "...", "recursive": true }` and ingests supported text-like files into the document/chunk RAG tables. Directories can be processed recursively. Each run emits a `document.ingested` event with source path, run id, document count, chunk count, and skipped files. PDF/DOCX/PPTX/XLSX/image ingestion uses the `docling` CLI when available in `PATH`; if Docling is missing, those files are skipped with a clear provenance/error entry in the run response.

`POST /api/chunks/search` accepts `{ "text": "...", "document_id": "optional", "limit": 20 }` and returns chunk results with document provenance and BM25 score. Use this as the evidence search surface; canonical memories remain separate.

`GET /api/ingestion-runs` returns recent ingestion runs with parser/version, source path, recursive flag, status, counts, timestamps, and error details. Documents include `ingestion_run_id`, so evidence can be traced from run → document → chunks.

`POST /api/documents/{id}/suggest` extracts memory candidates from the document chunks and preserves evidence provenance as `source.kind=chunk` and `source.ref=<document_id>:<chunk_id>`. Pass `{ "store": true }` to write the candidates immediately; default is review-only.

## Retrieval ranking metadata

Search and context endpoints should remain backwards compatible while allowing clients to opt into ranking details.

Planned endpoints:

- `POST /api/search`
- `POST /api/v1/search`
- `POST /api/context`
- `POST /api/v1/context`

Request flag:

```json
{ "include_ranking": true }
```

Recommended metadata shape:

```json
{
  "ranking": {
    "lexical_score": 0.82,
    "semantic_score": 0.76,
    "recency_score": 0.44,
    "confidence_score": 0.95,
    "provenance_score": 0.90,
    "final_score": 0.87,
    "rank_reason": "exact endpoint match + high confidence + project scope"
  }
}
```

Compatibility rules:

- Existing clients should keep receiving the current memory/context shapes unless they opt into ranking metadata.
- Ranking metadata is derived retrieval state, not canonical memory.
- `lexical_score` is available today for FTS5/BM25-backed memory search when `include_ranking` is true.
- `semantic_score` is planned and will be present only when an embedding adapter is configured and used.
- Superseded, deleted, inactive, or expired memories must not appear as current truth in default context responses.
- If active memories conflict, the API should surface ambiguity through metadata instead of silently choosing a winner.

## Sessions (v0.3 preview)

Sessions add narrative continuity for coding agents. They are project-scoped and stored in SQLite as canonical operational metadata, not chat history.

- `POST /api/sessions/start` with `{ "project": "my-project" }` starts or returns the active session.
- `POST /api/sessions/end` with `{ "project": "my-project", "summary": "..." }` closes the active session.
- `POST /api/sessions/summary` with `{ "project": "my-project" }` returns the active session or latest closed session.
- `POST /api/context` accepts optional `project`; when present it auto-starts an active session and includes the latest closed session summary within the token budget.

MCP exposes the same primitives as `memory_session_start`, `memory_session_end`, and `memory_session_summary`. In `memmcp`, omitted project/subject fields default to auto-detected project identity from `.llm-memory/config.json`, git remote, or directory basename.

## Privacy tags

All write paths strip content wrapped in `<private>...</private>` before persistence. This is enforced at the store layer and also at API/MCP ingress for memory writes and supersession.

Example:

```json
{
  "content": "Use the staging DB. <private>actual password is ...</private> Never commit credentials."
}
```

The stored canonical memory becomes:

```txt
Use the staging DB. Never commit credentials.
```

If stripping private blocks leaves empty content, the memory write is rejected by normal validation. Private blocks are not a substitute for secret storage; they are a redaction primitive for agents before canonical memory is saved.

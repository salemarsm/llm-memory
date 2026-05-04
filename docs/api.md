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
- `POST /api/chunks/search`
- `GET /healthz`


`POST /api/context` returns a `context_id`. Agents can later call `POST /api/feedback` with `{ "context_id": "...", "useful": true|false, "memory_ids_used": [...] }` to create retrieval-quality signals. Feedback is stored as `context.feedback` events for future evaluation/tuning.

`GET /api/usage` returns memories plus usage counters derived from `context.built` and `context.feedback` events. The embedded GUI uses this for the dense table view and zombie/hot memory indicators.

Memory suggestion details: [Suggestion engine](suggestion-engine.md).

`POST /api/ingest` accepts `{ "path": "...", "recursive": true }` and ingests supported text-like files into the document/chunk RAG tables. Directories can be processed recursively. Each run emits a `document.ingested` event with source path, run id, document count, chunk count, and skipped files. PDF/DOCX/PPTX/XLSX/image ingestion uses the `docling` CLI when available in `PATH`; if Docling is missing, those files are skipped with a clear provenance/error entry in the run response.

`POST /api/chunks/search` accepts `{ "text": "...", "document_id": "optional", "limit": 20 }` and returns chunk results with document provenance and BM25 score. Use this as the evidence search surface; canonical memories remain separate.

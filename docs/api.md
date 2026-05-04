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
- `GET /healthz`


`POST /api/context` returns a `context_id`. Agents can later call `POST /api/feedback` with `{ "context_id": "...", "useful": true|false, "memory_ids_used": [...] }` to create retrieval-quality signals. Feedback is stored as `context.feedback` events for future evaluation/tuning.

`GET /api/usage` returns memories plus usage counters derived from `context.built` and `context.feedback` events. The embedded GUI uses this for the dense table view and zombie/hot memory indicators.

Memory suggestion details: [Suggestion engine](suggestion-engine.md).

# HTTP API

See [openapi.yaml](openapi.yaml) for machine-readable API documentation.

Core endpoints are available under both `/api/...` and `/api/v1/...` during v0.x. Prefer `/api/v1/...` for new integrations.

Core endpoints:

- `POST /api/context`
- `POST /api/feedback`
- `POST /api/suggest`
- `POST /api/memories`
- `POST /api/search`
- `POST /api/supersede/{id}`
- `DELETE /api/memories/{id}`
- `GET /api/events`
- `GET /api/config`
- `GET /healthz`


`POST /api/context` returns a `context_id`. Agents can later call `POST /api/feedback` with `{ "context_id": "...", "useful": true|false, "memory_ids_used": [...] }` to create retrieval-quality signals. Feedback is stored as `context.feedback` events for future evaluation/tuning.

Memory suggestion details: [Suggestion engine](suggestion-engine.md).

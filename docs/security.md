# Security model

Current default assumption:

- local-only usage
- bind to `127.0.0.1`
- no secrets in memory content
- no production multi-user isolation yet

HTTP API token support is implemented for `/api` and `/api/v1` endpoints. `GET /healthz` remains public for local health checks.

## HTTP API auth

Loopback-only binds such as `127.0.0.1:8787`, `localhost:8787`, and `[::1]:8787` may run without auth for local development. Non-loopback binds such as `0.0.0.0:8787` fail config validation unless a bearer token is configured.

Prefer an environment variable instead of putting secrets in config:

```json
{
  "server": {
    "addr": "0.0.0.0:8787",
    "auth_token_env": "LLM_MEMORY_API_TOKEN"
  }
}
```

Clients call protected endpoints with:

```http
Authorization: Bearer <token>
```

This is still not production multi-user isolation; use it as single-user local/VPN/container protection.

## Status

Current security posture is suitable for local-first single-user usage. Avoid exposing the server directly to untrusted networks without TLS and additional network controls.

## Memory write policy

Agents should not store everything.

Store:

- explicit user preferences
- stable project facts
- architectural decisions
- corrections
- durable constraints
- long-lived tasks
- approved learnings

Do not store:

- transient chat context
- secrets or credentials
- sensitive personal data without explicit approval
- raw document chunks as memories
- uncertain inference as fact
- private data in shared/group contexts

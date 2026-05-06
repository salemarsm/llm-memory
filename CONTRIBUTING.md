# Contributing to Ginko

Ginko is experimental (v0.x). Contributions are welcome — bug reports, fixes, docs improvements, and new features aligned with the roadmap.

## Quick start

```bash
git clone https://github.com/salemarsm/ginko
cd ginko
make build        # builds all binaries to ./bin/
make check        # runs tests, vet, and staticcheck
```

Requirements: Go 1.22+. No CGO, no external services needed to run tests.

## Project structure

```
cmd/
  ginko/          umbrella CLI (delegates to memserver, memmcp, ginko-admin)
  ginko-admin/    init, doctor, token, setup, paths
  memserver/      HTTP API + embedded GUI
  memmcp/         MCP server (stdio, JSON-RPC)
  memctl/         CLI client for HTTP API
memory/           core data model, store, retrieval, session, RAG
config/           config loading, validation, default paths
internal/version/ version stamping
server/           HTTP handlers and embedded UI
plugin/           Claude Code plugin manifest
docs/             architecture, API, agent guides, roadmap, whitepaper
```

## Workflow

1. Open an issue before starting non-trivial work — describe the problem and rough approach.
2. Branch from `main`: `git checkout -b feat/my-feature`.
3. Keep changes focused; one concern per PR.
4. Run `make check` before pushing.
5. Write or update tests for any behavioral change.
6. Update `CHANGELOG.md` under `[Unreleased]`.
7. Open a PR with a clear description of what and why.

## Design principles (must-reads before coding)

- **SQLite is the source of truth.** All other representations (FTS5 indexes, embedding refs) are derived.
- **Embeddings are optional indexes, not canonical memory.** Store refs, never vectors, in the main schema.
- **The LLM is a client, not a database.** Durable writes are deliberate; `memory_suggest` proposes, `memory_remember` confirms.
- **Auditability is a feature.** Every meaningful operation appends an event; supersession is non-destructive.
- **Local-first.** The SQLite file is the user's. No mandatory external services.

These principles are in `docs/architecture.md` with rationale.

## Coding conventions

- Standard `gofmt` / `goimports` formatting.
- Errors returned, not panicked (except in tests with `t.Fatal`).
- No CGO. The project uses `modernc.org/sqlite` for portability.
- IDs are UUID strings prefixed by entity type: `mem_`, `evt_`, `doc_`, `chk_`, `ctx_`, `cfb_`.
- All timestamps stored as RFC3339Nano strings.
- New SQL migrations go at the end of the migration list in `memory/store.go`; they must be idempotent.

## Testing

```bash
make test          # go test ./...
make check         # test + vet + staticcheck
```

Tests are invariant-focused: they test observable behavior, not implementation details. Representative patterns:

- BM25-ranked preferences win over recent weakly-related tasks.
- Subject-scope fallback recovers memories when FTS misses.
- Low-confidence memories never enter automatic context.
- Technical short tokens (`AI`, `k8s`) survive query normalization.
- MCP contract tests in `cmd/memmcp/` verify tool schemas.

Avoid mocking the SQLite store in new tests — use an in-memory database (`memory.Open(":memory:")`) instead.

## Commit messages

Follow conventional commits loosely:

```
feat: add memory_get MCP tool
fix: subject fallback missing when scope is session
docs: update retrieval pipeline description
chore: bump modernc.org/sqlite to v1.35
```

The type prefix (`feat`, `fix`, `docs`, `chore`, `ci`, `test`) helps GoReleaser filter the changelog.

## Reporting bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md). Include:
- `ginko version` output
- Steps to reproduce
- Expected vs actual behavior
- Relevant log output (`memserver` logs with `-log-format json` if possible)

## Suggesting features

Use the [feature request template](.github/ISSUE_TEMPLATE/feature_request.md). Check the [roadmap](docs/roadmap.md) first — if it's already planned, a +1 comment on the relevant issue helps prioritize.

## License

By contributing you agree your changes are licensed under the [MIT License](LICENSE).

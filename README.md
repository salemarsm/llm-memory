# llm-memory

<p align="center">
  <strong>Persistent memory for Claude Code and coding agents. Single Go binary. SQLite. MCP-native.</strong>
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/salemarsm/llm-memory"><img alt="Go Reference" src="https://img.shields.io/badge/go-reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="https://github.com/salemarsm/llm-memory/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/salemarsm/llm-memory?style=for-the-badge"></a>
  <img alt="SQLite" src="https://img.shields.io/badge/sqlite-source%20of%20truth-003B57?style=for-the-badge&logo=sqlite&logoColor=white">
  <img alt="Experimental" src="https://img.shields.io/badge/status-experimental-orange?style=for-the-badge">
</p>

<p align="center">
  <a href="#ginko">Ginko</a> · <a href="#english">English</a> · <a href="#português">Português</a> · <a href="docs/quickstart.md">Quickstart</a> · <a href="docs/openapi.yaml">OpenAPI</a> · <a href="docs/whitepaper/llm-memory-whitepaper.pdf">White paper</a> · <a href="docs/competitive-engram.md">Engram comparison</a>
</p>

---

<a id="ginko"></a>

## Ginko — install for your Claude Code agent

> Persistent memory for your Claude Code agent. Like ginkgo biloba — but it actually works.

> **Note:** Ginko is unrelated to the [Ginkgo](https://onsi.github.io/ginkgo/) Go testing framework.

```bash
go install github.com/salemarsm/llm-memory/cmd/ginko@latest
ginko setup claude-code
```

Or via the Claude Code plugin marketplace (recommended):

```text
/plugin marketplace add salemarsm/llm-memory
/plugin install ginko
```

That's it. Restart Claude Code and the memory tools are available to the agent.

The plugin includes:

- MCP server (`ginko mcp`)
- Memory Protocol skill — teaches the agent when and how to remember
- SessionStart hook — recovers context from previous sessions automatically
- PreCompact hook — auto-checkpoints memory before context compaction
- Slash commands `/ginko:save` and `/ginko:recall`

For the design rationale, schema, retrieval pipeline, and competitive analysis, see the **[white paper](docs/whitepaper/llm-memory-whitepaper.pdf)** and the project documentation below. The codebase, white paper, and academic identity remain `llm-memory`; `ginko` is the distribution.

---

<a id="english"></a>

## English

`llm-memory` is a durable memory layer for AI agents, coding assistants, RAG systems, and local LLM workflows.

It solves a common problem: agents often confuse chat history, vector search, document chunks, and durable memory.

`llm-memory` keeps them separate.

- **Memories** are compact, structured, auditable records.
- **Documents and chunks** are evidence.
- **Embeddings** are optional indexes.
- **LLMs** are clients, not databases.
- **SQLite** is the canonical source of truth.

> **Vector search is not memory. Chat history is not memory. The LLM is not the database.**

## Status

**Experimental v0.x.** The local core, HTTP API, CLI, MCP server, token-budgeted context, and heuristic suggestion flow are implemented. APIs and schemas may change before v1.0.

Implemented today:

- SQLite canonical memory store
- append-only events
- FTS5 memory search
- HTTP API and local GUI
- `memctl`, `memmcp`, and `llm-memory` CLIs
- MCP tools: `memory_context`, `memory_suggest`, `memory_remember`, `memory_search`
- heuristic memory suggestions
- document/chunk schema foundation for future RAG ingestion
- HTTP API bearer-token auth for non-loopback binds

Planned / not production-ready yet:

- Docling ingestion command
- hybrid vector ranking
- full contradiction resolution policy
- production multi-user isolation
- stable v1 API guarantees

## Technical deep dive

For readers who want the full model and rationale, see the white paper:

- [llm-memory white paper PDF](docs/whitepaper/llm-memory-whitepaper.pdf)
- [LaTeX source](docs/whitepaper/llm-memory-whitepaper.tex)

It covers the canonical-memory model, SQLite schema philosophy, retrieval/context pipeline, RAG boundaries, governance/auditability, and why embeddings remain optional indexes instead of the source of truth.

## The problem

Most agent memory systems confuse three different things:

1. raw conversation history
2. retrieved document chunks
3. canonical durable memory

Vector databases are useful indexes, but they are poor sources of truth.

Prompts are not databases. LLMs are not memory owners. Chat logs are not durable knowledge.

`llm-memory` separates these layers:

```txt
raw events  -> audit trail
documents   -> evidence
chunks      -> retrievable evidence
memories    -> canonical conclusions
context     -> compact prompt-ready projection
LLM         -> client
SQLite      -> source of truth
```

## Why not just...?

| Approach | Good for | Weakness |
|---|---|---|
| Markdown files | Human-readable project knowledge | Weak querying, no audit model, hard to automate safely |
| Vector DB only | Semantic search over large corpora | Not canonical, hard to audit, embeddings drift |
| Chat history | Short-term continuity | No durable schema, noisy, context-window bound |
| LangChain-style memory | App-level convenience | Often runtime-coupled and prompt-format dependent |
| Plain SQLite | Durable local storage | You still need an agent memory model and retrieval policy |
| `llm-memory` | Canonical durable memory for agents | Needs integration with your agent/runtime |

## What you get

- canonical memory records in SQLite
- append-only event log
- SQLite FTS5 search without requiring embeddings
- optional embedding references without making vectors canonical
- supersession for replacing stale memories without destroying history
- scopes: `global`, `project`, `session`, `private`
- types: `preference`, `fact`, `decision`, `task`, `note`, `relationship`
- HTTP API
- embedded local web GUI
- CLI tools
- MCP server for transparent agent integration
- token-budgeted context endpoint
- memory suggestion flow
- RAG document/chunk foundation
- no CGO via `modernc.org/sqlite`

## Positioning vs similar projects

Projects like Mem0, Zep, LangMem, sqlite-memory, and Engram validate the category. `llm-memory` should not compete as a generic "memory layer" clone.

The sharper niche is:

> **Canonical local memory for coding agents and personal AI infrastructure.**

See [Choosing Engram or llm-memory](docs/competitive-engram.md) for the closest direct comparison and the implementation practices we should borrow without losing this project's canonical-memory focus.

## 5-minute quickstart

```bash
git clone https://github.com/salemarsm/llm-memory.git
cd llm-memory

make build

bin/llm-memory init
bin/llm-memory doctor
bin/llm-memory ui
```

Open:

```txt
http://127.0.0.1:8787
```

Store your first memory:

```bash
echo "The user prefers direct technical answers." \
  | bin/memctl -subject botmaster -scope global -type preference remember
```

Retrieve compact prompt context:

```bash
bin/memctl -subject botmaster -scope global -max-tokens 400 context "How should I answer?"
```

Suggest durable learnings from text:

```bash
bin/memctl -subject botmaster suggest "I prefer direct answers and Go examples."
```

More: [Quickstart](docs/quickstart.md), [CLI](docs/cli.md), [MCP](docs/mcp.md).

## Operational flow

```txt
User prompt
  ↓
Agent calls /api/context or MCP memory_context
  ↓
llm-memory retrieves compact relevant memories
  ↓
Agent answers normally
  ↓
Agent calls /api/suggest or MCP memory_suggest
  ↓
Human/policy approves durable candidates
  ↓
Memory is stored or superseded
  ↓
Event log records what happened
```

## RAG is evidence. Memory is conclusion.

Documents and chunks answer:

> Where did this come from?

Canonical memories answer:

> What durable thing should the agent remember?

Example:

```txt
Document chunk:
"In meeting notes, the team decided to use SQLite for local-first storage."

Canonical memory:
"The llm-memory project uses SQLite as the canonical local-first store."

Evidence:
doc_id=..., chunk_id=...
```

See [RAG pipeline](docs/rag-pipeline.md). **Docling ingestion is planned; document/chunk tables are implemented as the foundation.**

## Memory types

| Type | Use for | Example |
|---|---|---|
| `preference` | stable user/project preferences | `User prefers Go examples over Python examples.` |
| `fact` | durable factual statements | `The project uses SQLite as the canonical store.` |
| `decision` | architecture/project decisions | `Embeddings are optional indexes, not canonical truth.` |
| `task` | long-lived pending actions | `Add API token support.` |
| `note` | low-structure observations | `The GUI is currently local-only.` |
| `relationship` | links between entities | `Project X belongs to client Y.` |

Full model: [Memory model](docs/memory-model.md).

## Memory write policy

Agents should **not** store everything.

Store:

- explicit user preferences
- stable project facts
- architectural decisions
- corrections
- durable constraints
- long-lived tasks
- approved learnings inferred from repeated or explicit behavior

Do not store:

- transient chat context
- secrets or credentials
- sensitive personal data without explicit approval
- raw document chunks as memories
- uncertain inference as fact
- private data in shared/group contexts

More: [Security](docs/security.md), [MCP transparent usage](docs/mcp.md), [Suggestion engine](docs/suggestion-engine.md).

## HTTP API

The API is documented in [docs/openapi.yaml](docs/openapi.yaml).

Core endpoints are available under `/api/...` and `/api/v1/...`; new integrations should prefer `/api/v1/...`.

Core endpoints:

| Endpoint | Purpose |
|---|---|
| `POST /api/context` | compact token-budgeted prompt context |
| `POST /api/suggest` | suggest durable memories/learnings |
| `POST /api/memories` | create/update memory |
| `POST /api/search` | raw memory search |
| `POST /api/supersede/{id}` | replace stale memory |
| `DELETE /api/memories/{id}` | forget memory |
| `GET /api/events` | audit events |
| `GET /healthz` | local health check |

Example response from `POST /api/context`:

```json
{
  "context": "- [preference/global conf=0.95 src=conversation:msg-123] User prefers direct technical answers.",
  "items": [
    {
      "id": "mem_01J...",
      "type": "preference",
      "subject": "botmaster",
      "content": "User prefers direct technical answers.",
      "scope": "global",
      "confidence": 0.95
    }
  ],
  "estimated_tokens": 31,
  "budget_tokens": 400,
  "truncated": false
}
```

## Real SQLite schema excerpt

The canonical memory table is intentionally boring and inspectable:

```sql
CREATE TABLE memories (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL,
  subject TEXT NOT NULL,
  content TEXT NOT NULL,
  source_kind TEXT NOT NULL,
  source_ref TEXT NOT NULL,
  scope TEXT NOT NULL,
  confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  valid_from TEXT,
  valid_until TEXT,
  supersedes_id TEXT,
  superseded_by TEXT,
  tags_json TEXT NOT NULL DEFAULT '[]',
  embedding_refs_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE memory_tags (
  memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
  tag TEXT NOT NULL,
  PRIMARY KEY(memory_id, tag)
);

CREATE VIRTUAL TABLE memories_fts
USING fts5(id UNINDEXED, content, subject, tags);
```

Full schema notes: [Memory model](docs/memory-model.md).

## Transparent MCP integration

`memmcp` exposes memory as MCP tools:

- `memory_context`
- `memory_suggest`
- `memory_remember`
- `memory_search`

The target UX is invisible memory:

1. user asks normally
2. agent silently calls `memory_context`
3. agent answers normally
4. agent calls `memory_suggest`
5. safe/approved candidates are stored with `memory_remember`

Generate MCP snippets:

```bash
bin/llm-memory install-mcp claude-code
bin/llm-memory install-mcp codex
bin/llm-memory install-mcp openclaw
```

Details: [MCP integration](docs/mcp.md).

## Architecture

```txt
┌──────────────────────────┐
│ OpenClaw / Claude / Codex │
└────────────┬─────────────┘
             │ MCP / HTTP / CLI
┌────────────▼─────────────┐
│      llm-memory API       │
├──────────────────────────┤
│ Context builder           │
│ Suggestion engine         │
│ Write policy              │
├──────────────────────────┤
│ Canonical memories        │
│ Append-only events        │
│ Documents / chunks        │
├──────────────────────────┤
│ SQLite + FTS5             │
│ Optional vector indexes   │
└──────────────────────────┘
```

## Security model

Current default assumption:

- local-only usage
- bind to `127.0.0.1`
- no public network exposure
- no secrets in memory content
- no production multi-user isolation yet

Do **not** expose the HTTP server to untrusted networks until API token/local auth is implemented.

See [Security](docs/security.md).

## Use cases

### Coding agent memory

Remember project decisions, coding style, architecture, local commands, and constraints.

### Personal AI assistant

Remember stable preferences and recurring workflows without pinning them to one model vendor.

### RAG + canonical memory

Convert messy documents into evidence and durable conclusions.

### Research lab / security workflows

Keep project-specific knowledge compartmentalized per tool, target, client, or engagement.

## Roadmap

### v0.1 — Local core

- SQLite store
- migrations
- FTS5 search
- event log
- HTTP API
- embedded GUI
- CLI

### v0.2 — Agent integration and local safety

- MCP tools
- token-budgeted context
- memory suggestion
- CLI integration
- OpenClaw / Claude Code / Codex examples
- local API token

### v0.3 — Governance

- memory write policy enforcement
- sensitive-data guardrails
- soft delete vs hard delete
- audit UI

### v0.4 — RAG bridge

- document import
- Docling ingestion
- chunk evidence
- memory candidate extraction
- citation linking

### v0.5 — Retrieval quality

- hybrid ranking
- confidence scoring
- recency weighting
- supersession-aware retrieval
- optional vector adapters

## Documentation

- [Quickstart](docs/quickstart.md)
- [Concepts](docs/concepts.md)
- [Architecture](docs/architecture.md)
- [Memory model](docs/memory-model.md)
- [HTTP API](docs/api.md)
- [OpenAPI](docs/openapi.yaml)
- [White paper](docs/whitepaper/llm-memory-whitepaper.pdf)
- [White paper source](docs/whitepaper/llm-memory-whitepaper.tex)
- [CLI](docs/cli.md)
- [MCP](docs/mcp.md)
- [Suggestion engine](docs/suggestion-engine.md)
- [RAG pipeline](docs/rag-pipeline.md)
- [Security](docs/security.md)
- [Roadmap](docs/roadmap.md)
- [Backlog](docs/backlog.md)

## License

MIT.

---

<a id="português"></a>

## Português

`llm-memory` é uma camada local-first de memória canônica para agentes de IA, coding assistants, sistemas RAG e fluxos com LLM local/remota.

Ele resolve um problema comum: agentes confundem histórico de chat, busca vetorial, chunks de documentos e memória durável.

`llm-memory` separa essas camadas:

- **Memórias** são registros compactos, estruturados e auditáveis.
- **Documentos e chunks** são evidência.
- **Embeddings** são índices opcionais.
- **LLMs** são clientes, não bancos de dados.
- **SQLite** é a fonte canônica da verdade.

> **Busca vetorial não é memória. Histórico de chat não é memória. A LLM não é o banco de dados.**

## Aprofundamento técnico

Para leitores que quiserem a fundamentação completa, leia o white paper:

- [PDF do white paper](docs/whitepaper/llm-memory-whitepaper.pdf)
- [Fonte LaTeX](docs/whitepaper/llm-memory-whitepaper.tex)

Ele detalha o modelo de memória canônica, a filosofia do schema SQLite, o pipeline de retrieval/contexto, os limites entre RAG e memória, governança/auditoria e por que embeddings são índices opcionais, não a fonte da verdade.

## O problema

A maioria dos sistemas de memória para agentes mistura três coisas diferentes:

1. histórico bruto de conversa
2. chunks de documentos recuperados
3. memória canônica e durável

Vector DBs são índices úteis, mas são fontes ruins da verdade.

Prompts não são bancos de dados. LLMs não são donas da memória. Logs de chat não são conhecimento durável.

`llm-memory` separa:

```txt
eventos brutos -> trilha de auditoria
documentos     -> evidência
chunks         -> evidência recuperável
memórias       -> conclusões canônicas
contexto       -> projeção compacta pronta para prompt
LLM            -> cliente
SQLite         -> fonte da verdade
```

## Por que não só...?

| Abordagem | Boa para | Fraqueza |
|---|---|---|
| Arquivos Markdown | conhecimento legível por humanos | consulta fraca, pouca auditoria, automação difícil |
| Só vector DB | busca semântica em corpus grande | não é canônico, difícil auditar, embeddings mudam |
| Histórico de chat | continuidade curta | sem schema durável, ruidoso, preso à janela de contexto |
| Memória estilo LangChain | conveniência em app | frequentemente acoplada ao runtime/prompt |
| SQLite puro | storage local durável | ainda falta modelo de memória e política de retrieval |
| `llm-memory` | memória canônica durável para agentes | precisa integrar com o agente/runtime |

## Começo rápido

```bash
git clone https://github.com/salemarsm/llm-memory.git
cd llm-memory

make build

bin/llm-memory init
bin/llm-memory doctor
bin/llm-memory ui
```

Abra:

```txt
http://127.0.0.1:8787
```

Guardar primeira memória:

```bash
echo "O usuário prefere respostas diretas e técnicas." \
  | bin/memctl -subject botmaster -scope global -type preference remember
```

Recuperar contexto compacto:

```bash
bin/memctl -subject botmaster -scope global -max-tokens 400 context "Como devo responder?"
```

## RAG é evidência. Memória é conclusão.

Documentos e chunks respondem:

> De onde isso veio?

Memórias canônicas respondem:

> O que o agente deve lembrar de forma durável?

Exemplo:

```txt
Chunk documental:
"Nas notas da reunião, o time decidiu usar SQLite para storage local-first."

Memória canônica:
"O projeto llm-memory usa SQLite como store canônico local-first."
```

## Política de escrita

Agentes não devem salvar tudo.

Salvar:

- preferências explícitas
- fatos estáveis do projeto
- decisões arquiteturais
- correções
- constraints duráveis
- tarefas de longo prazo
- aprendizados aprovados

Não salvar:

- contexto transitório
- segredos ou credenciais
- dados pessoais sensíveis sem aprovação explícita
- chunks brutos como memória
- inferência incerta como fato
- dados privados em chats compartilhados

## Integração MCP transparente

A UX desejada é invisível:

1. usuário pergunta normalmente
2. agente chama `memory_context` silenciosamente
3. agente responde normalmente
4. agente chama `memory_suggest`
5. candidatos seguros/aprovados são salvos com `memory_remember`

Gerar snippets MCP:

```bash
bin/llm-memory install-mcp claude-code
bin/llm-memory install-mcp codex
bin/llm-memory install-mcp openclaw
```

## Segurança

Premissas atuais:

- uso local
- bind em `127.0.0.1`
- sem exposição pública
- sem segredos no conteúdo da memória
- sem isolamento multiusuário de produção ainda

Não exponha o servidor HTTP a redes não confiáveis até existir API token/auth local.

## Documentação

A documentação detalhada está em [`docs/`](docs/).

## Licença

MIT.

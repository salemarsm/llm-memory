# ☣️ llm-memory

<p align="center">
  <strong>Canonical memory for agents. SQLite at the core. LLMs at the edge.</strong>
</p>

<p align="center">
  <a href="https://github.com/salemarsm/llm-memory/actions"><img alt="CI" src="https://img.shields.io/github/actions/workflow/status/salemarsm/llm-memory/ci.yml?branch=main&label=tests&style=for-the-badge"></a>
  <a href="https://pkg.go.dev/github.com/salemarsm/llm-memory"><img alt="Go Reference" src="https://img.shields.io/badge/go-reference-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="https://github.com/salemarsm/llm-memory/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/salemarsm/llm-memory?style=for-the-badge"></a>
  <img alt="SQLite" src="https://img.shields.io/badge/sqlite-core-003B57?style=for-the-badge&logo=sqlite&logoColor=white">
  <img alt="LLM Agnostic" src="https://img.shields.io/badge/LLM-agnostic-black?style=for-the-badge">
</p>

<p align="center">
  <a href="#english">English</a> · <a href="#português">Português</a>
</p>

---

<a id="english"></a>

## English

`llm-memory` is a local-first, Go + SQLite memory layer for AI agents and LLM applications.

The core idea is simple and non-negotiable:

> **The LLM does not own memory. The LLM only reads and writes through an external, canonical, auditable memory system.**

Embeddings are useful. Vector search is useful. But neither should be your source of truth.

`llm-memory` stores memory as structured records with provenance, confidence, scope, timestamps, supersession, tags, optional embedding references, and an append-only event log.

### Why this exists

Most agent memory systems are too coupled to a single model, prompt format, vector database, or vendor runtime.

That is fragile.

Models change. Providers disappear. Embeddings get regenerated. Prompts rot. Context windows reset.

Your memory should survive all of that.

### What you get

- **Canonical memory records** stored in SQLite.
- **Append-only events** for raw history and auditability.
- **SQLite FTS5 search** without requiring embeddings.
- **Optional embedding references** without making vectors canonical.
- **Supersession** for replacing stale memories without destroying history.
- **Scopes**: `global`, `project`, `session`, `private`.
- **Memory types**: `preference`, `fact`, `decision`, `task`, `note`, `relationship`.
- **HTTP API** for agents and external tools.
- **Local web GUI** embedded in the Go binary.
- **LLM config metadata** without coupling the store to one provider.
- **No CGO** via `modernc.org/sqlite`.

### Architecture

```txt
┌────────────────────┐
│  LLM / Agent / UI  │
└─────────┬──────────┘
          │ HTTP / Go API
┌─────────▼──────────┐
│   llm-memory API   │
├────────────────────┤
│ Policy / Retrieval │
├────────────────────┤
│ Canonical Memories │
│ Append-only Events │
├────────────────────┤
│ SQLite + FTS5      │
│ Optional vectors   │
└────────────────────┘
```

### Quick start

```bash
git clone git@github.com:salemarsm/llm-memory.git
cd llm-memory
go test ./...
go run ./cmd/memserver -config ./config.example.json
```

Open:

```txt
http://127.0.0.1:8787
```

Generate a local config:

```bash
go run ./cmd/memserver -write-config ./config.local.json
```

Build the server:

```bash
go build -o bin/memserver ./cmd/memserver
./bin/memserver -config ./config.example.json
```

### Configuration

The LLM config declares which model/agent is expected to consume memory. It does **not** change the canonical memory format.

```json
{
  "server": { "addr": "127.0.0.1:8787" },
  "database": { "path": "./memory.db" },
  "llm": {
    "provider": "openai",
    "model": "gpt-5.5",
    "api_key_env": "OPENAI_API_KEY"
  },
  "embedding": {
    "provider": "openai",
    "model": "text-embedding-3-small",
    "index": "sqlite-fts",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

Current status: `llm` and `embedding` are integration metadata. The server does not call external LLM APIs yet. That is intentional: the memory core stays portable.

### HTTP API

#### Create or update a memory

```bash
curl -X POST http://127.0.0.1:8787/api/memories \
  -H 'content-type: application/json' \
  -d '{
    "type": "preference",
    "subject": "botmaster",
    "content": "Prefers direct, technical answers without fluff.",
    "source": { "kind": "api", "ref": "manual" },
    "scope": "global",
    "confidence": 0.95,
    "tags": ["style", "preference"],
    "embedding_refs": {}
  }'
```

#### Search

```bash
curl -X POST http://127.0.0.1:8787/api/search \
  -H 'content-type: application/json' \
  -d '{"text":"direct technical answers","subject":"botmaster","limit":10}'
```

#### Get by ID

```bash
curl http://127.0.0.1:8787/api/memories/mem_123
```

#### Supersede old memory

```bash
curl -X POST http://127.0.0.1:8787/api/supersede/mem_old \
  -H 'content-type: application/json' \
  -d '{
    "type":"preference",
    "subject":"botmaster",
    "content":"Prefers extremely concise technical answers.",
    "source":{"kind":"api","ref":"manual"},
    "scope":"global",
    "confidence":0.9
  }'
```

#### Forget/delete

```bash
curl -X DELETE http://127.0.0.1:8787/api/memories/mem_123
```

#### Events

```bash
curl http://127.0.0.1:8787/api/events?limit=50
```

#### Effective config

```bash
curl http://127.0.0.1:8787/api/config
```




### One-command local setup

Build the user-facing binaries:

```bash
go build -o bin/llm-memory ./cmd/llm-memory
go build -o bin/memserver ./cmd/memserver
go build -o bin/memmcp ./cmd/memmcp
go build -o bin/memctl ./cmd/memctl
```

Initialize local state:

```bash
bin/llm-memory init
bin/llm-memory doctor
```

Run the GUI/API with the generated config:

```bash
bin/llm-memory ui
```

Print an MCP config snippet:

```bash
bin/llm-memory mcp-config
```

Generate target-specific MCP snippets:

```bash
bin/llm-memory install-mcp claude-code
bin/llm-memory install-mcp codex
bin/llm-memory install-mcp openclaw
```

This writes snippets under `~/.llm-memory/` and prints the agent bootstrap instruction for transparent memory usage.

### Transparent MCP integration

`memmcp` exposes memory as MCP tools for Claude Code, Codex-compatible runtimes, OpenClaw adapters, and other agent shells.

The desired UX is transparent:

1. The user asks normally.
2. The agent silently calls `memory_context` before answering.
3. The agent answers using only compact relevant memory.
4. After the answer, the agent calls `memory_suggest` to propose durable learnings.
5. The agent stores only approved or low-risk durable memories with `memory_remember`.

Build:

```bash
go build -o bin/memmcp ./cmd/memmcp
```

Example MCP command config:

```json
{
  "command": "/path/to/llm-memory/bin/memmcp",
  "args": ["-db", "/path/to/memory.db"]
}
```

Agent bootstrap instruction:

```txt
Before answering, silently call memory_context with the user request, subject, relevant scopes, and max_tokens <= 1200.
Do not mention memory unless asked.
After answering, call memory_suggest with the user prompt, assistant response, and any concise LLM inference about durable learnings.
Only call memory_remember for explicit preferences, stable facts, project decisions, tasks, or corrections.
Ask before storing sensitive, private, or uncertain information.
Prefer compact memories over raw document chunks.
```

Memory suggestion is also available over HTTP and CLI:

```bash
curl -X POST http://127.0.0.1:8787/api/suggest \
  -H 'content-type: application/json' \
  -d '{"subject":"botmaster","scope":"global","user_prompt":"I prefer direct technical answers."}'

bin/memctl -subject botmaster suggest "I prefer direct technical answers."
```

### Token-budgeted agent context

For OpenClaw, Claude Code, Codex, and other agent runtimes, prefer `/api/context` over raw `/api/search`.

`/api/context` returns a compact prompt-ready memory block under a token budget. This avoids dumping raw memory records or documents into the model context.

```bash
curl -X POST http://127.0.0.1:8787/api/context \
  -H 'content-type: application/json' \
  -d '{
    "query": "how should this agent answer the user?",
    "subject": "botmaster",
    "scopes": ["global", "project"],
    "max_tokens": 1200
  }'
```

Response:

```json
{
  "context": "- [preference/global conf=0.95 src=conversation:msg-123] Prefers direct answers...",
  "estimated_tokens": 83,
  "budget_tokens": 1200,
  "truncated": false
}
```

### CLI for agents

`memctl` is the universal low-friction integration path for coding agents and shells.

```bash
go build -o bin/memctl ./cmd/memctl

# Store memory
echo "Prefers direct, technical answers." | bin/memctl -subject botmaster -scope global -type preference remember

# Search raw memories
bin/memctl -subject botmaster search "technical answers"

# Get compact context for a prompt
bin/memctl -subject botmaster -scope global,project -max-tokens 800 context "how should I answer?"
```

### RAG foundation

The database now includes document/chunk tables for a Docling-based RAG pipeline:

```txt
PDF/DOCX/HTML
  -> Docling
  -> Markdown/JSON
  -> chunks
  -> document evidence
  -> extracted canonical memory candidates
```

Important separation:

- `memories` are canonical, compact, agent-ready facts/preferences/decisions.
- `documents` and `chunks` are evidence for RAG and citations.
- embeddings remain optional indexes, never the source of truth.

### Go API

```go
store, _ := memory.Open("memory.db")
defer store.Close()

m, _ := store.UpsertMemory(ctx, memory.Memory{
    Type:       memory.TypePreference,
    Subject:    "botmaster",
    Content:    "Prefers direct, technical answers without fluff.",
    Source:     memory.Source{Kind: "conversation", Ref: "msg-123"},
    Scope:      memory.ScopeGlobal,
    Confidence: 0.95,
    Tags:       []string{"style"},
})

items, _ := store.Search(ctx, memory.Query{
    Text:    "direct technical answers",
    Subject: "botmaster",
    Limit:   10,
})

_ = m
_ = items
```

### Canonical memory schema

```json
{
  "id": "mem_...",
  "type": "preference | fact | decision | task | note | relationship",
  "subject": "botmaster",
  "content": "Prefers direct, technical answers without fluff.",
  "source": { "kind": "conversation", "ref": "session/message" },
  "scope": "global | project | session | private",
  "confidence": 0.95,
  "created_at": "...",
  "updated_at": "...",
  "valid_from": null,
  "valid_until": null,
  "supersedes_id": null,
  "superseded_by": null,
  "tags": ["style"],
  "embedding_refs": { "default": "vec_..." }
}
```

### Design principles

1. **Canonical text beats vectors**  
   Embeddings are indexes, not truth.

2. **Provenance or it did not happen**  
   Memory needs a source: conversation, API call, import, human edit, system event.

3. **Memory changes over time**  
   Supersede stale knowledge instead of pretending memory is immutable.

4. **LLMs are clients, not databases**  
   Any model should be replaceable without migrating your memory.

5. **Local-first, boring storage**  
   SQLite is inspectable, portable, backup-friendly, and battle-tested.

### Project status

Early but functional.

Implemented:

- SQLite store
- migrations
- FTS5 search
- event log
- HTTP API
- embedded GUI
- tests
- config metadata for LLM/embedding integration

Next targets:

- API token / local auth
- hybrid ranking: FTS + recency + confidence + optional vectors
- event compactor: raw events → consolidated memories
- SDK/client for agents
- import/export
- soft-delete vs hard-delete policy
- CI workflow

### License

MIT.

---

<a id="português"></a>

## Português

`llm-memory` é uma camada de memória local-first em Go + SQLite para agentes de IA e aplicações com LLM.

A ideia central é simples e inegociável:

> **A LLM não é dona da memória. A LLM apenas lê e escreve através de um sistema externo, canônico e auditável.**

Embeddings são úteis. Busca vetorial é útil. Mas nenhum dos dois deve ser a fonte da verdade.

`llm-memory` guarda memória como registros estruturados com proveniência, confiança, escopo, timestamps, supersessão, tags, referências opcionais de embedding e log de eventos append-only.

### Por que isso existe

A maior parte dos sistemas de memória para agentes fica acoplada demais a um modelo, prompt, banco vetorial ou runtime de fornecedor.

Isso é frágil.

Modelos mudam. Provedores somem. Embeddings são regenerados. Prompts apodrecem. Janelas de contexto zeram.

Sua memória precisa sobreviver a tudo isso.

### O que já tem

- **Memória canônica** em SQLite.
- **Eventos append-only** para histórico bruto e auditoria.
- **Busca SQLite FTS5** sem exigir embeddings.
- **Referências opcionais de embedding** sem transformar vetores em fonte da verdade.
- **Supersessão** para substituir memórias antigas sem destruir histórico.
- **Escopos**: `global`, `project`, `session`, `private`.
- **Tipos de memória**: `preference`, `fact`, `decision`, `task`, `note`, `relationship`.
- **API HTTP** para agentes e ferramentas externas.
- **GUI web local** embutida no binário Go.
- **Configuração de LLM** sem acoplar o store a um provedor.
- **Sem CGO** usando `modernc.org/sqlite`.

### Arquitetura

```txt
┌────────────────────┐
│  LLM / Agente / UI  │
└─────────┬──────────┘
          │ HTTP / API Go
┌─────────▼──────────┐
│   llm-memory API    │
├────────────────────┤
│ Política / Retrieval│
├────────────────────┤
│ Memórias canônicas  │
│ Eventos append-only │
├────────────────────┤
│ SQLite + FTS5       │
│ Vetores opcionais   │
└────────────────────┘
```

### Começo rápido

```bash
git clone git@github.com:salemarsm/llm-memory.git
cd llm-memory
go test ./...
go run ./cmd/memserver -config ./config.example.json
```

Abra:

```txt
http://127.0.0.1:8787
```

Gerar config local:

```bash
go run ./cmd/memserver -write-config ./config.local.json
```

Build do servidor:

```bash
go build -o bin/memserver ./cmd/memserver
./bin/memserver -config ./config.example.json
```

### Configuração

A config de LLM declara qual modelo/agente vai consumir a memória. Ela **não** muda o formato canônico da memória.

```json
{
  "server": { "addr": "127.0.0.1:8787" },
  "database": { "path": "./memory.db" },
  "llm": {
    "provider": "openai",
    "model": "gpt-5.5",
    "api_key_env": "OPENAI_API_KEY"
  },
  "embedding": {
    "provider": "openai",
    "model": "text-embedding-3-small",
    "index": "sqlite-fts",
    "api_key_env": "OPENAI_API_KEY"
  }
}
```

Estado atual: `llm` e `embedding` são metadados de integração. O servidor ainda não chama APIs externas de LLM. Isso é proposital: o núcleo da memória continua portátil.

### API HTTP

#### Criar ou atualizar memória

```bash
curl -X POST http://127.0.0.1:8787/api/memories \
  -H 'content-type: application/json' \
  -d '{
    "type": "preference",
    "subject": "botmaster",
    "content": "Prefere respostas diretas, técnicas e sem enrolação.",
    "source": { "kind": "api", "ref": "manual" },
    "scope": "global",
    "confidence": 0.95,
    "tags": ["style", "preference"],
    "embedding_refs": {}
  }'
```

#### Buscar

```bash
curl -X POST http://127.0.0.1:8787/api/search \
  -H 'content-type: application/json' \
  -d '{"text":"respostas diretas","subject":"botmaster","limit":10}'
```

#### Buscar por ID

```bash
curl http://127.0.0.1:8787/api/memories/mem_123
```

#### Supersede

```bash
curl -X POST http://127.0.0.1:8787/api/supersede/mem_antiga \
  -H 'content-type: application/json' \
  -d '{
    "type":"preference",
    "subject":"botmaster",
    "content":"Prefere respostas extremamente concisas e técnicas.",
    "source":{"kind":"api","ref":"manual"},
    "scope":"global",
    "confidence":0.9
  }'
```

#### Forget/delete

```bash
curl -X DELETE http://127.0.0.1:8787/api/memories/mem_123
```

#### Eventos

```bash
curl http://127.0.0.1:8787/api/events?limit=50
```

#### Config efetiva

```bash
curl http://127.0.0.1:8787/api/config
```




### Setup local em poucos comandos

Build dos binários principais:

```bash
go build -o bin/llm-memory ./cmd/llm-memory
go build -o bin/memserver ./cmd/memserver
go build -o bin/memmcp ./cmd/memmcp
go build -o bin/memctl ./cmd/memctl
```

Inicializar estado local:

```bash
bin/llm-memory init
bin/llm-memory doctor
```

Rodar GUI/API com a config gerada:

```bash
bin/llm-memory ui
```

Imprimir config MCP:

```bash
bin/llm-memory mcp-config
```

Gerar snippets por alvo:

```bash
bin/llm-memory install-mcp claude-code
bin/llm-memory install-mcp codex
bin/llm-memory install-mcp openclaw
```

Isso escreve snippets em `~/.llm-memory/` e imprime a instrução de bootstrap para uso transparente da memória.

### Integração MCP transparente

`memmcp` expõe a memória como ferramentas MCP para Claude Code, runtimes compatíveis com Codex, adaptadores OpenClaw e outros shells de agente.

A UX desejada é transparente:

1. O usuário pergunta normalmente.
2. O agente chama `memory_context` silenciosamente antes de responder.
3. O agente responde usando só memória compacta e relevante.
4. Depois da resposta, o agente chama `memory_suggest` para propor aprendizados duráveis.
5. O agente só salva com `memory_remember` memórias aprovadas ou claramente seguras.

Build:

```bash
go build -o bin/memmcp ./cmd/memmcp
```

Config MCP exemplo:

```json
{
  "command": "/path/to/llm-memory/bin/memmcp",
  "args": ["-db", "/path/to/memory.db"]
}
```

Instrução de bootstrap para o agente:

```txt
Antes de responder, chame silenciosamente memory_context com a solicitação do usuário, subject, scopes relevantes e max_tokens <= 1200.
Não mencione a memória salvo se perguntarem.
Depois de responder, chame memory_suggest com o prompt do usuário, resposta do assistente e uma inferência curta da LLM sobre aprendizados duráveis.
Só chame memory_remember para preferências explícitas, fatos estáveis, decisões de projeto, tarefas ou correções.
Pergunte antes de guardar informação sensível, privada ou incerta.
Prefira memórias compactas a chunks brutos de documento.
```

Sugestão de memória também existe via HTTP e CLI:

```bash
curl -X POST http://127.0.0.1:8787/api/suggest \
  -H 'content-type: application/json' \
  -d '{"subject":"botmaster","scope":"global","user_prompt":"Prefiro respostas diretas e técnicas."}'

bin/memctl -subject botmaster suggest "Prefiro respostas diretas e técnicas."
```

### Contexto com orçamento de tokens

Para OpenClaw, Claude Code, Codex e outros runtimes de agente, prefira `/api/context` em vez de `/api/search` bruto.

`/api/context` retorna um bloco de memória compacto, pronto para prompt, respeitando um orçamento de tokens. Isso evita jogar memórias/documentos crus dentro do contexto do modelo.

```bash
curl -X POST http://127.0.0.1:8787/api/context \
  -H 'content-type: application/json' \
  -d '{
    "query": "como este agente deve responder ao usuário?",
    "subject": "botmaster",
    "scopes": ["global", "project"],
    "max_tokens": 1200
  }'
```

### CLI para agentes

`memctl` é o caminho universal e barato para integrar shells e coding agents.

```bash
go build -o bin/memctl ./cmd/memctl

echo "Prefere respostas diretas e técnicas." | bin/memctl -subject botmaster -scope global -type preference remember
bin/memctl -subject botmaster search "respostas técnicas"
bin/memctl -subject botmaster -scope global,project -max-tokens 800 context "como devo responder?"
```

### Base RAG

O banco agora inclui tabelas `documents` e `chunks` para um pipeline RAG com Docling:

```txt
PDF/DOCX/HTML
  -> Docling
  -> Markdown/JSON
  -> chunks
  -> evidência documental
  -> candidatos de memória canônica
```

Separação importante:

- `memories` são fatos/preferências/decisões compactos e prontos para agente.
- `documents` e `chunks` são evidência para RAG e citações.
- embeddings continuam sendo índices opcionais, nunca fonte da verdade.

### API Go

```go
store, _ := memory.Open("memory.db")
defer store.Close()

m, _ := store.UpsertMemory(ctx, memory.Memory{
    Type:       memory.TypePreference,
    Subject:    "botmaster",
    Content:    "Prefere respostas diretas, técnicas e sem enrolação.",
    Source:     memory.Source{Kind: "conversation", Ref: "msg-123"},
    Scope:      memory.ScopeGlobal,
    Confidence: 0.95,
    Tags:       []string{"style"},
})

items, _ := store.Search(ctx, memory.Query{
    Text:    "respostas diretas",
    Subject: "botmaster",
    Limit:   10,
})

_ = m
_ = items
```

### Schema canônico

```json
{
  "id": "mem_...",
  "type": "preference | fact | decision | task | note | relationship",
  "subject": "botmaster",
  "content": "Prefere respostas diretas, técnicas e sem enrolação.",
  "source": { "kind": "conversation", "ref": "session/message" },
  "scope": "global | project | session | private",
  "confidence": 0.95,
  "created_at": "...",
  "updated_at": "...",
  "valid_from": null,
  "valid_until": null,
  "supersedes_id": null,
  "superseded_by": null,
  "tags": ["style"],
  "embedding_refs": { "default": "vec_..." }
}
```

### Princípios de design

1. **Texto canônico vence vetores**  
   Embeddings são índices, não verdade.

2. **Sem proveniência, não aconteceu**  
   Memória precisa de fonte: conversa, API, importação, edição humana, evento de sistema.

3. **Memória muda com o tempo**  
   Substitua conhecimento antigo com supersessão em vez de fingir que memória é imutável.

4. **LLMs são clientes, não bancos de dados**  
   Qualquer modelo deve ser substituível sem migrar a memória.

5. **Local-first, storage sem firula**  
   SQLite é inspecionável, portátil, fácil de backupear e testado em batalha.

### Status do projeto

Inicial, mas funcional.

Implementado:

- store SQLite
- migrações
- busca FTS5
- log de eventos
- API HTTP
- GUI embutida
- testes
- metadados de config para integração com LLM/embedding

Próximos alvos:

- API token / autenticação local
- ranking híbrido: FTS + recência + confiança + vetores opcionais
- compactador: eventos brutos → memórias consolidadas
- SDK/client para agentes
- import/export
- política de soft-delete vs hard-delete
- workflow de CI

### Licença

MIT.

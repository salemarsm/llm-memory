package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/salemarsm/llm-memory/internal/version"
	"github.com/salemarsm/llm-memory/memory"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type mcpServer struct {
	store   *memory.Store
	project string
}

func main() {
	db := flag.String("db", envDefault("LLM_MEMORY_DB", "./memory.db"), "SQLite database path")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("memmcp", version.String())
		return
	}

	store, err := memory.Open(*db)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	s := &mcpServer{store: store, project: memory.DetectProject("")}
	s.run()
}

func (s *mcpServer) run() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeResp(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		if req.ID == nil && len(req.Method) > 0 {
			continue // notification
		}
		resp := s.handle(req)
		writeResp(resp)
	}
	if err := scanner.Err(); err != nil {
		writeResp(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32000, Message: err.Error()}})
	}
}

func (s *mcpServer) handle(req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "llm-memory", "version": version.Version},
			"capabilities":    map[string]any{"tools": map[string]any{}},
		}
	case "tools/list":
		resp.Result = map[string]any{"tools": tools()}
	case "tools/call":
		var call toolCall
		if err := json.Unmarshal(req.Params, &call); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: err.Error()}
			return resp
		}
		result, err := s.callTool(call)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
			return resp
		}
		resp.Result = map[string]any{"content": []map[string]string{{"type": "text", "text": result}}}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func (s *mcpServer) callTool(call toolCall) (string, error) {
	ctx := context.Background()
	switch call.Name {
	case "memory_context":
		var req memory.ContextRequest
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		s.defaultProjectContext(&req.Project, &req.Subject)
		out, err := s.store.BuildContext(ctx, req)
		return pretty(out), err
	case "memory_remember":
		var m memory.Memory
		if err := json.Unmarshal(call.Arguments, &m); err != nil {
			return "", err
		}
		m.Content = memory.StripPrivateTags(m.Content)
		if m.Source.Kind == "" {
			m.Source = memory.Source{Kind: "mcp", Ref: "memory_remember"}
		}
		if m.Subject == "" {
			m.Subject = s.project
		}
		if m.Scope == "" {
			m.Scope = memory.ScopeProject
		}
		if m.Confidence == 0 {
			m.Confidence = 0.8
		}
		if m.EmbeddingRefs == nil {
			m.EmbeddingRefs = memory.EmbeddingRefs{}
		}
		out, err := s.store.UpsertMemory(ctx, m)
		return pretty(out), err
	case "memory_search":
		var q memory.Query
		if err := json.Unmarshal(call.Arguments, &q); err != nil {
			return "", err
		}
		if q.Subject == "" {
			q.Subject = s.project
		}
		out, err := s.store.Search(ctx, q)
		return pretty(out), err
	case "memory_session_start":
		var req memory.SessionStartRequest
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		if req.Project == "" {
			req.Project = s.project
		}
		out, err := s.store.StartSession(ctx, req.Project)
		return pretty(out), err
	case "memory_session_end":
		var req memory.SessionEndRequest
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		if req.Project == "" {
			req.Project = s.project
		}
		out, err := s.store.EndActiveSession(ctx, req.Project, req.Summary)
		return pretty(out), err
	case "memory_session_summary":
		var req memory.SessionSummaryRequest
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		if req.Project == "" && req.SessionID == "" {
			req.Project = s.project
		}
		out, err := s.store.SessionSummary(ctx, req)
		return pretty(out), err
	case "memory_suggest":
		var req memory.SuggestRequest
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		out, err := s.store.SuggestMemories(ctx, req)
		return pretty(out), err
	default:
		return "", fmt.Errorf("unknown tool %q", call.Name)
	}
}

func (s *mcpServer) defaultProjectContext(project, subject *string) {
	if *project == "" {
		*project = s.project
	}
	if *subject == "" {
		*subject = *project
	}
}

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "memory_context",
			"description": "Silently call before answering. Returns compact prompt-ready memory under a token budget. Do not expose raw records unless asked.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"query": map[string]string{"type": "string"}, "project": map[string]string{"type": "string"}, "subject": map[string]string{"type": "string"}, "scopes": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "max_tokens": map[string]string{"type": "integer"}}, "required": []string{"query"}},
		},
		{
			"name":        "memory_suggest",
			"description": "Call after useful user prompts/responses to propose durable memories or learnings. Returns candidates; ask for confirmation when sensitive or uncertain.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"subject": map[string]string{"type": "string"}, "scope": map[string]string{"type": "string"}, "user_prompt": map[string]string{"type": "string"}, "assistant_response": map[string]string{"type": "string"}, "llm_inference": map[string]string{"type": "string"}, "max_candidates": map[string]string{"type": "integer"}}},
		},
		{
			"name":        "memory_remember",
			"description": "Store an approved durable memory. Use for explicit preferences, stable facts, project decisions, tasks, and corrections. Avoid casual or sensitive content unless confirmed.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"type": map[string]string{"type": "string"}, "subject": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}, "scope": map[string]string{"type": "string"}, "confidence": map[string]string{"type": "number"}, "tags": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}}, "required": []string{"type", "content"}},
		},
		{
			"name":        "memory_search",
			"description": "Search raw memories. Prefer memory_context for normal prompt injection because it is token-budgeted.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]string{"type": "string"}, "subject": map[string]string{"type": "string"}, "limit": map[string]string{"type": "integer"}}},
		},
		{
			"name":        "memory_session_start",
			"description": "Start or return the active persistent memory session for a project. Project auto-detects when omitted.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"project": map[string]string{"type": "string"}}},
		},
		{
			"name":        "memory_session_end",
			"description": "End the active memory session for a project with a concise durable summary.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"project": map[string]string{"type": "string"}, "summary": map[string]string{"type": "string"}}, "required": []string{"summary"}},
		},
		{
			"name":        "memory_session_summary",
			"description": "Return the active or latest closed session summary for a project, or a specific session by id.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"project": map[string]string{"type": "string"}, "session_id": map[string]string{"type": "string"}}},
		},
	}
}

func writeResp(resp rpcResponse) {
	b, _ := json.Marshal(resp)
	fmt.Println(string(b))
}

func pretty(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func envDefault(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

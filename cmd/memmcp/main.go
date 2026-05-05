package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/salemarsm/llm-memory/config"
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
	profile config.AgentProfile
}

func main() {
	db := flag.String("db", envDefault("LLM_MEMORY_DB", "./memory.db"), "SQLite database path")
	agentName := flag.String("agent", envDefault("GINKO_AGENT", "claude-code"), "agent name for profile lookup")
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

	var profile config.AgentProfile
	if cfg, err := config.Load(config.DefaultConfigPath()); err == nil {
		if cfg.Agents != nil {
			if p, ok := cfg.Agents[*agentName]; ok {
				profile = p
			}
		}
	}

	project := memory.DetectProject("")
	if profile.DefaultSubject != "" && project == "" {
		project = profile.DefaultSubject
	}

	s := &mcpServer{store: store, project: project, profile: profile}
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
		if req.MaxTokens == 0 && s.profile.MaxContextTokens > 0 {
			req.MaxTokens = s.profile.MaxContextTokens
		}
		if len(req.Scopes) == 0 && s.profile.DefaultScope != "" {
			req.Scopes = []memory.Scope{memory.Scope(s.profile.DefaultScope)}
		}
		out, err := s.store.BuildContext(ctx, req)
		return pretty(out), err
	case "memory_remember":
		var req struct {
			memory.Memory
			DryRun bool `json:"dry_run"`
		}
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		m := req.Memory
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
		if req.DryRun {
			type dryRunResult struct {
				DryRun  bool          `json:"dry_run"`
				Preview memory.Memory `json:"preview"`
			}
			return pretty(dryRunResult{DryRun: true, Preview: m}), nil
		}
		// Apply per-scope write policy: save as pending if scope requires approval.
		if s.profile.WritePolicy.ScopeRequiresApproval(string(m.Scope)) {
			m.Status = memory.StatusPending
		}
		result, err := s.store.UpsertMemoryFull(ctx, m)
		if err != nil {
			return "", err
		}
		_ = s.store.AppendEvent(ctx, memory.Event{MemoryID: &result.Memory.ID, Kind: "memory.upserted", Payload: result.Memory.ID, Source: memory.Source{Kind: "mcp", Ref: "memory_remember"}})
		return pretty(result), nil
	case "memory_get":
		var req struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		if req.ID == "" {
			return "", fmt.Errorf("id is required")
		}
		out, err := s.store.GetMemory(ctx, req.ID)
		return pretty(out), err
	case "memory_timeline":
		var req struct {
			ID    string `json:"id"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(call.Arguments, &req); err != nil {
			return "", err
		}
		if req.ID == "" {
			return "", fmt.Errorf("id is required")
		}
		out, err := s.store.MemoryTimeline(ctx, req.ID, req.Limit)
		return pretty(out), err
	case "memory_search":
		var q memory.Query
		if err := json.Unmarshal(call.Arguments, &q); err != nil {
			return "", err
		}
		// Do NOT force-inject subject: memory_search is a cross-subject tool.
		// Callers can pass subject explicitly to narrow results.
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
			"description": "Store an approved durable memory. Use for explicit preferences, stable facts, project decisions, tasks, and corrections. Avoid casual or sensitive content unless confirmed. Set dry_run=true to preview. Set topic_key to a stable slug (e.g. 'rate-limiting-strategy') to auto-supersede previous memories on the same topic.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"type": map[string]string{"type": "string"}, "subject": map[string]string{"type": "string"}, "content": map[string]string{"type": "string"}, "scope": map[string]string{"type": "string"}, "confidence": map[string]string{"type": "number"}, "tags": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "topic_key": map[string]string{"type": "string", "description": "Stable slug for this topic. A new memory with the same topic_key auto-supersedes the previous one."}, "dry_run": map[string]any{"type": "boolean", "description": "If true, return the memory that would be saved without persisting it."}}, "required": []string{"type", "content"}},
		},
		{
			"name":        "memory_search",
			"description": "Search raw memories. Prefer memory_context for normal prompt injection because it is token-budgeted.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]string{"type": "string"}, "subject": map[string]string{"type": "string"}, "limit": map[string]string{"type": "integer"}}},
		},
		{
			"name":        "memory_get",
			"description": "Fetch the full detail of a single memory by ID. Use when memory_search or memory_context returns an item you want to inspect fully.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string", "description": "Memory ID (mem_...)"}}, "required": []string{"id"}},
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
		{
			"name":        "memory_timeline",
			"description": "Show lifecycle/audit events for one memory, including creation, supersession, deletion, and context usage when recorded.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]string{"type": "string"}, "limit": map[string]string{"type": "integer"}}, "required": []string{"id"}},
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

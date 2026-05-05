package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// contractTool asserts that a named tool exists in tools/list and returns its schema.
func contractTool(t *testing.T, srv *mcpServer, name string) map[string]any {
	t.Helper()
	resp := srv.handle(rpcRequest{JSONRPC: "2.0", ID: 1, Method: "tools/list"})
	if resp.Error != nil {
		t.Fatalf("tools/list error: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal tools/list: %v", err)
	}
	for _, tool := range result.Tools {
		if tool["name"] == name {
			return tool
		}
	}
	t.Fatalf("tool %q not found in tools/list", name)
	return nil
}

// callToolJSON invokes a tool and returns the unwrapped text content from the MCP response.
func callToolJSON(t *testing.T, srv *mcpServer, name, argsJSON string) string {
	t.Helper()
	resp := srv.handle(rpcRequest{
		JSONRPC: "2.0", ID: 99, Method: "tools/call",
		Params: json.RawMessage(`{"name":"` + name + `","arguments":` + argsJSON + `}`),
	})
	if resp.Error != nil {
		t.Fatalf("tools/call %s error %d: %s", name, resp.Error.Code, resp.Error.Message)
	}
	// Unwrap MCP content envelope: {"content":[{"type":"text","text":"..."}]}
	raw, _ := json.Marshal(resp.Result)
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Content) > 0 {
		return envelope.Content[0].Text
	}
	return string(raw)
}

func TestContractMemoryContext(t *testing.T) {
	tool := contractTool(t, &mcpServer{store: openTestStore(t)}, "memory_context")
	schema := tool["inputSchema"].(map[string]any)
	if schema["type"] != "object" {
		t.Fatal("expected object schema")
	}
	required, _ := schema["required"].([]any)
	hasQuery := false
	for _, r := range required {
		if r == "query" {
			hasQuery = true
		}
	}
	if !hasQuery {
		t.Fatal("memory_context: 'query' must be required")
	}

	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}
	out := callToolJSON(t, srv, "memory_context", `{"query":"anything"}`)
	if !strings.Contains(out, "context") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestContractMemoryRemember(t *testing.T) {
	tool := contractTool(t, &mcpServer{store: openTestStore(t)}, "memory_remember")
	schema := tool["inputSchema"].(map[string]any)
	props := schema["properties"].(map[string]any)
	for _, field := range []string{"type", "content", "subject", "scope", "confidence", "tags", "dry_run"} {
		if _, ok := props[field]; !ok {
			t.Errorf("memory_remember schema missing property %q", field)
		}
	}

	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}

	// normal save
	out := callToolJSON(t, srv, "memory_remember",
		`{"type":"fact","content":"contract test","subject":"ct","scope":"project","confidence":0.9}`)
	if !strings.Contains(out, "ct") {
		t.Fatalf("unexpected remember output: %s", out)
	}

	// dry-run must not persist
	out = callToolJSON(t, srv, "memory_remember",
		`{"type":"fact","content":"dry run only","subject":"ct","dry_run":true}`)
	if !strings.Contains(out, `"dry_run": true`) && !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("expected dry_run=true in response, got: %s", out)
	}
	// verify it was not saved
	searchResp := srv.handle(rpcRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"memory_search","arguments":{"text":"dry run only"}}`),
	})
	raw, _ := json.Marshal(searchResp.Result)
	if strings.Contains(string(raw), "dry run only") {
		t.Fatal("dry_run=true should not persist memory")
	}
}

func TestContractMemorySearch(t *testing.T) {
	contractTool(t, &mcpServer{store: openTestStore(t)}, "memory_search")

	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}
	// save with same subject as server project so search can find it
	callToolJSON(t, srv, "memory_remember",
		`{"type":"fact","content":"searchable content xyz","subject":"contract-test","scope":"project","confidence":0.9}`)
	out := callToolJSON(t, srv, "memory_search", `{"text":"searchable content xyz"}`)
	if !strings.Contains(out, "xyz") {
		t.Fatalf("search did not find saved memory: %s", out)
	}
}

func TestContractMemoryTimeline(t *testing.T) {
	contractTool(t, &mcpServer{store: openTestStore(t)}, "memory_timeline")

	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}
	out := callToolJSON(t, srv, "memory_remember",
		`{"type":"fact","content":"timeline test","subject":"ct","scope":"project"}`)

	// extract id from output
	var saved struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(out), &saved)
	// Unwrap content array if needed
	var wrapper struct {
		Content []struct{ Text string } `json:"content"`
	}
	if err := json.Unmarshal([]byte(out), &wrapper); err == nil && len(wrapper.Content) > 0 {
		json.Unmarshal([]byte(wrapper.Content[0].Text), &saved)
	}

	if saved.ID == "" {
		t.Skip("could not extract memory id from remember response")
	}
	tlOut := callToolJSON(t, srv, "memory_timeline", `{"id":"`+saved.ID+`"}`)
	if !strings.Contains(tlOut, saved.ID) {
		t.Fatalf("timeline did not include memory id: %s", tlOut)
	}
}

func TestContractMemorySuggest(t *testing.T) {
	contractTool(t, &mcpServer{store: openTestStore(t)}, "memory_suggest")
	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}
	// should not error even with empty store
	callToolJSON(t, srv, "memory_suggest", `{"subject":"ct","context":"some work context"}`)
}

func TestContractSessionTools(t *testing.T) {
	for _, name := range []string{"memory_session_start", "memory_session_end", "memory_session_summary"} {
		contractTool(t, &mcpServer{store: openTestStore(t)}, name)
	}

	srv := &mcpServer{store: openTestStore(t), project: "contract-test"}
	callToolJSON(t, srv, "memory_session_start", `{"project":"contract-test"}`)
	callToolJSON(t, srv, "memory_session_end", `{"project":"contract-test","summary":"done"}`)
}

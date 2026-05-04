package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/salemarsm/llm-memory/config"
	"github.com/salemarsm/llm-memory/memory"
)

func TestAPIAndGUI(t *testing.T) {
	store, err := memory.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	h := New(store, config.Default()).Handler()

	body := `{"type":"note","subject":"smoke","content":"teste api gui","source":{"kind":"test","ref":"httptest"},"scope":"project","confidence":0.8,"tags":["smoke"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/memories", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/memories status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewBufferString(`{"text":"teste","subject":"smoke","limit":5}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "teste api gui") {
		t.Fatalf("POST /api/search status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/context", bytes.NewBufferString(`{"query":"teste","subject":"smoke","max_tokens":200}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "teste api gui") || !strings.Contains(rec.Body.String(), "estimated_tokens") {
		t.Fatalf("POST /api/context status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewBufferString(`{"context_id":"ctx_smoke","useful":true,"memory_ids_used":["mem_smoke"]}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ctx_smoke") {
		t.Fatalf("POST /api/feedback status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/suggest", bytes.NewBufferString(`{"subject":"smoke","user_prompt":"Prefiro respostas diretas. Decidimos usar SQLite."}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "candidates") || !strings.Contains(rec.Body.String(), "preference") {
		t.Fatalf("POST /api/suggest status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewBufferString(`{"text":"teste","subject":"smoke","limit":5}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "teste api gui") {
		t.Fatalf("POST /api/v1/search status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("GET /healthz status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "api_key_env") {
		t.Fatalf("GET /api/config leaked internal config or failed status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>llm-memory</title>") {
		t.Fatalf("GET / status=%d body=%s", rec.Code, rec.Body.String())
	}
}

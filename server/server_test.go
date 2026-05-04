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

	req = httptest.NewRequest(http.MethodGet, "/api/usage?subject=smoke", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "context_uses") || !strings.Contains(rec.Body.String(), "teste api gui") {
		t.Fatalf("GET /api/usage status=%d body=%s", rec.Code, rec.Body.String())
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

func TestBearerAuthProtectsAPI(t *testing.T) {
	store, err := memory.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	cfg := config.Default()
	cfg.Server.AuthToken = "secret-token"
	h := New(store, cfg).Handler()

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/config without token status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/config with token status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz should remain public status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSearchIncludeRanking(t *testing.T) {
	store, err := memory.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, config.Default()).Handler()

	body := `{"type":"decision","subject":"rank","content":"Endpoint /api/search can expose lexical score.","source":{"kind":"test","ref":"rank"},"scope":"project","confidence":0.9}`
	req := httptest.NewRequest(http.MethodPost, "/api/memories", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/memories status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/search", bytes.NewBufferString(`{"text":"/api/search","subject":"rank","include_ranking":true}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lexical_score") || !strings.Contains(rec.Body.String(), "rank_reason") {
		t.Fatalf("POST /api/search include_ranking status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/memories?q=/api/search&subject=rank&include_ranking=true", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "lexical_score") {
		t.Fatalf("GET /api/memories include_ranking status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSessionAPIAndContext(t *testing.T) {
	store, err := memory.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	srv := New(store, config.Default())

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/start", bytes.NewBufferString(`{"project":"demo"}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("start status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/sessions/end", bytes.NewBufferString(`{"project":"demo","summary":"Fixed setup flow."}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("end status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/context", bytes.NewBufferString(`{"project":"demo","query":"start","max_tokens":300}`))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("context status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Fixed setup flow") {
		t.Fatalf("expected session summary in context: %s", rec.Body.String())
	}
}

func TestAPIUpsertStripsPrivateTags(t *testing.T) {
	store, err := memory.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	h := New(store, config.Default()).Handler()

	body := `{"type":"fact","subject":"privacy","content":"visible <private>secret</private> fact","source":{"kind":"test","ref":"privacy"},"scope":"project","confidence":0.9}`
	req := httptest.NewRequest(http.MethodPost, "/api/memories", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/memories status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), "visible fact") {
		t.Fatalf("private content leaked or public content missing: %s", rec.Body.String())
	}
}

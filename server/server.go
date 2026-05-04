package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/salemarsm/llm-memory/config"
	"github.com/salemarsm/llm-memory/memory"
)

type Server struct {
	store *memory.Store
	cfg   config.Config
	mux   *http.ServeMux
}

func New(store *memory.Store, cfg config.Config) *Server {
	s := &Server{store: store, cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.authMiddleware(s.mux) }

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	token, ok := s.cfg.Server.BearerToken()
	if !ok {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(got, prefix) || subtle.ConstantTimeCompare([]byte(strings.TrimSpace(strings.TrimPrefix(got, prefix))), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="llm-memory"`)
			writeErrorStatus(w, http.StatusUnauthorized, errors.New("missing or invalid bearer token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || path == "/api/v1" || strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/v1/")
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	for _, prefix := range []string{"/api", "/api/v1"} {
		s.mux.HandleFunc("GET "+prefix+"/config", s.handleConfig)
		s.mux.HandleFunc("GET "+prefix+"/memories", s.handleSearchGET)
		s.mux.HandleFunc("POST "+prefix+"/memories", s.handleUpsertMemory)
		s.mux.HandleFunc("GET "+prefix+"/memories/{id}", s.handleGetMemory)
		s.mux.HandleFunc("DELETE "+prefix+"/memories/{id}", s.handleForget)
		s.mux.HandleFunc("POST "+prefix+"/search", s.handleSearchPOST)
		s.mux.HandleFunc("GET "+prefix+"/usage", s.handleUsage)
		s.mux.HandleFunc("POST "+prefix+"/context", s.handleContext)
		s.mux.HandleFunc("POST "+prefix+"/feedback", s.handleFeedback)
		s.mux.HandleFunc("POST "+prefix+"/suggest", s.handleSuggest)
		s.mux.HandleFunc("POST "+prefix+"/supersede/{id}", s.handleSupersede)
		s.mux.HandleFunc("GET "+prefix+"/events", s.handleEvents)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"server":    map[string]any{"addr": s.cfg.Server.Addr},
		"database":  map[string]any{"path": s.cfg.Database.Path},
		"llm":       map[string]any{"provider": s.cfg.LLM.Provider, "model": s.cfg.LLM.Model},
		"embedding": map[string]any{"provider": s.cfg.Embedding.Provider, "model": s.cfg.Embedding.Model, "index": s.cfg.Embedding.Index},
	})
}

func (s *Server) handleSearchGET(w http.ResponseWriter, r *http.Request) {
	q := memory.Query{
		Text:    r.URL.Query().Get("q"),
		Subject: r.URL.Query().Get("subject"),
		Limit:   atoiDefault(r.URL.Query().Get("limit"), 50),
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		q.Types = []memory.MemoryType{memory.MemoryType(typ)}
	}
	if scope := r.URL.Query().Get("scope"); scope != "" {
		q.Scopes = []memory.Scope{memory.Scope(scope)}
	}
	items, err := s.store.Search(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	q := memory.Query{
		Text:    r.URL.Query().Get("q"),
		Subject: r.URL.Query().Get("subject"),
		Limit:   atoiDefault(r.URL.Query().Get("limit"), 200),
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		q.Types = []memory.MemoryType{memory.MemoryType(typ)}
	}
	if scope := r.URL.Query().Get("scope"); scope != "" {
		q.Scopes = []memory.Scope{memory.Scope(scope)}
	}
	rows, err := s.store.ListMemoryUsage(r.Context(), q, atoiDefault(r.URL.Query().Get("event_limit"), 1000))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleSearchPOST(w http.ResponseWriter, r *http.Request) {
	var q memory.Query
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.store.Search(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleContext(w http.ResponseWriter, r *http.Request) {
	var req memory.ContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.store.BuildContext(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	var req memory.ContextFeedback
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	if req.Source.Kind == "" {
		req.Source = memory.Source{Kind: "api", Ref: r.RemoteAddr}
	}
	if err := s.store.RecordContextFeedback(r.Context(), req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "context_id": req.ContextID})
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	var req memory.SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.store.SuggestMemories(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleUpsertMemory(w http.ResponseWriter, r *http.Request) {
	var m memory.Memory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.UpsertMemory(r.Context(), m)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	s.appendEvent(r, memory.Event{Kind: "memory.upserted", Payload: created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, created)
}

func (s *Server) handleGetMemory(w http.ResponseWriter, r *http.Request) {
	m, err := s.store.GetMemory(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Forget(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	s.appendEvent(r, memory.Event{Kind: "memory.forgotten", Payload: id, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleSupersede(w http.ResponseWriter, r *http.Request) {
	var m memory.Memory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.store.Supersede(r.Context(), r.PathValue("id"), m)
	if err != nil {
		writeError(w, err)
		return
	}
	s.appendEvent(r, memory.Event{Kind: "memory.superseded", Payload: r.PathValue("id") + " -> " + created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, created)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEvents(r.Context(), atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) appendEvent(r *http.Request, e memory.Event) {
	if err := s.store.AppendEvent(r.Context(), e); err != nil {
		log.Printf("llm-memory: append %s event failed: %v", e.Kind, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, memory.ErrNotFound) {
		writeErrorStatus(w, http.StatusNotFound, err)
		return
	}
	writeErrorStatus(w, http.StatusInternalServerError, err)
}

func writeErrorStatus(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func atoiDefault(s string, fallback int) int {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}

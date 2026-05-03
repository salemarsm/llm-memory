package server

import (
	"encoding/json"
	"errors"
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

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.handleIndex)
	s.mux.HandleFunc("GET /api/config", s.handleConfig)
	s.mux.HandleFunc("GET /api/memories", s.handleSearchGET)
	s.mux.HandleFunc("POST /api/memories", s.handleUpsertMemory)
	s.mux.HandleFunc("GET /api/memories/{id}", s.handleGetMemory)
	s.mux.HandleFunc("DELETE /api/memories/{id}", s.handleForget)
	s.mux.HandleFunc("POST /api/search", s.handleSearchPOST)
	s.mux.HandleFunc("POST /api/context", s.handleContext)
	s.mux.HandleFunc("POST /api/suggest", s.handleSuggest)
	s.mux.HandleFunc("POST /api/supersede/{id}", s.handleSupersede)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"server":    s.cfg.Server,
		"database":  s.cfg.Database,
		"llm":       s.cfg.LLM,
		"embedding": s.cfg.Embedding,
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
	_ = s.store.AppendEvent(r.Context(), memory.Event{Kind: "memory.upserted", Payload: created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
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
	_ = s.store.AppendEvent(r.Context(), memory.Event{Kind: "memory.forgotten", Payload: id, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
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
	_ = s.store.AppendEvent(r.Context(), memory.Event{Kind: "memory.superseded", Payload: r.PathValue("id") + " -> " + created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
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

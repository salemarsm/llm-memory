package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
		s.mux.HandleFunc("GET "+prefix+"/memories/{id}/timeline", s.handleMemoryTimeline)
		s.mux.HandleFunc("DELETE "+prefix+"/memories/{id}", s.handleForget)
		s.mux.HandleFunc("POST "+prefix+"/search", s.handleSearchPOST)
		s.mux.HandleFunc("GET "+prefix+"/usage", s.handleUsage)
		s.mux.HandleFunc("POST "+prefix+"/context", s.handleContext)
		s.mux.HandleFunc("POST "+prefix+"/sessions/start", s.handleSessionStart)
		s.mux.HandleFunc("POST "+prefix+"/sessions/end", s.handleSessionEnd)
		s.mux.HandleFunc("POST "+prefix+"/sessions/summary", s.handleSessionSummary)
		s.mux.HandleFunc("POST "+prefix+"/feedback", s.handleFeedback)
		s.mux.HandleFunc("POST "+prefix+"/suggest", s.handleSuggest)
		s.mux.HandleFunc("POST "+prefix+"/supersede/{id}", s.handleSupersede)
		s.mux.HandleFunc("GET "+prefix+"/events", s.handleEvents)
		s.mux.HandleFunc("GET "+prefix+"/documents", s.handleDocuments)
		s.mux.HandleFunc("GET "+prefix+"/ingestion-runs", s.handleIngestionRuns)
		s.mux.HandleFunc("POST "+prefix+"/ingest", s.handleIngest)
		s.mux.HandleFunc("POST "+prefix+"/chunks/search", s.handleChunkSearch)
		s.mux.HandleFunc("POST "+prefix+"/documents/{id}/suggest", s.handleDocumentSuggest)
		s.mux.HandleFunc("GET "+prefix+"/browse", s.handleBrowse)
		s.mux.HandleFunc("GET "+prefix+"/ingest/status", s.handleIngestStatus)
		s.mux.HandleFunc("POST "+prefix+"/memories/{id}/approve", s.handleApproveMemory)
		s.mux.HandleFunc("POST "+prefix+"/config", s.handleUpdateConfig)
		s.mux.HandleFunc("GET "+prefix+"/analytics/supersessions", s.handleSupersessionTimeline)
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
		Status:  memory.MemoryStatus(r.URL.Query().Get("status")),
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		q.Types = []memory.MemoryType{memory.MemoryType(typ)}
	}
	if scope := r.URL.Query().Get("scope"); scope != "" {
		q.Scopes = []memory.Scope{memory.Scope(scope)}
	}
	if truthy(r.URL.Query().Get("include_ranking")) {
		items, err := s.store.SearchRanked(r.Context(), q)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
		return
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
	if q.IncludeRanking {
		items, err := s.store.SearchRanked(r.Context(), q)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, items)
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

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	var req memory.SessionStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	if req.Project == "" {
		req.Project = memory.DetectProject("")
	}
	resp, err := s.store.StartSession(r.Context(), req.Project)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	var req memory.SessionEndRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	if req.Project == "" {
		req.Project = memory.DetectProject("")
	}
	resp, err := s.store.EndActiveSession(r.Context(), req.Project, req.Summary)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSessionSummary(w http.ResponseWriter, r *http.Request) {
	var req memory.SessionSummaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	if req.Project == "" && req.SessionID == "" {
		req.Project = memory.DetectProject("")
	}
	resp, err := s.store.SessionSummary(r.Context(), req)
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
	m.Content = memory.StripPrivateTags(m.Content)
	created, err := s.store.UpsertMemory(r.Context(), m)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	s.appendEvent(r, memory.Event{MemoryID: &created.ID, Kind: "memory.upserted", Payload: created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
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

func (s *Server) handleMemoryTimeline(w http.ResponseWriter, r *http.Request) {
	out, err := s.store.MemoryTimeline(r.Context(), r.PathValue("id"), atoiDefault(r.URL.Query().Get("limit"), 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleForget(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.Forget(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	s.appendEvent(r, memory.Event{MemoryID: &id, Kind: "memory.forgotten", Payload: id, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (s *Server) handleSupersede(w http.ResponseWriter, r *http.Request) {
	var m memory.Memory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	m.Content = memory.StripPrivateTags(m.Content)
	created, err := s.store.Supersede(r.Context(), r.PathValue("id"), m)
	if err != nil {
		writeError(w, err)
		return
	}
	oldID := r.PathValue("id")
	s.appendEvent(r, memory.Event{MemoryID: &oldID, Kind: "memory.superseded", Payload: oldID + " -> " + created.ID, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, created)
}

func (s *Server) handleDocumentSuggest(w http.ResponseWriter, r *http.Request) {
	var req memory.ChunkSuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	req.DocumentID = r.PathValue("id")
	resp, err := s.store.SuggestMemoriesFromDocument(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleChunkSearch(w http.ResponseWriter, r *http.Request) {
	var req memory.ChunkSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.store.SearchChunks(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleIngestionRuns(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListIngestionRuns(r.Context(), atoiDefault(r.URL.Query().Get("limit"), 100))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListDocuments(r.Context(), atoiDefault(r.URL.Query().Get("limit"), 200))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req memory.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.store.IngestPath(r.Context(), req)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListEvents(r.Context(), atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

type browseEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Path  string `json:"path"`
}

type browseResult struct {
	Path    string        `json:"path"`
	Parent  string        `json:"parent"`
	Entries []browseEntry `json:"entries"`
}

// handleIngestStatus checks a file path against the document store and reports
// whether the file is unchanged (hash matches), changed, or new.
func (s *Server) handleIngestStatus(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErrorStatus(w, http.StatusBadRequest, errors.New("path required"))
		return
	}
	status, err := s.store.IngestFileStatus(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleApproveMemory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.ApproveMemory(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	s.appendEvent(r, memory.Event{MemoryID: &id, Kind: "memory.approved", Payload: id, Source: memory.Source{Kind: "api", Ref: r.RemoteAddr}})
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var patch config.Config
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	// Merge patch into current config (preserve database path and auth)
	current := s.cfg
	if patch.Server.Addr != "" {
		current.Server.Addr = patch.Server.Addr
	}
	if patch.LLM.Provider != "" {
		current.LLM = patch.LLM
	}
	if patch.Embedding.Provider != "" {
		current.Embedding = patch.Embedding
	}
	if err := current.Validate(); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	cfgPath := config.DefaultConfigPath()
	b, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		writeError(w, err)
		return
	}
	if err := os.WriteFile(cfgPath, append(b, '\n'), 0o644); err != nil {
		writeError(w, err)
		return
	}
	s.cfg = current
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "path": cfgPath})
}

func (s *Server) handleSupersessionTimeline(w http.ResponseWriter, r *http.Request) {
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	rows, err := s.store.SupersessionTimeline(r.Context(), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeErrorStatus(w, http.StatusInternalServerError, err)
			return
		}
		path = home
	}
	path = filepath.Clean(path)

	des, err := os.ReadDir(path)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}

	result := browseResult{
		Path:    path,
		Parent:  filepath.Dir(path),
		Entries: make([]browseEntry, 0),
	}
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".") {
			continue
		}
		result.Entries = append(result.Entries, browseEntry{
			Name:  de.Name(),
			IsDir: de.IsDir(),
			Path:  filepath.Join(path, de.Name()),
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool {
		if result.Entries[i].IsDir != result.Entries[j].IsDir {
			return result.Entries[i].IsDir
		}
		return result.Entries[i].Name < result.Entries[j].Name
	})
	writeJSON(w, http.StatusOK, result)
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

func truthy(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "1" || s == "true" || s == "yes" || s == "on"
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

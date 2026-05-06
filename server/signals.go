package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/salemarsm/ginko/memory"
)

func (s *Server) handleCreateSignal(w http.ResponseWriter, r *http.Request) {
	var sig memory.AgentSignal
	if err := json.NewDecoder(r.Body).Decode(&sig); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	if sig.Project == "" {
		sig.Project = memory.DetectProject("")
	}
	created, err := s.store.CreateSignal(r.Context(), sig)
	if err != nil {
		writeErrorStatus(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListSignals(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		project = memory.DetectProject("")
	}
	q := memory.SignalQuery{
		Project: project,
		Kind:    memory.SignalKind(r.URL.Query().Get("kind")),
		Status:  memory.SignalStatus(r.URL.Query().Get("status")),
		Agent:   r.URL.Query().Get("agent"),
		Limit:   atoiDefault(r.URL.Query().Get("limit"), 100),
	}
	sigs, err := s.store.ListSignals(r.Context(), q)
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, err)
		return
	}
	if sigs == nil {
		sigs = []memory.AgentSignal{}
	}
	writeJSON(w, http.StatusOK, sigs)
}

func (s *Server) handleGetSignal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sig, err := s.store.GetSignal(r.Context(), id)
	if errors.Is(err, memory.ErrSignalNotFound) {
		writeErrorStatus(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sig)
}

func (s *Server) handleSignalAcknowledge(w http.ResponseWriter, r *http.Request) {
	s.transitionSignal(w, r, memory.SignalStatusAcknowledged)
}

func (s *Server) handleSignalResolve(w http.ResponseWriter, r *http.Request) {
	s.transitionSignal(w, r, memory.SignalStatusResolved)
}

func (s *Server) handleSignalCancel(w http.ResponseWriter, r *http.Request) {
	s.transitionSignal(w, r, memory.SignalStatusCancelled)
}

func (s *Server) transitionSignal(w http.ResponseWriter, r *http.Request, status memory.SignalStatus) {
	id := r.PathValue("id")
	sig, err := s.store.UpdateSignalStatus(r.Context(), id, status)
	if errors.Is(err, memory.ErrSignalNotFound) {
		writeErrorStatus(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sig)
}

func (s *Server) handleExpireSignals(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.ExpireStaleSignals(r.Context())
	if err != nil {
		writeErrorStatus(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"expired": n})
}

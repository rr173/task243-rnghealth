package httpapi

import (
	"net/http"
)

func (s *Server) handleAnalysis(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	a, err := s.svc.Analysis(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleGlobalStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.svc.GlobalStats()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

package httpapi

import (
	"encoding/json"
	"net/http"
)

type eventNoteReq struct {
	Note string `json:"note"`
}

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	sourceID := int64(0)
	if v := r.URL.Query().Get("source_id"); v != "" {
		if n, err := parseInt64(v); err == nil {
			sourceID = n
		}
	}
	state := r.URL.Query().Get("state")
	limit := parseQueryInt(r, "limit", 50)
	offset := parseQueryInt(r, "offset", 0)
	events, err := s.svc.ListEvents(sourceID, state, limit, offset)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events, "limit": limit, "offset": offset})
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	ev, err := s.svc.GetEvent(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) handleConfirmEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	var req eventNoteReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	ev, err := s.svc.ConfirmEvent(id, req.Note, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

func (s *Server) handleCloseEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	var req eventNoteReq
	_ = json.NewDecoder(r.Body).Decode(&req)
	ev, err := s.svc.CloseEvent(id, req.Note, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ev)
}

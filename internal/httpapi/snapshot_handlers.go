package httpapi

import (
	"encoding/json"
	"net/http"
)

type draftSnapshotReq struct {
	SourceID int64 `json:"source_id"`
}

func (s *Server) handleDraftSnapshot(w http.ResponseWriter, r *http.Request) {
	var req draftSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	if req.SourceID <= 0 {
		s.fail(w, errInvalid("source_id must be positive"))
		return
	}
	snap, err := s.svc.DraftSnapshot(req.SourceID, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	sourceID := int64(0)
	if v := r.URL.Query().Get("source_id"); v != "" {
		if n, err := parseInt64(v); err == nil {
			sourceID = n
		}
	}
	limit := parseQueryInt(r, "limit", 50)
	offset := parseQueryInt(r, "offset", 0)
	snaps, err := s.svc.ListSnapshots(sourceID, limit, offset)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"snapshots": snaps, "limit": limit, "offset": offset})
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	snap, err := s.svc.GetSnapshot(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handlePublishSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	snap, err := s.svc.PublishSnapshot(id, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleSupersedeSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	snap, err := s.svc.SupersedeSnapshot(id, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

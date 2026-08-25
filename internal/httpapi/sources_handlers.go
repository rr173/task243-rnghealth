package httpapi

import (
	"encoding/json"
	"net/http"
)

type registerSourceReq struct {
	Name   string `json:"name"`
	Device string `json:"device"`
}

func (s *Server) handleRegisterSource(w http.ResponseWriter, r *http.Request) {
	var req registerSourceReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	id, err := s.svc.RegisterSource(req.Name, req.Device, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int64{"id": id})
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	limit := parseQueryInt(r, "limit", 50)
	offset := parseQueryInt(r, "offset", 0)
	sources, err := s.svc.ListSources(limit, offset)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"sources": sources, "limit": limit, "offset": offset})
}

func (s *Server) handleGetSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	src, err := s.svc.GetSource(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, src)
}

func (s *Server) handleDegradeSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.svc.DegradeSource(id, now()); err != nil {
		s.fail(w, err)
		return
	}
	s.writeSourceState(w, id)
}

func (s *Server) handleIsolateSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.svc.IsolateSource(id, now()); err != nil {
		s.fail(w, err)
		return
	}
	s.writeSourceState(w, id)
}

func (s *Server) handleSealSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.svc.SealSource(id, now()); err != nil {
		s.fail(w, err)
		return
	}
	s.writeSourceState(w, id)
}

func (s *Server) handleRecoverSource(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.svc.RecoverSource(id, now()); err != nil {
		s.fail(w, err)
		return
	}
	s.writeSourceState(w, id)
}

type restartReq struct {
	AtSeq       int64 `json:"at_seq"`
	BaselineSeq int64 `json:"baseline_seq"`
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	var req restartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	if req.AtSeq <= 0 {
		s.fail(w, errInvalid("at_seq must be positive"))
		return
	}
	rb, err := s.svc.RecordRestart(id, req.AtSeq, req.BaselineSeq, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rb)
}

// writeSourceState 写回熵源当前状态。
func (s *Server) writeSourceState(w http.ResponseWriter, id int64) {
	src, err := s.svc.GetSource(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"id": src.ID, "state": src.State})
}

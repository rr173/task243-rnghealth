package httpapi

import (
	"encoding/json"
	"net/http"

	"task243-rnghealth/internal/health"
)

type reevaluateReq struct {
	WindowID int64 `json:"window_id"`
}

func (s *Server) handleReevaluate(w http.ResponseWriter, r *http.Request) {
	var req reevaluateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	if req.WindowID <= 0 {
		s.fail(w, errInvalid("window_id must be positive"))
		return
	}
	win, err := s.svc.ReevaluateWindow(req.WindowID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, win)
}

func (s *Server) handleHealthCategories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": health.CategoryMetaList(),
	})
}

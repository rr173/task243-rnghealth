package httpapi

import (
	"net/http"
)

func (s *Server) handleSelfCheck(w http.ResponseWriter, r *http.Request) {
	res := s.svc.SelfCheck()
	code := http.StatusOK
	if !res.OK {
		code = http.StatusInternalServerError
	}
	writeJSON(w, code, res)
}

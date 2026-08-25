package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"task243-rnghealth/internal/ingest"
)

type ingestWindowReq struct {
	SourceID int64  `json:"source_id"`
	Seq      int64  `json:"seq"`
	Sample   string `json:"sample"` // base64 编码的字节样本
}

func (s *Server) handleIngestWindow(w http.ResponseWriter, r *http.Request) {
	var req ingestWindowReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	if req.SourceID <= 0 || req.Seq <= 0 {
		s.fail(w, errInvalid("source_id and seq must be positive"))
		return
	}
	sample, err := base64.StdEncoding.DecodeString(req.Sample)
	if err != nil {
		s.fail(w, errInvalid("sample must be base64"))
		return
	}
	win, err := s.svc.IngestWindow(req.SourceID, req.Seq, sample, now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, win)
}

type batchWindowItem struct {
	Seq    int64  `json:"seq"`
	Sample string `json:"sample"`
}

type batchWindowsReq struct {
	SourceID int64             `json:"source_id"`
	Items    []batchWindowItem `json:"items"`
}

func (s *Server) handleBatchWindows(w http.ResponseWriter, r *http.Request) {
	var req batchWindowsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, err)
		return
	}
	if req.SourceID <= 0 {
		s.fail(w, errInvalid("source_id must be positive"))
		return
	}
	items := make([]ingest.BatchItem, 0, len(req.Items))
	for _, it := range req.Items {
		sample, err := base64.StdEncoding.DecodeString(it.Sample)
		if err != nil {
			s.fail(w, errInvalid("sample must be base64"))
			return
		}
		items = append(items, ingest.BatchItem{Sample: sample, Seq: it.Seq})
	}
	wins, errs := s.svc.BatchWindows(req.SourceID, items, now())
	results := make([]map[string]interface{}, 0, len(wins))
	for i := range wins {
		if errs[i] != nil {
			continue
		}
		results = append(results, map[string]interface{}{"window": wins[i]})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"results": results, "count": len(results)})
}

func (s *Server) handleListWindows(w http.ResponseWriter, r *http.Request) {
	sourceID := int64(0)
	if v := r.URL.Query().Get("source_id"); v != "" {
		if n, err := parseInt64(v); err == nil {
			sourceID = n
		}
	}
	status := r.URL.Query().Get("status")
	limit := parseQueryInt(r, "limit", 50)
	offset := parseQueryInt(r, "offset", 0)
	wins, err := s.svc.ListWindows(sourceID, status, limit, offset)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"windows": wins, "limit": limit, "offset": offset})
}

func (s *Server) handleGetWindow(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	win, err := s.svc.GetWindow(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, win)
}

func (s *Server) handleWindowTests(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		s.fail(w, err)
		return
	}
	tests, err := s.svc.ListWindowTests(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"window_id": id, "tests": tests})
}

// parseInt64 解析字符串为 int64（用于查询参数）。
func parseInt64(v string) (int64, error) {
	return strconv.ParseInt(v, 10, 64)
}

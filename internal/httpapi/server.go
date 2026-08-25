package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
)

// Server 持有服务并注册路由。
type Server struct {
	svc *service.Service
}

// New 构造 HTTP 服务。
func New(svc *service.Service) *Server { return &Server{svc: svc} }

// Routes 注册全部 /api 路由（Go 1.22+ 方法路由）。
func (s *Server) Routes() *http.ServeMux {
	m := http.NewServeMux()

	// 熵源
	m.HandleFunc("POST /api/entropy-sources", s.handleRegisterSource)
	m.HandleFunc("GET /api/entropy-sources", s.handleListSources)
	m.HandleFunc("GET /api/entropy-sources/{id}", s.handleGetSource)
	m.HandleFunc("POST /api/entropy-sources/{id}/degrade", s.handleDegradeSource)
	m.HandleFunc("POST /api/entropy-sources/{id}/isolate", s.handleIsolateSource)
	m.HandleFunc("POST /api/entropy-sources/{id}/seal", s.handleSealSource)
	m.HandleFunc("POST /api/entropy-sources/{id}/recover", s.handleRecoverSource)
	m.HandleFunc("POST /api/entropy-sources/{id}/restart", s.handleRestart)

	// 窗口
	m.HandleFunc("POST /api/windows", s.handleIngestWindow)
	m.HandleFunc("POST /api/windows/batch", s.handleBatchWindows)
	m.HandleFunc("GET /api/windows", s.handleListWindows)
	m.HandleFunc("GET /api/windows/{id}", s.handleGetWindow)
	m.HandleFunc("GET /api/windows/{id}/tests", s.handleWindowTests)

	// 健康
	m.HandleFunc("POST /api/health/reevaluate", s.handleReevaluate)
	m.HandleFunc("GET /api/health/categories", s.handleHealthCategories)

	// 事件
	m.HandleFunc("GET /api/events", s.handleListEvents)
	m.HandleFunc("GET /api/events/{id}", s.handleGetEvent)
	m.HandleFunc("POST /api/events/{id}/confirm", s.handleConfirmEvent)
	m.HandleFunc("POST /api/events/{id}/close", s.handleCloseEvent)

	// 分析与快照
	m.HandleFunc("GET /api/sources/{id}/analysis", s.handleAnalysis)
	m.HandleFunc("GET /api/stats", s.handleGlobalStats)
	m.HandleFunc("POST /api/snapshots", s.handleDraftSnapshot)
	m.HandleFunc("GET /api/snapshots", s.handleListSnapshots)
	m.HandleFunc("GET /api/snapshots/{id}", s.handleGetSnapshot)
	m.HandleFunc("POST /api/snapshots/{id}/publish", s.handlePublishSnapshot)
	m.HandleFunc("POST /api/snapshots/{id}/supersede", s.handleSupersedeSnapshot)

	// 自检
	m.HandleFunc("GET /api/selfcheck", s.handleSelfCheck)

	return m
}

// writeJSON 写 JSON 响应。
func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// fail 将领域错误映射为 HTTP 状态并写错误体。
func (s *Server) fail(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case model.IsNotFound(err):
		code = http.StatusNotFound
	case model.IsConflict(err), model.IsSealed(err), model.IsTransition(err):
		code = http.StatusConflict
	case model.IsInvalidArgument(err):
		code = http.StatusBadRequest
	case errors.Is(err, io.EOF):
		code = http.StatusBadRequest
	case errors.Is(err, io.ErrUnexpectedEOF):
		code = http.StatusBadRequest
	default:
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			code = http.StatusBadRequest
		}
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// parseID 从路径解析 int64 ID。
func parseID(r *http.Request, key string) (int64, error) {
	v := r.PathValue(key)
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, model.ErrInvalidArgument
	}
	return n, nil
}

// parseQueryInt 解析查询参数整数，带默认值。
func parseQueryInt(r *http.Request, key string, def int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// now 统一时间源。
func now() time.Time { return time.Now() }

// errInvalid 构造一个参数非法错误（带消息），被 fail 映射为 400。
func errInvalid(msg string) error {
	return fmt.Errorf("%w: %s", model.ErrInvalidArgument, msg)
}

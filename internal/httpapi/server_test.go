package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task243-rnghealth/internal/httpapi"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestRoutesRegisterSourceAndExposeSelfCheck(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	handler := httpapi.New(service.New(db)).Routes()

	register := httptest.NewRequest(http.MethodPost, "/api/entropy-sources", strings.NewReader(`{"name":"api-test","device":"device-3"}`))
	register.Header.Set("Content-Type", "application/json")
	registerResp := httptest.NewRecorder()
	handler.ServeHTTP(registerResp, register)
	if registerResp.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(registerResp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("created response = %+v", created)
	}

	selfCheck := httptest.NewRequest(http.MethodGet, "/api/selfcheck", nil)
	selfCheckResp := httptest.NewRecorder()
	handler.ServeHTTP(selfCheckResp, selfCheck)
	if selfCheckResp.Code != http.StatusOK {
		t.Fatalf("selfcheck status = %d, body = %s", selfCheckResp.Code, selfCheckResp.Body.String())
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(selfCheckResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode selfcheck response: %v", err)
	}
	if !result.OK {
		t.Fatalf("selfcheck response = %s", selfCheckResp.Body.String())
	}
}

// TestRegisterSourceConflictReportsExistingID 验证同名同设备重复注册时，
// 服务以 409 明确报告冲突并附上已存在 ID，而非再次以 201 成功响应。
func TestRegisterSourceConflictReportsExistingID(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	handler := httpapi.New(service.New(db)).Routes()

	body := `{"name":"conflict-test","device":"device-conflict"}`
	register := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/entropy-sources", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	first := register()
	if first.Code != http.StatusCreated {
		t.Fatalf("first register status = %d, body = %s", first.Code, first.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode first response: %v", err)
	}

	second := register()
	if second.Code != http.StatusConflict {
		t.Fatalf("second register status = %d, want %d, body = %s",
			second.Code, http.StatusConflict, second.Body.String())
	}
	var conf struct {
		Error      string `json:"error"`
		ExistingID int64  `json:"existing_id"`
	}
	if err := json.Unmarshal(second.Body.Bytes(), &conf); err != nil {
		t.Fatalf("decode conflict response: %v", err)
	}
	if conf.ExistingID != created.ID {
		t.Fatalf("conflict existing_id = %d, want %d", conf.ExistingID, created.ID)
	}
}

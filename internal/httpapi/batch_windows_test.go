package httpapi_test

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"task243-rnghealth/internal/httpapi"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

// TestBatchWindowsPreservesPositionsAroundDuplicateSeq 验证批量摄入响应中，
// 每个输入项按位置返回成功或错误结果；中间重复序号失败不丢失后续合法窗口，
// 也不丢失错误项的位置信息。
func TestBatchWindowsPreservesPositionsAroundDuplicateSeq(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	handler := httpapi.New(service.New(db)).Routes()

	// 注册熵源。
	register := httptest.NewRequest(http.MethodPost, "/api/entropy-sources", strings.NewReader(`{"name":"batch-api","device":"device-batch"}`))
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

	goodB64 := func(seed byte) string {
		s := make([]byte, 256)
		for i := range s {
			s[i] = byte(i) ^ seed
		}
		return base64.StdEncoding.EncodeToString(s)
	}

	// 序号 1（合法）→ 1（重复）→ 2（合法）→ 3（合法）。
	body := strings.NewReader(`{"source_id":` + strconv.FormatInt(created.ID, 10) + `,"items":[` +
		`{"seq":1,"sample":"` + goodB64(0) + `"},` +
		`{"seq":1,"sample":"` + goodB64(1) + `"},` +
		`{"seq":2,"sample":"` + goodB64(2) + `"},` +
		`{"seq":3,"sample":"` + goodB64(3) + `"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/windows/batch", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Results []struct {
			OK     bool        `json:"ok"`
			Seq    int64       `json:"seq"`
			Window interface{} `json:"window"`
			Error  string      `json:"error"`
		} `json:"results"`
		Count int `json:"count"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode batch response: %v\n%s", err, rec.Body.String())
	}

	if resp.Total != 4 {
		t.Fatalf("total = %d, want 4 (one per input item)", resp.Total)
	}
	if len(resp.Results) != 4 {
		t.Fatalf("results length = %d, want 4 (position-aligned)", len(resp.Results))
	}
	if resp.Count != 3 {
		t.Fatalf("count = %d, want 3 succeeded", resp.Count)
	}

	// 位置 0：成功。
	if !resp.Results[0].OK || resp.Results[0].Seq != 1 || resp.Results[0].Window == nil {
		t.Fatalf("position 0 = %+v, want success seq 1", resp.Results[0])
	}
	// 位置 1：重复序号失败，且保留位置与 seq。
	if resp.Results[1].OK || resp.Results[1].Seq != 1 || resp.Results[1].Error == "" {
		t.Fatalf("position 1 = %+v, want error seq 1", resp.Results[1])
	}
	// 位置 2：重复失败之后的合法窗口仍返回成功。
	if !resp.Results[2].OK || resp.Results[2].Seq != 2 || resp.Results[2].Window == nil {
		t.Fatalf("position 2 = %+v, want success seq 2", resp.Results[2])
	}
	// 位置 3：继续处理。
	if !resp.Results[3].OK || resp.Results[3].Seq != 3 || resp.Results[3].Window == nil {
		t.Fatalf("position 3 = %+v, want success seq 3", resp.Results[3])
	}
}

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

func TestBug09BatchContinuesAfterPerItemConflict(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	handler := httpapi.New(service.New(db)).Routes()

	register := httptest.NewRecorder()
	handler.ServeHTTP(register, httptest.NewRequest(http.MethodPost, "/api/entropy-sources", strings.NewReader("{\"name\":\"batch-source\",\"device\":\"device-09\"}")))
	if register.Code != http.StatusCreated { t.Fatalf("register status=%d body=%s", register.Code, register.Body.String()) }
	var registered map[string]int64
	if err := json.Unmarshal(register.Body.Bytes(), &registered); err != nil { t.Fatal(err) }
	sourceID := registered["id"]
	sample := base64.StdEncoding.EncodeToString(make([]byte, 256))
	body := "{\"source_id\":" + strconv.FormatInt(sourceID, 10) + ",\"items\":[{\"seq\":1,\"sample\":\"" + sample + "\"},{\"seq\":1,\"sample\":\"" + sample + "\"},{\"seq\":2,\"sample\":\"" + sample + "\"}]}"
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/windows/batch", strings.NewReader(body)))
	if resp.Code != http.StatusOK { t.Fatalf("batch status=%d body=%s", resp.Code, resp.Body.String()) }
	var payload struct { Results []map[string]interface{} `json:"results"` }
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
	if len(payload.Results) != 3 { t.Fatalf("results=%d body=%s, want one result per input item", len(payload.Results), resp.Body.String()) }
	if _, ok := payload.Results[1]["error"]; !ok { t.Fatalf("middle result=%v, want conflict error", payload.Results[1]) }
	if payload.Results[2]["window"] == nil { t.Fatalf("third result=%v, want successful window", payload.Results[2]) }

	list := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/windows?source_id="+strconv.FormatInt(sourceID, 10), nil)
	handler.ServeHTTP(list, req)
	var windows struct { Windows []map[string]interface{} `json:"windows"` }
	if err := json.Unmarshal(list.Body.Bytes(), &windows); err != nil { t.Fatal(err) }
	if len(windows.Windows) != 2 { t.Fatalf("persisted windows=%d, want 2", len(windows.Windows)) }
}

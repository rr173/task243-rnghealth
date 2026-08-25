package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/ingest"
	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

// TestServiceBatchContinuesPastDuplicateSeq 验证批量摄入中，中间一个重复序号失败
// 不会让后续合法窗口被跳过：结果与入参等长且按位置对齐，后续窗口仍持久化。
func TestServiceBatchContinuesPastDuplicateSeq(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	now := time.Unix(3000, 0)
	sourceID, err := svc.RegisterSource("batch-test", "device-b", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}

	good := func(seed byte) []byte {
		s := make([]byte, 256)
		for i := range s {
			s[i] = byte(i) ^ seed
		}
		return s
	}

	items := []ingest.BatchItem{
		{Seq: 1, Sample: good(0)},
		{Seq: 1, Sample: good(1)}, // 重复序号，应失败（中间项）
		{Seq: 2, Sample: good(2)}, // 合法，应在重复失败后仍被处理并落库
		{Seq: 3, Sample: good(3)}, // 合法，应继续处理
	}

	wins, errs := svc.BatchWindows(sourceID, items, now)

	if len(wins) != len(items) || len(errs) != len(items) {
		t.Fatalf("result length = wins:%d errs:%d, want %d (position-aligned, equal to input)", len(wins), len(errs), len(items))
	}

	// 位置 0：成功
	if errs[0] != nil || wins[0] == nil {
		t.Fatalf("position 0: err=%v win=%v, want success", errs[0], wins[0])
	}
	if wins[0].Seq != 1 {
		t.Fatalf("position 0 seq = %d, want 1", wins[0].Seq)
	}
	// 位置 1：重复序号失败，且无窗口产出
	if !model.IsConflict(errs[1]) {
		t.Fatalf("position 1: err=%v, want conflict", errs[1])
	}
	if wins[1] != nil {
		t.Fatalf("position 1: window = %+v, want nil", wins[1])
	}
	// 位置 2：重复失败之后仍处理并落库
	if errs[2] != nil || wins[2] == nil {
		t.Fatalf("position 2: err=%v win=%v, want success after failure", errs[2], wins[2])
	}
	if wins[2].Seq != 2 {
		t.Fatalf("position 2 seq = %d, want 2", wins[2].Seq)
	}
	// 位置 3：继续处理
	if errs[3] != nil || wins[3] == nil {
		t.Fatalf("position 3: err=%v win=%v, want success", errs[3], wins[3])
	}
	if wins[3].Seq != 3 {
		t.Fatalf("position 3 seq = %d, want 3", wins[3].Seq)
	}

	// 源末序号应推进到最后一个合法序号 3。
	src, err := svc.GetSource(sourceID)
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	if src.LastSeq != 3 {
		t.Fatalf("last seq = %d, want 3", src.LastSeq)
	}

	// 后续合法窗口确已持久化。
	listed, err := svc.ListWindows(sourceID, "", 100, 0)
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("persisted window count = %d, want 3 (dup not persisted)", len(listed))
	}
}

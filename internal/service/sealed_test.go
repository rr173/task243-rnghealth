package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

// TestSealedSourceRejectsAllEvidenceWrites 断言熵源封存后，诊断证据冻结：
// 采样窗口、设备重启记录、新的诊断快照均被拒绝。
func TestSealedSourceRejectsAllEvidenceWrites(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	now := time.Unix(5000, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("sealed-test", "device-sealed", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	// 摄入一窗以建立证据并推进到观察态。
	sample := make([]byte, 256)
	for i := range sample {
		sample[i] = byte((i * 31) % 256)
	}
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil {
		t.Fatalf("ingest pre-seal window: %v", err)
	}

	// 记录封存前的窗口/重启/快照计数，用于事后断言未新增。
	winsBefore, err := svc.ListWindows(sourceID, "", 1000, 0)
	if err != nil {
		t.Fatalf("list windows before seal: %v", err)
	}
	windowsBefore := len(winsBefore)
	snapsBefore, err := svc.ListSnapshots(sourceID, 1000, 0)
	if err != nil {
		t.Fatalf("list snapshots before seal: %v", err)
	}
	snapsBeforeN := len(snapsBefore)

	// 封存：诊断完成，证据冻结。
	if err := svc.SealSource(sourceID, now.Add(time.Second)); err != nil {
		t.Fatalf("seal source: %v", err)
	}
	src, err := svc.GetSource(sourceID)
	if err != nil {
		t.Fatalf("get source after seal: %v", err)
	}
	if !src.IsSealed() {
		t.Fatalf("source not sealed: state=%s", src.State)
	}

	// (1) 封存后摄入新采样窗口应被拒绝。
	if _, err := svc.IngestWindow(sourceID, 2, sample, now.Add(2*time.Second)); !model.IsSealed(err) {
		t.Fatalf("ingest after seal: err = %v, want ErrSealed", err)
	}

	// (2) 封存后记录设备重启边界应被拒绝。
	if _, err := svc.RecordRestart(sourceID, 2, 0, now.Add(2*time.Second)); !model.IsSealed(err) {
		t.Fatalf("record restart after seal: err = %v, want ErrSealed", err)
	}

	// (3) 封存后创建新的诊断快照应被拒绝（替代路径 Draft 同样会被拒绝）。
	if _, err := svc.DraftSnapshot(sourceID, now.Add(2*time.Second)); !model.IsSealed(err) {
		t.Fatalf("draft snapshot after seal: err = %v, want ErrSealed", err)
	}

	// 证据确未新增：窗口数与快照数与封存前一致。
	winsAfter, err := svc.ListWindows(sourceID, "", 1000, 0)
	if err != nil {
		t.Fatalf("list windows after seal: %v", err)
	}
	if len(winsAfter) != windowsBefore {
		t.Fatalf("windows after seal = %d, want %d (evidence must be frozen)", len(winsAfter), windowsBefore)
	}
	snapsAfter, err := svc.ListSnapshots(sourceID, 1000, 0)
	if err != nil {
		t.Fatalf("list snapshots after seal: %v", err)
	}
	if len(snapsAfter) != snapsBeforeN {
		t.Fatalf("snapshots after seal = %d, want %d (evidence must be frozen)", len(snapsAfter), snapsBeforeN)
	}
}

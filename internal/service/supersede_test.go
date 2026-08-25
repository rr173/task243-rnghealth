package service_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

// goodSample 由确定性字节构成 256 字节窗口，满足最小长度。
func goodSample(seed byte) []byte {
	out := make([]byte, 256)
	for i := range out {
		out[i] = byte(i*7) ^ seed
	}
	return out
}

// publishFirst 发布某熵源的首个不可变快照，供替代语义测试复用。
func publishFirst(t *testing.T, svc *service.Service, sourceID int64, now time.Time) *model.DiagnosticSnapshot {
	t.Helper()
	if _, err := svc.IngestWindow(sourceID, 1, goodSample(1), now); err != nil {
		t.Fatalf("ingest window: %v", err)
	}
	draft, err := svc.DraftSnapshot(sourceID, now)
	if err != nil {
		t.Fatalf("draft snapshot: %v", err)
	}
	pub, err := svc.PublishSnapshot(draft.ID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	return pub
}

// TestServiceSnapshotSupersedeConsistent 验证替代后新旧版本状态与替代关系一致：
// 新版本 published、旧版本 superseded、旧版本 superseded_by 指向新版本，
// 且已被替代的旧版本不能再被替代（同一旧版本只能被一个新版本替代）。
func TestServiceSnapshotSupersedeConsistent(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	now := time.Unix(5000, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("supersede-consistent", "device-sc", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	old := publishFirst(t, svc, sourceID, now)

	newSnap, err := svc.SupersedeSnapshot(old.ID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("supersede snapshot: %v", err)
	}
	if newSnap.State != model.SnapshotStatePublished {
		t.Fatalf("new snapshot state = %s, want published", newSnap.State)
	}
	reloadedOld, err := svc.GetSnapshot(old.ID)
	if err != nil {
		t.Fatalf("reload old snapshot: %v", err)
	}
	if reloadedOld.State != model.SnapshotStateSuperseded {
		t.Fatalf("old snapshot state = %s, want superseded", reloadedOld.State)
	}
	if reloadedOld.SupersededBy == nil || *reloadedOld.SupersededBy != newSnap.ID {
		t.Fatalf("old superseded_by = %v, want %d", reloadedOld.SupersededBy, newSnap.ID)
	}

	// 已被替代的旧版本不可再次替代。
	if _, err := svc.SupersedeSnapshot(old.ID, now.Add(2*time.Second)); !errors.Is(err, model.ErrTransition) {
		t.Fatalf("re-supersede already superseded = %v, want ErrTransition", err)
	}
}

// TestServiceSnapshotConcurrentSupersedeSingleWinner 验证多个工作流并发替代同一
// 已发布快照时，仅一个成功；其余得到 ErrTransition，不会产生多个成功的替代版本，
// 旧版本被唯一新版本替代。
func TestServiceSnapshotConcurrentSupersedeSingleWinner(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	now := time.Unix(9000, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("supersede-concurrent", "device-cc", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	old := publishFirst(t, svc, sourceID, now)

	const workers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		success []*model.DiagnosticSnapshot
		failed  int
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			snap, err := svc.SupersedeSnapshot(old.ID, now.Add(time.Second))
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				success = append(success, snap)
			} else if errors.Is(err, model.ErrTransition) {
				failed++
			}
		}()
	}
	wg.Wait()

	if len(success) != 1 {
		t.Fatalf("concurrent supersede produced %d winners, want exactly 1", len(success))
	}
	if failed != workers-1 {
		t.Fatalf("concurrent supersede failures = %d, want %d", failed, workers-1)
	}
	winner := success[0]
	if winner.State != model.SnapshotStatePublished {
		t.Fatalf("winner state = %s, want published", winner.State)
	}
	reloadedOld, err := svc.GetSnapshot(old.ID)
	if err != nil {
		t.Fatalf("reload old snapshot: %v", err)
	}
	if reloadedOld.State != model.SnapshotStateSuperseded {
		t.Fatalf("old snapshot state = %s, want superseded", reloadedOld.State)
	}
	if reloadedOld.SupersededBy == nil || *reloadedOld.SupersededBy != winner.ID {
		t.Fatalf("old superseded_by = %v, want %d", reloadedOld.SupersededBy, winner.ID)
	}
	// 没有悬挂的新版本：除旧版本外只有唯一胜出的新版本处于 published 态。
	all, err := svc.ListSnapshots(sourceID, 100, 0)
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	publishedN := 0
	for _, s := range all {
		if s.State == model.SnapshotStatePublished {
			publishedN++
		}
	}
	if publishedN != 1 {
		t.Fatalf("published snapshots = %d, want 1", publishedN)
	}
}

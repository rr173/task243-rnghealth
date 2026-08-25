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

// TestRegisterSourceConcurrentKeepsSingleAndReportsConflict 验证：当安全芯片工程师
// 同时以同一名称与设备提交多次熵源注册时，服务只保留一个注册结果，其余请求
// 明确报告冲突（携带已存在 ID），而非各自成功创建多个熵源。
func TestRegisterSourceConcurrentKeepsSingleAndReportsConflict(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(5000, 0)

	const concurrency = 16
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		ids     = make(map[int64]int)
		confIDs = make(map[int64]int)
		confErrs int
	)
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			id, err := svc.RegisterSource("concurrent-trng", "dev-conflict", now)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ids[id]++
				return
			}
			var ce *model.ConflictError
			if errors.As(err, &ce) {
				confErrs++
				confIDs[ce.ExistingID]++
			}
		}()
	}
	wg.Wait()

	// 仅允许一个成功创建。
	if len(ids) != 1 {
		t.Fatalf("expected exactly one successful registration, got %d distinct IDs: %v", len(ids), ids)
	}
	// 其余必须以携带已存在 ID 的冲突明确报告。
	if confErrs != concurrency-1 {
		t.Fatalf("expected %d conflict errors, got %d (success=%v, conflicts=%v)",
			concurrency-1, confErrs, ids, confIDs)
	}
	// 冲突报告的已存在 ID 必须指向唯一成功创建的那个熵源。
	successID := int64(0)
	for id := range ids {
		successID = id
	}
	for id := range confIDs {
		if id != successID {
			t.Fatalf("conflict reported existing_id=%d, want %d (the single created source)", id, successID)
		}
	}

	// 数据库中确实只落库一个同名同设备熵源。
	sources, err := svc.ListSources(100, 0)
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	matched := 0
	for _, src := range sources {
		if src.Name == "concurrent-trng" && src.Device == "dev-conflict" {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("expected exactly one persisted source for name+device, got %d", matched)
	}
}

// TestRegisterSourceSequentialDuplicateReportsConflict 验证顺序重复注册也明确报告冲突。
func TestRegisterSourceSequentialDuplicateReportsConflict(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(6000, 0)

	first, err := svc.RegisterSource("dup-trng", "dev-dup", now)
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	second, err := svc.RegisterSource("dup-trng", "dev-dup", now)
	if err == nil {
		t.Fatalf("second register returned id=%d, want conflict", second)
	}
	var ce *model.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("second register error = %v, want *model.ConflictError", err)
	}
	if ce.ExistingID != first {
		t.Fatalf("conflict existing_id = %d, want %d", ce.ExistingID, first)
	}
	if !model.IsConflict(err) {
		t.Fatalf("conflict error not recognized by IsConflict: %v", err)
	}
}

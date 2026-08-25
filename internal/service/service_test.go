package service_test

import (
	"sync"
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestServiceSnapshotSurvivesDatabaseReopen(t *testing.T) {
	dbPath := t.TempDir() + "/rnghealth.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	now := time.Unix(2000, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("snapshot-test", "device-2", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	sample := make([]byte, 256)
	for i := range sample {
		sample[i] = byte((i * 17) % 256)
	}
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil {
		t.Fatalf("ingest window: %v", err)
	}
	draft, err := svc.DraftSnapshot(sourceID, now)
	if err != nil {
		t.Fatalf("draft snapshot: %v", err)
	}
	published, err := svc.PublishSnapshot(draft.ID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("publish snapshot: %v", err)
	}
	if published.State != model.SnapshotStatePublished {
		t.Fatalf("snapshot state = %s, want published", published.State)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	db2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer db2.Close()
	svc2 := service.New(db2)
	source, err := svc2.GetSource(sourceID)
	if err != nil {
		t.Fatalf("read source after reopen: %v", err)
	}
	if source.LastSeq != 1 {
		t.Fatalf("last sequence after reopen = %d, want 1", source.LastSeq)
	}
	snapshot, err := svc2.GetSnapshot(published.ID)
	if err != nil {
		t.Fatalf("read snapshot after reopen: %v", err)
	}
	if snapshot.State != model.SnapshotStatePublished || snapshot.Payload == "" {
		t.Fatalf("snapshot after reopen = %+v", snapshot)
	}
}

// TestServiceSnapshotPublishOnceConcurrent 验证同一草稿在多个工作流并发发布时：
// 仅有一个请求成功，其余得到冲突；且发布后的 state 与 published_at 不可变。
func TestServiceSnapshotPublishOnceConcurrent(t *testing.T) {
	dbPath := t.TempDir() + "/rnghealth.db"
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	now := time.Unix(2000, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("concurrent-publish", "device-x", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	sample := make([]byte, 256)
	for i := range sample {
		sample[i] = byte((i * 17) % 256)
	}
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil {
		t.Fatalf("ingest window: %v", err)
	}
	draft, err := svc.DraftSnapshot(sourceID, now)
	if err != nil {
		t.Fatalf("draft snapshot: %v", err)
	}

	const workers = 8
	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		ok      int
		conflict int
		mu      sync.Mutex
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // 同时触发，最大化竞争窗口。
			published, err := svc.PublishSnapshot(draft.ID, now.Add(time.Duration(i)*time.Second))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
				if published.PublishedAt == nil {
					t.Errorf("worker %d: published_at missing", i)
				}
			case model.IsTransition(err):
				conflict++
			default:
				t.Errorf("worker %d: unexpected error: %v", i, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if ok != 1 {
		t.Fatalf("expected exactly 1 successful publish, got %d (conflicts=%d)", ok, conflict)
	}
	if ok+conflict != workers {
		t.Fatalf("expected %d total outcomes, got %d", workers, ok+conflict)
	}

	// 已发布快照不可变：再次发布不得改变 state 或 published_at。
	before, err := svc.GetSnapshot(draft.ID)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if before.State != model.SnapshotStatePublished || before.PublishedAt == nil {
		t.Fatalf("snapshot not immutable-published: %+v", before)
	}
	originalPublishedAt := *before.PublishedAt

	if _, err := svc.PublishSnapshot(draft.ID, now.Add(time.Hour)); err == nil {
		t.Fatalf("expected re-publish to fail (immutable), got success")
	}
	after, err := svc.GetSnapshot(draft.ID)
	if err != nil {
		t.Fatalf("get snapshot after re-publish: %v", err)
	}
	if after.State != model.SnapshotStatePublished {
		t.Fatalf("state mutated after re-publish: %s", after.State)
	}
	if after.PublishedAt == nil || !after.PublishedAt.Equal(originalPublishedAt) {
		t.Fatalf("published_at mutated after re-publish: was %v, now %v",
			originalPublishedAt, after.PublishedAt)
	}
}

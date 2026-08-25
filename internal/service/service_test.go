package service_test

import (
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

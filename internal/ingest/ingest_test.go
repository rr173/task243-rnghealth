package ingest_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestIngestPersistsSequenceAndFlagsReplay(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	now := time.Unix(1000, 0)
	sourceID, err := svc.RegisterSource("ingest-test", "device-1", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}
	sample := make([]byte, 256)
	for i := range sample {
		sample[i] = byte(i)
	}
	first, err := svc.IngestWindow(sourceID, 1, sample, now)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if first.Seq != 1 {
		t.Fatalf("first sequence = %d, want 1", first.Seq)
	}
	second, err := svc.IngestWindow(sourceID, 2, sample, now.Add(time.Second))
	if err != nil {
		t.Fatalf("replayed ingest: %v", err)
	}
	if second.Status != model.WindowStatusRepeatAnomaly || second.AnomalyType != model.AnomalyReplay {
		t.Fatalf("replayed window = %+v", second)
	}
	if _, err := svc.IngestWindow(sourceID, 2, sample, now.Add(2*time.Second)); err != model.ErrConflict {
		t.Fatalf("duplicate sequence error = %v, want %v", err, model.ErrConflict)
	}
}

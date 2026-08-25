package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug04RecoveryUsesPostRestartBaseline(t *testing.T) {
	dbPath := t.TempDir() + "/rnghealth.db"
	db, err := store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	now := time.Unix(4, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("recovery-generation-source", "device-04", now)
	if err != nil { t.Fatal(err) }
	sample := make([]byte, 256)
	for i := range sample { sample[i] = byte(i) }
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil { t.Fatal(err) }
	if _, err := svc.RecordRestart(sourceID, 1, 0, now.Add(time.Second)); err != nil { t.Fatal(err) }
	if err := svc.DegradeSource(sourceID, now.Add(2*time.Second)); err != nil { t.Fatal(err) }
	if err := svc.RecoverSource(sourceID, now.Add(3*time.Second)); err != nil { t.Fatal(err) }
	src, err := svc.GetSource(sourceID)
	if err != nil { t.Fatal(err) }
	if src.State != model.SourceStateObserving || src.RecoveryBaselineSeq != nil {
		t.Fatalf("source=%+v, want observing with a missing post-restart baseline", src)
	}
	if err := db.Close(); err != nil { t.Fatal(err) }
	db, err = store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	src, err = service.New(db).GetSource(sourceID)
	if err != nil { t.Fatal(err) }
	if src.RecoveryBaselineSeq != nil { t.Fatalf("reopened source=%+v, baseline must remain missing", src) }
}

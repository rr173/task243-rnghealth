package service_test

import (
	"errors"
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug05SealedSourceRejectsEveryDiagnosticWrite(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(5, 0)
	sourceID, err := svc.RegisterSource("sealed-source", "device-05", now)
	if err != nil { t.Fatal(err) }
	if err := svc.SealSource(sourceID, now); err != nil { t.Fatal(err) }
	sample := make([]byte, 256)
	if _, err := svc.IngestWindow(sourceID, 1, sample, now.Add(time.Second)); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("ingest error=%v, want ErrSealed", err)
	}
	if _, err := svc.RecordRestart(sourceID, 1, 0, now.Add(2*time.Second)); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("restart error=%v, want ErrSealed", err)
	}
	if _, err := svc.DraftSnapshot(sourceID, now.Add(3*time.Second)); !errors.Is(err, model.ErrSealed) {
		t.Fatalf("draft error=%v, want ErrSealed", err)
	}
	if wins, err := svc.ListWindows(sourceID, "", 10, 0); err != nil || len(wins) != 0 {
		t.Fatalf("windows=%v err=%v, sealed source must not accept windows", wins, err)
	}
}


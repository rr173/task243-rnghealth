package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug07ReevaluatePreservesReplayEvidence(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(7, 0)
	sourceID, err := svc.RegisterSource("reevaluate-source", "device-07", now)
	if err != nil { t.Fatal(err) }
	sample := make([]byte, 256)
	for i := range sample { sample[i] = byte((i*73 + 19) % 256) }
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil { t.Fatal(err) }
	win, err := svc.IngestWindow(sourceID, 2, sample, now.Add(time.Second))
	if err != nil { t.Fatal(err) }
	if win.Status != model.WindowStatusRepeatAnomaly { t.Fatalf("initial status=%s, want repeat_anomaly", win.Status) }
	updated, err := svc.ReevaluateWindow(win.ID)
	if err != nil { t.Fatal(err) }
	if updated.Status != model.WindowStatusRepeatAnomaly || updated.AnomalyType != model.AnomalyReplay {
		t.Fatalf("reevaluated window=%+v, want replay evidence preserved", updated)
	}
}


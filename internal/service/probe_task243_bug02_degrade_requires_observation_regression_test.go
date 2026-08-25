package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug02RestartBoundaryResetsAnomalyRun(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(2, 0)
	sourceID, err := svc.RegisterSource("restart-run-source", "device-02", now)
	if err != nil { t.Fatal(err) }
	sample := make([]byte, 256)
	if _, err := svc.IngestWindow(sourceID, 1, sample, now); err != nil { t.Fatal(err) }
	if _, err := svc.IngestWindow(sourceID, 2, sample, now.Add(time.Second)); err != nil { t.Fatal(err) }
	if _, err := svc.RecordRestart(sourceID, 3, 0, now.Add(2*time.Second)); err != nil { t.Fatal(err) }
	if _, err := svc.IngestWindow(sourceID, 3, sample, now.Add(3*time.Second)); err != nil { t.Fatal(err) }
	src, err := svc.GetSource(sourceID)
	if err != nil { t.Fatal(err) }
	if src.State != model.SourceStateObserving { t.Fatalf("state=%s, want observing after the first post-restart anomaly", src.State) }
	events, err := svc.ListEvents(sourceID, "", 10, 0)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 { t.Fatalf("events=%d, want 1", len(events)) }
	ev := events[0]
	if ev.State == model.EventStatePersistent || ev.WindowCount != 1 || !ev.AfterRestart {
		t.Fatalf("event=%+v, want a one-window post-restart event", ev)
	}
}


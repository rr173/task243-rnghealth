package service_test

import (
	"encoding/json"
	"testing"
	"time"

	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug10RestartFlagSurvivesEventAndSnapshot(t *testing.T) {
	dbPath := t.TempDir() + "/rnghealth.db"
	db, err := store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	now := time.Unix(10, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("restart-evidence-source", "device-10", now)
	if err != nil { t.Fatal(err) }
	if _, err := svc.RecordRestart(sourceID, 1, 0, now); err != nil { t.Fatal(err) }
	sample := make([]byte, 256)
	for seq := int64(1); seq <= 3; seq++ {
		if _, err := svc.IngestWindow(sourceID, seq, sample, now.Add(time.Duration(seq)*time.Second)); err != nil { t.Fatal(err) }
	}
	events, err := svc.ListEvents(sourceID, "", 10, 0)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 || !events[0].AfterRestart {
		t.Fatalf("events=%+v, want after_restart=true", events)
	}
	snap, err := svc.DraftSnapshot(sourceID, now.Add(4*time.Second))
	if err != nil { t.Fatal(err) }
	if _, err := svc.PublishSnapshot(snap.ID, now.Add(5*time.Second)); err != nil { t.Fatal(err) }
	var payload struct {
		OpenEvents []struct {
			AfterRestart bool `json:"after_restart"`
		} `json:"open_events"`
	}
	if err := json.Unmarshal([]byte(snap.Payload), &payload); err != nil { t.Fatal(err) }
	if len(payload.OpenEvents) != 1 || !payload.OpenEvents[0].AfterRestart {
		t.Fatalf("snapshot payload=%s, want after_restart=true", snap.Payload)
	}
	if err := db.Close(); err != nil { t.Fatal(err) }
	db, err = store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	reopened, err := service.New(db).ListEvents(sourceID, "", 10, 0)
	if err != nil { t.Fatal(err) }
	if len(reopened) != 1 || !reopened[0].AfterRestart { t.Fatalf("reopened events=%+v", reopened) }
}


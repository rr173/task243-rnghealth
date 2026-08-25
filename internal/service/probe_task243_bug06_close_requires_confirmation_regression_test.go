package service_test

import (
	"errors"
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

func TestBug06CloseRequiresConfirmationAndKeepsTimeline(t *testing.T) {
	dbPath := t.TempDir() + "/rnghealth.db"
	db, err := store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	now := time.Unix(6, 0)
	svc := service.New(db)
	sourceID, err := svc.RegisterSource("event-timeline-source", "device-06", now)
	if err != nil { t.Fatal(err) }
	if _, err := svc.IngestWindow(sourceID, 1, make([]byte, 256), now); err != nil { t.Fatal(err) }
	events, err := svc.ListEvents(sourceID, "", 10, 0)
	if err != nil { t.Fatal(err) }
	if len(events) != 1 { t.Fatalf("events=%d, want 1", len(events)) }
	if _, err := svc.CloseEvent(events[0].ID, "", now.Add(time.Second)); !errors.Is(err, model.ErrTransition) {
		t.Fatalf("close error=%v, want ErrTransition before confirmation", err)
	}
	confirmed, err := svc.ConfirmEvent(events[0].ID, "reviewed", now.Add(2*time.Second))
	if err != nil { t.Fatal(err) }
	if confirmed.ConfirmedAt == nil { t.Fatal("confirmation timestamp missing") }
	closed, err := svc.CloseEvent(events[0].ID, "resolved", now.Add(3*time.Second))
	if err != nil { t.Fatal(err) }
	if closed.State != model.EventStateClosed || closed.ConfirmedAt == nil || closed.ClosedAt == nil {
		t.Fatalf("closed event=%+v, want complete timeline", closed)
	}
	if err := db.Close(); err != nil { t.Fatal(err) }
	db, err = store.Open(dbPath)
	if err != nil { t.Fatal(err) }
	defer db.Close()
	reopened, err := service.New(db).GetEvent(events[0].ID)
	if err != nil { t.Fatal(err) }
	if reopened.ConfirmedAt == nil || reopened.ClosedAt == nil {
		t.Fatalf("reopened event=%+v, want persisted confirmation and close times", reopened)
	}
}


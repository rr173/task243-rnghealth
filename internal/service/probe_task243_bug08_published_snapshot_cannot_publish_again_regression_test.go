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

func TestBug08ConcurrentPublishIsSingleUse(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(8, 0)
	sourceID, err := svc.RegisterSource("publish-race-source", "device-08", now)
	if err != nil { t.Fatal(err) }
	draft, err := svc.DraftSnapshot(sourceID, now)
	if err != nil { t.Fatal(err) }

	const participants = 20
	start := make(chan struct{})
	results := make(chan error, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.PublishSnapshot(draft.ID, now.Add(time.Second))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil { successes++ } else if !errors.Is(err, model.ErrTransition) { t.Fatalf("unexpected error: %v", err) }
	}
	if successes != 1 { t.Fatalf("successful publishes=%d, want 1", successes) }
	snap, err := svc.GetSnapshot(draft.ID)
	if err != nil { t.Fatal(err) }
	if snap.State != model.SnapshotStatePublished || snap.PublishedAt == nil { t.Fatalf("snapshot=%+v", snap) }
}


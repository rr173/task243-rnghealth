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

func TestBug03ConcurrentSupersedeKeepsOnePublishedVersion(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	now := time.Unix(3, 0)
	sourceID, err := svc.RegisterSource("snapshot-race-source", "device-03", now)
	if err != nil { t.Fatal(err) }
	old, err := svc.DraftSnapshot(sourceID, now)
	if err != nil { t.Fatal(err) }
	if _, err := svc.PublishSnapshot(old.ID, now); err != nil { t.Fatal(err) }

	const participants = 20
	start := make(chan struct{})
	results := make(chan error, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.SupersedeSnapshot(old.ID, now.Add(time.Second))
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
	if successes != 1 { t.Fatalf("successful replacements=%d, want 1", successes) }
	snaps, err := svc.ListSnapshots(sourceID, 100, 0)
	if err != nil { t.Fatal(err) }
	published, superseded := 0, 0
	var replacementID int64
	for _, snap := range snaps {
		switch snap.State {
		case model.SnapshotStatePublished:
			published++
			replacementID = snap.ID
		case model.SnapshotStateSuperseded:
			superseded++
		}
	}
	if published != 1 || superseded != 1 { t.Fatalf("published=%d superseded=%d snapshots=%+v", published, superseded, snaps) }
	oldReload, err := svc.GetSnapshot(old.ID)
	if err != nil { t.Fatal(err) }
	if oldReload.SupersededBy == nil || *oldReload.SupersededBy != replacementID {
		t.Fatalf("old snapshot=%+v, want a pointer to the sole replacement", oldReload)
	}
}


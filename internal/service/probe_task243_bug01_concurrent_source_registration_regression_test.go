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

func TestBug01ConcurrentSourceRegistrationIsUnique(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil { t.Fatal(err) }
	defer db.Close()
	svc := service.New(db)
	const participants = 20
	start := make(chan struct{})
	results := make(chan error, participants)
	var wg sync.WaitGroup
	for i := 0; i < participants; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.RegisterSource("shared-source", "device-01", time.Unix(1, 0))
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil { successes++ } else if errors.Is(err, model.ErrConflict) { conflicts++ } else { t.Fatalf("unexpected error: %v", err) }
	}
	if successes != 1 || conflicts != participants-1 { t.Fatalf("successes=%d conflicts=%d", successes, conflicts) }
	sources, err := svc.ListSources(100, 0)
	if err != nil { t.Fatal(err) }
	if len(sources) != 1 { t.Fatalf("stored sources=%d, want 1", len(sources)) }
}


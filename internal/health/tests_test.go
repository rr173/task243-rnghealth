package health

import (
	"testing"

	"task243-rnghealth/internal/model"
)

func TestRunStatisticalTestsDetectsStalledSample(t *testing.T) {
	stalled := make([]byte, 256)
	results := RunStatisticalTests(stalled)
	if len(results) != 4 {
		t.Fatalf("got %d health results, want 4", len(results))
	}
	if AllPassed(results) {
		t.Fatal("stalled sample unexpectedly passed all health tests")
	}
	if got := FirstFailing(results); got == "" {
		t.Fatal("FirstFailing returned empty category for failed sample")
	}
	if results[0].Category != model.TestMonobit {
		t.Fatalf("first category = %q, want %q", results[0].Category, model.TestMonobit)
	}
}

func TestDetectRepetitionUsesPreviousAndRecentHashes(t *testing.T) {
	sample := []byte("replayed-window")
	first := DetectRepetition(sample, "", nil)
	if first.IsReplay {
		t.Fatal("first sample was classified as replay")
	}
	second := DetectRepetition(sample, SampleHash(sample), []string{SampleHash(sample)})
	if !second.IsReplay || second.Score <= 0 {
		t.Fatalf("repeated sample = %+v, want replay with positive score", second)
	}
}

package model

import (
	"errors"
	"testing"
	"time"
)

func TestSampleWindowValidateAndSourceSeal(t *testing.T) {
	window := &SampleWindow{SourceID: 7, Seq: 1, ByteLen: MinWindowBytes}
	if err := window.Validate(); err != nil {
		t.Fatalf("valid window rejected: %v", err)
	}
	window.ByteLen = MinWindowBytes - 1
	if !errors.Is(window.Validate(), ErrInvalidArgument) {
		t.Fatal("short window did not return ErrInvalidArgument")
	}

	source := &EntropySource{State: SourceStateObserving}
	now := time.Unix(100, 0)
	if err := source.Seal(now); err != nil {
		t.Fatalf("seal failed: %v", err)
	}
	if source.State != SourceStateSealed || source.SealedAt == nil || !source.SealedAt.Equal(now) {
		t.Fatalf("sealed source = %+v", source)
	}
	if !errors.Is(source.Seal(now), ErrTransition) {
		t.Fatal("sealing an already sealed source did not return ErrTransition")
	}
}

func TestHealthEventLifecycle(t *testing.T) {
	event := &HealthEvent{State: EventStatePersistent}
	if !event.CanConfirm() {
		t.Fatal("persistent event should be confirmable")
	}
	event.State = EventStateConfirmed
	if !event.CanClose() {
		t.Fatal("confirmed event should be closeable")
	}
	event.State = EventStateClosed
	if !event.IsClosed() {
		t.Fatal("closed event was not recognized as closed")
	}
}

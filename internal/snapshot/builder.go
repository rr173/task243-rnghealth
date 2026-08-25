package snapshot

import (
	"encoding/json"
	"time"

	"task243-rnghealth/internal/store"
)

// EventSummary 快照中携带的事件摘要。
type EventSummary struct {
	ID           int64  `json:"id"`
	State        string `json:"state"`
	AnomalyType  string `json:"anomaly_type"`
	FirstSeq     int64  `json:"first_seq"`
	LastSeq      int64  `json:"last_seq"`
	AfterRestart bool   `json:"after_restart"`
}

// Payload 冻结时的诊断证据结构（序列化为 JSON 存入 snapshots.payload）。
type Payload struct {
	SourceID            int64          `json:"source_id"`
	State               string         `json:"state"`
	GeneratedAt         string         `json:"generated_at"`
	RecoveryBaselineSeq *int64         `json:"recovery_baseline_seq"`
	BaselineMissing     bool           `json:"baseline_missing"`
	WindowStats         WindowStats    `json:"window_stats"`
	TestFailures        map[string]int `json:"test_failures"`
	OpenEvents          []EventSummary `json:"open_events"`
}

// WindowStats 窗口统计。
type WindowStats struct {
	Total         int     `json:"total"`
	Valid         int     `json:"valid"`
	RepeatAnomaly int     `json:"repeat_anomaly"`
	StatAnomaly   int     `json:"stat_anomaly"`
	AvgEntropy    float64 `json:"avg_entropy"`
}

// Builder 构造不可变诊断快照证据。
type Builder struct {
	sources *store.SourceStore
	windows *store.WindowStore
	events  *store.EventStore
	tests   *store.HealthTestStore
}

// NewBuilder 构造。
func NewBuilder(sources *store.SourceStore, windows *store.WindowStore, events *store.EventStore, tests *store.HealthTestStore) *Builder {
	return &Builder{sources: sources, windows: windows, events: events, tests: tests}
}

// BuildPayload 采集当前熵源健康证据并序列化为 JSON。
func (b *Builder) BuildPayload(sourceID int64, now time.Time) (string, error) {
	src, err := b.sources.Get(sourceID)
	if err != nil {
		return "", err
	}
	total, valid, repeat, stat, avgEnt, err := b.windows.WindowStats(sourceID)
	if err != nil {
		return "", err
	}
	failures, err := b.tests.SummaryForSource(sourceID, 5000)
	if err != nil {
		return "", err
	}
	open, err := b.events.List(sourceID, "", 50, 0)
	if err != nil {
		return "", err
	}
	summaries := make([]EventSummary, 0, len(open))
	for _, ev := range open {
		if ev.IsClosed() {
			continue
		}
		summaries = append(summaries, EventSummary{
			ID: ev.ID, State: ev.State, AnomalyType: ev.AnomalyType,
			FirstSeq: ev.FirstSeq, LastSeq: ev.LastSeq,
		})
	}
	baselineMissing := src.RecoveryBaselineSeq == nil && total > 0
	p := Payload{
		SourceID:            sourceID,
		State:               src.State,
		GeneratedAt:         now.UTC().Format(time.RFC3339Nano),
		RecoveryBaselineSeq: src.RecoveryBaselineSeq,
		BaselineMissing:     baselineMissing,
		WindowStats: WindowStats{
			Total: total, Valid: valid, RepeatAnomaly: repeat,
			StatAnomaly: stat, AvgEntropy: avgEnt,
		},
		TestFailures: failures,
		OpenEvents:   summaries,
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

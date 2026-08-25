package model

import "time"

// 诊断快照状态机：草稿 -> 发布 -> 替代。
const (
	SnapshotStateDraft      = "draft"      // 草稿：可编辑
	SnapshotStatePublished  = "published"  // 发布：不可变、可被替代
	SnapshotStateSuperseded = "superseded" // 替代：被新版本取代
)

// DiagnosticSnapshot 是不可变的熵源健康诊断证据。
type DiagnosticSnapshot struct {
	ID       int64  `json:"id"`
	SourceID int64  `json:"source_id"`
	Version  int    `json:"version"`
	State    string `json:"state"`
	// Payload 冻结时的诊断证据 JSON（源状态、窗口统计、事件、测试汇总、恢复基线）。
	Payload      string     `json:"payload"`
	CreatedAt    time.Time  `json:"created_at"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	SupersededBy *int64     `json:"superseded_by,omitempty"`
}

// IsPublished 是否已发布（含被替代）。
func (s *DiagnosticSnapshot) IsPublished() bool {
	return s.State == SnapshotStatePublished || s.State == SnapshotStateSuperseded
}

// CanPublish 是否可发布（仅草稿）。
func (s *DiagnosticSnapshot) CanPublish() bool {
	return s.State == SnapshotStateDraft
}

// CanSupersede 是否可被替代（仅处于 published 态：草稿不可替代，
// 已被替代的版本不可再次替代——同一旧版本只能被一个新版本替代）。
func (s *DiagnosticSnapshot) CanSupersede() bool {
	return s.State == SnapshotStatePublished
}

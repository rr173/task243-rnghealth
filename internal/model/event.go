package model

import "time"

// 健康事件状态机：候选 -> 瞬时 | 持续 -> 确认 -> 关闭。
const (
	EventStateCandidate  = "candidate"  // 候选：刚检出异常，尚在判定瞬态/持续
	EventStateTransient = "transient"  // 瞬时：孤立窗口异常，未持续
	EventStatePersistent = "persistent" // 持续：连续异常，疑似熵源退化
	EventStateConfirmed = "confirmed"  // 确认：工程师确认与熵源相关
	EventStateClosed    = "closed"     // 关闭：已处置或误报
)

// 异常类别。
const (
	AnomalyReplay    = "replay"    // 跨窗口重播
	AnomalyMonobit   = "monobit"   // 单比特失衡
	AnomalyRuns      = "runs"      // 游程异常
	AnomalyPoker     = "poker"     // 扑克卡方异常
	AnomalyLongRun   = "longrun"   // 长游程异常
)

// HealthEvent 表示一次关联到熵源的异常传播事件。
type HealthEvent struct {
	ID             int64     `json:"id"`
	SourceID       int64     `json:"source_id"`
	State          string    `json:"state"`
	AnomalyType    string    `json:"anomaly_type"`
	// FirstSeq / LastSeq 关联到的窗口序号区间（闭区间）。
	FirstSeq       int64     `json:"first_seq"`
	LastSeq        int64     `json:"last_seq"`
	// WindowCount 关联窗口数。
	WindowCount    int       `json:"window_count"`
	// AfterRestart 异常是否跨越了一次设备重启边界。
	AfterRestart   bool      `json:"after_restart"`
	CreatedAt      time.Time `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
	ClosedAt       *time.Time `json:"closed_at,omitempty"`
	// Note 工程师处置备注。
	Note           string    `json:"note,omitempty"`
}

// IsClosed 是否已关闭。
func (e *HealthEvent) IsClosed() bool {
	return e.State == EventStateClosed
}

// CanConfirm 是否可确认（候选/瞬时/持续均可确认）。
func (e *HealthEvent) CanConfirm() bool {
	switch e.State {
	case EventStateCandidate, EventStateTransient, EventStatePersistent:
		return true
	}
	return false
}

// CanClose 是否可关闭。
func (e *HealthEvent) CanClose() bool {
	return e.State == EventStateConfirmed
}

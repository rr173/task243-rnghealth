package model

import "time"

// RestartBoundary 记录一次设备重启事件，用于判定异常是否跨越重启（恢复基线缺失）。
type RestartBoundary struct {
	ID       int64 `json:"id"`
	SourceID int64 `json:"source_id"`
	// AtSeq 重启后第一个新窗口的序号（重启边界）。
	AtSeq      int64     `json:"at_seq"`
	OccurredAt time.Time `json:"occurred_at"`
	// BaselineSeq 重启时声称的恢复基线窗口序号；若为 0 表示未提供恢复基线。
	BaselineSeq int64 `json:"baseline_seq"`
	// BaselineMissing 重启后首窗即异常且未提供恢复基线。
	BaselineMissing bool `json:"baseline_missing"`
}

// Validate 校验重启边界入参。
func (r *RestartBoundary) Validate() error {
	if r.SourceID <= 0 {
		return ErrInvalidArgument
	}
	if r.AtSeq <= 0 {
		return ErrInvalidArgument
	}
	return nil
}

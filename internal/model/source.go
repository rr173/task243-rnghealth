package model

import "time"

// 熵源状态机：启用 -> 观察 -> 降级 -> 隔离；任意态可封存。
const (
	SourceStateEnabled   = "enabled"   // 启用：已注册尚未接收窗口
	SourceStateObserving = "observing" // 观察：已接收窗口并持续健康
	SourceStateDegraded  = "degraded"  // 降级：持续异常被确认
	SourceStateIsolated  = "isolated"  // 隔离：工程师主动隔离，停止用于生产
	SourceStateSealed    = "sealed"    // 封存：诊断完成，拒绝一切写入
)

// SourceStateOrder 提供状态流转的偏序参考（仅供业务包判定，非强制全集）。
var SourceStateOrder = map[string]int{
	SourceStateEnabled:   0,
	SourceStateObserving: 1,
	SourceStateDegraded:  2,
	SourceStateIsolated:  3,
	SourceStateSealed:    4,
}

// EntropySource 表示一个硬件随机数熵源（如 TRNG / 噪声二极管）。
type EntropySource struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Device    string     `json:"device"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	SealedAt  *time.Time `json:"sealed_at,omitempty"`
	// LastSeq 当前已确认的最大窗口序号（幂等依据）。
	LastSeq int64 `json:"last_seq"`
	// RecoveryBaselineSeq 最近一次确认恢复时的窗口序号，缺失表示恢复基线未建立。
	RecoveryBaselineSeq *int64 `json:"recovery_baseline_seq,omitempty"`
}

// Validate 校验注册入参。
func (s *EntropySource) Validate() error {
	if s.Name == "" {
		return ErrInvalidArgument
	}
	if s.Device == "" {
		return ErrInvalidArgument
	}
	return nil
}

// IsSealed 是否已封存。
func (s *EntropySource) IsSealed() bool {
	return s.State == SourceStateSealed
}

// CanWrite 是否仍接受新窗口写入（封存后拒绝）。
func (s *EntropySource) CanWrite() bool {
	return true
}

// SealedStates 可封存的合法来源态（除已封存外皆可）。
func (s *EntropySource) Seal(now time.Time) error {
	if s.State == SourceStateSealed {
		return ErrTransition
	}
	s.State = SourceStateSealed
	t := now
	s.SealedAt = &t
	return nil
}

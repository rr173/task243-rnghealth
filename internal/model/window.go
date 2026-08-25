package model

import "time"

// 采样窗口状态机：新到 -> 有效 | 重复异常 | 统计异常。
const (
	WindowStatusNew           = "new"            // 新到：已落库尚未评估
	WindowStatusValid         = "valid"          // 有效：健康测试与重复检测均通过
	WindowStatusRepeatAnomaly = "repeat_anomaly" // 重复异常：检出跨窗口重播/长重复串
	WindowStatusStatAnomaly   = "stat_anomaly"   // 统计异常：健康测试未通过
)

// SampleWindow 是熵源在一次采样中产出的字节样本摘要。
type SampleWindow struct {
	ID         int64     `json:"id"`
	SourceID   int64     `json:"source_id"`
	Seq        int64     `json:"seq"`
	ReceivedAt time.Time `json:"received_at"`
	ByteLen    int       `json:"byte_len"`
	// SampleHash 窗口原始字节的 SHA-256（用于重播检测，原始样本不落库以控量）。
	SampleHash string `json:"sample_hash"`
	// EstEntropy 估计熵（bits/byte，0~8）。
	EstEntropy float64 `json:"est_entropy"`
	// RepeatScore 重复倾向得分（0~1，越大越像重播）。
	RepeatScore float64 `json:"repeat_score"`
	Status      string  `json:"status"`
	// AnomalyType 异常类别（空表示无），如 "replay" / "monobit" / "longrun"。
	AnomalyType string `json:"anomaly_type,omitempty"`
}

// Validate 校验窗口入参与长度约束。
func (w *SampleWindow) Validate() error {
	if w.SourceID <= 0 {
		return ErrInvalidArgument
	}
	if w.Seq <= 0 {
		return ErrInvalidArgument
	}
	if w.ByteLen <= 0 {
		return ErrInvalidArgument
	}
	if w.ByteLen < MinWindowBytes || w.ByteLen > MaxWindowBytes {
		return ErrInvalidArgument
	}
	return nil
}

// MinWindowBytes / MaxWindowBytes 单个窗口字节数上下限。
const (
	MinWindowBytes = 64
	MaxWindowBytes = 65536
)

package model

import "time"

// 健康测试类别：NIST SP 800-90B 风格的统计健康测试。
const (
	TestMonobit = "monobit" // 单比特比例（平衡性）
	TestRuns    = "runs"    // 游程（连续相同比特段数）
	TestPoker   = "poker"   // 扑克（分块频率卡方）
	TestLongRun = "longrun" // 长游程（是否存在过长连续相同比特）
)

// HealthTestCategories 全部受支持的测试类别。
var HealthTestCategories = []string{TestMonobit, TestRuns, TestPoker, TestLongRun}

// IsKnownTest 判断类别是否受支持。
func IsKnownTest(category string) bool {
	for _, c := range HealthTestCategories {
		if c == category {
			return true
		}
	}
	return false
}

// HealthTest 是某一窗口在某类别上的健康测试结果。
type HealthTest struct {
	ID       int64  `json:"id"`
	WindowID int64  `json:"window_id"`
	SourceID int64  `json:"source_id"`
	Category string `json:"category"`
	Passed   bool   `json:"passed"`
	// Statistic 测试统计量（如 |S_n|/sqrt(n)）。
	Statistic float64 `json:"statistic"`
	// Threshold 判定阈值。
	Threshold float64 `json:"threshold"`
	// Detail 人类可读说明。
	Detail      string    `json:"detail"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

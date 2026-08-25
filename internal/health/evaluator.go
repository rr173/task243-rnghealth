package health

import (
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// Evaluator 对采样窗口执行统计健康测试并持久化证据。
type Evaluator struct {
	tests *store.HealthTestStore
}

// NewEvaluator 构造。
func NewEvaluator(t *store.HealthTestStore) *Evaluator {
	return &Evaluator{tests: t}
}

// Evaluate 运行全部统计测试并落库，返回仅由统计测试决定的窗口状态与首个失败类别。
// 注意：窗口最终状态还需与重复重播检测结果合并（见 ingest 包）。
func (e *Evaluator) Evaluate(sample []byte, windowID, sourceID int64, now time.Time) (status string, anomalyType string, err error) {
	results := RunStatisticalTests(sample)
	for _, r := range results {
		t := &model.HealthTest{
			WindowID:    windowID,
			SourceID:    sourceID,
			Category:    r.Category,
			Passed:      r.Passed,
			Statistic:   r.Statistic,
			Threshold:   r.Threshold,
			Detail:      "",
			EvaluatedAt: now,
		}
		if _, err := e.tests.Create(t, now); err != nil {
			return "", "", err
		}
	}
	if AllPassed(results) {
		return model.WindowStatusValid, "", nil
	}
	return model.WindowStatusStatAnomaly, FirstFailing(results), nil
}

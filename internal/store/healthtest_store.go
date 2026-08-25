package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// HealthTestStore 健康测试结果持久化。
type HealthTestStore struct {
	db *sql.DB
}

// NewHealthTestStore 构造。
func NewHealthTestStore(db *sql.DB) *HealthTestStore { return &HealthTestStore{db: db} }

// Create 写入一条健康测试。
func (h *HealthTestStore) Create(t *model.HealthTest, now time.Time) (int64, error) {
	if !model.IsKnownTest(t.Category) {
		return 0, model.ErrUnknownTest
	}
	res, err := h.db.Exec(
		`INSERT INTO health_tests(window_id, source_id, category, passed, statistic, threshold, detail, evaluated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		t.WindowID, t.SourceID, t.Category, boolToInt(t.Passed), t.Statistic, t.Threshold, t.Detail,
		now.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

// ListByWindow 列出某窗口全部测试。
func (h *HealthTestStore) ListByWindow(windowID int64) ([]*model.HealthTest, error) {
	rows, err := h.db.Query(
		`SELECT id, window_id, source_id, category, passed, statistic, threshold, detail, evaluated_at
		 FROM health_tests WHERE window_id = ? ORDER BY category`, windowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.HealthTest, 0, 4)
	for rows.Next() {
		t, err := scanHealthTest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SummaryForSource 统计某熵源各测试类别的失败次数（最近 limit 窗口）。
func (h *HealthTestStore) SummaryForSource(sourceID int64, limit int) (map[string]int, error) {
	rows, err := h.db.Query(
		`SELECT category, passed FROM health_tests WHERE source_id = ? ORDER BY id DESC LIMIT ?`, sourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var category string
		var passed int
		if err := rows.Scan(&category, &passed); err != nil {
			return nil, err
		}
		if passed == 0 {
			out[category]++
		}
	}
	return out, rows.Err()
}

func scanHealthTest(row scannable) (*model.HealthTest, error) {
	var (
		id, windowID, sourceID int64
		category, detail       string
		passed                 int
		statistic, threshold   float64
		evaluatedAt            string
	)
	if err := row.Scan(&id, &windowID, &sourceID, &category, &passed, &statistic, &threshold, &detail, &evaluatedAt); err != nil {
		return nil, mapErr(err)
	}
	ea, _ := time.Parse(time.RFC3339Nano, evaluatedAt)
	return &model.HealthTest{
		ID:          id,
		WindowID:    windowID,
		SourceID:    sourceID,
		Category:    category,
		Passed:      passed != 0,
		Statistic:   statistic,
		Threshold:   threshold,
		Detail:      detail,
		EvaluatedAt: ea,
	}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

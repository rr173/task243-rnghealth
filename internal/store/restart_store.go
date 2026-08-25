package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// RestartStore 设备重启边界持久化。
type RestartStore struct {
	db *sql.DB
}

// NewRestartStore 构造。
func NewRestartStore(db *sql.DB) *RestartStore { return &RestartStore{db: db} }

// Create 写入重启边界。
func (r *RestartStore) Create(rb *model.RestartBoundary, now time.Time) (int64, error) {
	if err := rb.Validate(); err != nil {
		return 0, err
	}
	res, err := r.db.Exec(
		`INSERT INTO restart_boundaries(source_id, at_seq, occurred_at, baseline_seq, baseline_missing)
		 VALUES(?,?,?,?,?)`,
		rb.SourceID, rb.AtSeq, now.UTC().Format(time.RFC3339Nano), rb.BaselineSeq, boolToInt(rb.BaselineMissing),
	)
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

// LatestBefore 取某熵源在给定序号之前（含）的最新重启边界。
func (r *RestartStore) LatestBefore(sourceID, seq int64) (*model.RestartBoundary, error) {
	row := r.db.QueryRow(
		`SELECT id, source_id, at_seq, occurred_at, baseline_seq, baseline_missing
		 FROM restart_boundaries WHERE source_id = ? AND at_seq < ? ORDER BY at_seq DESC LIMIT 1`,
		sourceID, seq)
	var (
		id, sid, atSeq, baselineSeq int64
		occurredAt                  string
		baselineMissing             int
	)
	if err := row.Scan(&id, &sid, &atSeq, &occurredAt, &baselineSeq, &baselineMissing); err != nil {
		return nil, mapErr(err)
	}
	oa, _ := time.Parse(time.RFC3339Nano, occurredAt)
	return &model.RestartBoundary{
		ID:              id,
		SourceID:        sid,
		AtSeq:           atSeq,
		OccurredAt:      oa,
		BaselineSeq:     baselineSeq,
		BaselineMissing: baselineMissing != 0,
	}, nil
}

// List 列出某熵源全部重启边界。
func (r *RestartStore) List(sourceID int64) ([]*model.RestartBoundary, error) {
	rows, err := r.db.Query(
		`SELECT id, source_id, at_seq, occurred_at, baseline_seq, baseline_missing
		 FROM restart_boundaries WHERE source_id = ? ORDER BY at_seq`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.RestartBoundary, 0)
	for rows.Next() {
		var (
			id, sid, atSeq, baselineSeq int64
			occurredAt                  string
			baselineMissing             int
		)
		if err := rows.Scan(&id, &sid, &atSeq, &occurredAt, &baselineSeq, &baselineMissing); err != nil {
			return nil, err
		}
		oa, _ := time.Parse(time.RFC3339Nano, occurredAt)
		out = append(out, &model.RestartBoundary{
			ID: id, SourceID: sid, AtSeq: atSeq, OccurredAt: oa,
			BaselineSeq: baselineSeq, BaselineMissing: baselineMissing != 0,
		})
	}
	return out, rows.Err()
}

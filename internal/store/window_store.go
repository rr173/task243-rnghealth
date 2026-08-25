package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// WindowStore 采样窗口持久化。
type WindowStore struct {
	db *sql.DB
}

// NewWindowStore 构造。
func NewWindowStore(db *sql.DB) *WindowStore { return &WindowStore{db: db} }

// Create 写入新窗口（source_id, seq 唯一）。
func (w *WindowStore) Create(win *model.SampleWindow, now time.Time) (int64, error) {
	if err := win.Validate(); err != nil {
		return 0, err
	}
	res, err := w.db.Exec(
		`INSERT INTO sample_windows(source_id, seq, received_at, byte_len, sample_hash, est_entropy, repeat_score, status, anomaly_type)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		win.SourceID, win.Seq, now.UTC().Format(time.RFC3339Nano),
		win.ByteLen, win.SampleHash, win.EstEntropy, win.RepeatScore, win.Status, nullStr(win.AnomalyType),
	)
	if err != nil {
		return 0, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Get 按 ID 读取。
func (w *WindowStore) Get(id int64) (*model.SampleWindow, error) {
	row := w.db.QueryRow(
		`SELECT id, source_id, seq, received_at, byte_len, sample_hash, est_entropy, repeat_score, status, anomaly_type
		 FROM sample_windows WHERE id = ?`, id)
	return scanWindow(row)
}

// GetBySourceSeq 按熵源+序号读取（序号幂等）。
func (w *WindowStore) GetBySourceSeq(sourceID, seq int64) (*model.SampleWindow, error) {
	row := w.db.QueryRow(
		`SELECT id, source_id, seq, received_at, byte_len, sample_hash, est_entropy, repeat_score, status, anomaly_type
		 FROM sample_windows WHERE source_id = ? AND seq = ?`, sourceID, seq)
	return scanWindow(row)
}

// UpdateStatus 修正窗口最终状态与异常类别（评估后落库）。
func (w *WindowStore) UpdateStatus(id int64, status, anomalyType string) error {
	_, err := w.db.Exec(
		`UPDATE sample_windows SET status = ?, anomaly_type = ? WHERE id = ?`,
		status, nullStr(anomalyType), id)
	return mapErr(err)
}

// Count 窗口总数。
func (w *WindowStore) Count() (int, error) {
	var n int
	err := w.db.QueryRow(`SELECT COUNT(*) FROM sample_windows`).Scan(&n)
	return n, mapErr(err)
}

// List 分页列出（支持按 source_id / status 过滤）。
func (w *WindowStore) List(sourceID int64, status string, limit, offset int) ([]*model.SampleWindow, error) {
	q := `SELECT id, source_id, seq, received_at, byte_len, sample_hash, est_entropy, repeat_score, status, anomaly_type FROM sample_windows WHERE 1=1`
	args := []interface{}{}
	if sourceID > 0 {
		q += ` AND source_id = ?`
		args = append(args, sourceID)
	}
	if status != "" {
		q += ` AND status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY source_id, seq DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := w.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.SampleWindow, 0, limit)
	for rows.Next() {
		win, err := scanWindowRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, win)
	}
	return out, rows.Err()
}

// CountAnomalousSince 统计某熵源在 [fromSeq, toSeq] 区间内的异常窗口数。
func (w *WindowStore) CountAnomalousSince(sourceID, fromSeq, toSeq int64) (int, error) {
	var n int
	err := w.db.QueryRow(
		`SELECT COUNT(*) FROM sample_windows WHERE source_id = ? AND seq >= ? AND seq <= ? AND status != ?`,
		sourceID, fromSeq, toSeq, model.WindowStatusValid,
	).Scan(&n)
	return n, mapErr(err)
}

// PrevWindowHash 取某熵源当前最大序号窗口的哈希（重播检测）。
func (w *WindowStore) PrevWindowHash(sourceID int64) (string, int64, error) {
	var h string
	var seq int64
	err := w.db.QueryRow(
		`SELECT sample_hash, seq FROM sample_windows WHERE source_id = ? ORDER BY seq DESC LIMIT 1`, sourceID,
	).Scan(&h, &seq)
	if err != nil {
		return "", 0, mapErr(err)
	}
	return h, seq, nil
}

// RecentWindows 取某熵源在 beforeSeq（含）之前按序号倒序的最近若干窗口。
// 用于关联模块判定「尾部连续异常游程」。
func (w *WindowStore) RecentWindows(sourceID, beforeSeq int64, limit int) ([]*model.SampleWindow, error) {
	rows, err := w.db.Query(
		`SELECT id, source_id, seq, received_at, byte_len, sample_hash, est_entropy, repeat_score, status, anomaly_type
		 FROM sample_windows WHERE source_id = ? AND seq <= ? ORDER BY seq DESC LIMIT ?`,
		sourceID, beforeSeq, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.SampleWindow, 0, limit)
	for rows.Next() {
		win, err := scanWindowRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, win)
	}
	return out, rows.Err()
}

// MaxSeq 取某熵源当前最大窗口序号。
func (w *WindowStore) MaxSeq(sourceID int64) (int64, error) {
	var seq int64
	err := w.db.QueryRow(
		`SELECT COALESCE(MAX(seq), 0) FROM sample_windows WHERE source_id = ?`, sourceID,
	).Scan(&seq)
	return seq, mapErr(err)
}

// LatestValidSeq 取某熵源最近一个有效窗口的序号（恢复基线依据）；无则 0。
func (w *WindowStore) LatestValidSeq(sourceID int64) (int64, error) {
	var seq int64
	err := w.db.QueryRow(
		`SELECT COALESCE(MAX(seq), 0) FROM sample_windows WHERE source_id = ? AND status = ?`,
		sourceID, model.WindowStatusValid,
	).Scan(&seq)
	return seq, mapErr(err)
}

// WindowStats 聚合某熵源的窗口计数与平均估计熵。
func (w *WindowStore) WindowStats(sourceID int64) (total, valid, repeatAnomaly, statAnomaly int, avgEntropy float64, err error) {
	if err = w.db.QueryRow(
		`SELECT COUNT(*),
		        COALESCE(SUM(CASE WHEN status='valid' THEN 1 ELSE 0 END),0),
		        COALESCE(SUM(CASE WHEN status='repeat_anomaly' THEN 1 ELSE 0 END),0),
		        COALESCE(SUM(CASE WHEN status='stat_anomaly' THEN 1 ELSE 0 END),0),
		        COALESCE(AVG(est_entropy),0)
		 FROM sample_windows WHERE source_id = ?`, sourceID,
	).Scan(&total, &valid, &repeatAnomaly, &statAnomaly, &avgEntropy); err != nil {
		return 0, 0, 0, 0, 0, mapErr(err)
	}
	return
}

func scanWindow(row scannable) (*model.SampleWindow, error) { return scanWindowRow(row) }

func scanWindowRow(row scannable) (*model.SampleWindow, error) {
	var (
		id, sourceID, seq, byteLen                        int64
		receivedAt, sampleHash, status                   string
		estEntropy, repeatScore                          float64
		anomalyType                                      sql.NullString
	)
	if err := row.Scan(&id, &sourceID, &seq, &receivedAt, &byteLen, &sampleHash, &estEntropy, &repeatScore, &status, &anomalyType); err != nil {
		return nil, mapErr(err)
	}
	ra, _ := time.Parse(time.RFC3339Nano, receivedAt)
	return &model.SampleWindow{
		ID:          id,
		SourceID:    sourceID,
		Seq:         seq,
		ReceivedAt:  ra,
		ByteLen:     int(byteLen),
		SampleHash:  sampleHash,
		EstEntropy:  estEntropy,
		RepeatScore: repeatScore,
		Status:      status,
		AnomalyType: nullStrVal(anomalyType),
	}, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullStrVal(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

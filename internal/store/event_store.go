package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// EventStore 健康事件持久化。
type EventStore struct {
	db *sql.DB
}

// NewEventStore 构造。
func NewEventStore(db *sql.DB) *EventStore { return &EventStore{db: db} }

// Create 写入健康事件。
func (e *EventStore) Create(ev *model.HealthEvent, now time.Time) (int64, error) {
	res, err := e.db.Exec(
		`INSERT INTO health_events(source_id, state, anomaly_type, first_seq, last_seq, window_count, after_restart, created_at, confirmed_at, closed_at, note)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		ev.SourceID, ev.State, ev.AnomalyType, ev.FirstSeq, ev.LastSeq, ev.WindowCount, boolToInt(ev.AfterRestart),
		now.UTC().Format(time.RFC3339Nano), nullTimeVal(nil), nullTimeVal(nil), nullStr(ev.Note),
	)
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

// Get 按 ID 读取。
func (e *EventStore) Get(id int64) (*model.HealthEvent, error) {
	row := e.db.QueryRow(
		`SELECT id, source_id, state, anomaly_type, first_seq, last_seq, window_count, after_restart, created_at, confirmed_at, closed_at, note
		 FROM health_events WHERE id = ?`, id)
	return scanEvent(row)
}

// Count 事件总数。
func (e *EventStore) Count() (int, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM health_events`).Scan(&n)
	return n, mapErr(err)
}

// CountOpen 未关闭事件数。
func (e *EventStore) CountOpen() (int, error) {
	var n int
	err := e.db.QueryRow(`SELECT COUNT(*) FROM health_events WHERE state != ?`, model.EventStateClosed).Scan(&n)
	return n, mapErr(err)
}

// List 分页列出（支持按 source_id / state 过滤）。
func (e *EventStore) List(sourceID int64, state string, limit, offset int) ([]*model.HealthEvent, error) {
	q := `SELECT id, source_id, state, anomaly_type, first_seq, last_seq, window_count, after_restart, created_at, confirmed_at, closed_at, note FROM health_events WHERE 1=1`
	args := []interface{}{}
	if sourceID > 0 {
		q += ` AND source_id = ?`
		args = append(args, sourceID)
	}
	if state != "" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.HealthEvent, 0, limit)
	for rows.Next() {
		ev, err := scanEventRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// SetState 更新事件状态与时间戳。
func (e *EventStore) SetState(id int64, state string, confirmedAt, closedAt *time.Time) error {
	_, err := e.db.Exec(
		`UPDATE health_events SET state = ?, confirmed_at = ?, closed_at = ? WHERE id = ?`,
		state, nullTimeVal(confirmedAt), nullTimeVal(closedAt), id)
	return mapErr(err)
}

// UpdateRun 更新事件关联的游程区间与类别（不动时间字段）。
func (e *EventStore) UpdateRun(id, firstSeq, lastSeq int64, windowCount int, anomalyType string, afterRestart bool) error {
	_, err := e.db.Exec(
		`UPDATE health_events SET first_seq = ?, last_seq = ?, window_count = ?, anomaly_type = ?, after_restart = ? WHERE id = ?`,
		firstSeq, lastSeq, windowCount, anomalyType, 0, id)
	return mapErr(err)
}

// SetNote 更新处置备注。
func (e *EventStore) SetNote(id int64, note string) error {
	_, err := e.db.Exec(`UPDATE health_events SET note = ? WHERE id = ?`, nullStr(note), id)
	return mapErr(err)
}

// OpenForSource 取某熵源未关闭的最近事件（关联判定用）。
func (e *EventStore) OpenForSource(sourceID int64) (*model.HealthEvent, error) {
	row := e.db.QueryRow(
		`SELECT id, source_id, state, anomaly_type, first_seq, last_seq, window_count, after_restart, created_at, confirmed_at, closed_at, note
		 FROM health_events WHERE source_id = ? AND state != ? ORDER BY id DESC LIMIT 1`,
		sourceID, model.EventStateClosed)
	return scanEvent(row)
}

func scanEvent(row scannable) (*model.HealthEvent, error) { return scanEventRow(row) }

func scanEventRow(row scannable) (*model.HealthEvent, error) {
	var (
		id, sourceID, firstSeq, lastSeq  int64
		windowCount, afterRestart        int
		state, anomalyType               string
		note                             sql.NullString
		createdAt, confirmedAt, closedAt sql.NullString
	)
	if err := row.Scan(&id, &sourceID, &state, &anomalyType, &firstSeq, &lastSeq, &windowCount, &afterRestart, &createdAt, &confirmedAt, &closedAt, &note); err != nil {
		return nil, mapErr(err)
	}
	ca, _ := time.Parse(time.RFC3339Nano, createdAt.String)
	ev := &model.HealthEvent{
		ID:           id,
		SourceID:     sourceID,
		State:        state,
		AnomalyType:  anomalyType,
		FirstSeq:     firstSeq,
		LastSeq:      lastSeq,
		WindowCount:  windowCount,
		AfterRestart: afterRestart != 0,
		CreatedAt:    ca,
		ConfirmedAt:  scanNullTimeStr(confirmedAt),
		ClosedAt:     scanNullTimeStr(closedAt),
		Note:         note.String,
	}
	return ev, nil
}

func scanNullTimeStr(ns sql.NullString) *time.Time {
	if !ns.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// SnapshotStore 诊断快照持久化。
type SnapshotStore struct {
	db *sql.DB
}

// NewSnapshotStore 构造。
func NewSnapshotStore(db *sql.DB) *SnapshotStore { return &SnapshotStore{db: db} }

// executor 抽象 *sql.DB 与 *sql.Tx 共有的执行能力，使事务内复用扫描逻辑。
type executor interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// NextVersion 取某熵源下一个快照版本号（发布版本单调）。
func (s *SnapshotStore) NextVersion(sourceID int64) (int, error) {
	return nextVersionTx(s.db, sourceID)
}

// nextVersionTx 在指定执行器上取下一版本号（事务内可复用）。
func nextVersionTx(ex executor, sourceID int64) (int, error) {
	var v int
	err := ex.QueryRow(
		`SELECT COALESCE(MAX(version), 0) + 1 FROM diagnostic_snapshots WHERE source_id = ?`, sourceID,
	).Scan(&v)
	return v, mapErr(err)
}

// Create 写入草稿快照。
func (s *SnapshotStore) Create(snap *model.DiagnosticSnapshot, now time.Time) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO diagnostic_snapshots(source_id, version, state, payload, created_at, published_at, superseded_by)
		 VALUES(?,?,?,?,?,?,?)`,
		snap.SourceID, snap.Version, model.SnapshotStateDraft, snap.Payload,
		now.UTC().Format(time.RFC3339Nano), nullTimeVal(nil), nil,
	)
	if err != nil {
		return 0, mapErr(err)
	}
	return res.LastInsertId()
}

// Get 按 ID 读取。
func (s *SnapshotStore) Get(id int64) (*model.DiagnosticSnapshot, error) {
	return getSnapshotRow(s.db, id)
}

// getSnapshotRow 在指定执行器上按 ID 读取（事务内可复用）。
func getSnapshotRow(ex executor, id int64) (*model.DiagnosticSnapshot, error) {
	row := ex.QueryRow(
		`SELECT id, source_id, version, state, payload, created_at, published_at, superseded_by
		 FROM diagnostic_snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

// GetLatest 取某熵源最新快照（按版本）。
func (s *SnapshotStore) GetLatest(sourceID int64) (*model.DiagnosticSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, source_id, version, state, payload, created_at, published_at, superseded_by
		 FROM diagnostic_snapshots WHERE source_id = ? ORDER BY version DESC LIMIT 1`, sourceID)
	return scanSnapshot(row)
}

// Count 快照总数。
func (s *SnapshotStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM diagnostic_snapshots`).Scan(&n)
	return n, mapErr(err)
}

// List 分页列出（支持按 source_id 过滤）。
func (s *SnapshotStore) List(sourceID int64, limit, offset int) ([]*model.DiagnosticSnapshot, error) {
	q := `SELECT id, source_id, version, state, payload, created_at, published_at, superseded_by FROM diagnostic_snapshots WHERE 1=1`
	args := []interface{}{}
	if sourceID > 0 {
		q += ` AND source_id = ?`
		args = append(args, sourceID)
	}
	q += ` ORDER BY source_id, version DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.DiagnosticSnapshot, 0, limit)
	for rows.Next() {
		snap, err := scanSnapshotRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// Publish 将草稿发布为不可变版本。
func (s *SnapshotStore) Publish(id int64, now time.Time) error {
	_, err := s.db.Exec(
		`UPDATE diagnostic_snapshots SET state = ?, published_at = ? WHERE id = ?`,
		model.SnapshotStatePublished, now.UTC().Format(time.RFC3339Nano), id)
	return mapErr(err)
}

// SupersedePublished 在单个写事务内原子替代已发布快照：仅当 publishedID 仍处于
// published 态时，写入新草稿（payload 由调用方在事务外预先冻结）、发布为新版本，
// 并令旧版本转入 superseded 且 superseded_by 指向新版本；否则返回 ErrTransition。
//
// 同一旧版本只能被一个新版本替代：条件 UPDATE（state='published' -> superseded）
// 是最终守卫——并发替代中，第一个提交者赢，其余 affected=0 回滚为 ErrTransition。
// 新旧版本状态与替代关系在事务内一致落库。
func (s *SnapshotStore) SupersedePublished(publishedID int64, payload string, now time.Time) (*model.DiagnosticSnapshot, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, mapErr(err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	old, err := getSnapshotRow(tx, publishedID)
	if err != nil {
		return nil, err
	}
	// 仅 published 态可被替代；草稿与已被替代的版本都拒绝——
	// 否则同一旧版本会被多个新版本替代。
	if old.State != model.SnapshotStatePublished {
		return nil, model.ErrTransition
	}

	// 在事务内确定新版本号，保证单调且不被并发 Draft 抢占。
	version, err := nextVersionTx(tx, old.SourceID)
	if err != nil {
		return nil, err
	}
	created := now.UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(
		`INSERT INTO diagnostic_snapshots(source_id, version, state, payload, created_at, published_at, superseded_by)
		 VALUES(?,?,?,?,?,?,?)`,
		old.SourceID, version, model.SnapshotStateDraft, payload, created, created, nil,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	// 发布新版本。
	if _, err := tx.Exec(
		`UPDATE diagnostic_snapshots SET state = ?, published_at = ? WHERE id = ?`,
		model.SnapshotStatePublished, created, newID,
	); err != nil {
		return nil, mapErr(err)
	}
	// 条件替代旧版本：仅当仍为 published 时置为 superseded 并记录 superseded_by。
	// 这是并发守卫——同一旧版本只能被一个新版本成功替代。
	claimRes, err := tx.Exec(
		`UPDATE diagnostic_snapshots SET state = ?, superseded_by = ? WHERE id = ? AND state = ?`,
		model.SnapshotStateSuperseded, newID, publishedID, model.SnapshotStatePublished,
	)
	if err != nil {
		return nil, mapErr(err)
	}
	affected, err := claimRes.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		// 并发竞争中被另一个工作流抢先替代。
		return nil, model.ErrTransition
	}
	if err := tx.Commit(); err != nil {
		return nil, mapErr(err)
	}
	committed = true
	return s.Get(newID)
}

func scanSnapshot(row scannable) (*model.DiagnosticSnapshot, error) { return scanSnapshotRow(row) }

func scanSnapshotRow(row scannable) (*model.DiagnosticSnapshot, error) {
	var (
		id, sourceID, version  int64
		state, payload         string
		createdAt, publishedAt sql.NullString
		supersededBy           sql.NullInt64
	)
	if err := row.Scan(&id, &sourceID, &version, &state, &payload, &createdAt, &publishedAt, &supersededBy); err != nil {
		return nil, mapErr(err)
	}
	ca, _ := time.Parse(time.RFC3339Nano, createdAt.String)
	snap := &model.DiagnosticSnapshot{
		ID:        id,
		SourceID:  sourceID,
		Version:   int(version),
		State:     state,
		Payload:   payload,
		CreatedAt: ca,
	}
	if publishedAt.Valid {
		pa, _ := time.Parse(time.RFC3339Nano, publishedAt.String)
		snap.PublishedAt = &pa
	}
	if supersededBy.Valid {
		v := supersededBy.Int64
		snap.SupersededBy = &v
	}
	return snap, nil
}

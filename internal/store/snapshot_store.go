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

// NextVersion 取某熵源下一个快照版本号（发布版本单调）。
func (s *SnapshotStore) NextVersion(sourceID int64) (int, error) {
	var v int
	err := s.db.QueryRow(
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
	row := s.db.QueryRow(
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

// Supersede 将已发布快照标记为被 newID 替代。
func (s *SnapshotStore) Supersede(id, newID int64) error {
	_, err := s.db.Exec(
		`UPDATE diagnostic_snapshots SET state = ? WHERE id = ?`,
		model.SnapshotStateSuperseded, id)
	return mapErr(err)
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

package store

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/model"
)

// SourceStore 熵源持久化。
type SourceStore struct {
	db *sql.DB
}

// NewSourceStore 构造。
func NewSourceStore(db *sql.DB) *SourceStore { return &SourceStore{db: db} }

// Create 写入新熵源，初始态 enabled。
//
// 同名同设备唯一：在事务内先查重再插入，并以 uq_sources_name_device 唯一索引兜底。
// 并发重复注册时，仅一个请求成功创建并返回其 ID；其余请求收到携带已存在 ID 的
// ConflictError，由上层以冲突明确报告，而非模糊地各自成功。
func (s *SourceStore) Create(src *model.EntropySource, now time.Time) (int64, error) {
	if err := src.Validate(); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }() // 提交成功后为已提交事务的无操作回滚。

	// 先查重：若同名同设备已存在，直接以冲突报告，携带已存在 ID。
	if existing, err := txGetByNameDevice(tx, src.Name, src.Device); err != nil {
		if !model.IsNotFound(err) {
			return 0, err
		}
		// 未找到，继续插入。
	} else {
		return existing.ID, model.NewConflictError(existing.ID)
	}

	res, err := tx.Exec(
		`INSERT INTO entropy_sources(name, device, state, created_at, sealed_at, last_seq, recovery_baseline_seq)
			 VALUES(?,?,?,?,?,?,?)`,
		src.Name, src.Device, model.SourceStateEnabled, now.UTC().Format(time.RFC3339Nano),
		nullTimeVal(nil), 0, nil,
	)
	if err != nil {
		// 唯一索引兜底：并发竞态下对方刚插入，此处捕获唯一约束冲突，
		// 再回查已存在实体，报告携带其 ID 的冲突。
		if model.IsConflict(err) {
			if existing, qerr := txGetByNameDevice(tx, src.Name, src.Device); qerr == nil {
				return existing.ID, model.NewConflictError(existing.ID)
			}
		}
		return 0, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// Get 按 ID 读取。
func (s *SourceStore) Get(id int64) (*model.EntropySource, error) {
	row := s.db.QueryRow(
		`SELECT id, name, device, state, created_at, sealed_at, last_seq, recovery_baseline_seq
		 FROM entropy_sources WHERE id = ?`, id)
	return scanSource(row)
}

// GetByNameDevice 按名称+设备查重（注册幂等）。
func (s *SourceStore) GetByNameDevice(name, device string) (*model.EntropySource, error) {
	row := s.db.QueryRow(
		`SELECT id, name, device, state, created_at, sealed_at, last_seq, recovery_baseline_seq
		 FROM entropy_sources WHERE name = ? AND device = ?`, name, device)
	return scanSource(row)
}

// txGetByNameDevice 在事务内按名称+设备查重，供 Create 复用同一事务的可见性。
func txGetByNameDevice(tx *sql.Tx, name, device string) (*model.EntropySource, error) {
	row := tx.QueryRow(
		`SELECT id, name, device, state, created_at, sealed_at, last_seq, recovery_baseline_seq
		 FROM entropy_sources WHERE name = ? AND device = ?`, name, device)
	return scanSource(row)
}

// Count 熵源总数。
func (s *SourceStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM entropy_sources`).Scan(&n)
	return n, mapErr(err)
}

// List 分页列出（按创建时间倒序）。
func (s *SourceStore) List(limit, offset int) ([]*model.EntropySource, error) {
	rows, err := s.db.Query(
		`SELECT id, name, device, state, created_at, sealed_at, last_seq, recovery_baseline_seq
		 FROM entropy_sources ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*model.EntropySource, 0, limit)
	for rows.Next() {
		src, err := scanSourceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// UpdateState 仅更新状态与封存时间（状态机流转）。
func (s *SourceStore) UpdateState(id int64, state string, sealedAt *time.Time) error {
	_, err := s.db.Exec(
		`UPDATE entropy_sources SET state = ?, sealed_at = ? WHERE id = ?`,
		state, nullTimeVal(sealedAt), id)
	return mapErr(err)
}

// SetLastSeq 更新已确认最大窗口序号（序号幂等）。
func (s *SourceStore) SetLastSeq(id, lastSeq int64) error {
	_, err := s.db.Exec(`UPDATE entropy_sources SET last_seq = ? WHERE id = ?`, lastSeq, id)
	return mapErr(err)
}

// SetRecoveryBaseline 设定恢复基线序号（确认恢复时调用）。
func (s *SourceStore) SetRecoveryBaseline(id int64, baselineSeq *int64) error {
	_, err := s.db.Exec(`UPDATE entropy_sources SET recovery_baseline_seq = ? WHERE id = ?`, nullInt64(baselineSeq), id)
	return mapErr(err)
}

func scanSource(row scannable) (*model.EntropySource, error) {
	return scanSourceRow(row)
}

func scanSourceRow(row scannable) (*model.EntropySource, error) {
	var (
		id, lastSeq                 int64
		name, device, state         string
		createdAt                   string
		sealedAt                    sql.NullString
		recoveryBaselineSeqNullable sql.NullInt64
	)
	err := row.Scan(&id, &name, &device, &state, &createdAt, &sealedAt, &lastSeq, &recoveryBaselineSeqNullable)
	if err != nil {
		return nil, mapErr(err)
	}
	ca, _ := time.Parse(time.RFC3339Nano, createdAt)
	src := &model.EntropySource{
		ID:        id,
		Name:      name,
		Device:    device,
		State:     state,
		CreatedAt: ca,
		SealedAt:  scanNullTimeStr(sealedAt),
		LastSeq:   lastSeq,
	}
	if recoveryBaselineSeqNullable.Valid {
		v := recoveryBaselineSeqNullable.Int64
		src.RecoveryBaselineSeq = &v
	}
	return src, nil
}

func nullInt64(v *int64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

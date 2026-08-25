package snapshot

import (
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// Publisher 管理诊断快照的草稿/发布/替代生命周期。
type Publisher struct {
	snaps   *store.SnapshotStore
	builder *Builder
}

// NewPublisher 构造。
func NewPublisher(snaps *store.SnapshotStore, builder *Builder) *Publisher {
	return &Publisher{snaps: snaps, builder: builder}
}

// Draft 创建一份草稿快照（冻结当前证据）。
func (p *Publisher) Draft(sourceID int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	version, err := p.snaps.NextVersion(sourceID)
	if err != nil {
		return nil, err
	}
	payload, err := p.builder.BuildPayload(sourceID, now)
	if err != nil {
		return nil, err
	}
	snap := &model.DiagnosticSnapshot{
		SourceID: sourceID,
		Version:  version,
		State:    model.SnapshotStateDraft,
		Payload:  payload,
	}
	id, err := p.snaps.Create(snap, now)
	if err != nil {
		return nil, err
	}
	snap.ID = id
	return snap, nil
}

// Publish 发布草稿快照为不可变版本。
// 同一草稿并发发布时，store 的 UPDATE(state=draft) 仅会让一个请求受影响行数为 1，
// 其余请求得到 ErrTransition——即同一草稿有且仅有一次发布成功，
// 已发布快照的 state 与 published_at 永不被重复写入而保持不可变。
func (p *Publisher) Publish(id int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	existing, err := p.snaps.Get(id)
	if err != nil {
		return nil, err
	}
	if !existing.CanPublish() {
		// 已发布或已被替代：状态与发布时间不可变，拒绝重复发布。
		return nil, model.ErrTransition
	}
	if err := p.snaps.Publish(id, now); err != nil {
		// 并发赢家已先把草稿发布：本请求未完成发布，返回冲突而非覆盖发布时间。
		return nil, err
	}
	return p.snaps.Get(id)
}

// Supersede 以新草稿替代已发布快照（保留证据链）。
func (p *Publisher) Supersede(publishedID int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	old, err := p.snaps.Get(publishedID)
	if err != nil {
		return nil, err
	}
	if !old.CanSupersede() {
		return nil, model.ErrTransition
	}
	// 新版本基于当前证据重新冻结。
	draft, err := p.Draft(old.SourceID, now)
	if err != nil {
		return nil, err
	}
	if err := p.snaps.Publish(draft.ID, now); err != nil {
		return nil, err
	}
	if err := p.snaps.Supersede(old.ID, draft.ID); err != nil {
		return nil, err
	}
	return p.snaps.Get(draft.ID)
}

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
func (p *Publisher) Publish(id int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	snap, err := p.snaps.Get(id)
	if err != nil {
		return nil, err
	}
	if !snap.CanPublish() {
		return nil, model.ErrTransition
	}
	if err := p.snaps.Publish(id, now); err != nil {
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

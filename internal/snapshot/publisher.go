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
//
// 证据负载在事务外预先冻结（读取用独立连接，避免与写事务争用单连接），随后在
// 单个写事务内原子完成：写入新草稿、发布为新版本、令旧版本转入 superseded 且
// superseded_by 指向新版本。仅当旧版本仍处于 published 态时才替代；多个工作流
// 并发替代同一已发布快照时，只有第一个能成功，其余得到 ErrTransition，从而保证
// “同一旧版本只能被一个新版本替代”且新旧版本状态与替代关系一致。
func (p *Publisher) Supersede(publishedID int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	old, err := p.snaps.Get(publishedID)
	if err != nil {
		return nil, err
	}
	if !old.CanSupersede() {
		return nil, model.ErrTransition
	}
	// 证据在事务外冻结（读连接）；事务内只做写与并发认领。
	payload, err := p.builder.BuildPayload(old.SourceID, now)
	if err != nil {
		return nil, err
	}
	return p.snaps.SupersedePublished(old.ID, payload, now)
}

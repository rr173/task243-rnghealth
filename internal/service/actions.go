package service

import (
	"time"

	"task243-rnghealth/internal/model"
)

// ---- 熵源动作（状态机） ----

// DegradeSource 手动降级熵源。
func (s *Service) DegradeSource(id int64, now time.Time) error { return s.iso.Degrade(id, now) }

// IsolateSource 隔离熵源。
func (s *Service) IsolateSource(id int64, now time.Time) error { return s.iso.Isolate(id, now) }

// SealSource 封存熵源。
func (s *Service) SealSource(id int64, now time.Time) error { return s.iso.Seal(id, now) }

// RecoverSource 确认恢复并设定恢复基线。
func (s *Service) RecoverSource(id int64, now time.Time) error { return s.recovery.Confirm(id, now) }

// ---- 诊断快照 ----

// DraftSnapshot 创建草稿快照。
func (s *Service) DraftSnapshot(sourceID int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	return s.snapPub.Draft(sourceID, now)
}

// PublishSnapshot 发布草稿快照。
func (s *Service) PublishSnapshot(id int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	return s.snapPub.Publish(id, now)
}

// SupersedeSnapshot 以新版本替代已发布快照。
func (s *Service) SupersedeSnapshot(id int64, now time.Time) (*model.DiagnosticSnapshot, error) {
	return s.snapPub.Supersede(id, now)
}

// GetSnapshot 读取快照。
func (s *Service) GetSnapshot(id int64) (*model.DiagnosticSnapshot, error) { return s.snaps.Get(id) }

// ListSnapshots 列出快照。
func (s *Service) ListSnapshots(sourceID int64, limit, offset int) ([]*model.DiagnosticSnapshot, error) {
	return s.snaps.List(sourceID, limit, offset)
}

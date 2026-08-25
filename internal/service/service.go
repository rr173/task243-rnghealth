package service

import (
	"database/sql"
	"time"

	"task243-rnghealth/internal/correlate"
	"task243-rnghealth/internal/ingest"
	"task243-rnghealth/internal/isolate"
	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/snapshot"
	"task243-rnghealth/internal/store"
)

// Service 是领域能力的总编排层，供 HTTP 层调用。
type Service struct {
	sources  *store.SourceStore
	windows  *store.WindowStore
	tests    *store.HealthTestStore
	events   *store.EventStore
	restarts *store.RestartStore
	snaps    *store.SnapshotStore

	ingest   *ingest.Ingestor
	iso      *isolate.StateMachine
	recovery *isolate.Recovery
	snapPub  *snapshot.Publisher
	corr     *correlate.Correlator
}

// New 装配全部存储与业务包。
func New(db *sql.DB) *Service {
	sources := store.NewSourceStore(db)
	windows := store.NewWindowStore(db)
	tests := store.NewHealthTestStore(db)
	events := store.NewEventStore(db)
	restarts := store.NewRestartStore(db)
	snaps := store.NewSnapshotStore(db)

	corr := correlate.New(windows, events, restarts, correlate.DefaultPolicy())
	iso := isolate.New(sources)
	recovery := isolate.NewRecovery(sources, windows)
	ing := ingest.New(sources, windows, tests, corr, iso)
	snapBuilder := snapshot.NewBuilder(sources, windows, events, tests)
	snapPub := snapshot.NewPublisher(snaps, snapBuilder)

	return &Service{
		sources: sources, windows: windows, tests: tests, events: events,
		restarts: restarts, snaps: snaps,
		ingest: ing, iso: iso, recovery: recovery, snapPub: snapPub, corr: corr,
	}
}

// ---- 熵源 ----

// RegisterSource 注册熵源（同名同设备幂等冲突）。
func (s *Service) RegisterSource(name, device string, now time.Time) (int64, error) {
	if existing, err := s.sources.GetByNameDevice(name, device); err == nil && existing != nil {
		return 0, model.ErrConflict
	} else if err != nil && !model.IsNotFound(err) {
		return 0, err
	}
	src := &model.EntropySource{Name: name, Device: device}
	return s.sources.Create(src, now)
}

// GetSource 读取熵源。
func (s *Service) GetSource(id int64) (*model.EntropySource, error) { return s.sources.Get(id) }

// ListSources 列出熵源。
func (s *Service) ListSources(limit, offset int) ([]*model.EntropySource, error) {
	return s.sources.List(limit, offset)
}

// ---- 窗口摄入 ----

// IngestWindow 摄入单个采样窗口。
func (s *Service) IngestWindow(sourceID, seq int64, sample []byte, now time.Time) (*model.SampleWindow, error) {
	return s.ingest.Ingest(sample, sourceID, seq, now)
}

// BatchWindows 批量摄入。
func (s *Service) BatchWindows(sourceID int64, items []ingest.BatchItem, now time.Time) ([]*model.SampleWindow, []error) {
	wins, errs := s.ingest.Batch(sourceID, items, now)
	for i, err := range errs {
		if err != nil {
			return wins[:i+1], errs[:i+1]
		}
	}
	return wins, errs
}

// GetWindow 读取窗口。
func (s *Service) GetWindow(id int64) (*model.SampleWindow, error) { return s.windows.Get(id) }

// ListWindows 列出窗口。
func (s *Service) ListWindows(sourceID int64, status string, limit, offset int) ([]*model.SampleWindow, error) {
	return s.windows.List(sourceID, status, limit, offset)
}

// ---- 健康测试 ----

// ListWindowTests 列出窗口全部健康测试。
func (s *Service) ListWindowTests(windowID int64) ([]*model.HealthTest, error) {
	return s.tests.ListByWindow(windowID)
}

// ReevaluateWindow 依据已存测试结果重算窗口状态（不依赖原始样本）。
func (s *Service) ReevaluateWindow(windowID int64) (*model.SampleWindow, error) {
	tests, err := s.tests.ListByWindow(windowID)
	if err != nil {
		return nil, err
	}
	if len(tests) == 0 {
		return nil, model.ErrInvalidArgument
	}
	win, err := s.windows.Get(windowID)
	if err != nil {
		return nil, err
	}
	allPass := true
	firstFail := ""
	for _, t := range tests {
		if !t.Passed {
			allPass = false
			if firstFail == "" {
				firstFail = t.Category
			}
		}
	}
	status := model.WindowStatusValid
	anomaly := ""
	if !allPass {
		status = model.WindowStatusStatAnomaly
		anomaly = firstFail
	}
	if err := s.windows.UpdateStatus(windowID, status, anomaly); err != nil {
		return nil, err
	}
	win.Status = status
	win.AnomalyType = anomaly
	return win, nil
}

// HealthCategories 返回受支持的健康测试类别。
func (s *Service) HealthCategories() []string { return model.HealthTestCategories }

// ---- 健康事件 ----

// GetEvent 读取事件。
func (s *Service) GetEvent(id int64) (*model.HealthEvent, error) { return s.events.Get(id) }

// ListEvents 列出事件。
func (s *Service) ListEvents(sourceID int64, state string, limit, offset int) ([]*model.HealthEvent, error) {
	return s.events.List(sourceID, state, limit, offset)
}

// ConfirmEvent 工程师确认事件与熵源相关。
func (s *Service) ConfirmEvent(id int64, note string, now time.Time) (*model.HealthEvent, error) {
	ev, err := s.events.Get(id)
	if err != nil {
		return nil, err
	}
	if !ev.CanConfirm() {
		return nil, model.ErrTransition
	}
	if err := s.events.SetState(id, model.EventStateConfirmed, &now, nil); err != nil {
		return nil, err
	}
	if note != "" {
		_ = s.events.SetNote(id, note)
	}
	return s.events.Get(id)
}

// CloseEvent 关闭事件（已确认后）。
func (s *Service) CloseEvent(id int64, note string, now time.Time) (*model.HealthEvent, error) {
	ev, err := s.events.Get(id)
	if err != nil {
		return nil, err
	}
	if !ev.CanClose() {
		return nil, model.ErrTransition
	}
	if err := s.events.SetState(id, model.EventStateClosed, ev.ConfirmedAt, &now); err != nil {
		return nil, err
	}
	if note != "" {
		_ = s.events.SetNote(id, note)
	}
	return s.events.Get(id)
}

// ---- 重启边界 ----

// RecordRestart 记录设备重启边界。
func (s *Service) RecordRestart(sourceID, atSeq, baselineSeq int64, now time.Time) (*model.RestartBoundary, error) {
	src, err := s.sources.Get(sourceID)
	if err != nil {
		return nil, err
	}
	if src.IsSealed() {
		return nil, model.ErrSealed
	}
	rb := &model.RestartBoundary{SourceID: sourceID, AtSeq: atSeq, BaselineSeq: baselineSeq, BaselineMissing: baselineSeq == 0}
	id, err := s.restarts.Create(rb, now)
	if err != nil {
		return nil, err
	}
	rb.ID = id
	rb.OccurredAt = now
	return rb, nil
}

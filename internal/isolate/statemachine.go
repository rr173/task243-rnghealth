package isolate

import (
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// StateMachine 熵源状态机：启用 -> 观察 -> 降级 -> 隔离；任意态 -> 封存。
// 同一熵源的状态转换由 service 层串行保证。
type StateMachine struct {
	sources *store.SourceStore
}

// New 构造。
func New(sources *store.SourceStore) *StateMachine {
	return &StateMachine{sources: sources}
}

// Degrade 将熵源降级（持续异常被确认后的自动动作）。
func (m *StateMachine) Degrade(id int64, now time.Time) error {
	src, err := m.sources.Get(id)
	if err != nil {
		return err
	}
	switch src.State {
	case model.SourceStateEnabled, model.SourceStateObserving:
		return m.sources.UpdateState(id, model.SourceStateDegraded, nil)
	default:
		return model.ErrTransition
	}
}

// Observe 首窗摄入后将启用态推进为观察态（单向，幂等）。
func (m *StateMachine) Observe(id int64, now time.Time) error {
	src, err := m.sources.Get(id)
	if err != nil {
		return err
	}
	if src.State == model.SourceStateEnabled {
		return m.sources.UpdateState(id, model.SourceStateObserving, nil)
	}
	return nil
}

// Isolate 隔离熵源（工程师主动停用，停止用于生产）。
func (m *StateMachine) Isolate(id int64, now time.Time) error {
	src, err := m.sources.Get(id)
	if err != nil {
		return err
	}
	switch src.State {
	case model.SourceStateEnabled, model.SourceStateObserving, model.SourceStateDegraded:
		return m.sources.UpdateState(id, model.SourceStateIsolated, nil)
	default:
		return model.ErrTransition
	}
}

// Seal 封存熵源（诊断完成，拒绝一切写入）。
func (m *StateMachine) Seal(id int64, now time.Time) error {
	src, err := m.sources.Get(id)
	if err != nil {
		return err
	}
	if src.State == model.SourceStateSealed {
		return model.ErrTransition
	}
	return m.sources.UpdateState(id, model.SourceStateSealed, &now)
}

// Recover 确认恢复：降级/隔离 -> 观察，并设定恢复基线序号。
func (m *StateMachine) Recover(id int64, baselineSeq int64, now time.Time) error {
	src, err := m.sources.Get(id)
	if err != nil {
		return err
	}
	switch src.State {
	case model.SourceStateDegraded, model.SourceStateIsolated:
		if err := m.sources.UpdateState(id, model.SourceStateObserving, nil); err != nil {
			return err
		}
		return m.sources.SetRecoveryBaseline(id, &baselineSeq)
	default:
		return model.ErrTransition
	}
}

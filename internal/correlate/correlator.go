package correlate

import (
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// Policy 关联判定策略。
type Policy struct {
	// PersistentThreshold 连续异常窗口数达到该值即判定为持续异常（疑似熵源退化）。
	PersistentThreshold int
}

// DefaultPolicy 默认策略：连续 3 个异常窗口判定为持续。
func DefaultPolicy() Policy {
	return Policy{PersistentThreshold: 3}
}

// Correlator 将离散窗口异常关联到健康事件，并判定瞬态/持续。
type Correlator struct {
	windows  *store.WindowStore
	events   *store.EventStore
	restarts *store.RestartStore
	policy   Policy
}

// New 构造关联器。
func New(windows *store.WindowStore, events *store.EventStore, restarts *store.RestartStore, policy Policy) *Correlator {
	return &Correlator{windows: windows, events: events, restarts: restarts, policy: policy}
}

// OnWindow 处理一个已落库窗口的关联判定，返回是否应将熵源降级。
// 同一熵源的状态转换由 service 层串行保证。
func (c *Correlator) OnWindow(src *model.EntropySource, win *model.SampleWindow, now time.Time) (shouldDegrade bool, err error) {
	anomaly := win.Status != model.WindowStatusValid

	// 计算尾部连续异常游程（recent[0] 即当前窗口，序号倒序）。
	recent, err := c.windows.RecentWindows(src.ID, win.Seq, c.policy.PersistentThreshold+2)
	if err != nil {
		return false, err
	}
	runLen := 0
	firstSeq := win.Seq
	for _, w := range recent {
		if w.Status == model.WindowStatusValid {
			break
		}
		runLen++
		// recent 为序号倒序，遍历到最后一个异常窗口即游程起点（最小序号）。
		firstSeq = w.Seq
	}

	// 判定异常是否跨越设备重启边界。
	afterRestart := false
	if runLen > 0 {
		if rb, e := c.restarts.LatestBefore(src.ID, win.Seq); e == nil && rb != nil {
			if rb.AtSeq < firstSeq-1 {
				afterRestart = true
			}
		}
	}

	if !anomaly {
		// 正常窗口打断异常游程：将未确认的候选/瞬时事件收尾为瞬时并关闭。
		open, e := c.events.OpenForSource(src.ID)
		if e == nil && open != nil {
			if open.State == model.EventStateCandidate || open.State == model.EventStateTransient {
				if err := c.events.SetState(open.ID, model.EventStateTransient, nil, &now); err != nil {
					return false, err
				}
				if err := c.events.SetState(open.ID, model.EventStateClosed, nil, &now); err != nil {
					return false, err
				}
			}
		}
		return false, nil
	}

	// 异常窗口：创建或升级事件。
	state := model.EventStateCandidate
	switch {
	case runLen >= c.policy.PersistentThreshold:
		state = model.EventStatePersistent
	case runLen > 1:
		state = model.EventStateTransient
	}

	open, e := c.events.OpenForSource(src.ID)
	if e != nil && !model.IsNotFound(e) {
		return false, e
	}

	if open == nil {
		ev := &model.HealthEvent{
			SourceID:     src.ID,
			State:        state,
			AnomalyType:  win.AnomalyType,
			FirstSeq:     firstSeq,
			LastSeq:      win.Seq,
			WindowCount:  runLen,
			AfterRestart: afterRestart,
			CreatedAt:    now,
			Note:         "",
		}
		if _, err := c.events.Create(ev, now); err != nil {
			return false, err
		}
	} else {
		newState := open.State
		if state == model.EventStatePersistent {
			newState = model.EventStatePersistent
		}
		if err := c.events.SetState(open.ID, newState, open.ConfirmedAt, open.ClosedAt); err != nil {
			return false, err
		}
		// 更新游程区间与类别（用最新一次写入覆盖统计）。
		if err := c.events.UpdateRun(open.ID, firstSeq, win.Seq, runLen, win.AnomalyType, afterRestart); err != nil {
			return false, err
		}
	}

	// 持续异常且熵源仍在观察/启用态 → 应降级。
	shouldDegrade = state == model.EventStatePersistent &&
		(src.State == model.SourceStateEnabled || src.State == model.SourceStateObserving)
	return shouldDegrade, nil
}

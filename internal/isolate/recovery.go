package isolate

import (
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// Recovery 处理熵源恢复确认与恢复基线设定。
type Recovery struct {
	sources  *store.SourceStore
	windows  *store.WindowStore
	restarts *store.RestartStore
}

// NewRecovery 构造。
func NewRecovery(sources *store.SourceStore, windows *store.WindowStore, restarts *store.RestartStore) *Recovery {
	return &Recovery{sources: sources, windows: windows, restarts: restarts}
}

// Confirm 确认熵源恢复：以「最近有效窗口序号」为恢复基线，将状态转回观察。
// 恢复基线缺失（无有效窗口）时返回 0 基线并仍允许恢复，由快照环节标注。
func (r *Recovery) Confirm(sourceID int64, now time.Time) error {
	src, err := r.sources.Get(sourceID)
	if err != nil {
		return err
	}
	if src.State != model.SourceStateDegraded && src.State != model.SourceStateIsolated {
		return model.ErrTransition
	}
	fromSeq := int64(0)
	if r.restarts != nil {
		if boundary, boundaryErr := r.restarts.LatestBefore(sourceID, src.LastSeq); boundaryErr == nil && boundary != nil {
			fromSeq = boundary.AtSeq
		}
	}
	baseline, err := r.windows.LatestValidSeqAfter(sourceID, fromSeq)
	if err != nil {
		return err
	}
	if err := r.sources.UpdateState(sourceID, model.SourceStateObserving, nil); err != nil {
		return err
	}
	return r.sources.SetRecoveryBaseline(sourceID, &baseline)
}

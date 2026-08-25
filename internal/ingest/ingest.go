package ingest

import (
	"sync"
	"time"

	"task243-rnghealth/internal/correlate"
	"task243-rnghealth/internal/health"
	"task243-rnghealth/internal/isolate"
	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/store"
)

// BatchItem 批量摄入的单项。
type BatchItem struct {
	Sample []byte
	Seq    int64
}

// Ingestor 接收模块：校验窗口、做熵估计/重复检测/统计测试，并触发关联与降级。
// 同一熵源的状态转换串行（按 source_id 加锁），避免 LastSeq 与事件状态竞争。
type Ingestor struct {
	sources *store.SourceStore
	windows *store.WindowStore
	eval    *health.Evaluator
	corr    *correlate.Correlator
	iso     *isolate.StateMachine
	mu      sync.Map // sourceID -> *sync.Mutex
}

// New 构造。
func New(sources *store.SourceStore, windows *store.WindowStore, tests *store.HealthTestStore, corr *correlate.Correlator, iso *isolate.StateMachine) *Ingestor {
	return &Ingestor{
		sources: sources,
		windows: windows,
		eval:    health.NewEvaluator(tests),
		corr:    corr,
		iso:     iso,
	}
}

// lockFor 取某熵源的串行锁。
func (i *Ingestor) lockFor(sourceID int64) func() {
	v, _ := i.mu.LoadOrStore(sourceID, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return func() { m.Unlock() }
}

// Ingest 摄入一个采样窗口：校验 → 熵估计 → 重复检测 → 统计测试 → 关联 → 可能降级。
func (i *Ingestor) Ingest(sample []byte, sourceID, seq int64, now time.Time) (*model.SampleWindow, error) {
	unlock := i.lockFor(sourceID)
	defer unlock()

	src, err := i.sources.Get(sourceID)
	if err != nil {
		return nil, err
	}
	if src.IsSealed() {
		return nil, model.ErrSealed
	}

	win := &model.SampleWindow{SourceID: sourceID, Seq: seq, ByteLen: len(sample)}
	if err := win.Validate(); err != nil {
		return nil, err
	}
	// 序号幂等：必须严格递增。
	if seq <= src.LastSeq {
		return nil, model.ErrConflict
	}

	est := health.EstimateEntropy(sample)
	rep := health.DetectRepetition(sample, i.prevHash(sourceID), i.recentHashes(sourceID, seq))

	win.EstEntropy = est
	win.RepeatScore = rep.Score
	win.SampleHash = health.SampleHash(sample)
	win.Status = model.WindowStatusNew
	win.ReceivedAt = now

	id, err := i.windows.Create(win, now)
	if err != nil {
		return nil, err
	}
	win.ID = id

	statStatus, statAnomaly, err := i.eval.Evaluate(sample, id, sourceID, now)
	if err != nil {
		return nil, err
	}

	finalStatus, finalAnomaly := resolveStatus(rep, statStatus, statAnomaly)
	win.Status = finalStatus
	win.AnomalyType = finalAnomaly
	if err := i.windows.UpdateStatus(id, finalStatus, finalAnomaly); err != nil {
		return nil, err
	}

	if err := i.sources.SetLastSeq(sourceID, seq); err != nil {
		return nil, err
	}

	// 首窗摄入：启用态推进为观察态。
	if src.State == model.SourceStateEnabled {
		if err := i.iso.Observe(sourceID, now); err != nil {
			return nil, err
		}
	}

	corrWin := &model.SampleWindow{ID: id, SourceID: sourceID, Seq: seq, Status: finalStatus, AnomalyType: finalAnomaly}
	shouldDegrade, err := i.corr.OnWindow(src, corrWin, now)
	if err != nil {
		return nil, err
	}
	if shouldDegrade {
		if err := i.iso.Degrade(sourceID, now); err != nil {
			return nil, err
		}
	}
	return win, nil
}

// Batch 批量摄入（同一熵源串行）。返回与入参对齐的结果与错误。
func (i *Ingestor) Batch(sourceID int64, items []BatchItem, now time.Time) ([]*model.SampleWindow, []error) {
	out := make([]*model.SampleWindow, len(items))
	errs := make([]error, len(items))
	for idx, it := range items {
		w, err := i.Ingest(it.Sample, sourceID, it.Seq, now)
		out[idx] = w
		errs[idx] = err
	}
	return out, errs
}

// resolveStatus 合并重复检测与统计测试结论，得到最终窗口状态。
func resolveStatus(rep health.RepetitionResult, statStatus, statAnomaly string) (string, string) {
	if rep.IsReplay {
		return model.WindowStatusRepeatAnomaly, model.AnomalyReplay
	}
	if statStatus == model.WindowStatusStatAnomaly {
		return model.WindowStatusStatAnomaly, statAnomaly
	}
	return model.WindowStatusValid, ""
}

// prevHash 取该熵源上一窗口哈希（重播检测基线）。
func (i *Ingestor) prevHash(sourceID int64) string {
	h, _, err := i.windows.PrevWindowHash(sourceID)
	if err != nil {
		return ""
	}
	return h
}

// recentHashes 取该熵源当前序号之前的若干窗口哈希（更长窗口重播识别）。
func (i *Ingestor) recentHashes(sourceID, seq int64) []string {
	recents, err := i.windows.RecentWindows(sourceID, seq-1, 8)
	if err != nil || len(recents) == 0 {
		return nil
	}
	out := make([]string, 0, len(recents))
	for _, w := range recents {
		out = append(out, w.SampleHash)
	}
	return out
}

package service

import (
	"task243-rnghealth/internal/model"
)

// WindowStat 窗口统计视图。
type WindowStat struct {
	Total         int     `json:"total"`
	Valid         int     `json:"valid"`
	RepeatAnomaly int     `json:"repeat_anomaly"`
	StatAnomaly   int     `json:"stat_anomaly"`
	AvgEntropy    float64 `json:"avg_entropy"`
}

// SourceAnalysis 单熵源端到端分析视图。
type SourceAnalysis struct {
	Source                  *model.EntropySource `json:"source"`
	Windows                 WindowStat           `json:"windows"`
	TestFailures            map[string]int       `json:"test_failures"`
	OpenEvents              []*model.HealthEvent `json:"open_events"`
	RestartCount            int                  `json:"restart_count"`
	RecoveryBaselinePresent bool                 `json:"recovery_baseline_present"`
}

// GlobalStats 全局统计。
type GlobalStats struct {
	Sources    int `json:"sources"`
	Windows    int `json:"windows"`
	Events     int `json:"events"`
	OpenEvents int `json:"open_events"`
	Snapshots  int `json:"snapshots"`
}

// SelfCheckResult 自检结果。
type SelfCheckResult struct {
	OK        bool              `json:"ok"`
	Checks    map[string]string `json:"checks"`
}

// Analysis 构建单熵源分析视图。
func (s *Service) Analysis(sourceID int64) (*SourceAnalysis, error) {
	src, err := s.sources.Get(sourceID)
	if err != nil {
		return nil, err
	}
	total, valid, repeat, stat, avg, err := s.windows.WindowStats(sourceID)
	if err != nil {
		return nil, err
	}
	failures, err := s.tests.SummaryForSource(sourceID, 5000)
	if err != nil {
		return nil, err
	}
	all, err := s.events.List(sourceID, "", 50, 0)
	if err != nil {
		return nil, err
	}
	open := make([]*model.HealthEvent, 0, len(all))
	for _, ev := range all {
		if !ev.IsClosed() {
			open = append(open, ev)
		}
	}
	restarts, err := s.restarts.List(sourceID)
	if err != nil {
		return nil, err
	}
	return &SourceAnalysis{
		Source:                  src,
		Windows:                 WindowStat{Total: total, Valid: valid, RepeatAnomaly: repeat, StatAnomaly: stat, AvgEntropy: avg},
		TestFailures:            failures,
		OpenEvents:              open,
		RestartCount:            len(restarts),
		RecoveryBaselinePresent: src.RecoveryBaselineSeq != nil,
	}, nil
}

// GlobalStats 汇总全局统计。
func (s *Service) GlobalStats() (*GlobalStats, error) {
	srcN, err := s.sources.Count()
	if err != nil {
		return nil, err
	}
	winN, err := s.windows.Count()
	if err != nil {
		return nil, err
	}
	evN, err := s.events.Count()
	if err != nil {
		return nil, err
	}
	openN, err := s.events.CountOpen()
	if err != nil {
		return nil, err
	}
	snapN, err := s.snaps.Count()
	if err != nil {
		return nil, err
	}
	return &GlobalStats{
		Sources: srcN, Windows: winN, Events: evN, OpenEvents: openN, Snapshots: snapN,
	}, nil
}

// SelfCheck 对核心读写链路做轻量自检。
func (s *Service) SelfCheck() *SelfCheckResult {
	checks := map[string]string{}
	ok := true

	if _, err := s.sources.Count(); err != nil {
		checks["sources"] = "FAIL: " + err.Error()
		ok = false
	} else {
		checks["sources"] = "ok"
	}
	if _, err := s.windows.Count(); err != nil {
		checks["windows"] = "FAIL: " + err.Error()
		ok = false
	} else {
		checks["windows"] = "ok"
	}
	if _, err := s.tests.SummaryForSource(0, 1); err != nil {
		checks["health_tests"] = "FAIL: " + err.Error()
		ok = false
	} else {
		checks["health_tests"] = "ok"
	}
	if _, err := s.events.Count(); err != nil {
		checks["events"] = "FAIL: " + err.Error()
		ok = false
	} else {
		checks["events"] = "ok"
	}
	if _, err := s.snaps.Count(); err != nil {
		checks["snapshots"] = "FAIL: " + err.Error()
		ok = false
	} else {
		checks["snapshots"] = "ok"
	}
	return &SelfCheckResult{OK: ok, Checks: checks}
}

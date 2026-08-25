package service_test

import (
	"testing"
	"time"

	"task243-rnghealth/internal/model"
	"task243-rnghealth/internal/service"
	"task243-rnghealth/internal/store"
)

// TestReevaluatePreservesReplayAnomaly 验证跨窗口重播窗口在重新评估后仍保留
// repeat_anomaly / replay 类别，而非退化为 valid 或 stat_anomaly。
func TestReevaluatePreservesReplayAnomaly(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/rnghealth.db")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	svc := service.New(db)
	now := time.Unix(5000, 0)
	sourceID, err := svc.RegisterSource("reeval-test", "device-r", now)
	if err != nil {
		t.Fatalf("register source: %v", err)
	}

	// 用一个统计上健康的样本作为基线（全 0~255 轮转，256 字节）。
	sample := make([]byte, 256)
	for i := range sample {
		sample[i] = byte(i)
	}

	// 窗口 1：基线，应当有效。
	first, err := svc.IngestWindow(sourceID, 1, sample, now)
	if err != nil {
		t.Fatalf("ingest first: %v", err)
	}
	if first.Status != model.WindowStatusValid {
		t.Fatalf("first status = %s, want valid", first.Status)
	}

	// 窗口 2：重放窗口 1 的样本，应判为跨窗口重播。
	replayed, err := svc.IngestWindow(sourceID, 2, sample, now.Add(time.Second))
	if err != nil {
		t.Fatalf("ingest replayed: %v", err)
	}
	if replayed.Status != model.WindowStatusRepeatAnomaly || replayed.AnomalyType != model.AnomalyReplay {
		t.Fatalf("replayed window = %+v, want repeat_anomaly/replay", replayed)
	}

	// 重新评估窗口 2：必须保留重播异常，而非退化为 valid / stat_anomaly。
	got, err := svc.ReevaluateWindow(replayed.ID)
	if err != nil {
		t.Fatalf("reevaluate replayed: %v", err)
	}
	if got.Status != model.WindowStatusRepeatAnomaly || got.AnomalyType != model.AnomalyReplay {
		t.Fatalf("reevaluated replayed window = %+v, want repeat_anomaly/replay", got)
	}

	// 重新评估窗口 1（有效窗口）：应保持有效，不被重播证据误判。
	reFirst, err := svc.ReevaluateWindow(first.ID)
	if err != nil {
		t.Fatalf("reevaluate first: %v", err)
	}
	if reFirst.Status != model.WindowStatusValid || reFirst.AnomalyType != "" {
		t.Fatalf("reevaluated first window = %+v, want valid", reFirst)
	}
}

package health

import "testing"

func TestDetectHashRepetitionMatchesPreviousAndRecent(t *testing.T) {
	cur := SampleHash([]byte("replayed-window"))

	// 无前序证据：不判为重播。
	if got := DetectHashRepetition(cur, "", nil); got.IsReplay {
		t.Fatalf("no evidence should not be replay, got %+v", got)
	}

	// 与上一窗口哈希相同：判为重播，得分 1。
	if got := DetectHashRepetition(cur, cur, nil); !got.IsReplay || got.Score != 1.0 {
		t.Fatalf("previous hash repeated = %+v, want replay score 1", got)
	}

	// 与更早期窗口哈希相同：判为重播，得分 0.95。
	if got := DetectHashRepetition(cur, "different", []string{"other", cur}); !got.IsReplay || got.Score != 0.95 {
		t.Fatalf("recent hash repeated = %+v, want replay score 0.95", got)
	}

	// 全部不同：不判为重播。
	if got := DetectHashRepetition(cur, "different", []string{"other1", "other2"}); got.IsReplay {
		t.Fatalf("distinct hashes should not be replay, got %+v", got)
	}
}

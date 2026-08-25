package health

import (
	"crypto/sha256"
	"encoding/hex"
)

// RepetitionResult 重复重播检测结果。
type RepetitionResult struct {
	// Score 重复倾向得分（0~1，越大越像重播）。
	Score float64
	// IsReplay 是否判定为重播（与上一窗口或历史最近窗口字节级重复）。
	IsReplay bool
	// Detail 人类可读说明。
	Detail string
}

// DetectHashRepetition is used when reevaluation has only persisted hashes.
func DetectHashRepetition(sampleHash, prevHash string, recentHashes []string) RepetitionResult {
	if prevHash != "" && sampleHash == prevHash {
		return RepetitionResult{IsReplay: true, Score: 1, Detail: "previous hash repeated"}
	}
	return RepetitionResult{Score: 0}
}

// SampleHash 计算窗口原始样本的 SHA-256（十六进制）。
// 原始样本不落库，仅保存哈希用于重播检测，控制存储量。
func SampleHash(sample []byte) string {
	sum := sha256.Sum256(sample)
	return hex.EncodeToString(sum[:])
}

// DetectRepetition 检测跨窗口重复重播。
// prevHash 为该熵源上一窗口的样本哈希；recentHashes 为更早期窗口哈希（可选，用于更长窗口的重播识别）。
func DetectRepetition(sample []byte, prevHash string, recentHashes []string) RepetitionResult {
	cur := SampleHash(sample)
	score := 0.0
	detail := "no repetition"

	if prevHash != "" && cur == prevHash {
		score = 1.0
		return RepetitionResult{Score: 1.0, IsReplay: true, Detail: "identical to previous window (replay)"}
	}

	// 与历史最近窗口逐字节比较相似度（汉明式重合比例）。
	for _, h := range recentHashes {
		if h == cur {
			score = 0.95
			return RepetitionResult{Score: 0.95, IsReplay: true, Detail: "duplicate of an earlier window (replay)"}
		}
	}

	// 长重复串检测：统计最长连续相同字节长度。
	maxRun := maxIdenticalRun(sample)
	const longRunThreshold = 32
	if maxRun >= longRunThreshold {
		score = max(score, float64(maxRun)/float64(len(sample)))
		detail = "long identical-byte run"
	}

	// 单一字节占比过高（如全 0x00）也提高得分。
	dom := dominantByteRatio(sample)
	if dom > 0.8 {
		score = max(score, dom)
		if detail == "no repetition" {
			detail = "dominant byte dominates sample"
		}
	}

	if score > 0.9 {
		return RepetitionResult{Score: score, IsReplay: true, Detail: detail}
	}
	return RepetitionResult{Score: score, IsReplay: false, Detail: detail}
}

// maxIdenticalRun 返回样本中最长连续相同字节长度。
func maxIdenticalRun(sample []byte) int {
	if len(sample) == 0 {
		return 0
	}
	best, run := 1, 1
	for i := 1; i < len(sample); i++ {
		if sample[i] == sample[i-1] {
			run++
			if run > best {
				best = run
			}
		} else {
			run = 1
		}
	}
	return best
}

// dominantByteRatio 返回样本中出现次数最多的单字节占比。
func dominantByteRatio(sample []byte) float64 {
	if len(sample) == 0 {
		return 0
	}
	counts := [256]int{}
	for _, b := range sample {
		counts[b]++
	}
	best := 0
	for _, c := range counts {
		if c > best {
			best = c
		}
	}
	return float64(best) / float64(len(sample))
}

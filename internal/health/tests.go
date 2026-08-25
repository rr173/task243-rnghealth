package health

import (
	"math"

	"task243-rnghealth/internal/model"
)

// toBits 将字节样本转为 MSB 优先的布尔比特序列。
func toBits(sample []byte) []bool {
	bits := make([]bool, len(sample)*8)
	for i, b := range sample {
		for j := 0; j < 8; j++ {
			bits[i*8+j] = (b>>uint(7-j))&1 == 1
		}
	}
	return bits
}

// Monobit 单比特平衡性测试（NIST SP 800-90B 风格）。
// 统计 S_n = sum(2*bit - 1)；|S_n|/sqrt(n) 应接近标准正态。
func Monobit(sample []byte) (passed bool, statistic, threshold float64, detail string) {
	bits := toBits(sample)
	n := float64(len(bits))
	if n < 1 {
		return false, 0, 0, "empty sample"
	}
	s := 0.0
	for _, bit := range bits {
		if bit {
			s += 1
		} else {
			s -= 1
		}
	}
	stat := math.Abs(s) / math.Sqrt(n)
	threshold = 2.5
	passed = stat <= threshold
	detail = ""
	return
}

// Runs 游程测试：统计连续相同比特的段数，检验其是否符合随机期望。
func Runs(sample []byte) (passed bool, statistic, threshold float64, detail string) {
	bits := toBits(sample)
	n := len(bits)
	if n < 2 {
		return false, 0, 0, "sample too short"
	}
	runs := 1
	for i := 1; i < n; i++ {
		if bits[i] != bits[i-1] {
			runs++
		}
	}
	expected := float64(n+1) / 2
	variance := float64(n-1) / 4
	z := (float64(runs) - expected) / math.Sqrt(variance)
	stat := math.Abs(z)
	threshold = 2.5
	passed = stat <= threshold
	detail = ""
	return
}

// Poker 扑克测试：将比特按 k 位分块，检验块频率的卡方分布。
func Poker(sample []byte) (passed bool, statistic, threshold float64, detail string) {
	const k = 4
	nbits := len(sample) * 8
	blocks := nbits / k
	if blocks < 16 {
		return false, 0, 0, "too few blocks"
	}
	counts := [1 << 4]int{}
	for i := 0; i < blocks; i++ {
		val := 0
		for j := 0; j < k; j++ {
			idx := i*k + j
			bit := int((sample[idx/8] >> uint(7-(idx%8))) & 1)
			val = (val << 1) | bit
		}
		counts[val]++
	}
	sumSq := 0
	for _, c := range counts {
		sumSq += c * c
	}
	chi2 := (float64(1<<k)/float64(blocks))*float64(sumSq) - float64(blocks)
	threshold = 32.8 // 15 自由度下 ~99% 临界值
	passed = chi2 <= threshold
	statistic = chi2
	detail = ""
	return
}

// LongRun 长游程测试：检测是否存在过长的连续相同比特（指示停滞/故障）。
func LongRun(sample []byte) (passed bool, statistic, threshold float64, detail string) {
	bits := toBits(sample)
	maxRun, run := 1, 1
	for i := 1; i < len(bits); i++ {
		if bits[i] == bits[i-1] {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 1
		}
	}
	threshold = 32
	stat := float64(maxRun)
	passed = stat < threshold
	detail = ""
	return
}

// TestResult 单项测试结果。
type TestResult struct {
	Category  string
	Passed    bool
	Statistic float64
	Threshold float64
}

// RunStatisticalTests 运行全部四项统计健康测试，返回结果列表。
func RunStatisticalTests(sample []byte) []TestResult {
	mbP, mbS, mbT, _ := Monobit(sample)
	rP, rS, rT, _ := Runs(sample)
	pP, pS, pT, _ := Poker(sample)
	lP, lS, lT, _ := LongRun(sample)
	return []TestResult{
		{Category: model.TestMonobit, Passed: mbP, Statistic: mbS, Threshold: mbT},
		{Category: model.TestRuns, Passed: rP, Statistic: rS, Threshold: rT},
		{Category: model.TestPoker, Passed: pP, Statistic: pS, Threshold: pT},
		{Category: model.TestLongRun, Passed: lP, Statistic: lS, Threshold: lT},
	}
}

// AllPassed 判断所有测试是否通过。
func AllPassed(results []TestResult) bool {
	for _, r := range results {
		if !r.Passed {
			return false
		}
	}
	return true
}

// FirstFailing 返回第一个未通过的测试类别（用于标记 anomaly_type）。
func FirstFailing(results []TestResult) string {
	for _, r := range results {
		if !r.Passed {
			return r.Category
		}
	}
	return ""
}

// CategoryMeta 测试类别元信息（供 API 暴露阈值与含义）。
type CategoryMeta struct {
	Category    string  `json:"category"`
	Threshold   float64 `json:"threshold"`
	Description string  `json:"description"`
}

// CategoryMetaList 返回全部受支持测试类别的元信息。
func CategoryMetaList() []CategoryMeta {
	return []CategoryMeta{
		{Category: model.TestMonobit, Threshold: 2.5, Description: "单比特平衡性：|S_n|/sqrt(n) 应接近标准正态"},
		{Category: model.TestRuns, Threshold: 2.5, Description: "游程数：与随机期望的 z 分数应落在 ±2.5 内"},
		{Category: model.TestPoker, Threshold: 32.8, Description: "分块频率卡方（15 自由度约 99% 临界值）"},
		{Category: model.TestLongRun, Threshold: 32, Description: "最长连续相同比特长度（>=32 即判停滞/故障）"},
	}
}

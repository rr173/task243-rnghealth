package health

import "math"

// EstimateEntropy 估计样本的香农熵（bits/byte，取值 0~8）。
// 基于 256 个字节取值的频率分布。
func EstimateEntropy(sample []byte) float64 {
	if len(sample) == 0 {
		return 0
	}
	counts := [256]int{}
	for _, b := range sample {
		counts[b]++
	}
	n := float64(len(sample))
	var h float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// EstimateMinEntropy 估计样本的最小熵（bits/byte）。
// min-entropy = -log2(max p)，比香农熵更保守，是密码学健康度常用下界。
func EstimateMinEntropy(sample []byte) float64 {
	if len(sample) == 0 {
		return 0
	}
	counts := [256]int{}
	for _, b := range sample {
		counts[b]++
	}
	n := float64(len(sample))
	maxP := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		if p > maxP {
			maxP = p
		}
	}
	if maxP <= 0 {
		return 0
	}
	return -math.Log2(maxP)
}

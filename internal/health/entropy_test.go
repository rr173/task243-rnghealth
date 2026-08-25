package health

import "testing"

func TestEstimateEntropyAndMinEntropy(t *testing.T) {
	constant := make([]byte, 256)
	if got := EstimateEntropy(constant); got != 0 {
		t.Fatalf("constant sample Shannon entropy = %v, want 0", got)
	}
	if got := EstimateMinEntropy(constant); got != 0 {
		t.Fatalf("constant sample min entropy = %v, want 0", got)
	}

	balanced := make([]byte, 256)
	for i := range balanced {
		balanced[i] = byte(i)
	}
	if got := EstimateEntropy(balanced); got < 7.9 {
		t.Fatalf("balanced sample Shannon entropy = %v, want close to 8", got)
	}
	if got := EstimateMinEntropy(balanced); got < 7.9 {
		t.Fatalf("balanced sample min entropy = %v, want close to 8", got)
	}
}

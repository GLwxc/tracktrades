package tests

import (
	"testing"

	"tracktrades/internal/util"
)

func TestRequiredRecoveryPct(t *testing.T) {
	tests := []struct {
		loss float64
		want float64
	}{
		{0, 0},
		{10, 11.1111111111},
		{50, 100},
	}

	for _, tt := range tests {
		got := util.RequiredRecoveryPct(tt.loss)
		if tt.want == 0 {
			if got != 0 {
				t.Errorf("loss=%.2f got=%.6f want 0", tt.loss, got)
			}
			continue
		}
		diff := (got - tt.want) / tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			t.Errorf("loss=%.2f got=%.6f want≈%.6f", tt.loss, got, tt.want)
		}
	}
}

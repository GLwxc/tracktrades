package util

import "math"

func RequiredRecoveryPct(lossPct float64) float64 {
	if lossPct <= 0 {
		return 0
	}
	if lossPct >= 100 {
		return math.Inf(1)
	}
	return (lossPct / (100 - lossPct)) * 100
}

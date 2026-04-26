package reco

import (
	"sort"
)

type Engine struct {
	WindowSize        int
	MinSamples        int
	DiscountThreshold float64
}

type Decision struct {
	BaselinePrice float64
	CurrentPrice  float64
	DiscountPct   float64
	Recommend     bool
}

func (e Engine) Evaluate(recentPrices []float64, currentPrice float64) Decision {
	if e.WindowSize <= 0 {
		e.WindowSize = 30
	}
	if e.MinSamples <= 0 {
		e.MinSamples = 8
	}
	if e.DiscountThreshold <= 0 {
		e.DiscountThreshold = 0.2
	}

	if len(recentPrices) > e.WindowSize {
		recentPrices = recentPrices[:e.WindowSize]
	}

	if len(recentPrices) < e.MinSamples || currentPrice <= 0 {
		return Decision{
			CurrentPrice: currentPrice,
			Recommend:    false,
		}
	}

	baseline := median(recentPrices)
	if baseline <= 0 {
		return Decision{CurrentPrice: currentPrice}
	}

	discount := (baseline - currentPrice) / baseline
	return Decision{
		BaselinePrice: baseline,
		CurrentPrice:  currentPrice,
		DiscountPct:   discount,
		Recommend:     discount >= e.DiscountThreshold,
	}
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cloned := append([]float64(nil), values...)
	sort.Float64s(cloned)
	mid := len(cloned) / 2
	if len(cloned)%2 == 1 {
		return cloned[mid]
	}
	return (cloned[mid-1] + cloned[mid]) / 2
}


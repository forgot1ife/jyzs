package reco

import "testing"

func TestEngineEvaluateRecommend(t *testing.T) {
	engine := Engine{
		WindowSize:        10,
		MinSamples:        5,
		DiscountThreshold: 0.2,
	}

	recent := []float64{100, 95, 105, 110, 90, 100}
	decision := engine.Evaluate(recent, 70)

	if !decision.Recommend {
		t.Fatalf("expected recommendation, got %+v", decision)
	}
	if decision.BaselinePrice != 100 {
		t.Fatalf("expected baseline 100, got %.2f", decision.BaselinePrice)
	}
}

func TestEngineEvaluateNotEnoughSamples(t *testing.T) {
	engine := Engine{
		WindowSize:        10,
		MinSamples:        5,
		DiscountThreshold: 0.2,
	}

	recent := []float64{100, 101, 102}
	decision := engine.Evaluate(recent, 70)
	if decision.Recommend {
		t.Fatalf("expected no recommendation, got %+v", decision)
	}
}


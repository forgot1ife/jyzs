package processor

import (
	"testing"

	"jyzs_proxy_poc/internal/reco"
	"jyzs_proxy_poc/internal/storage"
)

type mockStore struct {
	events          []storage.CaptureEvent
	snapshots       []storage.PriceSnapshot
	recommendations []storage.Recommendation
	recent          []float64
}

func (m *mockStore) InsertCaptureEvent(evt storage.CaptureEvent) error {
	m.events = append(m.events, evt)
	return nil
}

func (m *mockStore) InsertPriceSnapshot(snapshot storage.PriceSnapshot) error {
	m.snapshots = append(m.snapshots, snapshot)
	return nil
}

func (m *mockStore) RecentPrices(_ string, _ int) ([]float64, error) {
	return m.recent, nil
}

func (m *mockStore) InsertRecommendation(rec storage.Recommendation) error {
	m.recommendations = append(m.recommendations, rec)
	return nil
}

func TestProcessorHandleMarketItemRecommendation(t *testing.T) {
	store := &mockStore{
		recent: []float64{100, 98, 102, 105, 95, 100, 99, 101},
	}
	p := New(store, reco.Engine{
		WindowSize:        20,
		MinSamples:        5,
		DiscountThreshold: 0.15,
	})

	err := p.Handle(EventInput{
		RuleName:   "market_item_json",
		RecordType: "market_item",
		Method:     "GET",
		Host:       "example.com",
		Path:       "/api/market/item",
		StatusCode: 200,
		Fields: map[string]any{
			"item_key":   "sword_1",
			"item_name":  "玄铁剑",
			"unit_price": 70.0,
			"quantity":   1,
		},
	})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if len(store.snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(store.snapshots))
	}
	if len(store.recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(store.recommendations))
	}
}


package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jyzs_proxy_poc/internal/storage"
)

type mockAdminStore struct {
	recommendations []storage.Recommendation
	counts          map[string]int64
	events          []storage.CaptureEvent
	snapshots       []storage.PriceSnapshot
	characterLatest *storage.CharacterStatus

	lastRecoLimit   int
	lastEventsLimit int
	lastEventsRule  string
	lastPriceLimit  int
	lastPriceItem   string
}

func (m *mockAdminStore) ListRecommendations(limit int) ([]storage.Recommendation, error) {
	m.lastRecoLimit = limit
	return m.recommendations, nil
}

func (m *mockAdminStore) Counts() (map[string]int64, error) {
	return m.counts, nil
}

func (m *mockAdminStore) ListCaptureEvents(limit int, ruleName string) ([]storage.CaptureEvent, error) {
	m.lastEventsLimit = limit
	m.lastEventsRule = ruleName
	return m.events, nil
}

func (m *mockAdminStore) ListPriceSnapshots(limit int, itemKey string) ([]storage.PriceSnapshot, error) {
	m.lastPriceLimit = limit
	m.lastPriceItem = itemKey
	return m.snapshots, nil
}

func (m *mockAdminStore) LatestCharacterStatus() (*storage.CharacterStatus, error) {
	return m.characterLatest, nil
}

func TestEventsEndpoint(t *testing.T) {
	store := &mockAdminStore{
		events: []storage.CaptureEvent{
			{
				At:         time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC),
				RuleName:   "market_item_json",
				RecordType: "market_item",
				Method:     http.MethodGet,
				Host:       "example.com",
				Path:       "/api/market/item",
				StatusCode: 200,
				Fields: map[string]any{
					"item_name": "sword",
				},
			},
		},
	}

	srv := NewServer("127.0.0.1:0", store)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events?limit=7&rule_name=market_item_json")
	if err != nil {
		t.Fatalf("get /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if store.lastEventsLimit != 7 {
		t.Fatalf("unexpected limit: %d", store.lastEventsLimit)
	}
	if store.lastEventsRule != "market_item_json" {
		t.Fatalf("unexpected rule_name: %s", store.lastEventsRule)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["items"]; !ok {
		t.Fatalf("items field missing in response")
	}
}

func TestPricesEndpoint(t *testing.T) {
	store := &mockAdminStore{
		snapshots: []storage.PriceSnapshot{
			{
				At:        time.Date(2026, 4, 26, 13, 0, 0, 0, time.UTC),
				ItemKey:   "sword_1",
				ItemName:  "sword",
				UnitPrice: 90,
				Quantity:  1,
			},
		},
	}

	srv := NewServer("127.0.0.1:0", store)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/prices?limit=12&item_key=sword_1")
	if err != nil {
		t.Fatalf("get /prices: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	if store.lastPriceLimit != 12 {
		t.Fatalf("unexpected limit: %d", store.lastPriceLimit)
	}
	if store.lastPriceItem != "sword_1" {
		t.Fatalf("unexpected item_key: %s", store.lastPriceItem)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := body["items"]; !ok {
		t.Fatalf("items field missing in response")
	}
}

func TestCharacterLatestEndpoint(t *testing.T) {
	store := &mockAdminStore{
		characterLatest: &storage.CharacterStatus{
			PlayerName:     "甄怼怼",
			LoginSucceeded: true,
			AreaName:       "武侠服专区",
			ServerName:     "侠骨丹心",
		},
	}

	srv := NewServer("127.0.0.1:0", store)
	ts := httptest.NewServer(srv.Handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/character/latest")
	if err != nil {
		t.Fatalf("get /character/latest: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	var body struct {
		Item storage.CharacterStatus `json:"item"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Item.PlayerName != "甄怼怼" {
		t.Fatalf("unexpected player_name: %q", body.Item.PlayerName)
	}
}

package rules

import (
	"net/http"
	"testing"
)

func TestRuleMatchesAndExtract(t *testing.T) {
	rule := Rule{
		Name:       "test_market",
		RecordType: "market_item",
		Match: Match{
			Method:       "GET",
			HostContains: "example.com",
			PathContains: "/api/market/item",
		},
		Extractors: []Extractor{
			{
				Field:   "item_name",
				Source:  "response_body",
				Kind:    "gjson",
				Pattern: "item.name",
			},
			{
				Field:   "unit_price",
				Source:  "response_body",
				Kind:    "gjson",
				Pattern: "item.price",
			},
			{
				Field:   "quality",
				Source:  "response_body",
				Kind:    "regex",
				Pattern: "\"quality\":\"([A-Z]+)\"",
				Group:   1,
			},
		},
	}
	req, err := http.NewRequest(http.MethodGet, "https://example.com/api/market/item?id=1", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	if !rule.Matches(req) {
		t.Fatalf("expected rule to match")
	}

	fields := rule.Extract(nil, []byte(`{"item":{"name":"玄铁剑","price":8888},"quality":"A"}`))
	if fields["item_name"] != "玄铁剑" {
		t.Fatalf("unexpected item_name: %#v", fields["item_name"])
	}
	if fields["unit_price"] != float64(8888) {
		t.Fatalf("unexpected unit_price: %#v", fields["unit_price"])
	}
	if fields["quality"] != "A" {
		t.Fatalf("unexpected quality: %#v", fields["quality"])
	}
}


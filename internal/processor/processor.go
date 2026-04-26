package processor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"jyzs_proxy_poc/internal/reco"
	"jyzs_proxy_poc/internal/storage"
)

type Store interface {
	InsertCaptureEvent(storage.CaptureEvent) error
	InsertPriceSnapshot(storage.PriceSnapshot) error
	RecentPrices(itemKey string, limit int) ([]float64, error)
	InsertRecommendation(storage.Recommendation) error
}

type Processor struct {
	store      Store
	recoEngine reco.Engine
}

type EventInput struct {
	RuleName     string
	RecordType   string
	Method       string
	Host         string
	Path         string
	StatusCode   int
	RequestSize  int
	ResponseSize int
	Fields       map[string]any
}

func New(store Store, recoEngine reco.Engine) *Processor {
	return &Processor{
		store:      store,
		recoEngine: recoEngine,
	}
}

func (p *Processor) Handle(input EventInput) error {
	now := time.Now()
	if err := p.store.InsertCaptureEvent(storage.CaptureEvent{
		At:           now,
		RuleName:     input.RuleName,
		RecordType:   input.RecordType,
		Method:       input.Method,
		Host:         input.Host,
		Path:         input.Path,
		StatusCode:   input.StatusCode,
		RequestSize:  input.RequestSize,
		ResponseSize: input.ResponseSize,
		Fields:       input.Fields,
	}); err != nil {
		return err
	}

	if input.RecordType != "market_item" {
		return nil
	}

	snapshot, ok := buildPriceSnapshot(now, input.RuleName, input.Fields)
	if !ok {
		return nil
	}
	if err := p.store.InsertPriceSnapshot(snapshot); err != nil {
		return err
	}

	prices, err := p.store.RecentPrices(snapshot.ItemKey, p.recoEngine.WindowSize)
	if err != nil {
		return err
	}
	decision := p.recoEngine.Evaluate(prices, snapshot.UnitPrice)
	if !decision.Recommend {
		return nil
	}

	reason := fmt.Sprintf(
		"price %.2f is %.2f%% below median baseline %.2f",
		decision.CurrentPrice,
		decision.DiscountPct*100,
		decision.BaselinePrice,
	)
	return p.store.InsertRecommendation(storage.Recommendation{
		At:            now,
		ItemKey:       snapshot.ItemKey,
		ItemName:      snapshot.ItemName,
		MarketPrice:   decision.CurrentPrice,
		BaselinePrice: decision.BaselinePrice,
		DiscountPct:   decision.DiscountPct,
		Reason:        reason,
	})
}

func buildPriceSnapshot(at time.Time, ruleName string, fields map[string]any) (storage.PriceSnapshot, bool) {
	itemName := asString(fields["item_name"])
	itemKey := asString(fields["item_key"])
	if itemKey == "" {
		itemKey = itemName
	}
	price, ok := asFloat(fields["unit_price"])
	if !ok || itemKey == "" || itemName == "" {
		return storage.PriceSnapshot{}, false
	}

	quantity := int64(1)
	if q, ok := asInt64(fields["quantity"]); ok && q > 0 {
		quantity = q
	}
	raw, _ := json.Marshal(fields)
	return storage.PriceSnapshot{
		At:         at,
		ItemKey:    itemKey,
		ItemName:   itemName,
		UnitPrice:  price,
		Quantity:   quantity,
		SourceRule: ruleName,
		RawJSON:    string(raw),
	}, true
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}


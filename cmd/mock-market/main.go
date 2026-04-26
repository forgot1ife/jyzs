package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type marketResponse struct {
	Item struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Quantity int64   `json:"quantity"`
	} `json:"item"`
}

var itemPool = map[string]struct {
	Name      string
	BasePrice float64
}{
	"sword_1": {Name: "玄铁剑", BasePrice: 100},
	"pill_1":  {Name: "九花玉露丸", BasePrice: 65},
	"book_1":  {Name: "内功残页", BasePrice: 220},
}

func main() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	http.HandleFunc("/api/market/item", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			id = "sword_1"
		}
		def, ok := itemPool[id]
		if !ok {
			http.Error(w, "unknown id", http.StatusNotFound)
			return
		}

		discount := 0.8 + rng.Float64()*0.5 // 0.8 ~ 1.3
		if v := r.URL.Query().Get("price"); v != "" {
			if p, err := strconv.ParseFloat(v, 64); err == nil && p > 0 {
				discount = p / def.BasePrice
			}
		}

		var resp marketResponse
		resp.Item.ID = id
		resp.Item.Name = def.Name
		resp.Item.Price = def.BasePrice * discount
		resp.Item.Quantity = int64(1 + rng.Intn(9))

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(resp)
	})

	log.Println("mock market API on http://127.0.0.1:19090")
	log.Fatal(http.ListenAndServe("127.0.0.1:19090", nil))
}


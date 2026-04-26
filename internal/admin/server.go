package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"jyzs_proxy_poc/internal/storage"
)

type Store interface {
	ListRecommendations(limit int) ([]storage.Recommendation, error)
	ListCaptureEvents(limit int, ruleName string) ([]storage.CaptureEvent, error)
	ListPriceSnapshots(limit int, itemKey string) ([]storage.PriceSnapshot, error)
	LatestCharacterStatus() (*storage.CharacterStatus, error)
	Counts() (map[string]int64, error)
}

func NewServer(addr string, store Store) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		counts, err := store.Counts()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"counts": counts,
		})
	})

	mux.HandleFunc("/recommendations", func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r, 20)
		rows, err := store.ListRecommendations(limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": rows,
		})
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r, 20)
		ruleName := r.URL.Query().Get("rule_name")
		rows, err := store.ListCaptureEvents(limit, ruleName)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": rows,
		})
	})

	mux.HandleFunc("/prices", func(w http.ResponseWriter, r *http.Request) {
		limit := parseLimit(r, 20)
		itemKey := r.URL.Query().Get("item_key")
		rows, err := store.ListPriceSnapshots(limit, itemKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"items": rows,
		})
	})

	mux.HandleFunc("/character/latest", func(w http.ResponseWriter, _ *http.Request) {
		row, err := store.LatestCharacterStatus()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"item": row,
		})
	})

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

func parseLimit(r *http.Request, defaultVal int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return defaultVal
	}
	if v > 200 {
		return 200
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

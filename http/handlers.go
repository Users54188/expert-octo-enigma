package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cloudquant/db"
	"cloudquant/market"
)

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/tick/{symbol}", handleTick)
	mux.HandleFunc("GET /api/indicators/{symbol}", handleIndicators)
	mux.HandleFunc("GET /api/klines/{symbol}", handleKLines)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleTick(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	tick, err := market.FetchTick(symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tick)
}

func handleIndicators(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil {
			days = d
		}
	}

	// Fetch historical data to calculate indicators
	klines, err := market.FetchHistoricalData(symbol, days+30) // Fetch more to be sure
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(klines) == 0 {
		http.Error(w, "no data found", http.StatusNotFound)
		return
	}

	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}

	ma5 := market.CalculateMA(closes, 5)
	ma20 := market.CalculateMA(closes, 20)
	rsi := market.CalculateRSI(closes, 14)
	diff, _, _ := market.CalculateMACD(closes)

	response := map[string]interface{}{
		"symbol":    symbol,
		"ma5":       ma5,
		"ma20":      ma20,
		"rsi":       rsi,
		"macd":      diff,
		"timestamp": klines[len(klines)-1].Timestamp,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleKLines(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	klines, err := db.QueryKLines(symbol, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If DB is empty, try to fetch and save some
	if len(klines) == 0 {
		klines, err = market.FetchHistoricalData(symbol, limit)
		if err == nil {
			for i := range klines {
				// Calculate indicators for each kline
				// This is slow but for the sake of demo...
				// Actually we should calculate them properly.
				
				// Simplified: just save klines without indicators or with current ones
				// Better: calculate indicators for each point
				subset := make([]float64, 0)
				fullKlines, _ := market.FetchHistoricalData(symbol, limit+30)
				for j, fk := range fullKlines {
					subset = append(subset, fk.Close)
					if j >= 30 {
						fk.Indicators.MA5 = market.CalculateMA(subset, 5)
						fk.Indicators.MA20 = market.CalculateMA(subset, 20)
						fk.Indicators.RSI = market.CalculateRSI(subset, 14)
						fk.Indicators.MACD, _, _ = market.CalculateMACD(subset)
						db.SaveKLine(fk)
					}
				}
				// Re-query
				klines, _ = db.QueryKLines(symbol, limit)
			}
		}
	}

	response := map[string]interface{}{
		"symbol": symbol,
		"data":   klines,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloudquant/db"
	"cloudquant/llm"
	"cloudquant/market"
	"cloudquant/ml"
)

type Analyzer interface {
	Analyze(ctx context.Context, kline market.KLine, indicator market.Indicator) (*llm.AnalysisResult, error)
}

type ModelProvider interface {
	Predict(features []float64) (int, float64, error)
}

var deepSeekAnalyzer Analyzer
var mlModel ModelProvider
var trainingConfig TrainingConfig
var fetchLatestMarketData = loadLatestMarketData
var latestFeatureBuilder = buildLatestFeatures

func RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/tick/{symbol}", handleTick)
	mux.HandleFunc("GET /api/indicators/{symbol}", handleIndicators)
	mux.HandleFunc("GET /api/klines/{symbol}", handleKLines)
	mux.HandleFunc("GET /api/analysis/{symbol}", handleAnalysis)
	mux.HandleFunc("GET /api/analysis/batch", handleBatchAnalysis)
	mux.HandleFunc("POST /api/train", handleTrain)
	mux.HandleFunc("GET /api/predict/{symbol}", handlePredict)
}

func SetAnalyzer(analyzer Analyzer) {
	deepSeekAnalyzer = analyzer
}

func SetModelProvider(model ModelProvider) {
	mlModel = model
}

func SetTrainingConfig(config TrainingConfig) {
	trainingConfig = config
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

	klines, err := market.FetchHistoricalData(symbol, days+30)
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

	if len(klines) == 0 {
		klines, err = market.FetchHistoricalData(symbol, limit)
		if err == nil {
			fullKlines, _ := market.FetchHistoricalData(symbol, limit+30)
			subset := make([]float64, 0, len(fullKlines))
			for j, fk := range fullKlines {
				subset = append(subset, fk.Close)
				if j >= 30 {
					fk.Indicators.MA5 = market.CalculateMA(subset, 5)
					fk.Indicators.MA20 = market.CalculateMA(subset, 20)
					fk.Indicators.RSI = market.CalculateRSI(subset, 14)
					fk.Indicators.MACD, _, _ = market.CalculateMACD(subset)
					if err := db.SaveKLine(fk); err != nil {
						// Non-fatal: cache save failure shouldn't stop the request
					}
				}
			}
			klines, _ = db.QueryKLines(symbol, limit)
		}
	}

	response := map[string]interface{}{
		"symbol": symbol,
		"data":   klines,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleAnalysis(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}
	if deepSeekAnalyzer == nil {
		http.Error(w, "deepseek analyzer not configured", http.StatusServiceUnavailable)
		return
	}

	kline, indicator, err := fetchLatestMarketData(symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	result, err := deepSeekAnalyzer.Analyze(ctx, kline, indicator)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	response := map[string]interface{}{
		"symbol":    symbol,
		"trend":     result.Trend,
		"risk":      result.Risk,
		"action":    result.Action,
		"reason":    result.Reason,
		"timestamp": time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleBatchAnalysis(w http.ResponseWriter, r *http.Request) {
	if deepSeekAnalyzer == nil {
		http.Error(w, "deepseek analyzer not configured", http.StatusServiceUnavailable)
		return
	}

	symbolsParam := r.URL.Query().Get("symbols")
	if symbolsParam == "" {
		http.Error(w, "symbols are required", http.StatusBadRequest)
		return
	}

	symbols := strings.Split(symbolsParam, ",")
	results := make([]map[string]interface{}, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		kline, indicator, err := fetchLatestMarketData(symbol)
		if err != nil {
			results = append(results, map[string]interface{}{"symbol": symbol, "error": err.Error()})
			continue
		}

		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		result, err := deepSeekAnalyzer.Analyze(ctx, kline, indicator)
		cancel()
		if err != nil {
			results = append(results, map[string]interface{}{"symbol": symbol, "error": err.Error()})
			continue
		}
		results = append(results, map[string]interface{}{
			"symbol":    symbol,
			"trend":     result.Trend,
			"risk":      result.Risk,
			"action":    result.Action,
			"reason":    result.Reason,
			"timestamp": time.Now().UTC(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func handleTrain(w http.ResponseWriter, r *http.Request) {
	if trainingConfig.Symbol == "" {
		http.Error(w, "training config not set", http.StatusServiceUnavailable)
		return
	}
	if err := trainModel(trainingConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{"status": "training_completed"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handlePredict(w http.ResponseWriter, r *http.Request) {
	symbol := r.PathValue("symbol")
	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return
	}
	if mlModel == nil {
		http.Error(w, "model not loaded", http.StatusServiceUnavailable)
		return
	}

	kline, _, err := fetchLatestMarketData(symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	features, err := latestFeatureBuilder(symbol)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	label, confidence, err := mlModel.Predict(features)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := db.SavePredictions([]int{label}, []float64{confidence}, symbol); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"symbol":       symbol,
		"label":        label,
		"confidence":   confidence,
		"timestamp":    kline.Timestamp,
		"feature_size": len(features),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func loadLatestMarketData(symbol string) (market.KLine, market.Indicator, error) {
	klines, err := market.FetchHistoricalData(symbol, 60)
	if err != nil {
		return market.KLine{}, market.Indicator{}, err
	}
	if len(klines) == 0 {
		return market.KLine{}, market.Indicator{}, errors.New("no data")
	}
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}
	latest := klines[len(klines)-1]
	indicator := market.Indicator{
		MA5:       market.CalculateMA(closes, 5),
		MA20:      market.CalculateMA(closes, 20),
		RSI:       market.CalculateRSI(closes, 14),
		MACD:      func() float64 { diff, _, _ := market.CalculateMACD(closes); return diff }(),
		Timestamp: latest.Timestamp,
	}
	latest.Indicators = indicator
	return latest, indicator, nil
}

func buildLatestFeatures(symbol string) ([]float64, error) {
	klines, err := market.FetchHistoricalData(symbol, 80)
	if err != nil {
		return nil, err
	}
	featureSet, err := ml.ExtractFeatures(klines)
	if err != nil {
		return nil, err
	}
	if len(featureSet) == 0 {
		return nil, errors.New("not enough data for features")
	}
	latest := featureSet[len(featureSet)-1]
	if err := db.SaveFeatures(latest); err != nil {
		return nil, err
	}
	return ml.FeatureVector(latest), nil
}

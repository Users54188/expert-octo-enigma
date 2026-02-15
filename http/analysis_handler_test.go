package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloudquant/llm"
	"cloudquant/market"
)

type fakeAnalyzer struct {
	result *llm.AnalysisResult
	err    error
}

func (f *fakeAnalyzer) Analyze(ctx context.Context, kline market.KLine, indicator market.Indicator) (*llm.AnalysisResult, error) {
	return f.result, f.err
}

func TestHandleAnalysis(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux)
	SetAnalyzer(&fakeAnalyzer{result: &llm.AnalysisResult{Trend: "看涨", Risk: "中", Action: "买入", Reason: "测试"}})
	fetchLatestMarketData = func(symbol string) (market.KLine, market.Indicator, error) {
		return market.KLine{Symbol: symbol, Close: 10, Volume: 1000, Timestamp: time.Now()}, market.Indicator{MA5: 9, MA20: 8, RSI: 60, MACD: 0.2}, nil
	}
	defer func() {
		fetchLatestMarketData = loadLatestMarketData
		SetAnalyzer(nil)
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/analysis/sh600000", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["trend"] != "看涨" {
		t.Fatalf("unexpected trend: %v", payload["trend"])
	}
}

package http

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "cloudquant/market"
)

type fakeModel struct {
    label      int
    confidence float64
    err        error
}

func (f *fakeModel) Predict(features []float64) (int, float64, error) {
    return f.label, f.confidence, f.err
}

func TestHandlePredict(t *testing.T) {
    mux := http.NewServeMux()
    RegisterHandlers(mux)
    SetModelProvider(&fakeModel{label: 2, confidence: 0.75})
    fetchLatestMarketData = func(symbol string) (market.KLine, market.Indicator, error) {
        return market.KLine{Symbol: symbol, Close: 10, Volume: 1000, Timestamp: time.Now()}, market.Indicator{}, nil
    }
    latestFeatureBuilder = func(symbol string) ([]float64, error) {
        return []float64{0.1, 0.2}, nil
    }
    defer func() {
        fetchLatestMarketData = loadLatestMarketData
        latestFeatureBuilder = buildLatestFeatures
        SetModelProvider(nil)
    }()

    req := httptest.NewRequest(http.MethodGet, "/api/predict/sh600000", nil)
    w := httptest.NewRecorder()
    mux.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", w.Code)
    }

    var payload map[string]interface{}
    if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
        t.Fatalf("invalid json: %v", err)
    }
    if payload["label"].(float64) != 2 {
        t.Fatalf("unexpected label: %v", payload["label"])
    }
}

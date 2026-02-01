package ml

import (
	"testing"
	"time"

	"cloudquant/market"
)

func TestExtractFeatures(t *testing.T) {
	klines := make([]market.KLine, 0)
	start := time.Now().AddDate(0, 0, -80)
	for i := 0; i < 80; i++ {
		klines = append(klines, market.KLine{
			Symbol:    "sh600000",
			Close:     10 + float64(i)*0.1,
			Volume:    int64(1000 + i*10),
			Timestamp: start.AddDate(0, 0, i),
		})
	}

	features, err := ExtractFeatures(klines)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(features) == 0 {
		t.Fatal("expected features")
	}
	latest := features[len(features)-1]
	if latest.MA5 == 0 || latest.MA20 == 0 || latest.RSI == 0 {
		t.Fatalf("expected indicators to be computed: %+v", latest)
	}
	if latest.PriceChange == 0 {
		t.Fatalf("expected price change")
	}
	if latest.Symbol != "sh600000" {
		t.Fatalf("expected symbol to be set")
	}
}

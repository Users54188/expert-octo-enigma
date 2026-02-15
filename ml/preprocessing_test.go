package ml

import (
	"testing"
	"time"

	"cloudquant/market"
)

func TestDataPreprocessorNormalize(t *testing.T) {
	features := []MLFeatures{
		{
			Symbol:         "sh600000",
			MA5:            10,
			MA20:           9,
			MA60:           8,
			RSI:            55,
			MACD:           0.2,
			MACDSignal:     0.1,
			BB_Upper:       12,
			BB_Lower:       8,
			PriceChange:    0.01,
			VolumeChange:   0.05,
			MA5_MA20_Ratio: 1.1,
			RSI_Momentum:   0.5,
			Volatility:     0.2,
			TrendStrength:  0.1,
			Timestamp:      time.Now(),
		},
		{
			Symbol:         "sh600000",
			MA5:            11,
			MA20:           10,
			MA60:           9,
			RSI:            60,
			MACD:           0.3,
			MACDSignal:     0.2,
			BB_Upper:       13,
			BB_Lower:       9,
			PriceChange:    0.02,
			VolumeChange:   0.06,
			MA5_MA20_Ratio: 1.2,
			RSI_Momentum:   0.4,
			Volatility:     0.3,
			TrendStrength:  0.15,
			Timestamp:      time.Now(),
		},
	}

	preprocessor := &DataPreprocessor{}
	if err := preprocessor.ComputeStats(features); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vectors, err := preprocessor.Normalize(features)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vectors) != len(features) {
		t.Fatalf("expected %d vectors, got %d", len(features), len(vectors))
	}
	for _, vector := range vectors {
		if len(vector) != len(FeatureNames()) {
			t.Fatalf("unexpected vector length: %d", len(vector))
		}
		for _, value := range vector {
			if value < 0 || value > 1 {
				t.Fatalf("expected normalized value between 0 and 1, got %f", value)
			}
		}
	}

	stats := preprocessor.FeatureStats()
	if len(stats) == 0 {
		t.Fatal("expected feature stats")
	}
}

func TestExtractFeaturesFromKLines(t *testing.T) {
	klines := []market.KLine{{
		Symbol:    "sh600000",
		Close:     10,
		Volume:    1000,
		Timestamp: time.Now(),
	}}
	if _, err := ExtractFeatures(klines); err == nil {
		t.Fatal("expected error for insufficient klines")
	}
}

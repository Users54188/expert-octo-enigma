package ml

import (
	"testing"

	"cloudquant/market"
)

func TestGenerateLabels(t *testing.T) {
	klines := []market.KLine{
		{Close: 100},
		{Close: 97},
		{Close: 96},
		{Close: 95},
	}
	labels, err := GenerateLabels(klines, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if labels[0] != 0 {
		t.Fatalf("expected label 0, got %d", labels[0])
	}
}

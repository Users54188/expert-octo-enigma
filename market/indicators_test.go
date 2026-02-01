package market

import (
	"testing"
)

func TestCalculateMA(t *testing.T) {
	data := []float64{10, 20, 30, 40, 50}
	ma5 := CalculateMA(data, 5)
	if ma5 != 30 {
		t.Errorf("Expected MA5 to be 30, got %f", ma5)
	}

	ma2 := CalculateMA(data, 2)
	if ma2 != 45 {
		t.Errorf("Expected MA2 to be 45, got %f", ma2)
	}
}

func TestCalculateRSI(t *testing.T) {
	data := []float64{10, 12, 11, 13, 12, 14, 13, 15, 14, 16}
	rsi := CalculateRSI(data, 9)
	if rsi <= 0 || rsi >= 100 {
		t.Errorf("RSI should be between 0 and 100, got %f", rsi)
	}
}

func TestCalculateMACD(t *testing.T) {
	data := make([]float64, 30)
	for i := 0; i < 30; i++ {
		data[i] = float64(100 + i)
	}
	diff, dea, macd := CalculateMACD(data)
	if diff == 0 && dea == 0 && macd == 0 {
		t.Errorf("MACD should not be all zeros for 30 days of data")
	}
}

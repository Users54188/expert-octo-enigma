package ml

import (
	"errors"
	"math"
)

func CalculatePriceChange(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	prev := closes[len(closes)-2]
	if prev == 0 {
		return 0
	}
	return (closes[len(closes)-1] - prev) / prev
}

func CalculateVolumeChange(volumes []int64) float64 {
	if len(volumes) < 2 {
		return 0
	}
	prev := volumes[len(volumes)-2]
	if prev == 0 {
		return 0
	}
	return float64(volumes[len(volumes)-1]-prev) / float64(prev)
}

func CalculateVolatility(closes []float64, period int) float64 {
	if len(closes) < period || period <= 1 {
		return 0
	}
	start := len(closes) - period
	mean := 0.0
	for _, v := range closes[start:] {
		mean += v
	}
	mean /= float64(period)
	variance := 0.0
	for _, v := range closes[start:] {
		diff := v - mean
		variance += diff * diff
	}
	return math.Sqrt(variance / float64(period))
}

func CalculateTrendStrength(closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}
	gains := 0.0
	losses := 0.0
	for i := 1; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		if diff >= 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}
	if gains+losses == 0 {
		return 0
	}
	return (gains - losses) / (gains + losses)
}

func CalculateBollingerBands(closes []float64, period int, stdDev float64) (upper, middle, lower float64) {
	if len(closes) < period || period <= 1 {
		return 0, 0, 0
	}
	start := len(closes) - period
	segment := closes[start:]
	mean := 0.0
	for _, v := range segment {
		mean += v
	}
	mean /= float64(period)
	variance := 0.0
	for _, v := range segment {
		diff := v - mean
		variance += diff * diff
	}
	std := math.Sqrt(variance / float64(period))
	middle = mean
	upper = mean + stdDev*std
	lower = mean - stdDev*std
	return upper, middle, lower
}

func NormalizeFeature(value, min, max float64) float64 {
	if max == min {
		return 0
	}
	return (value - min) / (max - min)
}

func NormalizeVector(values []float64, mins []float64, maxs []float64) ([]float64, error) {
	if len(values) != len(mins) || len(values) != len(maxs) {
		return nil, errors.New("values/mins/maxs length mismatch")
	}
	result := make([]float64, len(values))
	for i := range values {
		result[i] = NormalizeFeature(values[i], mins[i], maxs[i])
	}
	return result, nil
}

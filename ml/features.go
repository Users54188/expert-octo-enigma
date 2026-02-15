package ml

import (
	"errors"
	"time"

	"cloudquant/market"
)

type MLFeatures struct {
	Symbol string

	MA5        float64
	MA20       float64
	MA60       float64
	RSI        float64
	MACD       float64
	MACDSignal float64
	BB_Upper   float64
	BB_Lower   float64

	PriceChange    float64
	VolumeChange   float64
	MA5_MA20_Ratio float64
	RSI_Momentum   float64

	Volatility    float64
	TrendStrength float64

	Label     int
	Timestamp time.Time
}

func ExtractFeatures(klines []market.KLine) ([]MLFeatures, error) {
	if len(klines) == 0 {
		return nil, errors.New("klines is empty")
	}

	maxPeriod := 60
	features := make([]MLFeatures, 0, len(klines))
	closes := make([]float64, len(klines))
	volumes := make([]int64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		volumes[i] = k.Volume
	}

	var prevRSI float64
	for i := range klines {
		if i+1 < maxPeriod {
			continue
		}
		window := closes[:i+1]
		ma5 := market.CalculateMA(window, 5)
		ma20 := market.CalculateMA(window, 20)
		ma60 := market.CalculateMA(window, 60)
		rsi := market.CalculateRSI(window, 14)
		macd, signal, _ := market.CalculateMACD(window)
		upper, _, lower := CalculateBollingerBands(window, 20, 2)
		priceChange := CalculatePriceChange(window)
		volumeChange := CalculateVolumeChange(volumes[:i+1])
		maRatio := 0.0
		if ma20 != 0 {
			maRatio = ma5 / ma20
		}
		rsiMomentum := rsi - prevRSI
		volatility := CalculateVolatility(window, 20)
		trendStrength := CalculateTrendStrength(window)

		features = append(features, MLFeatures{
			Symbol:         klines[i].Symbol,
			MA5:            ma5,
			MA20:           ma20,
			MA60:           ma60,
			RSI:            rsi,
			MACD:           macd,
			MACDSignal:     signal,
			BB_Upper:       upper,
			BB_Lower:       lower,
			PriceChange:    priceChange,
			VolumeChange:   volumeChange,
			MA5_MA20_Ratio: maRatio,
			RSI_Momentum:   rsiMomentum,
			Volatility:     volatility,
			TrendStrength:  trendStrength,
			Timestamp:      klines[i].Timestamp,
		})
		prevRSI = rsi
	}

	return features, nil
}

func NormalizeFeatures(features []MLFeatures) ([]MLFeatures, error) {
	if len(features) == 0 {
		return nil, errors.New("features is empty")
	}

	stats := computeFeatureStats(features)
	normalized := make([]MLFeatures, len(features))
	for i, f := range features {
		normalized[i] = f
		normalized[i].MA5 = NormalizeFeature(f.MA5, stats["MA5"][0], stats["MA5"][1])
		normalized[i].MA20 = NormalizeFeature(f.MA20, stats["MA20"][0], stats["MA20"][1])
		normalized[i].MA60 = NormalizeFeature(f.MA60, stats["MA60"][0], stats["MA60"][1])
		normalized[i].RSI = NormalizeFeature(f.RSI, stats["RSI"][0], stats["RSI"][1])
		normalized[i].MACD = NormalizeFeature(f.MACD, stats["MACD"][0], stats["MACD"][1])
		normalized[i].MACDSignal = NormalizeFeature(f.MACDSignal, stats["MACDSignal"][0], stats["MACDSignal"][1])
		normalized[i].BB_Upper = NormalizeFeature(f.BB_Upper, stats["BB_Upper"][0], stats["BB_Upper"][1])
		normalized[i].BB_Lower = NormalizeFeature(f.BB_Lower, stats["BB_Lower"][0], stats["BB_Lower"][1])
		normalized[i].PriceChange = NormalizeFeature(f.PriceChange, stats["PriceChange"][0], stats["PriceChange"][1])
		normalized[i].VolumeChange = NormalizeFeature(f.VolumeChange, stats["VolumeChange"][0], stats["VolumeChange"][1])
		normalized[i].MA5_MA20_Ratio = NormalizeFeature(f.MA5_MA20_Ratio, stats["MA5_MA20_Ratio"][0], stats["MA5_MA20_Ratio"][1])
		normalized[i].RSI_Momentum = NormalizeFeature(f.RSI_Momentum, stats["RSI_Momentum"][0], stats["RSI_Momentum"][1])
		normalized[i].Volatility = NormalizeFeature(f.Volatility, stats["Volatility"][0], stats["Volatility"][1])
		normalized[i].TrendStrength = NormalizeFeature(f.TrendStrength, stats["TrendStrength"][0], stats["TrendStrength"][1])
	}

	return normalized, nil
}

func FeatureVector(feature MLFeatures) []float64 {
	return []float64{
		feature.MA5,
		feature.MA20,
		feature.MA60,
		feature.RSI,
		feature.MACD,
		feature.MACDSignal,
		feature.BB_Upper,
		feature.BB_Lower,
		feature.PriceChange,
		feature.VolumeChange,
		feature.MA5_MA20_Ratio,
		feature.RSI_Momentum,
		feature.Volatility,
		feature.TrendStrength,
	}
}

func FeatureNames() []string {
	return []string{
		"MA5",
		"MA20",
		"MA60",
		"RSI",
		"MACD",
		"MACDSignal",
		"BB_Upper",
		"BB_Lower",
		"PriceChange",
		"VolumeChange",
		"MA5_MA20_Ratio",
		"RSI_Momentum",
		"Volatility",
		"TrendStrength",
	}
}

func computeFeatureStats(features []MLFeatures) map[string][2]float64 {
	stats := make(map[string][2]float64)
	for _, name := range FeatureNames() {
		stats[name] = [2]float64{0, 0}
	}

	for i, f := range features {
		values := map[string]float64{
			"MA5":            f.MA5,
			"MA20":           f.MA20,
			"MA60":           f.MA60,
			"RSI":            f.RSI,
			"MACD":           f.MACD,
			"MACDSignal":     f.MACDSignal,
			"BB_Upper":       f.BB_Upper,
			"BB_Lower":       f.BB_Lower,
			"PriceChange":    f.PriceChange,
			"VolumeChange":   f.VolumeChange,
			"MA5_MA20_Ratio": f.MA5_MA20_Ratio,
			"RSI_Momentum":   f.RSI_Momentum,
			"Volatility":     f.Volatility,
			"TrendStrength":  f.TrendStrength,
		}
		for key, value := range values {
			if i == 0 {
				stats[key] = [2]float64{value, value}
				continue
			}
			current := stats[key]
			if value < current[0] {
				current[0] = value
			}
			if value > current[1] {
				current[1] = value
			}
			stats[key] = current
		}
	}

	return stats
}

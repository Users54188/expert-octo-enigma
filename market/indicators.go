package market

// CalculateMA calculates the simple moving average
func CalculateMA(closes []float64, period int) float64 {
	if len(closes) < period || period <= 0 {
		return 0
	}
	sum := 0.0
	for i := len(closes) - period; i < len(closes); i++ {
		sum += closes[i]
	}
	return sum / float64(period)
}

// CalculateRSI calculates the Relative Strength Index
func CalculateRSI(closes []float64, period int) float64 {
	if len(closes) <= period || period <= 0 {
		return 0
	}

	gains := 0.0
	losses := 0.0

	for i := len(closes) - period; i < len(closes); i++ {
		diff := closes[i] - closes[i-1]
		if diff >= 0 {
			gains += diff
		} else {
			losses -= diff
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// CalculateMACD calculates the MACD indicator (DIFF, DEA, MACD)
func CalculateMACD(closes []float64) (float64, float64, float64) {
	if len(closes) < 26 {
		return 0, 0, 0
	}

	// Real MACD implementation
	diffSeries := make([]float64, len(closes))
	ema12Series := calculateEMA(closes, 12)
	ema26Series := calculateEMA(closes, 26)
	
	for i := 0; i < len(closes); i++ {
		diffSeries[i] = ema12Series[i] - ema26Series[i]
	}
	
	deaSeries := calculateEMA(diffSeries, 9)
	
	diff := diffSeries[len(diffSeries)-1]
	dea := deaSeries[len(deaSeries)-1]
	macd := 2 * (diff - dea)
	
	return diff, dea, macd
}

func calculateEMA(data []float64, period int) []float64 {
	ema := make([]float64, len(data))
	if len(data) == 0 {
		return ema
	}
	
	k := 2.0 / float64(period+1)
	ema[0] = data[0]
	for i := 1; i < len(data); i++ {
		ema[i] = data[i]*k + ema[i-1]*(1-k)
	}
	return ema
}

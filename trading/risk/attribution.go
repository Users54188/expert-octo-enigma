package risk

import (
	"math"
	"sync"
	"time"
)

type RiskAttribution struct {
	IndustryAttribution map[string]float64 `json:"industry_attribution"`
	StockAttribution    map[string]float64 `json:"stock_attribution"`
	FactorExposure      map[string]float64 `json:"factor_exposure"`
	Timestamp           time.Time          `json:"timestamp"`
}

type FactorReturn struct {
	Size       float64
	Value      float64
	Momentum   float64
	Volatility float64
}

type AttributionManager struct {
	mu            sync.RWMutex
	attributions  []RiskAttribution
	factorHistory []FactorReturn
}

func NewAttributionManager() *AttributionManager {
	return &AttributionManager{
		attributions:  make([]RiskAttribution, 0),
		factorHistory: make([]FactorReturn, 0),
	}
}

func (am *AttributionManager) CalculateAttribution(
	portfolioReturns map[string]float64,
	benchmarkReturns map[string]float64,
	industryMapping map[string]string,
) *RiskAttribution {
	am.mu.Lock()
	defer am.mu.Unlock()

	attribution := RiskAttribution{
		IndustryAttribution: make(map[string]float64),
		StockAttribution:    make(map[string]float64),
		FactorExposure:      make(map[string]float64),
		Timestamp:           time.Now(),
	}

	totalPortfolioReturn := 0.0
	totalBenchmarkReturn := 0.0

	for symbol, ret := range portfolioReturns {
		totalPortfolioReturn += ret
		attribution.StockAttribution[symbol] = ret

		industry := ""
		if ind, exists := industryMapping[symbol]; exists {
			industry = ind
		}

		if industry != "" {
			attribution.IndustryAttribution[industry] += ret
		}
	}

	for _, ret := range benchmarkReturns {
		totalBenchmarkReturn += ret
	}

	activeReturn := totalPortfolioReturn - totalBenchmarkReturn

	attribution.FactorExposure["active_return"] = activeReturn
	attribution.FactorExposure["portfolio_return"] = totalPortfolioReturn
	attribution.FactorExposure["benchmark_return"] = totalBenchmarkReturn

	am.attributions = append(am.attributions, attribution)

	return &attribution
}

func (am *AttributionManager) CalculateFactorExposure(
	portfolio map[string]float64,
	benchmarkReturns []float64,
	lookbackDays int,
) map[string]float64 {
	am.mu.Lock()
	defer am.mu.Unlock()

	exposure := make(map[string]float64)

	portfolioValues := make([]float64, 0, len(portfolio))
	for _, weight := range portfolio {
		portfolioValues = append(portfolioValues, weight)
	}

	if len(portfolioValues) == 0 {
		return exposure
	}

	sum := 0.0
	for _, v := range portfolioValues {
		sum += v
	}
	avgWeight := sum / float64(len(portfolioValues))
	exposure["concentration"] = avgWeight

	portfolioReturns := am.calculatePortfolioReturns(portfolioValues, lookbackDays)

	exposure["beta"] = am.calculateBeta(portfolioReturns, benchmarkReturns)

	exposure["volatility"] = am.calculateVolatility(portfolioReturns)

	exposure["momentum"] = am.calculateMomentum(portfolioReturns)

	sizeExposure := 0.0
	for _, weight := range portfolioValues {
		if weight > avgWeight {
			sizeExposure += weight
		}
	}
	exposure["size"] = sizeExposure / sum

	return exposure
}

func (am *AttributionManager) calculatePortfolioReturns(values []float64, days int) []float64 {
	if days <= 0 || days > len(values) {
		days = len(values)
	}

	returns := make([]float64, days-1)
	for i := 1; i < days; i++ {
		if values[i-1] > 0 {
			returns[i-1] = (values[i] - values[i-1]) / values[i-1]
		}
	}

	return returns
}

func (am *AttributionManager) calculateBeta(portfolioReturns, benchmarkReturns []float64) float64 {
	if len(portfolioReturns) == 0 || len(benchmarkReturns) == 0 {
		return 1.0
	}

	minLen := len(portfolioReturns)
	if len(benchmarkReturns) < minLen {
		minLen = len(benchmarkReturns)
	}

	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i := 0; i < minLen; i++ {
		sumX += benchmarkReturns[i]
		sumY += portfolioReturns[i]
		sumXY += benchmarkReturns[i] * portfolioReturns[i]
		sumX2 += benchmarkReturns[i] * benchmarkReturns[i]
	}

	n := float64(minLen)
	covariance := sumXY - (sumX*sumY)/n
	variance := sumX2 - (sumX*sumX)/n

	if variance == 0 {
		return 1.0
	}

	return covariance / variance
}

func (am *AttributionManager) calculateVolatility(returns []float64) float64 {
	if len(returns) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	variance := 0.0
	for _, r := range returns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(returns))

	return sqrt(variance)
}

func (am *AttributionManager) calculateMomentum(returns []float64) float64 {
	if len(returns) < 5 {
		return 0.0
	}

	recentSum := 0.0
	for i := len(returns) - 5; i < len(returns); i++ {
		recentSum += returns[i]
	}

	earlierSum := 0.0
	for i := 0; i < 5; i++ {
		earlierSum += returns[i]
	}

	return recentSum - earlierSum
}

func (am *AttributionManager) GetAttributionHistory(days int) []RiskAttribution {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if days <= 0 || days >= len(am.attributions) {
		result := make([]RiskAttribution, len(am.attributions))
		copy(result, am.attributions)
		return result
	}

	start := len(am.attributions) - days
	result := make([]RiskAttribution, days)
	copy(result, am.attributions[start:])
	return result
}

func (am *AttributionManager) GetLatestAttribution() *RiskAttribution {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if len(am.attributions) == 0 {
		return nil
	}

	latest := am.attributions[len(am.attributions)-1]
	return &latest
}

func (am *AttributionManager) GetTopIndustryContributors(limit int) []struct {
	Industry     string
	Contribution float64
} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if len(am.attributions) == 0 {
		return nil
	}

	latest := am.attributions[len(am.attributions)-1]
	contributors := make([]struct {
		Industry     string
		Contribution float64
	}, 0, len(latest.IndustryAttribution))

	for industry, contribution := range latest.IndustryAttribution {
		contributors = append(contributors, struct {
			Industry     string
			Contribution float64
		}{
			Industry:     industry,
			Contribution: contribution,
		})
	}

	for i := 0; i < len(contributors)-1; i++ {
		for j := i + 1; j < len(contributors); j++ {
			if math.Abs(contributors[j].Contribution) > math.Abs(contributors[i].Contribution) {
				contributors[i], contributors[j] = contributors[j], contributors[i]
			}
		}
	}

	if limit > 0 && limit < len(contributors) {
		contributors = contributors[:limit]
	}

	return contributors
}

func (am *AttributionManager) GetTopStockContributors(limit int) []struct {
	Symbol       string
	Contribution float64
} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if len(am.attributions) == 0 {
		return nil
	}

	latest := am.attributions[len(am.attributions)-1]
	contributors := make([]struct {
		Symbol       string
		Contribution float64
	}, 0, len(latest.StockAttribution))

	for symbol, contribution := range latest.StockAttribution {
		contributors = append(contributors, struct {
			Symbol       string
			Contribution float64
		}{
			Symbol:       symbol,
			Contribution: contribution,
		})
	}

	for i := 0; i < len(contributors)-1; i++ {
		for j := i + 1; j < len(contributors); j++ {
			if math.Abs(contributors[j].Contribution) > math.Abs(contributors[i].Contribution) {
				contributors[i], contributors[j] = contributors[j], contributors[i]
			}
		}
	}

	if limit > 0 && limit < len(contributors) {
		contributors = contributors[:limit]
	}

	return contributors
}

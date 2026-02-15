package risk

import (
	"math"
	"sort"
	"sync"
	"time"
)

type VaRResult struct {
	VaR         float64   `json:"var"`
	CVaR        float64   `json:"cvar"`
	Confidence  float64   `json:"confidence"`
	TimeHorizon int       `json:"time_horizon"`
	Method      string    `json:"method"`
	Timestamp   time.Time `json:"timestamp"`
}

type VaRCalculator struct {
	mu         sync.RWMutex
	historical []float64
	violations int
	totalCount int
	windowSize int
	confidence float64
}

func NewVaRCalculator(confidence float64, windowSize int) *VaRCalculator {
	return &VaRCalculator{
		historical: make([]float64, 0),
		windowSize: windowSize,
		confidence: confidence,
	}
}

func (vc *VaRCalculator) AddReturn(ret float64) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.historical = append(vc.historical, ret)

	if len(vc.historical) > vc.windowSize {
		vc.historical = vc.historical[1:]
	}
}

func (vc *VaRCalculator) CalculateVaR(method string, timeHorizonDays int) *VaRResult {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if len(vc.historical) < 10 {
		return nil
	}

	var varValue float64
	var cvarValue float64

	switch method {
	case "historical":
		varValue, cvarValue = vc.calculateHistoricalVaR()
	case "parametric":
		varValue, cvarValue = vc.calculateParametricVaR()
	case "montecarlo":
		varValue, cvarValue = vc.calculateMonteCarloVaR()
	default:
		varValue, cvarValue = vc.calculateHistoricalVaR()
	}

	if timeHorizonDays > 1 {
		varValue = varValue * math.Sqrt(float64(timeHorizonDays))
		cvarValue = cvarValue * math.Sqrt(float64(timeHorizonDays))
	}

	return &VaRResult{
		VaR:         varValue,
		CVaR:        cvarValue,
		Confidence:  vc.confidence,
		TimeHorizon: timeHorizonDays,
		Method:      method,
		Timestamp:   time.Now(),
	}
}

func (vc *VaRCalculator) calculateHistoricalVaR() (float64, float64) {
	sortedReturns := make([]float64, len(vc.historical))
	copy(sortedReturns, vc.historical)

	sort.Float64s(sortedReturns)

	index := int((1 - vc.confidence) * float64(len(sortedReturns)))
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	varValue := sortedReturns[index]

	sum := 0.0
	count := 0
	for i := 0; i <= index; i++ {
		sum += sortedReturns[i]
		count++
	}
	if count > 0 {
		cvarValue := sum / float64(count)
		return varValue, cvarValue
	}

	return varValue, 0.0
}

func (vc *VaRCalculator) calculateParametricVaR() (float64, float64) {
	sum := 0.0
	for _, r := range vc.historical {
		sum += r
	}
	mean := sum / float64(len(vc.historical))

	variance := 0.0
	for _, r := range vc.historical {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(vc.historical))
	stdDev := sqrt(variance)

	zScore := vc.getZScore(vc.confidence)
	varValue := mean - zScore*stdDev

	cvarValue := mean - (getPhi(zScore)/(1-vc.confidence))*stdDev

	return varValue, cvarValue
}

func (vc *VaRCalculator) calculateMonteCarloVaR() (float64, float64) {
	simulations := 10000
	simulatedReturns := make([]float64, simulations)

	sum := 0.0
	for _, r := range vc.historical {
		sum += r
	}
	mean := sum / float64(len(vc.historical))

	variance := 0.0
	for _, r := range vc.historical {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(vc.historical))
	stdDev := sqrt(variance)

	for i := 0; i < simulations; i++ {
		u1 := (float64(i) + 0.5) / float64(simulations)
		z := inverseNormalCDF(u1)
		simulatedReturns[i] = mean + stdDev*z
	}

	sort.Float64s(simulatedReturns)

	index := int((1 - vc.confidence) * float64(len(simulatedReturns)))
	if index >= len(simulatedReturns) {
		index = len(simulatedReturns) - 1
	}

	varValue := simulatedReturns[index]

	sumCVar := 0.0
	countCVar := 0
	for i := 0; i <= index; i++ {
		sumCVar += simulatedReturns[i]
		countCVar++
	}

	cvarValue := 0.0
	if countCVar > 0 {
		cvarValue = sumCVar / float64(countCVar)
	}

	return varValue, cvarValue
}

func (vc *VaRCalculator) getZScore(confidence float64) float64 {
	commonConfidences := map[float64]float64{
		0.90:  1.282,
		0.95:  1.645,
		0.99:  2.326,
		0.975: 1.96,
	}

	if z, exists := commonConfidences[confidence]; exists {
		return z
	}

	return 1.96
}

func getPhi(z float64) float64 {
	return math.Exp(-z*z/2) / math.Sqrt(2*math.Pi)
}

func inverseNormalCDF(p float64) float64 {
	if p <= 0 {
		p = 0.0001
	}
	if p >= 1 {
		p = 0.9999
	}

	a := []float64{-3.969683028665376e+01, 2.209460984245205e+02,
		-2.759285104469687e+02, 1.383577518672690e+02,
		-3.066479806614716e+01, 2.506628277459239e+00}

	b := []float64{-5.447609879822406e+01, 1.615858368580409e+02,
		-1.556989798598866e+02, 6.680131188771972e+01,
		-1.328068155288572e+01}

	c := []float64{-7.784894002430293e-03, -3.223964580411365e-01,
		-2.400758277161838e+00, -2.549732539343734e+00,
		4.374664141464968e+00, 2.938163982698783e+00}

	d := []float64{7.784695709041462e-03, 3.224671290700398e-01,
		2.445134137142996e+00, 3.754408661907416e+00}

	pLow := 0.02425
	pHigh := 1.0 - pLow
	q := 0.0
	r := 0.0

	if p < pLow {
		q := math.Sqrt(-2.0 * math.Log(p))
		return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) /
			((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1.0)
	} else if p <= pHigh {
		q = p - 0.5
		r = q * q
		return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q /
			(((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1.0)
	} else {
		q = math.Sqrt(-2.0 * math.Log(1.0-p))
		return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) /
			((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1.0)
	}
}

func (vc *VaRCalculator) CheckVaRBreach(portfolioValue, currentLoss float64) bool {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.totalCount++

	varResult := vc.CalculateVaR("historical", 1)
	if varResult == nil {
		return false
	}

	varThreshold := math.Abs(varResult.VaR * portfolioValue)

	if currentLoss > varThreshold {
		vc.violations++
		return true
	}

	return false
}

func (vc *VaRCalculator) GetBacktestResults() map[string]interface{} {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	results := make(map[string]interface{})
	results["violations"] = vc.violations
	results["total_observations"] = vc.totalCount
	results["violation_rate"] = float64(0)
	results["expected_violation_rate"] = 1.0 - vc.confidence

	if vc.totalCount > 0 {
		results["violation_rate"] = float64(vc.violations) / float64(vc.totalCount)
	}

	return results
}

func (vc *VaRCalculator) Clear() {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.historical = make([]float64, 0)
	vc.violations = 0
	vc.totalCount = 0
}

func CalculateVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0.0
	}

	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	sort.Float64s(sortedReturns)

	index := int((1 - confidence) * float64(len(sortedReturns)))
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	return sortedReturns[index]
}

func CalculateCVaR(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0.0
	}

	sortedReturns := make([]float64, len(returns))
	copy(sortedReturns, returns)
	sort.Float64s(sortedReturns)

	index := int((1 - confidence) * float64(len(sortedReturns)))
	if index >= len(sortedReturns) {
		index = len(sortedReturns) - 1
	}

	sum := 0.0
	count := 0
	for i := 0; i <= index; i++ {
		sum += sortedReturns[i]
		count++
	}

	if count > 0 {
		return sum / float64(count)
	}

	return 0.0
}

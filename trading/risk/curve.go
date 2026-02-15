package risk

import (
	"encoding/json"
	"sync"
	"time"
)

type EquityCurve struct {
	Date        time.Time `json:"date"`
	TotalEquity float64   `json:"total_equity"`
	Cash        float64   `json:"cash"`
	MarketValue float64   `json:"market_value"`
	DailyPnL    float64   `json:"daily_pnl"`
	CumPnL      float64   `json:"cum_pnl"`
	Drawdown    float64   `json:"drawdown"`
	MaxDrawdown float64   `json:"max_drawdown"`
}

type EquityCurveManager struct {
	mu             sync.RWMutex
	points         []EquityCurve
	initialCapital float64
	peakEquity     float64
	lastEquity     float64
	lastDate       time.Time
}

func NewEquityCurveManager(initialCapital float64) *EquityCurveManager {
	return &EquityCurveManager{
		initialCapital: initialCapital,
		peakEquity:     initialCapital,
		lastEquity:     initialCapital,
		lastDate:       time.Now(),
		points:         make([]EquityCurve, 0),
	}
}

func (ecm *EquityCurveManager) Update(equity, cash, marketValue float64, date time.Time) {
	ecm.mu.Lock()
	defer ecm.mu.Unlock()

	dailyPnL := 0.0
	cumPnL := equity - ecm.initialCapital

	if !date.IsZero() && !ecm.lastDate.IsZero() && date.Day() != ecm.lastDate.Day() {
		dailyPnL = equity - ecm.lastEquity
	}

	drawdown := 0.0
	if ecm.peakEquity > 0 {
		drawdown = (ecm.peakEquity - equity) / ecm.peakEquity
	}

	maxDrawdown := drawdown
	for _, point := range ecm.points {
		if point.MaxDrawdown > maxDrawdown {
			maxDrawdown = point.MaxDrawdown
		}
	}
	if maxDrawdown < drawdown {
		maxDrawdown = drawdown
	}

	point := EquityCurve{
		Date:        date,
		TotalEquity: equity,
		Cash:        cash,
		MarketValue: marketValue,
		DailyPnL:    dailyPnL,
		CumPnL:      cumPnL,
		Drawdown:    drawdown,
		MaxDrawdown: maxDrawdown,
	}

	ecm.points = append(ecm.points, point)

	if equity > ecm.peakEquity {
		ecm.peakEquity = equity
	}
	ecm.lastEquity = equity
	ecm.lastDate = date
}

func (ecm *EquityCurveManager) GetCurve(days int) []EquityCurve {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()

	if days <= 0 || days >= len(ecm.points) {
		result := make([]EquityCurve, len(ecm.points))
		copy(result, ecm.points)
		return result
	}

	start := len(ecm.points) - days
	result := make([]EquityCurve, days)
	copy(result, ecm.points[start:])
	return result
}

func (ecm *EquityCurveManager) GetLatest() *EquityCurve {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()

	if len(ecm.points) == 0 {
		return nil
	}

	latest := ecm.points[len(ecm.points)-1]
	return &latest
}

func (ecm *EquityCurveManager) GetDailyReturns(days int) []float64 {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()

	curve := ecm.points
	if days <= 0 || days >= len(curve) {
		days = len(curve)
	}

	start := len(curve) - days
	if start < 1 {
		return nil
	}

	returns := make([]float64, days-1)
	for i := start + 1; i < len(curve); i++ {
		if curve[i-1].TotalEquity > 0 {
			returns[i-start-1] = (curve[i].TotalEquity - curve[i-1].TotalEquity) / curve[i-1].TotalEquity
		}
	}

	return returns
}

func (ecm *EquityCurveManager) CalculateMetrics() map[string]float64 {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()

	if len(ecm.points) < 2 {
		return nil
	}

	metrics := make(map[string]float64)

	latest := ecm.points[len(ecm.points)-1]
	metrics["total_return"] = latest.CumPnL / ecm.initialCapital
	metrics["max_drawdown"] = latest.MaxDrawdown

	returns := make([]float64, len(ecm.points)-1)
	for i := 1; i < len(ecm.points); i++ {
		if ecm.points[i-1].TotalEquity > 0 {
			returns[i-1] = (ecm.points[i].TotalEquity - ecm.points[i-1].TotalEquity) / ecm.points[i-1].TotalEquity
		}
	}

	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	avgReturn := sum / float64(len(returns))
	metrics["avg_daily_return"] = avgReturn

	variance := 0.0
	for _, r := range returns {
		diff := r - avgReturn
		variance += diff * diff
	}
	variance /= float64(len(returns))
	stdDev := sqrt(variance)
	metrics["daily_volatility"] = stdDev

	if stdDev > 0 {
		metrics["sharpe_ratio"] = avgReturn / stdDev * sqrt(252)
	}

	negativeReturns := make([]float64, 0)
	for _, r := range returns {
		if r < 0 {
			negativeReturns = append(negativeReturns, r)
		}
	}

	if len(negativeReturns) > 0 {
		downsideVariance := 0.0
		for _, r := range negativeReturns {
			downsideVariance += r * r
		}
		downsideVariance /= float64(len(negativeReturns))
		downsideDev := sqrt(downsideVariance)

		if downsideDev > 0 {
			metrics["sortino_ratio"] = avgReturn / downsideDev * sqrt(252)
		}
	}

	winDays := 0
	for _, r := range returns {
		if r > 0 {
			winDays++
		}
	}
	metrics["win_rate"] = float64(winDays) / float64(len(returns))

	profit := 0.0
	loss := 0.0
	for _, r := range returns {
		if r > 0 {
			profit += r
		} else {
			loss -= r
		}
	}
	if loss > 0 {
		metrics["profit_factor"] = profit / loss
	}

	if latest.MaxDrawdown > 0 {
		metrics["calmar_ratio"] = metrics["total_return"] / latest.MaxDrawdown
	}

	return metrics
}

func (ecm *EquityCurveManager) SaveToFile(filePath string) error {
	ecm.mu.RLock()
	defer ecm.mu.RUnlock()

	data, err := json.MarshalIndent(ecm.points, "", "  ")
	if err != nil {
		return err
	}

	return writeFile(filePath, data)
}

func (ecm *EquityCurveManager) LoadFromFile(filePath string) error {
	ecm.mu.Lock()
	defer ecm.mu.Unlock()

	data, err := readFile(filePath)
	if err != nil {
		return err
	}

	var points []EquityCurve
	if err := json.Unmarshal(data, &points); err != nil {
		return err
	}

	ecm.points = points

	if len(points) > 0 {
		latest := points[len(points)-1]
		ecm.lastEquity = latest.TotalEquity
		ecm.lastDate = latest.Date

		for _, point := range points {
			if point.TotalEquity > ecm.peakEquity {
				ecm.peakEquity = point.TotalEquity
			}
		}
	}

	return nil
}

func (ecm *EquityCurveManager) Clear() {
	ecm.mu.Lock()
	defer ecm.mu.Unlock()

	ecm.points = make([]EquityCurve, 0)
	ecm.peakEquity = ecm.initialCapital
	ecm.lastEquity = ecm.initialCapital
}

func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}

	z := 1.0
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func writeFile(filePath string, data []byte) error {
	return nil
}

func readFile(filePath string) ([]byte, error) {
	return []byte("{}"), nil
}

package monitoring

import (
	"math"
	"sync"
	"time"
)

type PerformanceTracker struct {
	mu             sync.RWMutex
	trades         []TradeRecord
	equityHistory  []EquityRecord
	dailyReturns   []float64
	initialCapital float64
	currentEquity  float64
	winCount       int
	lossCount      int
	totalProfit    float64
	totalLoss      float64
	maxDrawdown    float64
	peakEquity     float64
	troughEquity   float64
	tradingDays    int
	lastTradeDate  time.Time
}

type TradeRecord struct {
	ID        string        `json:"id"`
	Symbol    string        `json:"symbol"`
	Side      string        `json:"side"`
	Quantity  int64         `json:"quantity"`
	Price     float64       `json:"price"`
	Amount    float64       `json:"amount"`
	PnL       float64       `json:"pnl"`
	PnLPct    float64       `json:"pnl_pct"`
	Fee       float64       `json:"fee"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

type EquityRecord struct {
	Date        time.Time `json:"date"`
	Equity      float64   `json:"equity"`
	Cash        float64   `json:"cash"`
	MarketValue float64   `json:"market_value"`
}

func NewPerformanceTracker(initialCapital float64) *PerformanceTracker {
	return &PerformanceTracker{
		initialCapital: initialCapital,
		currentEquity:  initialCapital,
		peakEquity:     initialCapital,
		trades:         make([]TradeRecord, 0),
		equityHistory:  make([]EquityRecord, 0),
		dailyReturns:   make([]float64, 0),
	}
}

func (pt *PerformanceTracker) RecordTrade(trade TradeRecord) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.trades = append(pt.trades, trade)

	if trade.PnL > 0 {
		pt.winCount++
		pt.totalProfit += trade.PnL
	} else if trade.PnL < 0 {
		pt.lossCount++
		pt.totalLoss -= trade.PnL
	}

	if trade.Timestamp.Day() != pt.lastTradeDate.Day() {
		pt.tradingDays++
		pt.lastTradeDate = trade.Timestamp
	}

	pt.currentEquity += trade.PnL - trade.Fee
	pt.updateDrawdown()
}

func (pt *PerformanceTracker) UpdateEquity(equity, cash, marketValue float64, date time.Time) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.currentEquity = equity

	if equity > pt.peakEquity {
		pt.peakEquity = equity
		pt.troughEquity = equity
	}

	pt.equityHistory = append(pt.equityHistory, EquityRecord{
		Date:        date,
		Equity:      equity,
		Cash:        cash,
		MarketValue: marketValue,
	})

	pt.updateDrawdown()
}

func (pt *PerformanceTracker) updateDrawdown() {
	if pt.peakEquity > 0 {
		drawdown := (pt.peakEquity - pt.currentEquity) / pt.peakEquity
		if drawdown > pt.maxDrawdown {
			pt.maxDrawdown = drawdown
		}
	}
}

func (pt *PerformanceTracker) CalculateMetrics() *PerformanceMetrics {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	metrics := &PerformanceMetrics{}

	totalReturn := (pt.currentEquity - pt.initialCapital) / pt.initialCapital
	metrics.TotalReturn = totalReturn

	if pt.tradingDays > 0 {
		years := float64(pt.tradingDays) / 252.0
		if years > 0 {
			metrics.AnnualizedReturn = math.Pow(1+totalReturn, 1/years) - 1
		}
	}

	dailyReturns := pt.calculateDailyReturns()
	if len(dailyReturns) > 0 {
		sum := 0.0
		for _, r := range dailyReturns {
			sum += r
		}
		meanReturn := sum / float64(len(dailyReturns))

		variance := 0.0
		for _, r := range dailyReturns {
			diff := r - meanReturn
			variance += diff * diff
		}
		variance /= float64(len(dailyReturns))
		stdDev := sqrt(variance)

		metrics.Volatility = stdDev

		if stdDev > 0 {
			metrics.SharpeRatio = meanReturn / stdDev * math.Sqrt(252)
		}

		negativeReturns := make([]float64, 0)
		for _, r := range dailyReturns {
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
				metrics.SortinoRatio = meanReturn / downsideDev * math.Sqrt(252)
			}
		}
	}

	metrics.MaxDrawdown = pt.maxDrawdown

	totalTrades := pt.winCount + pt.lossCount
	metrics.TotalTrades = totalTrades
	metrics.WinningTrades = pt.winCount
	metrics.LosingTrades = pt.lossCount

	if totalTrades > 0 {
		metrics.WinRate = float64(pt.winCount) / float64(totalTrades)
	}

	if pt.totalLoss > 0 {
		metrics.ProfitFactor = pt.totalProfit / pt.totalLoss
	}

	if pt.maxDrawdown > 0 {
		metrics.CalmarRatio = totalReturn / pt.maxDrawdown
	}

	return metrics
}

func (pt *PerformanceTracker) calculateDailyReturns() []float64 {
	returns := make([]float64, 0)

	for i := 1; i < len(pt.equityHistory); i++ {
		if pt.equityHistory[i-1].Equity > 0 {
			ret := (pt.equityHistory[i].Equity - pt.equityHistory[i-1].Equity) / pt.equityHistory[i-1].Equity
			returns = append(returns, ret)
		}
	}

	return returns
}

func (pt *PerformanceTracker) GetEquityHistory(days int) []EquityRecord {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if days <= 0 || days >= len(pt.equityHistory) {
		result := make([]EquityRecord, len(pt.equityHistory))
		copy(result, pt.equityHistory)
		return result
	}

	start := len(pt.equityHistory) - days
	result := make([]EquityRecord, days)
	copy(result, pt.equityHistory[start:])
	return result
}

func (pt *PerformanceTracker) GetTrades(limit int) []TradeRecord {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if limit <= 0 || limit >= len(pt.trades) {
		result := make([]TradeRecord, len(pt.trades))
		copy(result, pt.trades)
		return result
	}

	start := len(pt.trades) - limit
	result := make([]TradeRecord, limit)
	copy(result, pt.trades[start:])
	return result
}

func (pt *PerformanceTracker) GetLatestEquity() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	return pt.currentEquity
}

func (pt *PerformanceTracker) GetCurrentDrawdown() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.peakEquity > 0 {
		return (pt.peakEquity - pt.currentEquity) / pt.peakEquity
	}
	return 0.0
}

func (pt *PerformanceTracker) GetWinningTrades() []TradeRecord {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	winning := make([]TradeRecord, 0)
	for _, trade := range pt.trades {
		if trade.PnL > 0 {
			winning = append(winning, trade)
		}
	}
	return winning
}

func (pt *PerformanceTracker) GetLosingTrades() []TradeRecord {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	losing := make([]TradeRecord, 0)
	for _, trade := range pt.trades {
		if trade.PnL < 0 {
			losing = append(losing, trade)
		}
	}
	return losing
}

func (pt *PerformanceTracker) GetAverageWin() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.winCount == 0 {
		return 0.0
	}
	return pt.totalProfit / float64(pt.winCount)
}

func (pt *PerformanceTracker) GetAverageLoss() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.lossCount == 0 {
		return 0.0
	}
	return pt.totalLoss / float64(pt.lossCount)
}

func (pt *PerformanceTracker) GetProfitFactor() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	if pt.totalLoss == 0 {
		return 0.0
	}
	return pt.totalProfit / pt.totalLoss
}

func (pt *PerformanceTracker) GetExpectancy() float64 {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	totalTrades := pt.winCount + pt.lossCount
	if totalTrades == 0 {
		return 0.0
	}

	return (pt.totalProfit - pt.totalLoss) / float64(totalTrades)
}

func (pt *PerformanceTracker) Clear() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.trades = make([]TradeRecord, 0)
	pt.equityHistory = make([]EquityRecord, 0)
	pt.dailyReturns = make([]float64, 0)
	pt.winCount = 0
	pt.lossCount = 0
	pt.totalProfit = 0.0
	pt.totalLoss = 0.0
	pt.maxDrawdown = 0.0
	pt.peakEquity = pt.initialCapital
	pt.troughEquity = pt.initialCapital
	pt.tradingDays = 0
	pt.currentEquity = pt.initialCapital
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

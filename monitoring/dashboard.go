package monitoring

import (
	"encoding/json"
	"sync"
	"time"
)

type DashboardData struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type PerformanceMetrics struct {
	TotalReturn      float64 `json:"total_return"`
	AnnualizedReturn float64 `json:"annualized_return"`
	SharpeRatio      float64 `json:"sharpe_ratio"`
	SortinoRatio     float64 `json:"sortino_ratio"`
	CalmarRatio      float64 `json:"calmar_ratio"`
	MaxDrawdown      float64 `json:"max_drawdown"`
	WinRate          float64 `json:"win_rate"`
	ProfitFactor     float64 `json:"profit_factor"`
	Volatility       float64 `json:"volatility"`
	TotalTrades      int     `json:"total_trades"`
	WinningTrades    int     `json:"winning_trades"`
	LosingTrades     int     `json:"losing_trades"`
}

type DashboardManager struct {
	mu             sync.RWMutex
	subscribers    map[string]chan DashboardData
	metrics        *PerformanceMetrics
	equityCurve    []float64
	positions      map[string]PositionData
	riskMetrics    map[string]interface{}
	enabled        bool
	updateInterval time.Duration
	lastUpdate     time.Time
}

type PositionData struct {
	Symbol   string  `json:"symbol"`
	Name     string  `json:"name"`
	Quantity int64   `json:"quantity"`
	Price    float64 `json:"price"`
	Value    float64 `json:"value"`
	PnL      float64 `json:"pnl"`
	PnLPct   float64 `json:"pnl_pct"`
	Industry string  `json:"industry"`
}

func NewDashboardManager() *DashboardManager {
	return &DashboardManager{
		subscribers:    make(map[string]chan DashboardData),
		metrics:        &PerformanceMetrics{},
		equityCurve:    make([]float64, 0),
		positions:      make(map[string]PositionData),
		riskMetrics:    make(map[string]interface{}),
		enabled:        true,
		updateInterval: 5 * time.Second,
		lastUpdate:     time.Now(),
	}
}

func (dm *DashboardManager) Subscribe(id string) chan DashboardData {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if _, exists := dm.subscribers[id]; exists {
		return dm.subscribers[id]
	}

	ch := make(chan DashboardData, 100)
	dm.subscribers[id] = ch
	return ch
}

func (dm *DashboardManager) Unsubscribe(id string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if ch, exists := dm.subscribers[id]; exists {
		close(ch)
		delete(dm.subscribers, id)
	}
}

func (dm *DashboardManager) Broadcast(dataType string, data interface{}) {
	dm.mu.RLock()
	if !dm.enabled {
		dm.mu.RUnlock()
		return
	}
	dm.mu.RUnlock()

	dashboardData := DashboardData{
		Type:      dataType,
		Data:      data,
		Timestamp: time.Now(),
	}

	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, ch := range dm.subscribers {
		select {
		case ch <- dashboardData:
		default:
		}
	}
}

func (dm *DashboardManager) UpdateMetrics(metrics *PerformanceMetrics) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.metrics = metrics
	dm.lastUpdate = time.Now()

	dm.Broadcast("metrics", dm.metrics)
}

func (dm *DashboardManager) GetMetrics() *PerformanceMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.metrics
}

func (dm *DashboardManager) UpdateEquity(equity float64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.equityCurve = append(dm.equityCurve, equity)

	if len(dm.equityCurve) > 1000 {
		dm.equityCurve = dm.equityCurve[1:]
	}

	dm.Broadcast("equity", map[string]interface{}{
		"equity":       equity,
		"equity_curve": dm.equityCurve,
		"timestamp":    time.Now(),
	})
}

func (dm *DashboardManager) GetEquityCurve(days int) []float64 {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if days <= 0 || days >= len(dm.equityCurve) {
		result := make([]float64, len(dm.equityCurve))
		copy(result, dm.equityCurve)
		return result
	}

	start := len(dm.equityCurve) - days
	result := make([]float64, days)
	copy(result, dm.equityCurve[start:])
	return result
}

func (dm *DashboardManager) UpdatePositions(positions []PositionData) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	for _, pos := range positions {
		dm.positions[pos.Symbol] = pos
	}

	positionList := make([]PositionData, 0, len(dm.positions))
	for _, pos := range dm.positions {
		positionList = append(positionList, pos)
	}

	dm.Broadcast("position", positionList)
}

func (dm *DashboardManager) GetPositions() []PositionData {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	positions := make([]PositionData, 0, len(dm.positions))
	for _, pos := range dm.positions {
		positions = append(positions, pos)
	}
	return positions
}

func (dm *DashboardManager) UpdateRiskMetrics(metrics map[string]interface{}) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.riskMetrics = metrics
	dm.lastUpdate = time.Now()

	dm.Broadcast("risk", dm.riskMetrics)
}

func (dm *DashboardManager) GetRiskMetrics() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	riskMetrics := make(map[string]interface{})
	for k, v := range dm.riskMetrics {
		riskMetrics[k] = v
	}
	return riskMetrics
}

func (dm *DashboardManager) BroadcastSignal(signal map[string]interface{}) {
	dm.Broadcast("signal", signal)
}

func (dm *DashboardManager) BroadcastAlert(alert map[string]interface{}) {
	dm.Broadcast("alert", alert)
}

func (dm *DashboardManager) BroadcastTrade(trade map[string]interface{}) {
	dm.Broadcast("trade", trade)
}

func (dm *DashboardManager) GetSnapshot() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	snapshot := make(map[string]interface{})
	snapshot["timestamp"] = time.Now()
	snapshot["metrics"] = dm.metrics
	snapshot["equity"] = dm.equityCurve[len(dm.equityCurve)-1]
	snapshot["positions"] = dm.GetPositions()
	snapshot["risk_metrics"] = dm.GetRiskMetrics()
	snapshot["subscriber_count"] = len(dm.subscribers)

	return snapshot
}

func (dm *DashboardManager) MarshalJSON() ([]byte, error) {
	return json.Marshal(dm.GetSnapshot())
}

func (dm *DashboardManager) SetEnabled(enabled bool) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.enabled = enabled
}

func (dm *DashboardManager) IsEnabled() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return dm.enabled
}

func (dm *DashboardManager) GetSubscriberCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	return len(dm.subscribers)
}

func (dm *DashboardManager) ClearPositions() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.positions = make(map[string]PositionData)
}

func (dm *DashboardManager) ClearEquityCurve() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.equityCurve = make([]float64, 0)
}

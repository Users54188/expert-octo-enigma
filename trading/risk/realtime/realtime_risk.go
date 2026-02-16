package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading"
)

// RiskLevel 风险级别
type RiskLevel int

const (
	RiskLevelLow RiskLevel = iota
	RiskLevelMedium
	RiskLevelHigh
	RiskLevelCritical
)

func (r RiskLevel) String() string {
	return []string{"low", "medium", "high", "critical"}[r]
}

// RiskEvent 风险事件
type RiskEvent struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Level     RiskLevel         `json:"level"`
	Symbol    string            `json:"symbol"`
	Message   string            `json:"message"`
	Value     float64           `json:"value"`
	Threshold float64           `json:"threshold"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// RiskLimit 风险限额
type RiskLimit struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"` // position, portfolio, drawdown, volatility
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
	CurrentValue      float64 `json:"current_value"`
}

// RealtimeRiskMonitor 实时风控监控器
type RealtimeRiskMonitor struct {
	riskManager     *trading.RiskManager
	positionManager *trading.PositionManager
	alertCallback   func(RiskEvent)

	riskLimits     map[string]*RiskLimit
	riskEvents     []RiskEvent
	riskEventsLock sync.RWMutex

	checkInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup

	exposureCache map[string]float64
	exposureLock  sync.RWMutex
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	CheckInterval      time.Duration
	MaxEventHistory    int
	EnableAutoStopLoss bool
}

// NewRealtimeRiskMonitor 创建实时风控监控器
func NewRealtimeRiskMonitor(riskManager *trading.RiskManager, positionManager *trading.PositionManager, config MonitorConfig) *RealtimeRiskMonitor {
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Second
	}

	monitor := &RealtimeRiskMonitor{
		riskManager:     riskManager,
		positionManager: positionManager,
		checkInterval:   config.CheckInterval,
		stopChan:        make(chan struct{}),
		riskLimits:      make(map[string]*RiskLimit),
		riskEvents:      make([]RiskEvent, 0, config.MaxEventHistory),
		exposureCache:   make(map[string]float64),
	}

	// 初始化默认风险限额
	monitor.initDefaultLimits()

	return monitor
}

// initDefaultLimits 初始化默认风险限额
func (m *RealtimeRiskMonitor) initDefaultLimits() {
	defaultLimits := []*RiskLimit{
		{
			Name:              "max_single_position",
			Type:              "position",
			WarningThreshold:  0.25,
			CriticalThreshold: 0.30,
		},
		{
			Name:              "max_drawdown",
			Type:              "portfolio",
			WarningThreshold:  0.15,
			CriticalThreshold: 0.20,
		},
		{
			Name:              "daily_loss_limit",
			Type:              "portfolio",
			WarningThreshold:  0.08,
			CriticalThreshold: 0.10,
		},
		{
			Name:              "volatility_limit",
			Type:              "volatility",
			WarningThreshold:  0.25,
			CriticalThreshold: 0.30,
		},
	}

	for _, limit := range defaultLimits {
		m.riskLimits[limit.Name] = limit
	}
}

// SetAlertCallback 设置告警回调
func (m *RealtimeRiskMonitor) SetAlertCallback(callback func(RiskEvent)) {
	m.alertCallback = callback
}

// Start 启动监控
func (m *RealtimeRiskMonitor) Start() error {
	log.Println("Starting real-time risk monitor...")

	m.wg.Add(1)
	go m.runMonitoringLoop()

	return nil
}

// Stop 停止监控
func (m *RealtimeRiskMonitor) Stop() {
	log.Println("Stopping real-time risk monitor...")
	close(m.stopChan)
	m.wg.Wait()
	log.Println("Real-time risk monitor stopped")
}

// runMonitoringLoop 运行监控循环
func (m *RealtimeRiskMonitor) runMonitoringLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := m.checkRisks(ctx); err != nil {
				log.Printf("Risk check failed: %v", err)
			}
			cancel()
		}
	}
}

// checkRisks 检查风险
func (m *RealtimeRiskMonitor) checkRisks(ctx context.Context) error {
	// 检查持仓风险
	if err := m.checkPositionRisk(ctx); err != nil {
		return err
	}

	// 检查组合风险
	if err := m.checkPortfolioRisk(ctx); err != nil {
		return err
	}

	// 检查波动率风险
	if err := m.checkVolatilityRisk(ctx); err != nil {
		return err
	}

	return nil
}

// checkPositionRisk 检查持仓风险
func (m *RealtimeRiskMonitor) checkPositionRisk(ctx context.Context) error {
	_ = ctx
	if m.positionManager == nil {
		return nil
	}

	positions := m.positionManager.GetAllPositions()

	totalValue := 0.0
	for _, pos := range positions {
		totalValue += pos.MarketValue
	}

	// 检查单个持仓占比
	for _, pos := range positions {
		if totalValue > 0 {
			exposure := pos.MarketValue / totalValue
			m.exposureLock.Lock()
			m.exposureCache[pos.Symbol] = exposure
			m.exposureLock.Unlock()

			if limit, ok := m.riskLimits["max_single_position"]; ok {
				limit.CurrentValue = exposure

				if exposure >= limit.CriticalThreshold {
					event := RiskEvent{
						ID:        generateEventID(),
						Type:      "position_risk",
						Level:     RiskLevelCritical,
						Symbol:    pos.Symbol,
						Message:   fmt.Sprintf("Single position exposure %.2f%% exceeds critical threshold %.2f%%", exposure*100, limit.CriticalThreshold*100),
						Value:     exposure,
						Threshold: limit.CriticalThreshold,
						Timestamp: time.Now(),
						Metadata:  map[string]string{"position_value": fmt.Sprintf("%.2f", pos.MarketValue)},
					}
					m.triggerAlert(event)
				} else if exposure >= limit.WarningThreshold {
					event := RiskEvent{
						ID:        generateEventID(),
						Type:      "position_risk",
						Level:     RiskLevelHigh,
						Symbol:    pos.Symbol,
						Message:   fmt.Sprintf("Single position exposure %.2f%% exceeds warning threshold %.2f%%", exposure*100, limit.WarningThreshold*100),
						Value:     exposure,
						Threshold: limit.WarningThreshold,
						Timestamp: time.Now(),
					}
					m.triggerAlert(event)
				}
			}
		}
	}

	return nil
}

// checkPortfolioRisk 检查组合风险
func (m *RealtimeRiskMonitor) checkPortfolioRisk(ctx context.Context) error {
	_ = ctx
	if m.riskManager == nil {
		return nil
	}

	// 检查回撤
	portfolio := m.riskManager.GetPortfolioSummary()

	if limit, ok := m.riskLimits["max_drawdown"]; ok {
		drawdown := portfolio.Drawdown
		limit.CurrentValue = drawdown

		if drawdown >= limit.CriticalThreshold {
			event := RiskEvent{
				ID:        generateEventID(),
				Type:      "drawdown_risk",
				Level:     RiskLevelCritical,
				Message:   fmt.Sprintf("Portfolio drawdown %.2f%% exceeds critical threshold %.2f%%", drawdown*100, limit.CriticalThreshold*100),
				Value:     drawdown,
				Threshold: limit.CriticalThreshold,
				Timestamp: time.Now(),
			}
			m.triggerAlert(event)
		} else if drawdown >= limit.WarningThreshold {
			event := RiskEvent{
				ID:        generateEventID(),
				Type:      "drawdown_risk",
				Level:     RiskLevelHigh,
				Message:   fmt.Sprintf("Portfolio drawdown %.2f%% exceeds warning threshold %.2f%%", drawdown*100, limit.WarningThreshold*100),
				Value:     drawdown,
				Threshold: limit.WarningThreshold,
				Timestamp: time.Now(),
			}
			m.triggerAlert(event)
		}
	}

	// 检查当日亏损
	dailyPnL := portfolio.DailyPnL
	if dailyPnL < 0 {
		dailyLoss := -dailyPnL / portfolio.TotalValue
		if limit, ok := m.riskLimits["daily_loss_limit"]; ok {
			limit.CurrentValue = dailyLoss

			if dailyLoss >= limit.CriticalThreshold {
				event := RiskEvent{
					ID:        generateEventID(),
					Type:      "daily_loss_risk",
					Level:     RiskLevelCritical,
					Message:   fmt.Sprintf("Daily loss %.2f%% exceeds critical threshold %.2f%%", dailyLoss*100, limit.CriticalThreshold*100),
					Value:     dailyLoss,
					Threshold: limit.CriticalThreshold,
					Timestamp: time.Now(),
				}
				m.triggerAlert(event)
			} else if dailyLoss >= limit.WarningThreshold {
				event := RiskEvent{
					ID:        generateEventID(),
					Type:      "daily_loss_risk",
					Level:     RiskLevelHigh,
					Message:   fmt.Sprintf("Daily loss %.2f%% exceeds warning threshold %.2f%%", dailyLoss*100, limit.WarningThreshold*100),
					Value:     dailyLoss,
					Threshold: limit.WarningThreshold,
					Timestamp: time.Now(),
				}
				m.triggerAlert(event)
			}
		}
	}

	return nil
}

// checkVolatilityRisk 检查波动率风险
func (m *RealtimeRiskMonitor) checkVolatilityRisk(ctx context.Context) error {
	_ = ctx
	if m.positionManager == nil {
		return nil
	}

	positions := m.positionManager.GetAllPositions()

	// 简化版波动率计算（实际应使用历史数据计算）
	for _, pos := range positions {
		// 这里应该计算实际的波动率
		// 简化版：使用成本价与现价的变化作为代理
		dailyChange := 0.0
		if pos.CostPrice > 0 {
			dailyChange = (pos.CurrentPrice - pos.CostPrice) / pos.CostPrice
			if dailyChange < 0 {
				dailyChange = -dailyChange
			}
		}

		if limit, ok := m.riskLimits["volatility_limit"]; ok {
			if dailyChange >= limit.CriticalThreshold {
				event := RiskEvent{
					ID:        generateEventID(),
					Type:      "volatility_risk",
					Level:     RiskLevelCritical,
					Symbol:    pos.Symbol,
					Message:   fmt.Sprintf("High volatility detected for %s: %.2f%%", pos.Symbol, dailyChange*100),
					Value:     dailyChange,
					Threshold: limit.CriticalThreshold,
					Timestamp: time.Now(),
				}
				m.triggerAlert(event)
			} else if dailyChange >= limit.WarningThreshold {
				event := RiskEvent{
					ID:        generateEventID(),
					Type:      "volatility_risk",
					Level:     RiskLevelHigh,
					Symbol:    pos.Symbol,
					Message:   fmt.Sprintf("Elevated volatility for %s: %.2f%%", pos.Symbol, dailyChange*100),
					Value:     dailyChange,
					Threshold: limit.WarningThreshold,
					Timestamp: time.Now(),
				}
				m.triggerAlert(event)
			}
		}
	}

	return nil
}

// triggerAlert 触发告警
func (m *RealtimeRiskMonitor) triggerAlert(event RiskEvent) {
	m.riskEventsLock.Lock()
	defer m.riskEventsLock.Unlock()

	m.riskEvents = append(m.riskEvents, event)

	// 限制事件历史大小
	if len(m.riskEvents) > 1000 {
		m.riskEvents = m.riskEvents[100:]
	}

	// 调用告警回调
	if m.alertCallback != nil {
		m.alertCallback(event)
	}
}

// GetRiskLimits 获取所有风险限额
func (m *RealtimeRiskMonitor) GetRiskLimits() map[string]*RiskLimit {
	m.riskEventsLock.RLock()
	defer m.riskEventsLock.RUnlock()

	result := make(map[string]*RiskLimit)
	for k, v := range m.riskLimits {
		limit := *v
		result[k] = &limit
	}
	return result
}

// GetRiskEvents 获取风险事件
func (m *RealtimeRiskMonitor) GetRiskEvents(limit int) []RiskEvent {
	m.riskEventsLock.RLock()
	defer m.riskEventsLock.RUnlock()

	if limit <= 0 || limit > len(m.riskEvents) {
		limit = len(m.riskEvents)
	}

	events := make([]RiskEvent, limit)
	copy(events, m.riskEvents[len(m.riskEvents)-limit:])
	return events
}

// GetExposure 获取持仓敞口
func (m *RealtimeRiskMonitor) GetExposure(symbol string) (float64, bool) {
	m.exposureLock.RLock()
	defer m.exposureLock.RUnlock()

	value, ok := m.exposureCache[symbol]
	return value, ok
}

// UpdateRiskLimit 更新风险限额
func (m *RealtimeRiskMonitor) UpdateRiskLimit(name string, warningThreshold, criticalThreshold float64) error {
	m.riskEventsLock.Lock()
	defer m.riskEventsLock.Unlock()

	if limit, ok := m.riskLimits[name]; ok {
		limit.WarningThreshold = warningThreshold
		limit.CriticalThreshold = criticalThreshold
		return nil
	}
	return fmt.Errorf("risk limit %s not found", name)
}

// ToJSON 转换为JSON
func (r *RiskEvent) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// generateEventID 生成事件ID
func generateEventID() string {
	return fmt.Sprintf("risk_%d", time.Now().UnixNano())
}

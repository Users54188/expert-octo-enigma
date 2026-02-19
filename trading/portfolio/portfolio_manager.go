package portfolio

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"cloudquant/trading"
)

// PortfolioManager 组合管理器
type PortfolioManager struct {
	mu              sync.RWMutex
	config          *PortfolioConfig
	positions       map[string]*PortfolioPosition // 持仓信息
	strategyWeights map[string]float64            // 策略权重
	performance     *PortfolioPerformance         // 组合表现
	positionManager *trading.PositionManager
	riskManager     *trading.RiskManager
	createdAt       time.Time
	lastRebalance   time.Time
}

// PortfolioConfig 组合配置
type PortfolioConfig struct {
	RebalanceFrequency time.Duration `yaml:"rebalance_frequency"` // 调仓频率
	MaxTurnover        float64       `yaml:"max_turnover"`        // 最大换手率
	MinPositionWeight  float64       `yaml:"min_position_weight"` // 最小持仓权重
	MaxPositionWeight  float64       `yaml:"max_position_weight"` // 最大持仓权重
	TargetReturn       float64       `yaml:"target_return"`       // 目标收益率
	RiskFreeRate       float64       `yaml:"risk_free_rate"`      // 无风险利率
}

// PortfolioPosition 组合持仓
type PortfolioPosition struct {
	Symbol        string        `json:"symbol"`
	Quantity      int64         `json:"quantity"`
	MarketValue   float64       `json:"market_value"`
	Weight        float64       `json:"weight"`
	CostBasis     float64       `json:"cost_basis"`
	UnrealizedPL  float64       `json:"unrealized_pl"`
	WeightHistory []WeightPoint `json:"weight_history"` // 权重历史
	LastUpdate    time.Time     `json:"last_update"`
}

// WeightPoint 权重点
type WeightPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Weight    float64   `json:"weight"`
	Value     float64   `json:"value"`
}

// PortfolioPerformance 组合表现
type PortfolioPerformance struct {
	TotalValue           float64       `json:"total_value"`
	TotalReturn          float64       `json:"total_return"`
	DailyReturn          float64       `json:"daily_return"`
	WeeklyReturn         float64       `json:"weekly_return"`
	MonthlyReturn        float64       `json:"monthly_return"`
	AnnualizedReturn     float64       `json:"annualized_return"`
	MaxDrawdown          float64       `json:"max_drawdown"`
	SharpeRatio          float64       `json:"sharpe_ratio"`
	Volatility           float64       `json:"volatility"`
	WinRate              float64       `json:"win_rate"`
	ProfitFactor         float64       `json:"profit_factor"`
	CalmarRatio          float64       `json:"calmar_ratio"`
	Alpha                float64       `json:"alpha"`
	Beta                 float64       `json:"beta"`
	ValueAtRisk          float64       `json:"value_at_risk"`
	MaxConsecutiveLosses int           `json:"max_consecutive_losses"`
	ReturnHistory        []ReturnPoint `json:"return_history"`
	BenchmarkReturn      float64       `json:"benchmark_return"`
	ExcessReturn         float64       `json:"excess_return"`
	InformationRatio     float64       `json:"information_ratio"`
	TrackingError        float64       `json:"tracking_error"`
	LastUpdate           time.Time     `json:"last_update"`
}

// ReturnPoint 收益点
type ReturnPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Return    float64   `json:"return"`
}

// NewPortfolioManager 创建组合管理器
func NewPortfolioManager(config PortfolioConfig, positionManager *trading.PositionManager, riskManager *trading.RiskManager) *PortfolioManager {
	return &PortfolioManager{
		config:          &config,
		positions:       make(map[string]*PortfolioPosition),
		strategyWeights: make(map[string]float64),
		performance:     &PortfolioPerformance{},
		positionManager: positionManager,
		riskManager:     riskManager,
		createdAt:       time.Now(),
		lastRebalance:   time.Now(),
	}
}

// UpdatePositions 更新持仓信息
func (p *PortfolioManager) UpdatePositions(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.positionManager == nil {
		return fmt.Errorf("position manager not configured")
	}

	// 获取所有持仓
	positions := p.positionManager.GetAllPositions()

	// 计算总市值
	totalValue := 0.0
	for _, pos := range positions {
		totalValue += pos.MarketValue
	}

	// 更新组合持仓
	for _, pos := range positions {
		weight := 0.0
		if totalValue > 0 {
			weight = pos.MarketValue / totalValue
		}

		p.positions[pos.Symbol] = &PortfolioPosition{
			Symbol:       pos.Symbol,
			Quantity:     int64(pos.Amount),
			MarketValue:  pos.MarketValue,
			Weight:       weight,
			CostBasis:    pos.CostPrice,
			UnrealizedPL: pos.UnrealizedPnL,
			LastUpdate:   time.Now(),
		}

		// 更新权重历史
		if existing, exists := p.positions[pos.Symbol]; exists {
			if len(existing.WeightHistory) == 0 ||
				time.Since(existing.WeightHistory[len(existing.WeightHistory)-1].Timestamp) > time.Hour {
				existing.WeightHistory = append(existing.WeightHistory, WeightPoint{
					Timestamp: time.Now(),
					Weight:    weight,
					Value:     pos.MarketValue,
				})

				// 限制历史长度
				if len(existing.WeightHistory) > 100 {
					existing.WeightHistory = existing.WeightHistory[1:]
				}
			}
		}
	}

	// 更新组合表现
	p.updatePerformance(totalValue)

	log.Printf("Portfolio updated: %d positions, total value: %.2f", len(p.positions), totalValue)
	return nil
}

// updatePerformance 更新组合表现
func (p *PortfolioManager) updatePerformance(totalValue float64) {
	p.performance.TotalValue = totalValue

	// 计算收益率
	if len(p.performance.ReturnHistory) > 0 {
		latestReturn := p.performance.ReturnHistory[len(p.performance.ReturnHistory)-1]
		p.performance.DailyReturn = latestReturn.Return
	}

	// 计算累计收益率
	initialValue := p.getInitialValue()
	if initialValue > 0 {
		p.performance.TotalReturn = (totalValue - initialValue) / initialValue
	}

	// 计算年化收益率
	if len(p.performance.ReturnHistory) > 0 {
		days := time.Since(p.createdAt).Hours() / 24
		if days > 0 {
			p.performance.AnnualizedReturn = math.Pow(1+p.performance.TotalReturn, 365/days) - 1
		}
	}

	// 计算最大回撤
	p.performance.MaxDrawdown = p.calculateMaxDrawdown()

	// 计算夏普比率
	p.performance.SharpeRatio = p.calculateSharpeRatio()

	// 更新统计时间
	p.performance.LastUpdate = time.Now()
}

// calculateMaxDrawdown 计算最大回撤
func (p *PortfolioManager) calculateMaxDrawdown() float64 {
	if len(p.performance.ReturnHistory) == 0 {
		return 0.0
	}

	var maxDrawdown float64
	var peak float64

	for _, point := range p.performance.ReturnHistory {
		if point.Value > peak {
			peak = point.Value
		}

		drawdown := (peak - point.Value) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}

// calculateSharpeRatio 计算夏普比率
func (p *PortfolioManager) calculateSharpeRatio() float64 {
	if len(p.performance.ReturnHistory) < 2 {
		return 0.0
	}

	// 计算超额收益
	excessReturns := make([]float64, len(p.performance.ReturnHistory))
	for i, point := range p.performance.ReturnHistory {
		excessReturns[i] = point.Return - p.config.RiskFreeRate/365
	}

	// 计算超额收益的标准差
	var mean float64
	for _, ret := range excessReturns {
		mean += ret
	}
	mean /= float64(len(excessReturns))

	var variance float64
	for _, ret := range excessReturns {
		diff := ret - mean
		variance += diff * diff
	}
	variance /= float64(len(excessReturns) - 1)

	stdDev := math.Sqrt(variance)

	// 夏普比率
	if stdDev == 0 {
		return 0.0
	}

	annualizedStdDev := stdDev * math.Sqrt(252)
	annualizedExcessReturn := mean * 252

	return annualizedExcessReturn / annualizedStdDev
}

// getInitialValue 获取初始价值
func (p *PortfolioManager) getInitialValue() float64 {
	// 简化实现：使用组合创建时的总价值
	// 实际应用中应该从配置或数据库获取
	if len(p.performance.ReturnHistory) > 0 {
		return p.performance.ReturnHistory[0].Value
	}
	return p.performance.TotalValue
}

// Rebalance 组合调仓
func (p *PortfolioManager) Rebalance(ctx context.Context, strategyWeights map[string]float64) ([]*RebalanceOrder, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证权重
	if err := p.validateWeights(strategyWeights); err != nil {
		return nil, fmt.Errorf("invalid strategy weights: %v", err)
	}

	p.strategyWeights = strategyWeights

	// 计算目标持仓
	targetPositions, err := p.calculateTargetPositions(strategyWeights)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate target positions: %v", err)
	}

	// 生成调仓订单
	orders := p.generateRebalanceOrders(targetPositions)

	// 更新调仓时间
	p.lastRebalance = time.Now()

	log.Printf("Portfolio rebalanced: %d orders generated", len(orders))
	return orders, nil
}

// validateWeights 验证权重
func (p *PortfolioManager) validateWeights(weights map[string]float64) error {
	var totalWeight float64
	for _, weight := range weights {
		if weight < 0 {
			return fmt.Errorf("negative weight not allowed: %.2f", weight)
		}
		totalWeight += weight
	}

	if math.Abs(totalWeight-1.0) > 0.001 {
		return fmt.Errorf("weights must sum to 1.0, got %.2f", totalWeight)
	}

	return nil
}

// calculateTargetPositions 计算目标持仓
func (p *PortfolioManager) calculateTargetPositions(weights map[string]float64) (map[string]float64, error) {
	targetPositions := make(map[string]float64)

	// 基于策略权重分配持仓
	for strategyName, weight := range weights {
		if weight <= 0 {
			continue
		}

		// 为每个策略分配目标市值
		targetValue := p.performance.TotalValue * weight

		// 这里简化实现：平均分配给该策略推荐的股票
		// 实际应用中需要根据策略的具体推荐来分配

		// 模拟策略推荐的股票（实际应该从策略管理器获取）
		strategySymbols := p.getStrategySymbols(strategyName)
		if len(strategySymbols) == 0 {
			continue
		}

		// 平均分配
		valuePerSymbol := targetValue / float64(len(strategySymbols))
		for _, symbol := range strategySymbols {
			targetPositions[symbol] += valuePerSymbol
		}
	}

	return targetPositions, nil
}

// generateRebalanceOrders 生成调仓订单
func (p *PortfolioManager) generateRebalanceOrders(targetPositions map[string]float64) []*RebalanceOrder {
	var orders []*RebalanceOrder

	for symbol, targetValue := range targetPositions {
		currentPosition := p.positions[symbol]
		if currentPosition == nil {
			// 新建持仓
			orders = append(orders, &RebalanceOrder{
				Symbol:        symbol,
				Action:        "buy",
				TargetValue:   targetValue,
				CurrentValue:  0,
				OrderValue:    targetValue,
				OrderQuantity: 0, // 实际计算数量
			})
			continue
		}

		currentValue := currentPosition.MarketValue
		if math.Abs(currentValue-targetValue) < 100 { // 忽略小额差异
			continue
		}

		// 计算订单价值
		orderValue := targetValue - currentValue
		action := "buy"
		if orderValue < 0 {
			action = "sell"
			orderValue = -orderValue
		}

		// 检查是否超过换手率限制
		if orderValue/p.performance.TotalValue > p.config.MaxTurnover {
			orderValue = p.performance.TotalValue * p.config.MaxTurnover
		}

		orders = append(orders, &RebalanceOrder{
			Symbol:        symbol,
			Action:        action,
			TargetValue:   targetValue,
			CurrentValue:  currentValue,
			OrderValue:    orderValue,
			OrderQuantity: 0, // 实际计算数量
		})
	}

	return orders
}

// getStrategySymbols 获取策略推荐的股票
func (p *PortfolioManager) getStrategySymbols(strategyName string) []string {
	// 简化的策略股票映射
	// 实际应用中应该从策略管理器获取
	switch strategyName {
	case "ma_strategy":
		return []string{"sh600000", "sh600519"}
	case "rsi_strategy":
		return []string{"sh601398", "sh600036"}
	case "ai_strategy":
		return []string{"sh600000", "sh601398"}
	case "ml_strategy":
		return []string{"sh600519", "sh600036"}
	default:
		return []string{"sh600000"}
	}
}

// GetPortfolioOverview 获取组合概览
func (p *PortfolioManager) GetPortfolioOverview() *PortfolioOverview {
	p.mu.RLock()
	defer p.mu.RUnlock()

	overview := &PortfolioOverview{
		TotalValue:       p.performance.TotalValue,
		TotalReturn:      p.performance.TotalReturn,
		DailyReturn:      p.performance.DailyReturn,
		AnnualizedReturn: p.performance.AnnualizedReturn,
		MaxDrawdown:      p.performance.MaxDrawdown,
		SharpeRatio:      p.performance.SharpeRatio,
		PositionCount:    len(p.positions),
		CashBalance:      0.0, // 简化实现
		CreatedAt:        p.createdAt,
		LastRebalance:    p.lastRebalance,
		NextRebalance:    p.lastRebalance.Add(p.config.RebalanceFrequency),
	}

	// 计算持仓分布
	overview.PositionDistribution = p.calculatePositionDistribution()

	// 计算行业分布
	overview.IndustryDistribution = p.calculateIndustryDistribution()

	return overview
}

// calculatePositionDistribution 计算持仓分布
func (p *PortfolioManager) calculatePositionDistribution() map[string]float64 {
	distribution := make(map[string]float64)
	totalValue := p.performance.TotalValue

	if totalValue == 0 {
		return distribution
	}

	for _, position := range p.positions {
		distribution[position.Symbol] = position.Weight
	}

	return distribution
}

// calculateIndustryDistribution 计算行业分布
func (p *PortfolioManager) calculateIndustryDistribution() map[string]float64 {
	distribution := make(map[string]float64)
	totalValue := p.performance.TotalValue

	if totalValue == 0 {
		return distribution
	}

	for _, position := range p.positions {
		industry := p.getIndustryFromSymbol(position.Symbol)
		distribution[industry] += position.Weight
	}

	return distribution
}

// getIndustryFromSymbol 从股票代码获取行业（简化实现）
func (p *PortfolioManager) getIndustryFromSymbol(symbol string) string {
	if len(symbol) >= 6 {
		prefix := symbol[:3]
		switch prefix {
		case "600", "601", "603":
			return "主板"
		case "000", "002":
			return "中小板"
		case "300":
			return "创业板"
		default:
			return "其他"
		}
	}
	return "未知"
}

// SetStrategyWeights 设置策略权重
func (p *PortfolioManager) SetStrategyWeights(weights map[string]float64) error {
	if err := p.validateWeights(weights); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.strategyWeights = weights
	log.Printf("Strategy weights updated: %v", weights)
	return nil
}

// GetStrategyWeights 获取策略权重
func (p *PortfolioManager) GetStrategyWeights() map[string]float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]float64)
	for k, v := range p.strategyWeights {
		result[k] = v
	}
	return result
}

// GetPositionDetails 获取持仓详情
func (p *PortfolioManager) GetPositionDetails(symbol string) (*PortfolioPosition, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	position, exists := p.positions[symbol]
	if !exists {
		return nil, false
	}

	// 返回副本
	positionCopy := *position
	positionCopy.WeightHistory = make([]WeightPoint, len(position.WeightHistory))
	copy(positionCopy.WeightHistory, position.WeightHistory)

	return &positionCopy, true
}

// GetAllPositions 获取所有持仓
func (p *PortfolioManager) GetAllPositions() map[string]*PortfolioPosition {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*PortfolioPosition)
	for symbol, position := range p.positions {
		positionCopy := *position
		positionCopy.WeightHistory = make([]WeightPoint, len(position.WeightHistory))
		copy(positionCopy.WeightHistory, position.WeightHistory)
		result[symbol] = &positionCopy
	}

	return result
}

// GetPerformance 获取组合表现
func (p *PortfolioManager) GetPerformance() *PortfolioPerformance {
	p.mu.RLock()
	defer p.mu.RUnlock()

	performanceCopy := *p.performance
	performanceCopy.ReturnHistory = make([]ReturnPoint, len(p.performance.ReturnHistory))
	copy(performanceCopy.ReturnHistory, p.performance.ReturnHistory)

	return &performanceCopy
}

// RebalanceNow 立即调仓
func (p *PortfolioManager) RebalanceNow(ctx context.Context) ([]*RebalanceOrder, error) {
	return p.Rebalance(ctx, p.strategyWeights)
}

// ShouldRebalance 检查是否需要调仓
func (p *PortfolioManager) ShouldRebalance() bool {
	return time.Since(p.lastRebalance) >= p.config.RebalanceFrequency
}

// GetRebalanceRecommendation 获取调仓建议
func (p *PortfolioManager) GetRebalanceRecommendation(ctx context.Context) (*RebalanceRecommendation, error) {
	// 获取当前权重
	currentWeights := p.getCurrentWeights()
	targetWeights := p.strategyWeights

	// 计算权重偏离
	drifts := p.calculateWeightDrifts(currentWeights, targetWeights)

	// 判断是否需要调仓
	needsRebalancing := false
	maxDrift := 0.0
	for _, drift := range drifts {
		if math.Abs(drift) > 0.05 { // 5%阈值
			needsRebalancing = true
		}
		if math.Abs(drift) > maxDrift {
			maxDrift = math.Abs(drift)
		}
	}

	return &RebalanceRecommendation{
		NeedsRebalancing:  needsRebalancing,
		MaxWeightDrift:    maxDrift,
		CurrentWeights:    currentWeights,
		TargetWeights:     targetWeights,
		WeightDrifts:      drifts,
		RecommendedAction: p.getRecommendedAction(drifts),
		EstimatedCost:     p.estimateRebalanceCost(drifts),
		Timestamp:         time.Now(),
	}, nil
}

// getCurrentWeights 获取当前权重
func (p *PortfolioManager) getCurrentWeights() map[string]float64 {
	weights := make(map[string]float64)
	totalValue := p.performance.TotalValue

	if totalValue == 0 {
		return weights
	}

	for symbol, position := range p.positions {
		weights[symbol] = position.Weight
	}

	return weights
}

// calculateWeightDrifts 计算权重偏离
func (p *PortfolioManager) calculateWeightDrifts(current, target map[string]float64) map[string]float64 {
	drifts := make(map[string]float64)

	// 获取所有股票代码
	allSymbols := make(map[string]bool)
	for symbol := range current {
		allSymbols[symbol] = true
	}
	for symbol := range target {
		allSymbols[symbol] = true
	}

	for symbol := range allSymbols {
		currentWeight := current[symbol]
		targetWeight := target[symbol]
		drifts[symbol] = targetWeight - currentWeight
	}

	return drifts
}

// getRecommendedAction 获取推荐操作
func (p *PortfolioManager) getRecommendedAction(drifts map[string]float64) string {
	totalPositive := 0.0
	totalNegative := 0.0

	for _, drift := range drifts {
		if drift > 0 {
			totalPositive += drift
		} else {
			totalNegative += drift
		}
	}

	if math.Abs(totalPositive) > math.Abs(totalNegative)*1.5 {
		return "buy_more"
	} else if math.Abs(totalNegative) > math.Abs(totalPositive)*1.5 {
		return "sell_some"
	} else {
		return "balanced"
	}
}

// estimateRebalanceCost 估算调仓成本
func (p *PortfolioManager) estimateRebalanceCost(drifts map[string]float64) float64 {
	turnover := 0.0
	for _, drift := range drifts {
		turnover += math.Abs(drift)
	}

	// 假设交易成本为0.1%
	return p.performance.TotalValue * turnover * 0.001
}

// SetConfig 更新配置
func (p *PortfolioManager) SetConfig(config PortfolioConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = &config
	log.Printf("Portfolio config updated: rebalance_frequency=%v", config.RebalanceFrequency)
}

// GetConfig 获取配置
func (p *PortfolioManager) GetConfig() PortfolioConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return *p.config
}

// GetStats 获取统计信息
func (p *PortfolioManager) GetStats() *PortfolioStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &PortfolioStats{
		TotalPositions:   len(p.positions),
		ActiveStrategies: len(p.strategyWeights),
		Uptime:           time.Since(p.createdAt),
		LastUpdate:       p.performance.LastUpdate,
		LastRebalance:    p.lastRebalance,
		ShouldRebalance:  p.ShouldRebalance(),
	}
}

// RebalanceOrder 调仓订单
type RebalanceOrder struct {
	Symbol        string  `json:"symbol"`
	Action        string  `json:"action"`         // buy, sell
	TargetValue   float64 `json:"target_value"`   // 目标价值
	CurrentValue  float64 `json:"current_value"`  // 当前价值
	OrderValue    float64 `json:"order_value"`    // 订单价值
	OrderQuantity int64   `json:"order_quantity"` // 订单数量
	Priority      int     `json:"priority"`       // 优先级
}

// RebalanceRecommendation 调仓建议
type RebalanceRecommendation struct {
	NeedsRebalancing  bool               `json:"needs_rebalancing"`
	MaxWeightDrift    float64            `json:"max_weight_drift"`
	CurrentWeights    map[string]float64 `json:"current_weights"`
	TargetWeights     map[string]float64 `json:"target_weights"`
	WeightDrifts      map[string]float64 `json:"weight_drifts"`
	RecommendedAction string             `json:"recommended_action"`
	EstimatedCost     float64            `json:"estimated_cost"`
	Timestamp         time.Time          `json:"timestamp"`
}

// PortfolioOverview 组合概览
type PortfolioOverview struct {
	TotalValue           float64            `json:"total_value"`
	TotalReturn          float64            `json:"total_return"`
	DailyReturn          float64            `json:"daily_return"`
	AnnualizedReturn     float64            `json:"annualized_return"`
	MaxDrawdown          float64            `json:"max_drawdown"`
	SharpeRatio          float64            `json:"sharpe_ratio"`
	PositionCount        int                `json:"position_count"`
	CashBalance          float64            `json:"cash_balance"`
	CreatedAt            time.Time          `json:"created_at"`
	LastRebalance        time.Time          `json:"last_rebalance"`
	NextRebalance        time.Time          `json:"next_rebalance"`
	PositionDistribution map[string]float64 `json:"position_distribution"`
	IndustryDistribution map[string]float64 `json:"industry_distribution"`
}

// PortfolioStats 组合统计
type PortfolioStats struct {
	TotalPositions   int           `json:"total_positions"`
	ActiveStrategies int           `json:"active_strategies"`
	Uptime           time.Duration `json:"uptime"`
	LastUpdate       time.Time     `json:"last_update"`
	LastRebalance    time.Time     `json:"last_rebalance"`
	ShouldRebalance  bool          `json:"should_rebalance"`
}

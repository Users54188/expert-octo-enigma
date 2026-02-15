package risk

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"cloudquant/trading"
)

// VolatilityRisk 波动率风险管理
type VolatilityRisk struct {
	mu              sync.RWMutex
	config          *VolatilityRiskConfig
	positionManager *trading.PositionManager
	priceHistory    map[string][]PricePoint // 价格历史
	volatilityCache map[string]float64      // 波动率缓存
	lastCalculation time.Time
}

// PricePoint 价格点
type PricePoint struct {
	Price float64   `json:"price"`
	Time  time.Time `json:"time"`
}

// VolatilityRiskConfig 波动率风险配置
type VolatilityRiskConfig struct {
	MaxVolatility       float64 `yaml:"max_volatility"`       // 最大波动率
	VolatilityThreshold float64 `yaml:"volatility_threshold"` // 波动率阈值
	LookbackPeriod      int     `yaml:"lookback_period"`      // 回看期数
	AdjustmentFactor    float64 `yaml:"adjustment_factor"`    // 调整因子
}

// NewVolatilityRisk 创建波动率风险管理器
func NewVolatilityRisk(config VolatilityRiskConfig, positionManager *trading.PositionManager) *VolatilityRisk {
	return &VolatilityRisk{
		config:          &config,
		positionManager: positionManager,
		priceHistory:    make(map[string][]PricePoint),
		volatilityCache: make(map[string]float64),
	}
}

// CalculateVolatility 计算波动率
func (v *VolatilityRisk) CalculateVolatility(ctx context.Context, symbol string) (float64, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 获取价格历史
	history, exists := v.priceHistory[symbol]
	if !exists || len(history) < v.config.LookbackPeriod+1 {
		// 需要更多数据
		return 0, fmt.Errorf("insufficient price history for %s", symbol)
	}

	// 计算对数收益率
	var returns []float64
	for i := 1; i < len(history); i++ {
		if history[i-1].Price > 0 && history[i].Price > 0 {
			returnVal := math.Log(history[i].Price / history[i-1].Price)
			returns = append(returns, returnVal)
		}
	}

	if len(returns) < v.config.LookbackPeriod {
		return 0, fmt.Errorf("insufficient returns data for %s", symbol)
	}

	// 计算波动率（标准差）
	mean := 0.0
	for _, ret := range returns {
		mean += ret
	}
	mean /= float64(len(returns))

	variance := 0.0
	for _, ret := range returns {
		diff := ret - mean
		variance += diff * diff
	}
	variance /= float64(len(returns) - 1)

	volatility := math.Sqrt(variance)

	// 年化波动率（假设252个交易日）
	annualizedVolatility := volatility * math.Sqrt(252)

	// 缓存结果
	v.volatilityCache[symbol] = annualizedVolatility

	log.Printf("Calculated volatility for %s: %.4f", symbol, annualizedVolatility)
	return annualizedVolatility, nil
}

// CheckVolatilityRisk 检查波动率风险
func (v *VolatilityRisk) CheckVolatilityRisk(ctx context.Context) ([]*VolatilityRiskAlert, error) {
	alerts := make([]*VolatilityRiskAlert, 0)

	if v.positionManager == nil {
		return nil, fmt.Errorf("position manager not configured")
	}

	// 获取所有持仓
	positions := v.positionManager.GetAllPositions()

	for _, pos := range positions {
		// 计算或获取缓存的波动率
		volatility, err := v.CalculateVolatility(ctx, pos.Symbol)
		if err != nil {
			log.Printf("Failed to calculate volatility for %s: %v", pos.Symbol, err)
			continue
		}

		// 检查波动率风险
		if volatility > v.config.MaxVolatility {
			alerts = append(alerts, &VolatilityRiskAlert{
				Symbol:     pos.Symbol,
				Volatility: volatility,
				MaxAllowed: v.config.MaxVolatility,
				Level:      "error",
				Message:    fmt.Sprintf("波动率过高: %.4f > %.4f", volatility, v.config.MaxVolatility),
			})
		} else if volatility > v.config.VolatilityThreshold {
			alerts = append(alerts, &VolatilityRiskAlert{
				Symbol:     pos.Symbol,
				Volatility: volatility,
				MaxAllowed: v.config.MaxVolatility,
				Level:      "warning",
				Message:    fmt.Sprintf("波动率告警: %.4f > %.4f", volatility, v.config.VolatilityThreshold),
			})
		}
	}

	return alerts, nil
}

// GetPositionSizing 基于波动率的仓位调整
func (v *VolatilityRisk) GetPositionSizing(ctx context.Context, symbol string, baseSize float64) (float64, error) {
	volatility, err := v.CalculateVolatility(ctx, symbol)
	if err != nil {
		// 如果无法计算波动率，使用基准仓位
		return baseSize, nil
	}

	// 基于波动率调整仓位
	// 波动率越高，仓位越小
	var adjustmentRatio float64
	if volatility <= v.config.VolatilityThreshold {
		adjustmentRatio = 1.0
	} else if volatility >= v.config.MaxVolatility {
		adjustmentRatio = 0.1 // 最小仓位
	} else {
		// 线性插值
		adjustmentRatio = 1.0 - (volatility-v.config.VolatilityThreshold)/(v.config.MaxVolatility-v.config.VolatilityThreshold)*(0.9)
		if adjustmentRatio < 0.1 {
			adjustmentRatio = 0.1
		}
	}

	adjustedSize := baseSize * adjustmentRatio

	log.Printf("Position sizing for %s: base=%.2f, volatility=%.4f, adjusted=%.2f (ratio=%.2f)",
		symbol, baseSize, volatility, adjustedSize, adjustmentRatio)

	return adjustedSize, nil
}

// UpdatePrice 更新价格历史
func (v *VolatilityRisk) UpdatePrice(symbol string, price float64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// 添加新价格点
	point := PricePoint{
		Price: price,
		Time:  time.Now(),
	}

	// 初始化或追加价格历史
	if _, exists := v.priceHistory[symbol]; !exists {
		v.priceHistory[symbol] = make([]PricePoint, 0, 100)
	}

	v.priceHistory[symbol] = append(v.priceHistory[symbol], point)

	// 限制历史数据长度，避免内存泄露
	if len(v.priceHistory[symbol]) > 1000 {
		v.priceHistory[symbol] = v.priceHistory[symbol][len(v.priceHistory[symbol])-500:]
	}

	// 清除缓存的波动率，强制重新计算
	delete(v.volatilityCache, symbol)

	log.Printf("Updated price for %s: %.2f", symbol, price)
}

// GetVolatility 获取缓存的波动率
func (v *VolatilityRisk) GetVolatility(symbol string) (float64, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	volatility, exists := v.volatilityCache[symbol]
	return volatility, exists
}

// GetPriceHistory 获取价格历史
func (v *VolatilityRisk) GetPriceHistory(symbol string) []PricePoint {
	v.mu.RLock()
	defer v.mu.RUnlock()

	history, exists := v.priceHistory[symbol]
	if !exists {
		return nil
	}

	// 返回副本
	result := make([]PricePoint, len(history))
	copy(result, history)
	return result
}

// SetConfig 更新配置
func (v *VolatilityRisk) SetConfig(config VolatilityRiskConfig) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.config = &config
	v.volatilityCache = make(map[string]float64) // 清除缓存
	log.Printf("Volatility risk config updated: max=%.4f, threshold=%.4f",
		config.MaxVolatility, config.VolatilityThreshold)
}

// GetRiskMetrics 获取风险指标
func (v *VolatilityRisk) GetRiskMetrics(ctx context.Context) (*VolatilityRiskMetrics, error) {
	metrics := &VolatilityRiskMetrics{
		Timestamp: time.Now(),
	}

	// 获取所有持仓
	positions, err := v.positionManager.GetAllPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get positions: %v", err)
	}

	totalValue := 0.0
	weightedVolatility := 0.0

	for _, pos := range positions {
		volatility, err := v.CalculateVolatility(ctx, pos.Symbol)
		if err != nil {
			continue
		}

		totalValue += pos.MarketValue
		weightedVolatility += volatility * pos.MarketValue
	}

	if totalValue > 0 {
		metrics.PortfolioVolatility = weightedVolatility / totalValue
	}

	metrics.PositionCount = len(positions)
	metrics.HighVolPositions = len(v.filterHighVolPositions(ctx, positions))

	return metrics, nil
}

// filterHighVolPositions 过滤高波动率持仓
func (v *VolatilityRisk) filterHighVolPositions(ctx context.Context, positions []trading.Position) []trading.Position {
	var highVolPositions []trading.Position

	for _, pos := range positions {
		volatility, err := v.CalculateVolatility(ctx, pos.Symbol)
		if err != nil {
			continue
		}

		if volatility > v.config.VolatilityThreshold {
			highVolPositions = append(highVolPositions, pos)
		}
	}

	return highVolPositions
}

// VolatilityRiskAlert 波动率风险告警
type VolatilityRiskAlert struct {
	Symbol     string  `json:"symbol"`
	Volatility float64 `json:"volatility"`
	MaxAllowed float64 `json:"max_allowed"`
	Level      string  `json:"level"`
	Message    string  `json:"message"`
}

// VolatilityRiskMetrics 波动率风险指标
type VolatilityRiskMetrics struct {
	Timestamp           time.Time `json:"timestamp"`
	PortfolioVolatility float64   `json:"portfolio_volatility"`
	PositionCount       int       `json:"position_count"`
	HighVolPositions    int       `json:"high_vol_positions"`
}

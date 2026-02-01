package strategies

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"cloudquant/trading"
)

// Strategy type constants (must match strategy_loader.go)
const (
	MAStrategyType StrategyType = "ma"
	RSIStrategyType StrategyType = "rsi"
	AIStrategyType StrategyType = "ai"
	MLStrategyType StrategyType = "ml"
)

// MAStrategy 均线策略
type MAStrategy struct {
	*BaseStrategy
	shortPeriod int     // 短期均线周期
	longPeriod  int     // 长期均线周期
	dataSeries  []float64 // 价格数据序列
	maSeries    []float64 // 均线数据序列
}

// NewMAStrategy 创建均线策略
func NewMAStrategy() Strategy {
	strategy := &MAStrategy{
		BaseStrategy: NewBaseStrategy("ma_strategy", 0.3),
		shortPeriod: 5,
		longPeriod:  20,
		dataSeries:  make([]float64, 0, 100),
		maSeries:    make([]float64, 0, 100),
	}

	// 设置默认参数
	strategy.parameters = map[string]interface{}{
		"short_period": 5,
		"long_period":  20,
		"min_volume":   1000000, // 最小成交量
		"max_change":   0.05,    // 最大涨跌幅(5%)
	}

	return strategy
}

// Init 初始化策略
func (m *MAStrategy) Init(ctx context.Context, symbol string, config map[string]interface{}) error {
	if err := m.BaseStrategy.Init(ctx, symbol, config); err != nil {
		return err
	}

	// 从配置中获取参数
	if period, ok := config["short_period"].(int); ok {
		m.shortPeriod = period
	}
	if period, ok := config["long_period"].(int); ok {
		m.longPeriod = period
	}

	// 参数验证
	if m.shortPeriod >= m.longPeriod {
		return fmt.Errorf("short period must be less than long period")
	}
	if m.shortPeriod <= 0 || m.longPeriod <= 0 {
		return fmt.Errorf("periods must be positive")
	}

	log.Printf("MA strategy initialized: short=%d, long=%d", m.shortPeriod, m.longPeriod)
	return nil
}

// GenerateSignal 生成交易信号
func (m *MAStrategy) GenerateSignal(ctx context.Context, marketData *MarketData) (*Signal, error) {
	if marketData == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// 更新价格数据
	m.updatePriceData(marketData.Close)

	// 需要足够的数据才能计算均线
	if len(m.dataSeries) < m.longPeriod {
		return nil, nil // 数据不足，等待更多数据
	}

	// 计算均线
	shortMA := m.calculateSMA(m.dataSeries, m.shortPeriod)
	longMA := m.calculateSMA(m.dataSeries, m.longPeriod)

	if shortMA == 0 || longMA == 0 {
		return nil, nil // 计算失败
	}

	// 获取最新价格
	currentPrice := marketData.Close
	volume := marketData.Volume

	// 检查成交量
	minVolume := 1000000.0
	if vol, ok := m.parameters["min_volume"].(float64); ok {
		minVolume = vol
	}

	if float64(volume) < minVolume {
		return nil, nil // 成交量不足
	}

	// 检查涨跌幅
	maxChange := 0.05
	if change, ok := m.parameters["max_change"].(float64); ok {
		maxChange = change
	}

	changePercent := math.Abs(marketData.ChangePercent) / 100.0
	if changePercent > maxChange {
		return nil, nil // 涨跌幅过大
	}

	// 生成信号
	var signal *Signal
	var signalType string
	var strength float64
	var reason string

	// 金叉买入，死叉卖出
	if shortMA > longMA && m.dataSeries[len(m.dataSeries)-2] <= m.calculateSMA(m.dataSeries[:len(m.dataSeries)-1], m.longPeriod) {
		// 金叉
		signalType = "buy"
		strength = math.Min(shortMA/longMA-1, 1.0) // 标准化强度
		reason = fmt.Sprintf("Golden cross: short MA %.2f > long MA %.2f", shortMA, longMA)
	} else if shortMA < longMA && m.dataSeries[len(m.dataSeries)-2] >= m.calculateSMA(m.dataSeries[:len(m.dataSeries)-1], m.longPeriod) {
		// 死叉
		signalType = "sell"
		strength = math.Min(1-(shortMA/longMA), 1.0)
		reason = fmt.Sprintf("Death cross: short MA %.2f < long MA %.2f", shortMA, longMA)
	} else {
		// 持仓
		return nil, nil
	}

	signal = NewSignal(marketData.Symbol, signalType, strength, currentPrice)
	signal.TargetPrice = m.calculateTargetPrice(currentPrice, signalType, 0.03) // 3%目标收益
	signal.StopLoss = m.calculateStopLoss(currentPrice, signalType, 0.02)      // 2%止损
	signal.Reason = reason

	log.Printf("MA strategy generated signal: %s %s (strength: %.3f)", 
		marketData.Symbol, signalType, strength)

	return signal, nil
}

// OnTrade 交易回调
func (m *MAStrategy) OnTrade(ctx context.Context, trade *trading.TradeRecord) error {
	// MA策略的交易回调：记录交易信息
	log.Printf("MA strategy trade executed: %s %d shares at %.2f", 
		trade.Symbol, trade.Quantity, trade.Price)
	return nil
}

// OnDailyClose 收盘回调
func (m *MAStrategy) OnDailyClose(ctx context.Context, date time.Time) error {
	// MA策略的收盘处理：计算日均收益等
	log.Printf("MA strategy daily close processing for %s", date.Format("2006-01-02"))
	return nil
}

// updatePriceData 更新价格数据
func (m *MAStrategy) updatePriceData(price float64) {
	m.dataSeries = append(m.dataSeries, price)
	
	// 限制数据长度，避免内存泄露
	if len(m.dataSeries) > 200 {
		m.dataSeries = m.dataSeries[1:]
	}
}

// calculateSMA 计算简单移动平均
func (m *MAStrategy) calculateSMA(data []float64, period int) float64 {
	if len(data) < period {
		return 0
	}

	sum := 0.0
	for i := len(data) - period; i < len(data); i++ {
		sum += data[i]
	}
	return sum / float64(period)
}

// calculateTargetPrice 计算目标价格
func (m *MAStrategy) calculateTargetPrice(currentPrice float64, signalType string, targetPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 + targetPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 - targetPercent)
	}
	return currentPrice
}

// calculateStopLoss 计算止损价格
func (m *MAStrategy) calculateStopLoss(currentPrice float64, signalType string, stopLossPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 - stopLossPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 + stopLossPercent)
	}
	return currentPrice
}

// GetParameters 获取策略参数
func (m *MAStrategy) GetParameters() map[string]interface{} {
	params := m.BaseStrategy.GetParameters()
	params["short_period"] = m.shortPeriod
	params["long_period"] = m.longPeriod
	return params
}

// UpdateParameters 更新策略参数
func (m *MAStrategy) UpdateParameters(params map[string]interface{}) error {
	if err := m.BaseStrategy.UpdateParameters(params); err != nil {
		return err
	}

	// 更新周期参数
	if shortPeriod, ok := params["short_period"].(int); ok {
		m.shortPeriod = shortPeriod
	}
	if longPeriod, ok := params["long_period"].(int); ok {
		m.longPeriod = longPeriod
	}

	return nil
}
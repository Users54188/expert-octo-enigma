package strategies

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"cloudquant/trading"
)

// RSIStrategy RSI超买超卖策略
type RSIStrategy struct {
	*BaseStrategy
	period     int       // RSI周期
	oversold   float64   // 超卖阈值
	overbought float64   // 超买阈值
	dataSeries []float64 // 价格数据序列
	gains      []float64 // 上涨幅度
	losses     []float64 // 下跌幅度
}

// NewRSIStrategy 创建RSI策略
func NewRSIStrategy() Strategy {
	strategy := &RSIStrategy{
		BaseStrategy: NewBaseStrategy("rsi_strategy", 0.25),
		period:       14,
		oversold:     30.0,
		overbought:   70.0,
		dataSeries:   make([]float64, 0, 100),
		gains:        make([]float64, 0, 100),
		losses:       make([]float64, 0, 100),
	}

	// 设置默认参数
	strategy.parameters = map[string]interface{}{
		"period":       14,
		"oversold":     30.0,
		"overbought":   70.0,
		"min_volume":   500000, // 最小成交量
		"price_filter": true,   // 价格过滤
	}

	return strategy
}

// Init 初始化策略
func (r *RSIStrategy) Init(ctx context.Context, symbol string, config map[string]interface{}) error {
	if err := r.BaseStrategy.Init(ctx, symbol, config); err != nil {
		return err
	}

	// 从配置中获取参数
	if period, ok := config["period"].(int); ok {
		r.period = period
	}
	if oversold, ok := config["oversold"].(float64); ok {
		r.oversold = oversold
	}
	if overbought, ok := config["overbought"].(float64); ok {
		r.overbought = overbought
	}

	// 参数验证
	if r.period <= 0 {
		return fmt.Errorf("RSI period must be positive")
	}
	if r.oversold >= r.overbought {
		return fmt.Errorf("oversold threshold must be less than overbought threshold")
	}
	if r.oversold < 0 || r.overbought > 100 {
		return fmt.Errorf("RSI thresholds must be between 0 and 100")
	}

	log.Printf("RSI strategy initialized: period=%d, oversold=%.1f, overbought=%.1f",
		r.period, r.oversold, r.overbought)
	return nil
}

// GenerateSignal 生成交易信号
func (r *RSIStrategy) GenerateSignal(ctx context.Context, marketData *MarketData) (*Signal, error) {
	if marketData == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// 更新价格数据
	r.updatePriceData(marketData.Close)

	// 需要足够的数据才能计算RSI
	if len(r.dataSeries) < r.period+1 {
		return nil, nil // 数据不足
	}

	// 计算RSI
	rsi := r.calculateRSI()

	if rsi == 0 {
		return nil, nil // 计算失败
	}

	// 获取参数
	volume := marketData.Volume
	minVolume := 500000.0
	if vol, ok := r.parameters["min_volume"].(float64); ok {
		minVolume = vol
	}

	// 检查成交量
	if float64(volume) < minVolume {
		return nil, nil
	}

	// 价格过滤
	priceFilter := true
	if filter, ok := r.parameters["price_filter"].(bool); ok {
		priceFilter = filter
	}

	if priceFilter && marketData.ChangePercent > 5.0 {
		return nil, nil // 涨跌幅过大，过滤
	}

	// 生成信号
	var signal *Signal
	var signalType string
	var strength float64
	var reason string

	if rsi <= r.oversold {
		// 超卖信号，买入
		signalType = "buy"
		strength = (r.oversold - rsi) / r.oversold // 标准化强度
		strength = math.Min(strength, 1.0)
		reason = fmt.Sprintf("Oversold: RSI %.2f < %.2f", rsi, r.oversold)
	} else if rsi >= r.overbought {
		// 超买信号，卖出
		signalType = "sell"
		strength = (rsi - r.overbought) / (100 - r.overbought)
		strength = math.Min(strength, 1.0)
		reason = fmt.Sprintf("Overbought: RSI %.2f > %.2f", rsi, r.overbought)
	} else {
		// 正常范围，无信号
		return nil, nil
	}

	signal = NewSignal(marketData.Symbol, signalType, strength, marketData.Close)
	signal.TargetPrice = r.calculateTargetPrice(marketData.Close, signalType, 0.04)
	signal.StopLoss = r.calculateStopLoss(marketData.Close, signalType, 0.025)
	signal.Reason = reason

	// 添加RSI值到元数据
	signal.Metadata["rsi_value"] = rsi
	signal.Metadata["oversold_threshold"] = r.oversold
	signal.Metadata["overbought_threshold"] = r.overbought

	log.Printf("RSI strategy generated signal: %s %s (RSI: %.2f, strength: %.3f)",
		marketData.Symbol, signalType, rsi, strength)

	return signal, nil
}

// OnTrade 交易回调
func (r *RSIStrategy) OnTrade(ctx context.Context, trade *trading.TradeRecord) error {
	log.Printf("RSI strategy trade executed: %s %d shares at %.2f",
		trade.Symbol, trade.Volume, trade.Price)
	return nil
}

// OnDailyClose 收盘回调
func (r *RSIStrategy) OnDailyClose(ctx context.Context, date time.Time) error {
	log.Printf("RSI strategy daily close processing for %s", date.Format("2006-01-02"))
	return nil
}

// updatePriceData 更新价格数据
func (r *RSIStrategy) updatePriceData(price float64) {
	// 计算价格变化
	if len(r.dataSeries) > 0 {
		prevPrice := r.dataSeries[len(r.dataSeries)-1]
		change := price - prevPrice

		if change > 0 {
			r.gains = append(r.gains, change)
			r.losses = append(r.losses, 0.0)
		} else {
			r.gains = append(r.gains, 0.0)
			r.losses = append(r.losses, -change)
		}

		// 限制数据长度
		if len(r.gains) > 200 {
			r.gains = r.gains[1:]
			r.losses = r.losses[1:]
		}
	}

	r.dataSeries = append(r.dataSeries, price)
	if len(r.dataSeries) > 200 {
		r.dataSeries = r.dataSeries[1:]
	}
}

// calculateRSI 计算RSI值
func (r *RSIStrategy) calculateRSI() float64 {
	if len(r.gains) < r.period || len(r.losses) < r.period {
		return 0
	}

	// 计算平均涨幅和跌幅
	avgGain := 0.0
	avgLoss := 0.0

	for i := len(r.gains) - r.period; i < len(r.gains); i++ {
		avgGain += r.gains[i]
		avgLoss += r.losses[i]
	}

	avgGain /= float64(r.period)
	avgLoss /= float64(r.period)

	if avgLoss == 0 {
		return 100 // 无亏损，RSI=100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// calculateTargetPrice 计算目标价格
func (r *RSIStrategy) calculateTargetPrice(currentPrice float64, signalType string, targetPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 + targetPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 - targetPercent)
	}
	return currentPrice
}

// calculateStopLoss 计算止损价格
func (r *RSIStrategy) calculateStopLoss(currentPrice float64, signalType string, stopLossPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 - stopLossPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 + stopLossPercent)
	}
	return currentPrice
}

// GetCurrentRSI 获取当前RSI值
func (r *RSIStrategy) GetCurrentRSI() float64 {
	return r.calculateRSI()
}

// GetParameters 获取策略参数
func (r *RSIStrategy) GetParameters() map[string]interface{} {
	params := r.BaseStrategy.GetParameters()
	params["period"] = r.period
	params["oversold"] = r.oversold
	params["overbought"] = r.overbought
	return params
}

// UpdateParameters 更新策略参数
func (r *RSIStrategy) UpdateParameters(params map[string]interface{}) error {
	if err := r.BaseStrategy.UpdateParameters(params); err != nil {
		return err
	}

	if period, ok := params["period"].(int); ok {
		r.period = period
	}
	if oversold, ok := params["oversold"].(float64); ok {
		r.oversold = oversold
	}
	if overbought, ok := params["overbought"].(float64); ok {
		r.overbought = overbought
	}

	return nil
}

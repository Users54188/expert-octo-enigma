package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloudquant/ml"
	"cloudquant/trading"
)

// MLStrategy 机器学习策略
type MLStrategy struct {
	*BaseStrategy
	modelProvider  ml.ModelProvider
	features       []string // 特征列表
	lookbackDays   int     // 回看天数
	confidence     float64 // 置信度阈值
	lastPrediction *MLPrediction
	dataBuffer     []float64 // 价格数据缓存
}

// MLPrediction ML预测结果
type MLPrediction struct {
	Signal     string  `json:"signal"`     // buy, sell, hold
	Confidence float64 `json:"confidence"` // 置信度 0-1
	Probability float64 `json:"probability"` // 概率
	FeatureImportance map[string]float64 `json:"feature_importance"` // 特征重要性
	Timestamp  time.Time `json:"timestamp"`
	ModelInfo  map[string]interface{} `json:"model_info"` // 模型信息
}

// NewMLStrategy 创建ML策略
func NewMLStrategy() Strategy {
	strategy := &MLStrategy{
		BaseStrategy:  NewBaseStrategy("ml_strategy", 0.3),
		features:      make([]string, 0),
		lookbackDays:  20,
		confidence:    0.6,
		dataBuffer:    make([]float64, 0, 100),
		lastPrediction: nil,
	}

	// 设置默认参数
	strategy.parameters = map[string]interface{}{
		"lookback_days":      20,
		"confidence":         0.6,
		"features":            []string{"price", "volume", "ma5", "ma10", "rsi"},
		"update_frequency":   "1h", // 模型更新频率
		"use_real_time":      true, // 使用实时数据
	}

	return strategy
}

// Init 初始化策略
func (m *MLStrategy) Init(ctx context.Context, symbol string, config map[string]interface{}) error {
	if err := m.BaseStrategy.Init(ctx, symbol, config); err != nil {
		return err
	}

	// 从配置中获取参数
	if lookback, ok := config["lookback_days"].(int); ok {
		m.lookbackDays = lookback
	}
	if confidence, ok := config["confidence"].(float64); ok {
		m.confidence = confidence
	}

	// 设置特征列表
	if features, ok := config["features"].([]interface{}); ok {
		m.features = make([]string, len(features))
		for i, feature := range features {
			if str, ok := feature.(string); ok {
				m.features[i] = str
			}
		}
	} else {
		m.features = []string{"price", "volume", "ma5", "ma10", "rsi"}
	}

	// 参数验证
	if m.lookbackDays <= 0 {
		return fmt.Errorf("lookback_days must be positive")
	}
	if m.confidence < 0 || m.confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	log.Printf("ML strategy initialized: lookback=%d, confidence=%.2f, features=%v", 
		m.lookbackDays, m.confidence, m.features)
	return nil
}

// SetModelProvider 设置模型提供者
func (m *MLStrategy) SetModelProvider(provider ml.ModelProvider) {
	m.modelProvider = provider
	log.Printf("ML strategy model provider set")
}

// GenerateSignal 生成ML交易信号
func (m *MLStrategy) GenerateSignal(ctx context.Context, marketData *MarketData) (*Signal, error) {
	if marketData == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	// 更新数据缓存
	m.updateDataBuffer(marketData.Close)

	// 检查是否有足够的训练数据
	if len(m.dataBuffer) < m.lookbackDays+5 {
		return nil, nil // 数据不足
	}

	// 如果没有模型提供者，返回简单信号
	if m.modelProvider == nil {
		return m.generateFallbackSignal(marketData)
	}

	// 准备特征数据
	features, err := m.extractFeatures(marketData)
	if err != nil {
		log.Printf("Feature extraction failed: %v", err)
		return nil, err
	}

	// 进行ML预测
	prediction, err := m.performMLPrediction(ctx, features)
	if err != nil {
		log.Printf("ML prediction failed: %v", err)
		return nil, err
	}

	m.lastPrediction = prediction

	// 根据预测结果生成信号
	if prediction == nil || prediction.Confidence < m.confidence {
		return nil, nil // 置信度不足
	}

	var signal *Signal
	var signalType string
	var strength float64
	var reason string

	switch prediction.Signal {
	case "buy":
		signalType = "buy"
		strength = prediction.Confidence
		reason = fmt.Sprintf("ML Buy Signal: confidence=%.2f, probability=%.2f", 
			prediction.Confidence, prediction.Probability)
	case "sell":
		signalType = "sell"
		strength = prediction.Confidence
		reason = fmt.Sprintf("ML Sell Signal: confidence=%.2f, probability=%.2f", 
			prediction.Confidence, prediction.Probability)
	default:
		return nil, nil // hold信号不执行
	}

	signal = NewSignal(marketData.Symbol, signalType, strength, marketData.Close)
	signal.TargetPrice = m.calculateTargetPrice(marketData.Close, signalType, 0.06)
	signal.StopLoss = m.calculateStopLoss(marketData.Close, signalType, 0.04)
	signal.Reason = reason

	// 添加ML预测信息到元数据
	signal.Metadata["ml_confidence"] = prediction.Confidence
	signal.Metadata["ml_probability"] = prediction.Probability
	signal.Metadata["ml_features"] = features
	if len(prediction.FeatureImportance) > 0 {
		signal.Metadata["ml_feature_importance"] = prediction.FeatureImportance
	}

	log.Printf("ML strategy generated signal: %s %s (confidence: %.3f)", 
		marketData.Symbol, signalType, prediction.Confidence)

	return signal, nil
}

// extractFeatures 提取特征
func (m *MLStrategy) extractFeatures(marketData *MarketData) (map[string]float64, error) {
	features := make(map[string]float64)

	// 基础价格特征
	features["price"] = marketData.Close
	features["open"] = marketData.Open
	features["high"] = marketData.High
	features["low"] = marketData.Low
	features["volume"] = float64(marketData.Volume)
	features["change_percent"] = marketData.ChangePercent
	features["amount"] = marketData.Amount

	// 技术指标特征
	features["ma5"] = m.calculateMA(5)
	features["ma10"] = m.calculateMA(10)
	features["ma20"] = m.calculateMA(20)
	features["rsi"] = m.calculateRSI(14)

	// 价格动量特征
	features["price_momentum_1"] = m.calculateMomentum(1)
	features["price_momentum_3"] = m.calculateMomentum(3)
	features["price_momentum_5"] = m.calculateMomentum(5)

	// 成交量特征
	features["volume_ma5"] = m.calculateVolumeMA(5)
	features["volume_ratio"] = m.calculateVolumeRatio()

	// 波动率特征
	features["volatility_5"] = m.calculateVolatility(5)
	features["volatility_10"] = m.calculateVolatility(10)

	return features, nil
}

// performMLPrediction 执行ML预测
func (m *MLStrategy) performMLPrediction(ctx context.Context, features map[string]float64) (*MLPrediction, error) {
	if m.modelProvider == nil {
		return nil, fmt.Errorf("model provider not set")
	}

	// 调用ML模型预测
	result, err := m.modelProvider.Predict(ctx, features)
	if err != nil {
		return nil, fmt.Errorf("ML prediction error: %v", err)
	}

	// 解析预测结果
	prediction, err := m.parsePredictionResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prediction result: %v", err)
	}

	return prediction, nil
}

// parsePredictionResult 解析预测结果
func (m *MLStrategy) parsePredictionResult(result interface{}) (*MLPrediction, error) {
	prediction := &MLPrediction{
		Timestamp:        time.Now(),
		FeatureImportance: make(map[string]float64),
		ModelInfo:        make(map[string]interface{}),
	}

	// 如果结果是JSON字符串，尝试解析
	if jsonStr, ok := result.(string); ok {
		if err := json.Unmarshal([]byte(jsonStr), prediction); err != nil {
			// 如果解析失败，返回默认预测
			return m.createDefaultPrediction("hold", 0.5), nil
		}
		return prediction, nil
	}

	// 如果结果是map
	if resultMap, ok := result.(map[string]interface{}); ok {
		if signal, ok := resultMap["signal"].(string); ok {
			prediction.Signal = signal
		} else {
			prediction.Signal = "hold"
		}

		if confidence, ok := resultMap["confidence"].(float64); ok {
			prediction.Confidence = confidence
		} else {
			prediction.Confidence = 0.5
		}

		if prob, ok := resultMap["probability"].(float64); ok {
			prediction.Probability = prob
		} else {
			prediction.Probability = prediction.Confidence
		}

		return prediction, nil
	}

	// 默认预测
	return m.createDefaultPrediction("hold", 0.5), nil
}

// createDefaultPrediction 创建默认预测
func (m *MLStrategy) createDefaultPrediction(signal string, confidence float64) *MLPrediction {
	return &MLPrediction{
		Signal:            signal,
		Confidence:        confidence,
		Probability:       confidence,
		FeatureImportance: make(map[string]float64),
		Timestamp:         time.Now(),
		ModelInfo:         map[string]interface{}{"default": true},
	}
}

// generateFallbackSignal 生成后备信号（无模型时）
func (m *MLStrategy) generateFallbackSignal(marketData *MarketData) (*Signal, error) {
	// 简单的基于价格的逻辑
	ma20 := m.calculateMA(20)
	rsi := m.calculateRSI(14)

	if marketData.Close > ma20 && rsi < 70 {
		// 价格在均线之上且RSI未超买
		signal := NewSignal(marketData.Symbol, "buy", 0.6, marketData.Close)
		signal.Reason = "Fallback: price above MA20, RSI not overbought"
		return signal, nil
	} else if marketData.Close < ma20 && rsi > 30 {
		// 价格在均线之下且RSI未超卖
		signal := NewSignal(marketData.Symbol, "sell", 0.6, marketData.Close)
		signal.Reason = "Fallback: price below MA20, RSI not oversold"
		return signal, nil
	}

	return nil, nil
}

// updateDataBuffer 更新数据缓存
func (m *MLStrategy) updateDataBuffer(price float64) {
	m.dataBuffer = append(m.dataBuffer, price)
	if len(m.dataBuffer) > 200 {
		m.dataBuffer = m.dataBuffer[1:]
	}
}

// calculateMA 计算移动平均
func (m *MLStrategy) calculateMA(period int) float64 {
	if len(m.dataBuffer) < period {
		return 0
	}

	sum := 0.0
	for i := len(m.dataBuffer) - period; i < len(m.dataBuffer); i++ {
		sum += m.dataBuffer[i]
	}
	return sum / float64(period)
}

// calculateRSI 计算RSI
func (m *MLStrategy) calculateRSI(period int) float64 {
	if len(m.dataBuffer) < period+1 {
		return 50.0 // 默认中性值
	}

	gains := 0.0
	losses := 0.0

	for i := len(m.dataBuffer) - period; i < len(m.dataBuffer); i++ {
		change := m.dataBuffer[i] - m.dataBuffer[i-1]
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// calculateMomentum 计算动量
func (m *MLStrategy) calculateMomentum(period int) float64 {
	if len(m.dataBuffer) < period+1 {
		return 0
	}

	return (m.dataBuffer[len(m.dataBuffer)-1] - m.dataBuffer[len(m.dataBuffer)-1-period]) / m.dataBuffer[len(m.dataBuffer)-1-period]
}

// calculateVolumeMA 计算成交量移动平均（简化版，返回估计值）
func (m *MLStrategy) calculateVolumeMA(period int) float64 {
	// 简化实现：基于价格变化估计成交量
	return float64(period) * 1000000 // 模拟成交量
}

// calculateVolumeRatio 计算成交量比率（简化版）
func (m *MLStrategy) calculateVolumeRatio() float64 {
	return 1.0 // 简化实现
}

// calculateVolatility 计算波动率
func (m *MLStrategy) calculateVolatility(period int) float64 {
	if len(m.dataBuffer) < period {
		return 0
	}

	var sum float64
	var sumSq float64

	for i := len(m.dataBuffer) - period; i < len(m.dataBuffer); i++ {
		sum += m.dataBuffer[i]
		sumSq += m.dataBuffer[i] * m.dataBuffer[i]
	}

	mean := sum / float64(period)
	variance := (sumSq / float64(period)) - (mean * mean)

	if variance < 0 {
		variance = 0
	}

	return variance
}

// calculateTargetPrice 计算目标价格
func (m *MLStrategy) calculateTargetPrice(currentPrice float64, signalType string, targetPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 + targetPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 - targetPercent)
	}
	return currentPrice
}

// calculateStopLoss 计算止损价格
func (m *MLStrategy) calculateStopLoss(currentPrice float64, signalType string, stopLossPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 - stopLossPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 + stopLossPercent)
	}
	return currentPrice
}

// OnTrade 交易回调
func (m *MLStrategy) OnTrade(ctx context.Context, trade *trading.TradeRecord) error {
	log.Printf("ML strategy trade executed: %s %d shares at %.2f", 
		trade.Symbol, trade.Quantity, trade.Price)
	return nil
}

// OnDailyClose 收盘回调
func (m *MLStrategy) OnDailyClose(ctx context.Context, date time.Time) error {
	log.Printf("ML strategy daily close processing for %s", date.Format("2006-01-02"))
	return nil
}

// GetLatestPrediction 获取最新ML预测
func (m *MLStrategy) GetLatestPrediction() *MLPrediction {
	return m.lastPrediction
}

// GetParameters 获取策略参数
func (m *MLStrategy) GetParameters() map[string]interface{} {
	params := m.BaseStrategy.GetParameters()
	params["lookback_days"] = m.lookbackDays
	params["confidence"] = m.confidence
	params["features"] = m.features
	return params
}

// UpdateParameters 更新策略参数
func (m *MLStrategy) UpdateParameters(params map[string]interface{}) error {
	if err := m.BaseStrategy.UpdateParameters(params); err != nil {
		return err
	}

	if lookback, ok := params["lookback_days"].(int); ok {
		m.lookbackDays = lookback
	}
	if confidence, ok := params["confidence"].(float64); ok {
		m.confidence = confidence
	}

	return nil
}
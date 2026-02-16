package strategies

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cloudquant/llm"
	"cloudquant/trading"
)

// AIStrategy DeepSeek AI策略
type AIStrategy struct {
	*BaseStrategy
	llmAnalyzer    *llm.DeepSeekAnalyzer
	threshold      float64 // AI信号阈值
	confidence     float64 // 置信度阈值
	marketData     *MarketData
	lastAnalysis   time.Time
	analysisResult *AIAnalysisResult
}

// AIAnalysisResult AI分析结果
type AIAnalysisResult struct {
	Signal     string    `json:"signal"`     // buy, sell, hold
	Confidence float64   `json:"confidence"` // 置信度 0-1
	Reason     string    `json:"reason"`     // 分析原因
	Score      float64   `json:"score"`      // 综合评分
	RiskLevel  string    `json:"risk_level"` // 风险等级: low, medium, high
	Timestamp  time.Time `json:"timestamp"`
}

// NewAIStrategy 创建AI策略
func NewAIStrategy() Strategy {
	strategy := &AIStrategy{
		BaseStrategy: NewBaseStrategy("ai_strategy", 0.4),
		threshold:    0.7,
		confidence:   0.6,
		marketData:   nil,
		lastAnalysis: time.Time{},
	}

	// 设置默认参数
	strategy.parameters = map[string]interface{}{
		"threshold":         0.7,
		"confidence":        0.6,
		"analysis_interval": "1h", // 分析间隔
		"market_context":    true, // 是否包含市场上下文
		"risk_analysis":     true, // 是否包含风险分析
	}

	return strategy
}

// Init 初始化策略
func (a *AIStrategy) Init(ctx context.Context, symbol string, config map[string]interface{}) error {
	if err := a.BaseStrategy.Init(ctx, symbol, config); err != nil {
		return err
	}

	// 从配置中获取参数
	if threshold, ok := config["threshold"].(float64); ok {
		a.threshold = threshold
	}
	if confidence, ok := config["confidence"].(float64); ok {
		a.confidence = confidence
	}

	// 创建DeepSeek分析器（如果配置中有API Key）
	if apiKey, ok := config["api_key"].(string); ok && apiKey != "" {
		a.llmAnalyzer = llm.NewDeepSeekAnalyzer(apiKey, "deepseek-chat", 10*time.Second, 200)
		log.Printf("AI strategy initialized with DeepSeek analyzer")
	} else {
		log.Printf("AI strategy initialized without DeepSeek analyzer (no API key)")
	}

	// 参数验证
	if a.threshold < 0 || a.threshold > 1 {
		return fmt.Errorf("threshold must be between 0 and 1")
	}
	if a.confidence < 0 || a.confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	log.Printf("AI strategy initialized: threshold=%.2f, confidence=%.2f", a.threshold, a.confidence)
	return nil
}

// GenerateSignal 生成AI交易信号
func (a *AIStrategy) GenerateSignal(ctx context.Context, marketData *MarketData) (*Signal, error) {
	if marketData == nil {
		return nil, fmt.Errorf("market data is nil")
	}

	a.marketData = marketData

	// 如果没有LLM分析器，返回简单信号
	if a.llmAnalyzer == nil {
		return a.generateSimpleSignal(marketData)
	}

	// 检查是否需要重新分析
	now := time.Now()
	analysisInterval := time.Hour // 默认1小时
	if interval, ok := a.parameters["analysis_interval"].(string); ok {
		if d, err := time.ParseDuration(interval); err == nil {
			analysisInterval = d
		}
	}

	if a.lastAnalysis.IsZero() || now.Sub(a.lastAnalysis) > analysisInterval {
		// 进行AI分析
		result, err := a.performAIAnalysis(ctx, marketData)
		if err != nil {
			log.Printf("AI analysis failed: %v", err)
			return nil, err
		}
		a.analysisResult = result
		a.lastAnalysis = now
	}

	// 根据AI分析结果生成信号
	if a.analysisResult == nil {
		return nil, nil
	}

	var signal *Signal
	var signalType string
	var strength float64
	var reason string

	// 检查置信度
	if a.analysisResult.Confidence < a.confidence {
		return nil, nil // 置信度不足
	}

	switch a.analysisResult.Signal {
	case "buy":
		if a.analysisResult.Confidence >= a.threshold {
			signalType = "buy"
			strength = a.analysisResult.Confidence
			reason = fmt.Sprintf("AI Buy Signal: %s (confidence: %.2f)", a.analysisResult.Reason, a.analysisResult.Confidence)
		}
	case "sell":
		if a.analysisResult.Confidence >= a.threshold {
			signalType = "sell"
			strength = a.analysisResult.Confidence
			reason = fmt.Sprintf("AI Sell Signal: %s (confidence: %.2f)", a.analysisResult.Reason, a.analysisResult.Confidence)
		}
	default:
		return nil, nil // hold或其他信号不执行
	}

	if signalType == "" {
		return nil, nil
	}

	signal = NewSignal(marketData.Symbol, signalType, strength, marketData.Close)
	signal.TargetPrice = a.calculateTargetPrice(marketData.Close, signalType, 0.05) // 5%目标收益
	signal.StopLoss = a.calculateStopLoss(marketData.Close, signalType, 0.03)       // 3%止损
	signal.Reason = reason

	// 添加AI分析信息到元数据
	signal.Metadata["ai_confidence"] = a.analysisResult.Confidence
	signal.Metadata["ai_risk_level"] = a.analysisResult.RiskLevel
	signal.Metadata["ai_score"] = a.analysisResult.Score
	signal.Metadata["ai_reason"] = a.analysisResult.Reason

	log.Printf("AI strategy generated signal: %s %s (confidence: %.3f, risk: %s)",
		marketData.Symbol, signalType, a.analysisResult.Confidence, a.analysisResult.RiskLevel)

	return signal, nil
}

// performAIAnalysis 执行AI分析
func (a *AIStrategy) performAIAnalysis(ctx context.Context, marketData *MarketData) (*AIAnalysisResult, error) {
	if a.llmAnalyzer == nil {
		return nil, fmt.Errorf("LLM analyzer not initialized")
	}

	// 构建分析提示
	prompt := a.buildAnalysisPrompt(marketData)

	// 调用DeepSeek分析
	response, err := a.llmAnalyzer.AnalyzePrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek analysis failed: %v", err)
	}

	// 解析AI响应
	result, err := a.parseAIResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v", err)
	}

	return result, nil
}

// buildAnalysisPrompt 构建分析提示
func (a *AIStrategy) buildAnalysisPrompt(marketData *MarketData) string {
	includeContext := true
	if context, ok := a.parameters["market_context"].(bool); ok {
		includeContext = context
	}

	includeRisk := true
	if risk, ok := a.parameters["risk_analysis"].(bool); ok {
		includeRisk = risk
	}

	prompt := fmt.Sprintf(`请分析股票 %s 的投资价值：

当前价格信息：
- 当前价格: %.2f
- 今日涨跌: %.2f%% 
- 成交量: %d
- 开盘价: %.2f
- 最高价: %.2f
- 最低价: %.2f

`, marketData.Symbol, marketData.Close, marketData.ChangePercent, marketData.Volume, marketData.Open, marketData.High, marketData.Low)

	if includeContext {
		prompt += `请结合以下因素进行分析：
1. 技术指标表现
2. 市场趋势
3. 成交量变化
4. 风险收益比
5. 投资建议

`
	}

	if includeRisk {
		prompt += `请评估风险等级并提供：
- 投资建议：买入/卖出/持有
- 置信度：0-1之间的小数
- 风险等级：低/中/高
- 详细理由说明

`
	}

	prompt += `请以JSON格式回复，包含字段：signal, confidence, reason, score, risk_level`

	return prompt
}

// parseAIResponse 解析AI响应
func (a *AIStrategy) parseAIResponse(response string) (*AIAnalysisResult, error) {
	// 尝试解析JSON
	var result AIAnalysisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		// 如果不是JSON，尝试提取关键信息
		return a.extractInfoFromText(response)
	}

	// 验证结果
	if result.Signal == "" {
		result.Signal = "hold"
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		result.Confidence = 0.5
	}
	if result.RiskLevel == "" {
		result.RiskLevel = "medium"
	}

	result.Timestamp = time.Now()

	return &result, nil
}

// extractInfoFromText 从文本中提取信息
func (a *AIStrategy) extractInfoFromText(text string) (*AIAnalysisResult, error) {
	result := &AIAnalysisResult{
		Signal:     "hold",
		Confidence: 0.5,
		Reason:     text,
		Score:      0.0,
		RiskLevel:  "medium",
		Timestamp:  time.Now(),
	}

	// 简单关键词检测
	text = fmt.Sprintf(" %s ", text) // 添加空格便于匹配

	// 检测买入信号
	if containsAny(text, []string{"买入", "买入", "买入", "做多", "做多", "做多", "建议买入", "买入时机"}) {
		result.Signal = "buy"
		result.Confidence = 0.7
	}

	// 检测卖出信号
	if containsAny(text, []string{"卖出", "卖出", "卖出", "做空", "做空", "做空", "建议卖出", "卖出时机"}) {
		result.Signal = "sell"
		result.Confidence = 0.7
	}

	// 检测风险等级
	if containsAny(text, []string{"低风险", "风险较低", "安全"}) {
		result.RiskLevel = "low"
	} else if containsAny(text, []string{"高风险", "风险较高", "危险", "注意风险"}) {
		result.RiskLevel = "high"
	}

	return result, nil
}

// generateSimpleSignal 生成简单信号（无AI时）
func (a *AIStrategy) generateSimpleSignal(marketData *MarketData) (*Signal, error) {
	// 基于价格的简单逻辑
	if marketData.ChangePercent > 3.0 {
		// 大涨，可能回调
		signal := NewSignal(marketData.Symbol, "sell", 0.6, marketData.Close)
		signal.Reason = "Simple signal: price surge detected"
		return signal, nil
	} else if marketData.ChangePercent < -3.0 {
		// 大跌，可能反弹
		signal := NewSignal(marketData.Symbol, "buy", 0.6, marketData.Close)
		signal.Reason = "Simple signal: price drop detected"
		return signal, nil
	}

	return nil, nil
}

// OnTrade 交易回调
func (a *AIStrategy) OnTrade(ctx context.Context, trade *trading.TradeRecord) error {
	log.Printf("AI strategy trade executed: %s %d shares at %.2f",
		trade.Symbol, trade.Volume, trade.Price)
	return nil
}

// OnDailyClose 收盘回调
func (a *AIStrategy) OnDailyClose(ctx context.Context, date time.Time) error {
	// 重置分析状态
	a.lastAnalysis = time.Time{}
	a.analysisResult = nil
	log.Printf("AI strategy daily close processing for %s", date.Format("2006-01-02"))
	return nil
}

// calculateTargetPrice 计算目标价格
func (a *AIStrategy) calculateTargetPrice(currentPrice float64, signalType string, targetPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 + targetPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 - targetPercent)
	}
	return currentPrice
}

// calculateStopLoss 计算止损价格
func (a *AIStrategy) calculateStopLoss(currentPrice float64, signalType string, stopLossPercent float64) float64 {
	if signalType == "buy" {
		return currentPrice * (1 - stopLossPercent)
	} else if signalType == "sell" {
		return currentPrice * (1 + stopLossPercent)
	}
	return currentPrice
}

// GetLatestAnalysis 获取最新AI分析结果
func (a *AIStrategy) GetLatestAnalysis() *AIAnalysisResult {
	return a.analysisResult
}

// GetParameters 获取策略参数
func (a *AIStrategy) GetParameters() map[string]interface{} {
	params := a.BaseStrategy.GetParameters()
	params["threshold"] = a.threshold
	params["confidence"] = a.confidence
	return params
}

// UpdateParameters 更新策略参数
func (a *AIStrategy) UpdateParameters(params map[string]interface{}) error {
	if err := a.BaseStrategy.UpdateParameters(params); err != nil {
		return err
	}

	if threshold, ok := params["threshold"].(float64); ok {
		a.threshold = threshold
	}
	if confidence, ok := params["confidence"].(float64); ok {
		a.confidence = confidence
	}

	return nil
}

// 工具函数
func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if len(keyword) > 0 && contains(text, keyword) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

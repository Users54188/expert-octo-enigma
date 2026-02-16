package risk

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/llm"
	"cloudquant/trading"
)

// AIRisk DeepSeek AI风险评分
type AIRisk struct {
	mu              sync.RWMutex
	config          *AIRiskConfig
	llmAnalyzer     *llm.DeepSeekAnalyzer
	scoreCache      map[string]*RiskScore // 风险评分缓存
	analysisHistory []RiskAnalysis        // 分析历史
	positionManager *trading.PositionManager
	lastAnalysis    time.Time
}

// RiskScore AI风险评分
type RiskScore struct {
	Symbol          string    `json:"symbol"`
	OverallScore    float64   `json:"overall_score"`    // 总体风险评分 0-1
	MarketRisk      float64   `json:"market_risk"`      // 市场风险 0-1
	TechnicalRisk   float64   `json:"technical_risk"`   // 技术风险 0-1
	FundamentalRisk float64   `json:"fundamental_risk"` // 基本面风险 0-1
	VolatilityRisk  float64   `json:"volatility_risk"`  // 波动率风险 0-1
	TrendRisk       float64   `json:"trend_risk"`       // 趋势风险 0-1
	VolumeRisk      float64   `json:"volume_risk"`      // 成交量风险 0-1
	AIConfidence    float64   `json:"ai_confidence"`    // AI分析置信度 0-1
	RiskLevel       string    `json:"risk_level"`       // low, medium, high, extreme
	Recommendations []string  `json:"recommendations"`  // 建议
	Timestamp       time.Time `json:"timestamp"`
	ModelVersion    string    `json:"model_version"`
}

// RiskAnalysis AI风险分析
type RiskAnalysis struct {
	Symbol      string          `json:"symbol"`
	Score       *RiskScore      `json:"score"`
	RawAnalysis string          `json:"raw_analysis"` // 原始AI分析文本
	MarketData  json.RawMessage `json:"market_data"`  // 市场数据快照
	Timestamp   time.Time       `json:"timestamp"`
}

// AIRiskConfig AI风险配置
type AIRiskConfig struct {
	Enabled           bool          `yaml:"enabled"`            // 是否启用
	AnalysisInterval  time.Duration `yaml:"analysis_interval"`  // 分析间隔
	CacheExpiry       time.Duration `yaml:"cache_expiry"`       // 缓存过期时间
	RiskThreshold     float64       `yaml:"risk_threshold"`     // 风险阈值
	AutoAlert         bool          `yaml:"auto_alert"`         // 自动告警
	DeepLearning      bool          `yaml:"deep_learning"`      // 深度学习分析
	SentimentAnalysis bool          `yaml:"sentiment_analysis"` // 情绪分析
	NewsAnalysis      bool          `yaml:"news_analysis"`      // 新闻分析
}

// NewAIRisk 创建AI风险评分器
func NewAIRisk(config AIRiskConfig, llmAnalyzer *llm.DeepSeekAnalyzer, positionManager *trading.PositionManager) *AIRisk {
	return &AIRisk{
		config:          &config,
		llmAnalyzer:     llmAnalyzer,
		scoreCache:      make(map[string]*RiskScore),
		analysisHistory: make([]RiskAnalysis, 0, 100),
		positionManager: positionManager,
	}
}

// AnalyzeRisk 执行AI风险分析
func (a *AIRisk) AnalyzeRisk(ctx context.Context, symbol string, marketData map[string]interface{}) (*RiskScore, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 检查缓存
	if cached := a.getCachedScore(symbol); cached != nil {
		return cached, nil
	}

	// 检查是否启用AI风险分析
	if !a.config.Enabled {
		return a.generateDefaultScore(symbol), nil
	}

	// 检查分析间隔
	if a.shouldSkipAnalysis() {
		log.Printf("Skipping AI analysis for %s due to rate limiting", symbol)
		return a.generateDefaultScore(symbol), nil
	}

	// 执行AI分析
	score, err := a.performAIRiskAnalysis(ctx, symbol, marketData)
	if err != nil {
		log.Printf("AI risk analysis failed for %s: %v", symbol, err)
		return a.generateDefaultScore(symbol), nil
	}

	// 缓存结果
	a.scoreCache[symbol] = score
	a.lastAnalysis = time.Now()

	// 添加到历史
	analysis := RiskAnalysis{
		Symbol:     symbol,
		Score:      score,
		MarketData: a.serializeMarketData(marketData),
		Timestamp:  time.Now(),
	}
	a.addToHistory(analysis)

	// 检查是否需要告警
	if a.config.AutoAlert && score.OverallScore > a.config.RiskThreshold {
		go a.triggerRiskAlert(symbol, score)
	}

	log.Printf("AI risk analysis completed for %s: overall=%.3f, level=%s",
		symbol, score.OverallScore, score.RiskLevel)

	return score, nil
}

// performAIRiskAnalysis 执行具体的AI风险分析
func (a *AIRisk) performAIRiskAnalysis(ctx context.Context, symbol string, marketData map[string]interface{}) (*RiskScore, error) {
	if a.llmAnalyzer == nil {
		return nil, fmt.Errorf("LLM analyzer not initialized")
	}

	// 构建分析提示
	prompt := a.buildRiskAnalysisPrompt(symbol, marketData)

	// 调用AI分析
	response, err := a.llmAnalyzer.AnalyzePrompt(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %v", err)
	}

	// 解析AI响应
	score, err := a.parseRiskScoreResponse(response, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %v", err)
	}

	return score, nil
}

// buildRiskAnalysisPrompt 构建风险分析提示
func (a *AIRisk) buildRiskAnalysisPrompt(symbol string, marketData map[string]interface{}) string {
	prompt := fmt.Sprintf(`请对股票 %s 进行全面的风险评估分析：

股票基本信息：
- 代码: %s
`, symbol, symbol)

	// 添加市场数据
	if data, ok := marketData["price"].(float64); ok {
		prompt += fmt.Sprintf("- 当前价格: %.2f\n", data)
	}
	if data, ok := marketData["volume"].(int64); ok {
		prompt += fmt.Sprintf("- 成交量: %d\n", data)
	}
	if data, ok := marketData["change_percent"].(float64); ok {
		prompt += fmt.Sprintf("- 涨跌幅: %.2f%%\n", data)
	}
	if data, ok := marketData["pe_ratio"].(float64); ok {
		prompt += fmt.Sprintf("- PE比率: %.2f\n", data)
	}

	prompt += `
请从以下维度进行风险评估（每个维度0-1分，分数越高风险越大）：

1. 市场风险 (market_risk)：整体市场环境、宏观经济因素
2. 技术风险 (technical_risk)：技术指标、图表形态、价格走势
3. 基本面风险 (fundamental_risk)：财务状况、行业前景、竞争地位
4. 波动率风险 (volatility_risk)：价格波动剧烈程度
5. 趋势风险 (trend_risk)：当前趋势方向和强度
6. 成交量风险 (volume_risk)：成交量变化、流动性

请给出：
- 各个维度的风险评分
- 总体风险评分（各维度加权平均）
- 风险等级：low(0-0.25), medium(0.25-0.5), high(0.5-0.75), extreme(0.75-1.0)
- 具体投资建议和风险控制措施

请以JSON格式回复，包含字段：
{
  "market_risk": 0.0-1.0,
  "technical_risk": 0.0-1.0,
  "fundamental_risk": 0.0-1.0,
  "volatility_risk": 0.0-1.0,
  "trend_risk": 0.0-1.0,
  "volume_risk": 0.0-1.0,
  "ai_confidence": 0.0-1.0,
  "recommendations": ["建议1", "建议2"]
}`

	return prompt
}

// parseRiskScoreResponse 解析AI风险评分响应
func (a *AIRisk) parseRiskScoreResponse(response string, symbol string) (*RiskScore, error) {
	var aiData struct {
		MarketRisk      float64  `json:"market_risk"`
		TechnicalRisk   float64  `json:"technical_risk"`
		FundamentalRisk float64  `json:"fundamental_risk"`
		VolatilityRisk  float64  `json:"volatility_risk"`
		TrendRisk       float64  `json:"trend_risk"`
		VolumeRisk      float64  `json:"volume_risk"`
		AIConfidence    float64  `json:"ai_confidence"`
		Recommendations []string `json:"recommendations"`
	}

	// 尝试解析JSON
	if err := json.Unmarshal([]byte(response), &aiData); err != nil {
		// 如果不是JSON，尝试提取关键信息
		return a.extractRiskInfoFromText(response, symbol), nil
	}

	// 计算总体风险评分
	overallScore := (aiData.MarketRisk + aiData.TechnicalRisk + aiData.FundamentalRisk +
		aiData.VolatilityRisk + aiData.TrendRisk + aiData.VolumeRisk) / 6

	// 确定风险等级
	riskLevel := a.determineRiskLevel(overallScore)

	score := &RiskScore{
		Symbol:          symbol,
		OverallScore:    overallScore,
		MarketRisk:      aiData.MarketRisk,
		TechnicalRisk:   aiData.TechnicalRisk,
		FundamentalRisk: aiData.FundamentalRisk,
		VolatilityRisk:  aiData.VolatilityRisk,
		TrendRisk:       aiData.TrendRisk,
		VolumeRisk:      aiData.VolumeRisk,
		AIConfidence:    aiData.AIConfidence,
		RiskLevel:       riskLevel,
		Recommendations: aiData.Recommendations,
		Timestamp:       time.Now(),
		ModelVersion:    "deepseek-v1",
	}

	return score, nil
}

// extractRiskInfoFromText 从文本中提取风险信息
func (a *AIRisk) extractRiskInfoFromText(text string, symbol string) *RiskScore {
	score := &RiskScore{
		Symbol:          symbol,
		OverallScore:    0.5, // 默认中等风险
		MarketRisk:      0.5,
		TechnicalRisk:   0.5,
		FundamentalRisk: 0.5,
		VolatilityRisk:  0.5,
		TrendRisk:       0.5,
		VolumeRisk:      0.5,
		AIConfidence:    0.3,
		RiskLevel:       "medium",
		Recommendations: []string{"建议谨慎投资"},
		Timestamp:       time.Now(),
		ModelVersion:    "text-extraction-v1",
	}

	// 简单关键词检测
	text = fmt.Sprintf(" %s ", text)

	// 检测风险关键词
	if containsAny(text, []string{"高风险", "高风险", "风险较大", "注意风险", "谨慎", "避免"}) {
		score.OverallScore = 0.7
		score.RiskLevel = "high"
	}

	if containsAny(text, []string{"低风险", "风险较低", "安全", "稳健", "推荐"}) {
		score.OverallScore = 0.3
		score.RiskLevel = "low"
	}

	if containsAny(text, []string{"极高风险", "极高风险", "非常危险", "强烈不建议", "避免投资"}) {
		score.OverallScore = 0.9
		score.RiskLevel = "extreme"
	}

	return score
}

// determineRiskLevel 确定风险等级
func (a *AIRisk) determineRiskLevel(score float64) string {
	if score < 0.25 {
		return "low"
	} else if score < 0.5 {
		return "medium"
	} else if score < 0.75 {
		return "high"
	} else {
		return "extreme"
	}
}

// getCachedScore 获取缓存的风险评分
func (a *AIRisk) getCachedScore(symbol string) *RiskScore {
	score, exists := a.scoreCache[symbol]
	if !exists {
		return nil
	}

	// 检查缓存是否过期
	if time.Since(score.Timestamp) > a.config.CacheExpiry {
		delete(a.scoreCache, symbol)
		return nil
	}

	return score
}

// shouldSkipAnalysis 检查是否应该跳过分析（基于频率限制）
func (a *AIRisk) shouldSkipAnalysis() bool {
	if a.lastAnalysis.IsZero() {
		return false
	}

	return time.Since(a.lastAnalysis) < a.config.AnalysisInterval
}

// addToHistory 添加到分析历史
func (a *AIRisk) addToHistory(analysis RiskAnalysis) {
	a.analysisHistory = append(a.analysisHistory, analysis)

	// 限制历史长度
	if len(a.analysisHistory) > 1000 {
		a.analysisHistory = a.analysisHistory[1:]
	}
}

// generateDefaultScore 生成默认风险评分
func (a *AIRisk) generateDefaultScore(symbol string) *RiskScore {
	return &RiskScore{
		Symbol:          symbol,
		OverallScore:    0.5,
		MarketRisk:      0.5,
		TechnicalRisk:   0.5,
		FundamentalRisk: 0.5,
		VolatilityRisk:  0.5,
		TrendRisk:       0.5,
		VolumeRisk:      0.5,
		AIConfidence:    0.0,
		RiskLevel:       "medium",
		Recommendations: []string{"未进行AI分析"},
		Timestamp:       time.Now(),
		ModelVersion:    "default",
	}
}

// serializeMarketData 序列化市场数据
func (a *AIRisk) serializeMarketData(data map[string]interface{}) json.RawMessage {
	if data == nil {
		return nil
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	return json.RawMessage(raw)
}

// triggerRiskAlert 触发风险告警
func (a *AIRisk) triggerRiskAlert(symbol string, score *RiskScore) {
	// 这里应该调用告警系统
	// 由于告警系统可能在其他包中，这里只记录日志
	log.Printf("🚨 AI Risk Alert: %s - Overall Risk: %.3f (%s) - %v",
		symbol, score.OverallScore, score.RiskLevel, score.Recommendations)
}

// GetRiskScore 获取指定股票的风险评分
func (a *AIRisk) GetRiskScore(symbol string) (*RiskScore, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	score, exists := a.getCachedScore(symbol)
	return score, exists
}

// GetAllRiskScores 获取所有股票的风险评分
func (a *AIRisk) GetAllRiskScores() map[string]*RiskScore {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]*RiskScore)
	for symbol, score := range a.scoreCache {
		result[symbol] = score
	}

	return result
}

// GetAnalysisHistory 获取分析历史
func (a *AIRisk) GetAnalysisHistory(symbol string, limit int) []RiskAnalysis {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var history []RiskAnalysis
	for i := len(a.analysisHistory) - 1; i >= 0 && len(history) < limit; i-- {
		if a.analysisHistory[i].Symbol == symbol {
			history = append(history, a.analysisHistory[i])
		}
	}

	return history
}

// GetPortfolioRiskScore 获取组合风险评分
func (a *AIRisk) GetPortfolioRiskScore(ctx context.Context) (*PortfolioAIRiskScore, error) {
	_ = ctx
	positions := a.positionManager.GetAllPositions()

	if len(positions) == 0 {
		return &PortfolioAIRiskScore{
			OverallScore: 0.0,
			RiskLevel:    "low",
			Message:      "无持仓，无风险",
			Timestamp:    time.Now(),
		}, nil
	}

	var totalRisk float64
	var totalValue float64
	highRiskCount := 0

	for _, pos := range positions {
		score, exists := a.GetRiskScore(pos.Symbol)
		if !exists {
			// 如果没有评分，使用默认值
			score = a.generateDefaultScore(pos.Symbol)
		}

		totalRisk += score.OverallScore * pos.MarketValue
		totalValue += pos.MarketValue

		if score.OverallScore > 0.7 {
			highRiskCount++
		}
	}

	portfolioRisk := totalRisk / totalValue

	var riskLevel string
	if portfolioRisk < 0.25 {
		riskLevel = "low"
	} else if portfolioRisk < 0.5 {
		riskLevel = "medium"
	} else if portfolioRisk < 0.75 {
		riskLevel = "high"
	} else {
		riskLevel = "extreme"
	}

	return &PortfolioAIRiskScore{
		OverallScore:  portfolioRisk,
		RiskLevel:     riskLevel,
		TotalValue:    totalValue,
		HighRiskCount: highRiskCount,
		Message:       fmt.Sprintf("组合包含 %d 只股票，其中 %d 只为高风险", len(positions), highRiskCount),
		Timestamp:     time.Now(),
	}, nil
}

// SetConfig 更新配置
func (a *AIRisk) SetConfig(config AIRiskConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = &config

	// 清除过期缓存
	a.cleanExpiredCache()

	log.Printf("AI risk config updated: enabled=%v, threshold=%.3f", config.Enabled, config.RiskThreshold)
}

// cleanExpiredCache 清理过期缓存
func (a *AIRisk) cleanExpiredCache() {
	now := time.Now()
	for symbol, score := range a.scoreCache {
		if now.Sub(score.Timestamp) > a.config.CacheExpiry {
			delete(a.scoreCache, symbol)
		}
	}
}

// GetConfig 获取配置
func (a *AIRisk) GetConfig() AIRiskConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return *a.config
}

// GetStats 获取统计信息
func (a *AIRisk) GetStats() *AIRiskStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var avgConfidence float64
	var avgOverallRisk float64
	if len(a.scoreCache) > 0 {
		var totalConfidence, totalRisk float64
		for _, score := range a.scoreCache {
			totalConfidence += score.AIConfidence
			totalRisk += score.OverallScore
		}
		avgConfidence = totalConfidence / float64(len(a.scoreCache))
		avgOverallRisk = totalRisk / float64(len(a.scoreCache))
	}

	return &AIRiskStats{
		CachedScores:    len(a.scoreCache),
		AnalysisHistory: len(a.analysisHistory),
		AvgAIConfidence: avgConfidence,
		AvgOverallRisk:  avgOverallRisk,
		LastAnalysis:    a.lastAnalysis,
		Enabled:         a.config.Enabled,
	}
}

// PortfolioAIRiskScore 组合AI风险评分
type PortfolioAIRiskScore struct {
	OverallScore  float64   `json:"overall_score"`
	RiskLevel     string    `json:"risk_level"`
	TotalValue    float64   `json:"total_value"`
	HighRiskCount int       `json:"high_risk_count"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
}

// AIRiskStats AI风险统计
type AIRiskStats struct {
	CachedScores    int       `json:"cached_scores"`
	AnalysisHistory int       `json:"analysis_history"`
	AvgAIConfidence float64   `json:"avg_ai_confidence"`
	AvgOverallRisk  float64   `json:"avg_overall_risk"`
	LastAnalysis    time.Time `json:"last_analysis"`
	Enabled         bool      `json:"enabled"`
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

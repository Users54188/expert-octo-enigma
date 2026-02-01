package portfolio

import (
	"fmt"
	"log"
	"math"
	"time"
)

// PortfolioOptimizer 组合优化器（轻量版）
type PortfolioOptimizer struct {
	config *OptimizerConfig
}

// OptimizerConfig 优化器配置
type OptimizerConfig struct {
	Method           string  `yaml:"method"`            // 优化方法: equal_weight, risk_parity, max_sharpe
	RiskFreeRate     float64 `yaml:"risk_free_rate"`     // 无风险利率
	LookbackPeriod  int     `yaml:"lookback_period"`   // 回看期数
	MinWeight       float64 `yaml:"min_weight"`        // 最小权重
	MaxWeight       float64 `yaml:"max_weight"`        // 最大权重
	RebalancePeriod int     `yaml:"rebalance_period"`   // 调仓周期
}

// OptimizationResult 优化结果
type OptimizationResult struct {
	Weights     map[string]float64 `json:"weights"`      // 权重分配
	Method      string            `json:"method"`       // 优化方法
	SharpeRatio float64          `json:"sharpe_ratio"` // 夏普比率
	Return      float64          `json:"return"`       // 预期收益
	Risk        float64          `json:"risk"`         // 风险（标准差）
	MaxDrawdown float64          `json:"max_drawdown"` // 最大回撤
	Alpha       float64          `json:"alpha"`        // 阿尔法
	Beta        float64          `json:"beta"`         // 贝塔
	InfoRatio   float64          `json:"info_ratio"`   // 信息比率
	TrackingError float64        `json:"tracking_error"` // 跟踪误差
	ExecutionTime time.Duration  `json:"execution_time"` // 执行时间
	Timestamp   time.Time        `json:"timestamp"`
	Notes       string           `json:"notes"`
}

// AssetData 资产数据
type AssetData struct {
	Symbol      string    `json:"symbol"`
	Returns     []float64 `json:"returns"`      // 收益率序列
	MeanReturn float64   `json:"mean_return"`  // 平均收益率
	Volatility float64   `json:"volatility"`   // 波动率
	MinReturn  float64   `json:"min_return"`   // 最小收益率
	MaxReturn  float64   `json:"max_return"`   // 最大收益率
}

// CorrelationMatrix 相关性矩阵
type CorrelationMatrix struct {
	Assets     []string              `json:"assets"`     // 资产列表
	Covariance [][]float64           `json:"covariance"` // 协方差矩阵
	Correlation [][]float64          `json:"correlation"` // 相关性矩阵
}

// NewPortfolioOptimizer 创建组合优化器
func NewPortfolioOptimizer(config OptimizerConfig) *PortfolioOptimizer {
	return &PortfolioOptimizer{
		config: &config,
	}
}

// Optimize 执行组合优化
func (p *PortfolioOptimizer) Optimize(symbols []string, historicalData map[string][]float64) (*OptimizationResult, error) {
	startTime := time.Now()

	if len(symbols) == 0 {
		return nil, fmt.Errorf("no symbols provided for optimization")
	}

	// 验证数据
	if err := p.validateData(symbols, historicalData); err != nil {
		return nil, fmt.Errorf("invalid data: %v", err)
	}

	// 计算资产数据
	assetData, err := p.calculateAssetData(symbols, historicalData)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate asset data: %v", err)
	}

	// 根据优化方法执行优化
	var weights map[string]float64
	var metrics OptimizationMetrics

	switch p.config.Method {
	case "equal_weight":
		weights, metrics = p.equalWeightOptimization(symbols)
	case "risk_parity":
		weights, metrics = p.riskParityOptimization(assetData)
	case "max_sharpe":
		weights, metrics = p.maxSharpeOptimization(assetData)
	default:
		return nil, fmt.Errorf("unsupported optimization method: %s", p.config.Method)
	}

	result := &OptimizationResult{
		Weights:      weights,
		Method:       p.config.Method,
		SharpeRatio:  metrics.SharpeRatio,
		Return:       metrics.ExpectedReturn,
		Risk:         metrics.Risk,
		MaxDrawdown:  metrics.MaxDrawdown,
		Alpha:        metrics.Alpha,
		Beta:         metrics.Beta,
		InfoRatio:    metrics.InfoRatio,
		TrackingError: metrics.TrackingError,
		ExecutionTime: time.Since(startTime),
		Timestamp:    startTime,
		Notes:        p.generateOptimizationNotes(metrics),
	}

	log.Printf("Portfolio optimization completed: method=%s, sharpe_ratio=%.3f, execution_time=%v", 
		p.config.Method, result.SharpeRatio, result.ExecutionTime)

	return result, nil
}

// validateData 验证输入数据
func (p *PortfolioOptimizer) validateData(symbols []string, historicalData map[string][]float64) error {
	if len(symbols) == 0 {
		return fmt.Errorf("symbols list is empty")
	}

	for _, symbol := range symbols {
		returns, exists := historicalData[symbol]
		if !exists {
			return fmt.Errorf("missing data for symbol: %s", symbol)
		}

		if len(returns) < 10 {
			return fmt.Errorf("insufficient data for symbol %s: need at least 10 periods", symbol)
		}

		// 检查收益率是否有效
		for _, ret := range returns {
			if math.IsNaN(ret) || math.IsInf(ret, 0) {
				return fmt.Errorf("invalid return value for symbol %s: %v", symbol, ret)
			}
		}
	}

	return nil
}

// calculateAssetData 计算资产数据
func (p *PortfolioOptimizer) calculateAssetData(symbols []string, historicalData map[string][]float64) ([]*AssetData, error) {
	assetData := make([]*AssetData, len(symbols))

	for i, symbol := range symbols {
		returns := historicalData[symbol]
		
		// 计算统计量
		meanReturn := p.calculateMean(returns)
		volatility := p.calculateVolatility(returns)
		minReturn := p.calculateMin(returns)
		maxReturn := p.calculateMax(returns)

		assetData[i] = &AssetData{
			Symbol:      symbol,
			Returns:     returns,
			MeanReturn:  meanReturn,
			Volatility:  volatility,
			MinReturn:   minReturn,
			MaxReturn:   maxReturn,
		}
	}

	return assetData, nil
}

// equalWeightOptimization 等权重优化
func (p *PortfolioOptimizer) equalWeightOptimization(symbols []string) (map[string]float64, OptimizationMetrics) {
	weights := make(map[string]float64)
	equalWeight := 1.0 / float64(len(symbols))

	for _, symbol := range symbols {
		weights[symbol] = equalWeight
	}

	// 计算等权重组合的指标
	metrics := p.calculatePortfolioMetrics(weights, nil)

	return weights, metrics
}

// riskParityOptimization 风险平价优化
func (p *PortfolioOptimizer) riskParityOptimization(assetData []*AssetData) (map[string]float64, OptimizationMetrics) {
	// 简化的风险平价实现
	// 实际应该基于协方差矩阵计算
	n := len(assetData)
	weights := make(map[string]float64)

	// 等权重作为初始值
	baseWeight := 1.0 / float64(n)

	// 基于波动率调整权重（波动率越小，权重越大）
	var totalInverseVol float64
	inverseVols := make([]float64, n)

	for i, asset := range assetData {
		if asset.Volatility > 0 {
			inverseVols[i] = 1.0 / asset.Volatility
		} else {
			inverseVols[i] = 1.0
		}
		totalInverseVol += inverseVols[i]
	}

	for i, asset := range assetData {
		if totalInverseVol > 0 {
			weights[asset.Symbol] = inverseVols[i] / totalInverseVol
		} else {
			weights[asset.Symbol] = baseWeight
		}
	}

	// 应用权重限制
	p.applyWeightConstraints(weights)

	metrics := p.calculatePortfolioMetrics(weights, assetData)

	return weights, metrics
}

// maxSharpeOptimization 最大夏普比率优化（简化版）
func (p *PortfolioOptimizer) maxSharpeOptimization(assetData []*AssetData) (map[string]float64, OptimizationMetrics) {
	// 简化的最大夏普比率优化
	// 实际应该使用二次规划求解
	
	n := len(assetData)
	weights := make(map[string]float64)

	// 计算每个资产的夏普比率
	sharpeRatios := make([]float64, n)
	for i, asset := range assetData {
		if asset.Volatility > 0 {
			sharpeRatios[i] = (asset.MeanReturn - p.config.RiskFreeRate/252) / asset.Volatility
		} else {
			sharpeRatios[i] = 0
		}
	}

	// 按夏普比率排序，选择表现最好的资产
	// 简化实现：直接使用夏普比率作为权重
	var totalSharpe float64
	for _, sr := range sharpeRatios {
		if sr > 0 {
			totalSharpe += sr
		}
	}

	if totalSharpe > 0 {
		for i, asset := range assetData {
			if sharpeRatios[i] > 0 {
				weights[asset.Symbol] = sharpeRatios[i] / totalSharpe
			}
		}
	} else {
		// 如果所有夏普比率都为负，使用等权重
		equalWeight := 1.0 / float64(n)
		for _, asset := range assetData {
			weights[asset.Symbol] = equalWeight
		}
	}

	// 应用权重限制
	p.applyWeightConstraints(weights)

	metrics := p.calculatePortfolioMetrics(weights, assetData)

	return weights, metrics
}

// applyWeightConstraints 应用权重约束
func (p *PortfolioOptimizer) applyWeightConstraints(weights map[string]float64) {
	// 归一化权重
	var totalWeight float64
	for _, weight := range weights {
		totalWeight += weight
	}

	if totalWeight > 0 {
		for symbol := range weights {
			weights[symbol] /= totalWeight
		}
	}

	// 应用最小/最大权重限制
	for symbol := range weights {
		if weights[symbol] < p.config.MinWeight {
			weights[symbol] = p.config.MinWeight
		}
		if weights[symbol] > p.config.MaxWeight {
			weights[symbol] = p.config.MaxWeight
		}
	}

	// 重新归一化
	var adjustedTotal float64
	for _, weight := range weights {
		adjustedTotal += weight
	}

	if adjustedTotal > 0 {
		for symbol := range weights {
			weights[symbol] /= adjustedTotal
		}
	}
}

// calculatePortfolioMetrics 计算组合指标
func (p *PortfolioOptimizer) calculatePortfolioMetrics(weights map[string]float64, assetData []*AssetData) OptimizationMetrics {
	var expectedReturn float64
	var portfolioVariance float64

	// 计算预期收益率
	for symbol, weight := range weights {
		// 简化：直接使用历史平均收益率
		var assetReturn float64
		if assetData != nil {
			for _, asset := range assetData {
				if asset.Symbol == symbol {
					assetReturn = asset.MeanReturn
					break
				}
			}
		}
		expectedReturn += weight * assetReturn
	}

	// 简化的风险计算（忽略相关性）
	for symbol, weight := range weights {
		var assetVolatility float64
		if assetData != nil {
			for _, asset := range assetData {
				if asset.Symbol == symbol {
					assetVolatility = asset.Volatility
					break
				}
			}
		}
		portfolioVariance += (weight * assetVolatility) * (weight * assetVolatility)
	}

	risk := math.Sqrt(portfolioVariance)

	// 计算夏普比率
	var sharpeRatio float64
	if risk > 0 {
		sharpeRatio = (expectedReturn - p.config.RiskFreeRate/252) / risk
	}

	return OptimizationMetrics{
		ExpectedReturn: expectedReturn * 252, // 年化
		Risk:           risk * math.Sqrt(252), // 年化
		SharpeRatio:   sharpeRatio * math.Sqrt(252), // 年化
		MaxDrawdown:   p.estimateMaxDrawdown(weights, assetData),
		Alpha:         0.0, // 简化实现
		Beta:          1.0, // 简化实现
		InfoRatio:     0.0, // 简化实现
		TrackingError: risk, // 简化实现
	}
}

// estimateMaxDrawdown 估算最大回撤
func (p *PortfolioOptimizer) estimateMaxDrawdown(weights map[string]float64, assetData []*AssetData) float64 {
	// 简化实现：基于历史最大回撤的加权平均
	var maxDrawdown float64
	var totalWeight float64

	for symbol, weight := range weights {
		totalWeight += weight
		
		// 查找资产的历史最大回撤
		var assetMaxDD float64
		if assetData != nil {
			for _, asset := range assetData {
				if asset.Symbol == symbol {
					// 简化：基于收益率序列估算
					assetMaxDD = p.estimateAssetMaxDrawdown(asset.Returns)
					break
				}
			}
		}

		maxDrawdown += weight * assetMaxDD
	}

	return maxDrawdown
}

// estimateAssetMaxDrawdown 估算资产最大回撤
func (p *PortfolioOptimizer) estimateAssetMaxDrawdown(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	var maxDrawdown float64
	var peak float64
	var cumulative float64 = 1.0

	for _, ret := range returns {
		cumulative *= (1 + ret)
		
		if cumulative > peak {
			peak = cumulative
		}

		drawdown := (peak - cumulative) / peak
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}

	return maxDrawdown
}

// calculateMean 计算平均值
func (p *PortfolioOptimizer) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

// calculateVolatility 计算波动率
func (p *PortfolioOptimizer) calculateVolatility(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	mean := p.calculateMean(returns)
	var variance float64

	for _, ret := range returns {
		diff := ret - mean
		variance += diff * diff
	}

	variance /= float64(len(returns) - 1)
	return math.Sqrt(variance)
}

// calculateMin 计算最小值
func (p *PortfolioOptimizer) calculateMin(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	min := values[0]
	for _, value := range values {
		if value < min {
			min = value
		}
	}
	return min
}

// calculateMax 计算最大值
func (p *PortfolioOptimizer) calculateMax(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	max := values[0]
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

// generateOptimizationNotes 生成优化说明
func (p *PortfolioOptimizer) generateOptimizationNotes(metrics OptimizationMetrics) string {
	return fmt.Sprintf("优化完成，采用 %s 方法，预期年化收益 %.2f%%，年化风险 %.2f%%，夏普比率 %.3f", 
		p.config.Method, metrics.ExpectedReturn*100, metrics.Risk*100, metrics.SharpeRatio)
}

// GetCorrelationMatrix 计算相关性矩阵
func (p *PortfolioOptimizer) GetCorrelationMatrix(assetData []*AssetData) (*CorrelationMatrix, error) {
	n := len(assetData)
	if n == 0 {
		return nil, fmt.Errorf("no asset data provided")
	}

	// 初始化矩阵
	covariance := make([][]float64, n)
	correlation := make([][]float64, n)
	assets := make([]string, n)

	for i := 0; i < n; i++ {
		covariance[i] = make([]float64, n)
		correlation[i] = make([]float64, n)
		assets[i] = assetData[i].Symbol
	}

	// 计算协方差和相关性矩阵
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				covariance[i][j] = assetData[i].Volatility * assetData[i].Volatility
				correlation[i][j] = 1.0
			} else {
				// 简化：假设资产间相关性为0.3
				// 实际应该基于历史数据计算
				cov := 0.3 * assetData[i].Volatility * assetData[j].Volatility
				covariance[i][j] = cov
				correlation[i][j] = 0.3
			}
		}
	}

	return &CorrelationMatrix{
		Assets:      assets,
		Covariance:  covariance,
		Correlation: correlation,
	}, nil
}

// OptimizeWithConstraints 带约束的优化
func (p *PortfolioOptimizer) OptimizeWithConstraints(symbols []string, historicalData map[string][]float64, constraints map[string]float64) (*OptimizationResult, error) {
	// 先执行基础优化
	result, err := p.Optimize(symbols, historicalData)
	if err != nil {
		return nil, err
	}

	// 应用约束
	if constraints != nil {
		for symbol, minWeight := range constraints {
			if minWeight > 0 {
				// 简化：直接设置最小权重
				if result.Weights[symbol] < minWeight {
					result.Weights[symbol] = minWeight
				}
			}
		}
		
		// 重新归一化
		var totalWeight float64
		for _, weight := range result.Weights {
			totalWeight += weight
		}
		
		if totalWeight > 0 {
			for symbol := range result.Weights {
				result.Weights[symbol] /= totalWeight
			}
		}
	}

	return result, nil
}

// SetConfig 更新配置
func (p *PortfolioOptimizer) SetConfig(config OptimizerConfig) {
	p.config = &config
	log.Printf("Portfolio optimizer config updated: method=%s", config.Method)
}

// GetConfig 获取配置
func (p *PortfolioOptimizer) GetConfig() OptimizerConfig {
	return *p.config
}

// OptimizationMetrics 优化指标
type OptimizationMetrics struct {
	ExpectedReturn float64 `json:"expected_return"`
	Risk           float64 `json:"risk"`
	SharpeRatio   float64 `json:"sharpe_ratio"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	Alpha          float64 `json:"alpha"`
	Beta           float64 `json:"beta"`
	InfoRatio      float64 `json:"info_ratio"`
	TrackingError  float64 `json:"tracking_error"`
}
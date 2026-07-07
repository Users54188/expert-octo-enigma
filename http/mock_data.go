package http

import "time"

// MockDataProvider 模拟数据提供者
// 用于在未接入真实数据源时提供模拟数据
// TODO: 接入真实数据源后，替换为真实实现
type MockDataProvider struct{}

// NewMockDataProvider 创建模拟数据提供者
func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{}
}

// GetIndustryRotationData 获取行业轮动模拟数据
func (m *MockDataProvider) GetIndustryRotationData() map[string]float64 {
	return map[string]float64{
		"银行":     0.05,
		"食品饮料": 0.08,
		"医药生物": 0.03,
		"电力设备": -0.02,
		"电子":     0.12,
		"计算机":   0.06,
	}
}

// GetRiskMetrics 获取风险指标模拟数据
func (m *MockDataProvider) GetRiskMetrics() map[string]interface{} {
	return map[string]interface{}{
		"sharpe_ratio":  1.25,
		"sortino_ratio": 1.45,
		"max_drawdown":  0.08,
		"volatility":    0.12,
		"beta":          0.95,
		"alpha":         0.02,
		"var_95":        0.025,
		"var_99":        0.035,
		"timestamp":     time.Now(),
	}
}

// GetVaRData 获取VaR模拟数据
func (m *MockDataProvider) GetVaRData(confidence float64, method string) map[string]interface{} {
	return map[string]interface{}{
		"confidence": confidence,
		"method":     method,
		"var":        0.025,
		"cvar":       0.035,
		"timestamp":  time.Now(),
	}
}

// GetFactorExposure 获取因子暴露模拟数据
func (m *MockDataProvider) GetFactorExposure() map[string]interface{} {
	return map[string]interface{}{
		"market":     0.95,
		"size":       0.82,
		"value":      0.35,
		"momentum":   0.12,
		"quality":    0.25,
		"volatility": -0.15,
		"timestamp":  time.Now(),
	}
}

// GetRiskCurveData 获取风险曲线模拟数据
func (m *MockDataProvider) GetRiskCurveData(days int) []map[string]interface{} {
	curve := make([]map[string]interface{}, days)
	equity := 100000.0
	baseDate := time.Now().AddDate(0, 0, -days)

	for i := 0; i < days; i++ {
		change := (float64(i%10) - 5) / 1000
		equity = equity * (1 + change)

		curve[i] = map[string]interface{}{
			"date":         baseDate.AddDate(0, 0, i).Format("2006-01-02"),
			"equity":       equity,
			"daily_return": change,
			"drawdown":     float64(i%20) / 1000,
		}
	}
	return curve
}

// GetProviderStatus 获取数据源状态模拟数据
func (m *MockDataProvider) GetProviderStatus() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":       "sina",
			"healthy":    true,
			"latency":    150,
			"priority":   1,
			"last_check": time.Now(),
		},
		{
			"name":       "eastmoney",
			"healthy":    true,
			"latency":    200,
			"priority":   2,
			"last_check": time.Now(),
		},
		{
			"name":       "tencent",
			"healthy":    true,
			"latency":    180,
			"priority":   3,
			"last_check": time.Now(),
		},
	}
}

// GetMarketAnomalies 获取市场异常模拟数据
func (m *MockDataProvider) GetMarketAnomalies() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type":      "price_jump",
			"symbol":    "sh600519",
			"severity":  "medium",
			"message":   "价格跳变超过5%",
			"timestamp": time.Now().Add(-time.Hour),
		},
	}
}

// GetMarketQuality 获取数据质量模拟数据
func (m *MockDataProvider) GetMarketQuality() map[string]interface{} {
	return map[string]interface{}{
		"overall_score":  95,
		"latency_score":  90,
		"accuracy_score": 98,
		"coverage_score": 96,
		"timestamp":      time.Now(),
	}
}

// GetPortfolioReturns 获取投资组合收益率模拟数据
func (m *MockDataProvider) GetPortfolioReturns() map[string]float64 {
	return map[string]float64{
		"sh600000": 0.05,
		"sh601398": 0.03,
		"sh600519": 0.08,
		"sh600036": 0.04,
	}
}

// GetBenchmarkReturns 获取基准收益率模拟数据
func (m *MockDataProvider) GetBenchmarkReturns() map[string]float64 {
	return map[string]float64{
		"sh600000": 0.04,
		"sh601398": 0.02,
		"sh600519": 0.06,
		"sh600036": 0.03,
	}
}

// GetIndustryMapping 获取行业映射模拟数据
func (m *MockDataProvider) GetIndustryMapping() map[string]string {
	return map[string]string{
		"sh600000": "银行",
		"sh601398": "银行",
		"sh600519": "食品饮料",
		"sh600036": "银行",
	}
}

// GetCorrelationReturns 获取相关性分析模拟数据
func (m *MockDataProvider) GetCorrelationReturns() map[string][]float64 {
	return map[string][]float64{
		"银行":     {0.01, 0.02, -0.01, 0.015, 0.005},
		"食品饮料": {0.02, 0.03, 0.01, 0.02, 0.015},
		"医药生物": {0.015, 0.01, 0.02, 0.005, 0.01},
		"电力设备": {-0.01, 0.005, -0.02, 0.01, 0.005},
		"电子":     {0.03, 0.025, 0.035, 0.02, 0.03},
	}
}

// GetHeatmapData 获取热力图模拟数据
func (m *MockDataProvider) GetHeatmapData(symbols, periods []string) [][]float64 {
	data := make([][]float64, len(symbols))
	for i := range data {
		data[i] = make([]float64, len(periods))
		for j := range data[i] {
			data[i][j] = (float64((i+j)%20) - 10) / 100
		}
	}
	return data
}

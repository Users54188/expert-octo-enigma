// Package industry 提供行业数据分析计算
package industry

import (
	"math"
	"sort"
	"time"
)

// Analyzer 行业分析器
type Analyzer struct {
	cache *Cache
}

// NewAnalyzer 创建行业分析器
func NewAnalyzer(cache *Cache) *Analyzer {
	return &Analyzer{cache: cache}
}

// CalculateExposure 计算组合行业暴露
func (a *Analyzer) CalculateExposure(positions map[string]float64, benchmark string) *ExposureResponse {
	if a.cache == nil {
		return nil
	}

	industryWeights := make(map[string]float64)
	industrySymbols := make(map[string][]string)
	totalWeight := 0.0

	// 计算组合的行业权重
	for symbol, weight := range positions {
		if info, exists := a.cache.GetStockIndustry(symbol); exists {
			industryWeights[info.SWIndustry] += weight
			industrySymbols[info.SWIndustry] = append(industrySymbols[info.SWIndustry], symbol)
			totalWeight += weight
		}
	}

	// 获取基准权重
	benchmarkWeights := a.cache.GetBenchmarkWeights(benchmark)
	if benchmarkWeights == nil {
		benchmarkWeights = getDefaultBenchmarkWeights()
	}

	// 构建暴露分析结果
	var exposures []IndustryExposure
	var totalActiveShare float64

	for industry, weight := range industryWeights {
		normalizedWeight := weight
		if totalWeight > 0 {
			normalizedWeight = weight / totalWeight
		}

		benchmarkWeight := 0.0
		if bw, exists := benchmarkWeights[industry]; exists {
			benchmarkWeight = bw
		}

		activeShare := normalizedWeight - benchmarkWeight
		totalActiveShare += math.Abs(activeShare)

		exposure := IndustryExposure{
			Industry:        industry,
			Weight:          normalizedWeight,
			BenchmarkWeight: benchmarkWeight,
			ActiveShare:     activeShare,
			Symbols:         industrySymbols[industry],
			SectorReturns:   make(map[string]float64),
		}
		exposures = append(exposures, exposure)
	}

	// 添加基准中有但组合中没有的行业
	for industry, benchmarkWeight := range benchmarkWeights {
		if _, exists := industryWeights[industry]; !exists {
			exposure := IndustryExposure{
				Industry:        industry,
				Weight:          0,
				BenchmarkWeight: benchmarkWeight,
				ActiveShare:     -benchmarkWeight,
				Symbols:         []string{},
				SectorReturns:   make(map[string]float64),
			}
			exposures = append(exposures, exposure)
			totalActiveShare += benchmarkWeight
		}
	}

	// 按权重排序
	sort.Slice(exposures, func(i, j int) bool {
		return exposures[i].Weight > exposures[j].Weight
	})

	// 提取前N个行业
	var topIndustries []string
	for i, exp := range exposures {
		if i >= 5 {
			break
		}
		topIndustries = append(topIndustries, exp.Industry)
	}

	return &ExposureResponse{
		Exposures:        exposures,
		TotalActiveShare: totalActiveShare,
		TopIndustries:    topIndustries,
	}
}

// DetectSectorRotation 检测板块轮动
func (a *Analyzer) DetectSectorRotation(returns map[string]float64, lookbackDays int, threshold float64) []SectorRotation {
	if len(returns) < 2 {
		return nil
	}

	// 按收益率排序
	type industryReturn struct {
		industry string
		returns  float64
	}

	var sorted []industryReturn
	for industry, ret := range returns {
		sorted = append(sorted, industryReturn{industry, ret})
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].returns > sorted[j].returns
	})

	var rotations []SectorRotation
	now := time.Now()

	// 检测轮动脉冲
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			strength := sorted[i].returns - sorted[j].returns
			if strength > threshold {
				rotation := SectorRotation{
					FromSector:    sorted[j].industry,
					ToSector:      sorted[i].industry,
					Strength:      strength,
					MomentumScore: calculateMomentumScore(sorted[i].returns, sorted[j].returns),
					Timestamp:     now,
				}
				rotations = append(rotations, rotation)
			}
		}
	}

	// 按轮动强度排序
	sort.Slice(rotations, func(i, j int) bool {
		return rotations[i].Strength > rotations[j].Strength
	})

	return rotations
}

// CalculateCorrelationMatrix 计算行业相关性矩阵
func (a *Analyzer) CalculateCorrelationMatrix(returns map[string][]float64, industries []string) *CorrelationResponse {
	if len(industries) == 0 {
		// 使用所有有数据的行业
		for industry := range returns {
			industries = append(industries, industry)
		}
	}

	n := len(industries)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
	}

	// 计算相关性
	for i, ind1 := range industries {
		matrix[i][i] = 1.0
		for j := i + 1; j < n; j++ {
			ind2 := industries[j]
			ret1, ok1 := returns[ind1]
			ret2, ok2 := returns[ind2]

			if !ok1 || !ok2 || len(ret1) == 0 || len(ret2) == 0 {
				matrix[i][j] = 0
				matrix[j][i] = 0
				continue
			}

			corr := calculateCorrelation(ret1, ret2)
			matrix[i][j] = corr
			matrix[j][i] = corr
		}
	}

	return &CorrelationResponse{
		Matrix:     matrix,
		Industries: industries,
		Period:     "1m",
		Timestamp:  time.Now(),
	}
}

// GetIndustryPerformance 获取行业表现
func (a *Analyzer) GetIndustryPerformance(industry string, periods []string) *IndustryPerformance {
	// 模拟数据，实际应从数据库或API获取
	now := time.Now()

	// 这里使用模拟数据，实际实现应查询历史数据
	performance := &IndustryPerformance{
		Industry:  industry,
		Timestamp: now,
	}

	// 根据行业名称生成一些模拟数据（实际应查询真实数据）
	industryHash := hashString(industry)
	performance.Return1D = (float64(industryHash%100) - 50) / 1000
	performance.Return1W = (float64(industryHash%200) - 100) / 1000
	performance.Return1M = (float64(industryHash%500) - 250) / 1000
	performance.Return3M = (float64(industryHash%1000) - 500) / 1000
	performance.ReturnYTD = (float64(industryHash%2000) - 1000) / 1000
	performance.Volatility = 0.15 + float64(industryHash%50)/1000
	performance.Turnover = 0.02 + float64(industryHash%30)/1000
	performance.PeRatio = 15 + float64(industryHash%20)
	performance.PbRatio = 1.5 + float64(industryHash%30)/10

	return performance
}

// calculateMomentumScore 计算动量评分
func calculateMomentumScore(fromReturn, toReturn float64) float64 {
	// 简化的动量评分计算
	return (toReturn - fromReturn) * 100
}

// calculateCorrelation 计算皮尔逊相关系数
func calculateCorrelation(x, y []float64) float64 {
	minLen := len(x)
	if len(y) < minLen {
		minLen = len(y)
	}

	if minLen == 0 {
		return 0
	}

	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0

	for i := 0; i < minLen; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	n := float64(minLen)
	numerator := sumXY - (sumX*sumY)/n
	denominator := math.Sqrt((sumX2 - (sumX*sumX)/n) * (sumY2 - (sumY*sumY)/n))

	if denominator == 0 {
		return 0
	}

	return numerator / denominator
}

// hashString 简单的字符串哈希
func hashString(s string) int {
	hash := 0
	for _, c := range s {
		hash = 31*hash + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// getDefaultBenchmarkWeights 获取默认基准权重
func getDefaultBenchmarkWeights() map[string]float64 {
	return map[string]float64{
		"银行":   0.13,
		"非银金融": 0.08,
		"食品饮料": 0.09,
		"医药生物": 0.07,
		"电力设备": 0.08,
		"电子":   0.11,
		"计算机":  0.04,
		"通信":   0.02,
		"传媒":   0.03,
		"家用电器": 0.04,
		"汽车":   0.05,
		"机械设备": 0.05,
		"基础化工": 0.04,
		"有色金属": 0.04,
		"石油石化": 0.04,
		"煤炭":   0.02,
		"钢铁":   0.02,
		"建筑材料": 0.02,
		"建筑装饰": 0.01,
		"房地产":  0.02,
		"公用事业": 0.03,
		"商贸零售": 0.02,
		"社会服务": 0.02,
		"农林牧渔": 0.01,
		"纺织服饰": 0.01,
		"轻工制造": 0.01,
		"美容护理": 0.01,
		"环保":   0.01,
		"交通运输": 0.03,
		"国防军工": 0.02,
	}
}

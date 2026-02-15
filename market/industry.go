package market

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

var (
	industryCache     *IndustryCache
	industryCacheOnce sync.Once
)

// IndustryInfo 股票行业信息
type IndustryInfo struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Industry  string `json:"industry"`
	Sector    string `json:"sector"`
	MarketCap string `json:"market_cap"`
}

// IndustryExposure 行业暴露分析
type IndustryExposure struct {
	Industry    string   `json:"industry"`
	Weight      float64  `json:"weight"`
	Benchmark   float64  `json:"benchmark"`
	ActiveShare float64  `json:"active_share"`
	Symbols     []string `json:"symbols"`
}

// IndustryMapping 行业映射数据结构
type IndustryMapping struct {
	Description  string         `json:"description"`
	LastUpdated  string         `json:"last_updated"`
	Data         []IndustryInfo `json:"data"`
	IndustryList []string       `json:"industry_list"`
}

// IndustryCache 行业信息缓存
type IndustryCache struct {
	mapping   *IndustryMapping
	symbolMap map[string]*IndustryInfo
	lastLoad  time.Time
	mu        sync.RWMutex
	filePath  string
}

// LoadIndustryMapping 从文件加载行业映射数据
func LoadIndustryMapping(filePath string) (*IndustryMapping, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var mapping IndustryMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return nil, err
	}

	return &mapping, nil
}

// GetIndustryCache 获取行业信息缓存（单例）
func GetIndustryCache() (*IndustryCache, error) {
	var initErr error
	industryCacheOnce.Do(func() {
		industryCache = &IndustryCache{
			symbolMap: make(map[string]*IndustryInfo),
			filePath:  "./data/industry_mapping.json",
		}
		initErr = industryCache.Reload()
	})
	return industryCache, initErr
}

// Reload 重新加载行业映射数据
func (ic *IndustryCache) Reload() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	mapping, err := LoadIndustryMapping(ic.filePath)
	if err != nil {
		return err
	}

	ic.mapping = mapping
	ic.symbolMap = make(map[string]*IndustryInfo)
	for i := range mapping.Data {
		ic.symbolMap[mapping.Data[i].Symbol] = &mapping.Data[i]
	}
	ic.lastLoad = time.Now()

	return nil
}

// GetStockIndustry 获取股票的行业信息
func (ic *IndustryCache) GetStockIndustry(symbol string) (*IndustryInfo, bool) {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	info, exists := ic.symbolMap[symbol]
	return info, exists
}

// GetAllStocks 获取所有股票的行业信息
func (ic *IndustryCache) GetAllStocks() []IndustryInfo {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if ic.mapping == nil {
		return nil
	}

	result := make([]IndustryInfo, len(ic.mapping.Data))
	copy(result, ic.mapping.Data)
	return result
}

// GetIndustryList 获取所有行业列表
func (ic *IndustryCache) GetIndustryList() []string {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if ic.mapping == nil {
		return nil
	}

	result := make([]string, len(ic.mapping.IndustryList))
	copy(result, ic.mapping.IndustryList)
	return result
}

// GetStocksByIndustry 获取指定行业的所有股票
func (ic *IndustryCache) GetStocksByIndustry(industry string) []IndustryInfo {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if ic.mapping == nil {
		return nil
	}

	var result []IndustryInfo
	for _, stock := range ic.mapping.Data {
		if stock.Industry == industry {
			result = append(result, stock)
		}
	}
	return result
}

// GetStocksBySector 获取指定板块的所有股票
func (ic *IndustryCache) GetStocksBySector(sector string) []IndustryInfo {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if ic.mapping == nil {
		return nil
	}

	var result []IndustryInfo
	for _, stock := range ic.mapping.Data {
		if stock.Sector == sector {
			result = append(result, stock)
		}
	}
	return result
}

// GetStocksByMarketCap 获取指定市值分类的所有股票
func (ic *IndustryCache) GetStocksByMarketCap(marketCap string) []IndustryInfo {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	if ic.mapping == nil {
		return nil
	}

	var result []IndustryInfo
	for _, stock := range ic.mapping.Data {
		if stock.MarketCap == marketCap {
			result = append(result, stock)
		}
	}
	return result
}

// CalculateIndustryExposure 计算组合行业暴露
func CalculateIndustryExposure(positions map[string]float64, benchmark map[string]float64) []IndustryExposure {
	cache, err := GetIndustryCache()
	if err != nil {
		return nil
	}

	industryWeights := make(map[string]float64)
	industrySymbols := make(map[string][]string)
	totalWeight := 0.0

	for symbol, weight := range positions {
		if info, exists := cache.GetStockIndustry(symbol); exists {
			industryWeights[info.Industry] += weight
			industrySymbols[info.Industry] = append(industrySymbols[info.Industry], symbol)
			totalWeight += weight
		}
	}

	var result []IndustryExposure
	for industry, weight := range industryWeights {
		normalizedWeight := weight
		if totalWeight > 0 {
			normalizedWeight = weight / totalWeight
		}

		benchmarkWeight := 0.0
		if bw, exists := benchmark[industry]; exists {
			benchmarkWeight = bw
		}

		exposure := IndustryExposure{
			Industry:    industry,
			Weight:      normalizedWeight,
			Benchmark:   benchmarkWeight,
			ActiveShare: normalizedWeight - benchmarkWeight,
			Symbols:     industrySymbols[industry],
		}
		result = append(result, exposure)
	}

	return result
}

// GetBenchmarkWeights 获取基准权重（示例：沪深300行业权重）
func GetBenchmarkWeights() map[string]float64 {
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

// GetStockIndustry 获取股票行业信息（便捷函数）
func GetStockIndustry(symbol string) (*IndustryInfo, bool) {
	cache, err := GetIndustryCache()
	if err != nil {
		return nil, false
	}
	return cache.GetStockIndustry(symbol)
}

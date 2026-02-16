// Package industry 提供行业数据分析功能
package industry

import (
	"time"
)

// IndustryInfo 股票行业信息 - 完整数据结构
type IndustryInfo struct {
	Symbol       string `json:"symbol"`        // 股票代码
	Name         string `json:"name"`          // 股票名称
	SWIndustry   string `json:"sw_industry"`   // 申万一级行业
	SWSector     string `json:"sw_sector"`     // 申万二级行业
	CSRCIndustry string `json:"csrc_industry"` // 证监会行业
	Market       string `json:"market"`        // 市场板块
	MarketCap    string `json:"market_cap"`    // 市值分类
	ListDate     string `json:"list_date"`     // 上市日期
}

// IndustryExposure 行业暴露分析
type IndustryExposure struct {
	Industry        string             `json:"industry"`         // 行业名称
	Weight          float64            `json:"weight"`           // 组合权重
	BenchmarkWeight float64            `json:"benchmark_weight"` // 基准权重(沪深300)
	ActiveShare     float64            `json:"active_share"`     // 主动偏离
	Symbols         []string           `json:"symbols"`          // 包含股票
	SectorReturns   map[string]float64 `json:"sector_returns"`   // 行业收益贡献
}

// SectorRotation 板块轮动检测
type SectorRotation struct {
	FromSector    string    `json:"from_sector"`    // 转出板块
	ToSector      string    `json:"to_sector"`      // 转入板块
	Strength      float64   `json:"strength"`       // 轮动强度
	MomentumScore float64   `json:"momentum_score"` // 动量评分
	Timestamp     time.Time `json:"timestamp"`
}

// IndustryCorrelation 行业相关性
type IndustryCorrelation struct {
	Industry1   string  `json:"industry1"`
	Industry2   string  `json:"industry2"`
	Correlation float64 `json:"correlation"`
	Period      string  `json:"period"`
}

// IndustryPerformance 行业表现
type IndustryPerformance struct {
	Industry   string    `json:"industry"`
	Return1D   float64   `json:"return_1d"`
	Return1W   float64   `json:"return_1w"`
	Return1M   float64   `json:"return_1m"`
	Return3M   float64   `json:"return_3m"`
	ReturnYTD  float64   `json:"return_ytd"`
	Volatility float64   `json:"volatility"`
	Turnover   float64   `json:"turnover"`
	PeRatio    float64   `json:"pe_ratio"`
	PbRatio    float64   `json:"pb_ratio"`
	Timestamp  time.Time `json:"timestamp"`
}

// IndustryMapping 行业映射数据结构
type IndustryMapping struct {
	Description      string                        `json:"description"`
	LastUpdated      string                        `json:"last_updated"`
	Data             []IndustryInfo                `json:"data"`
	IndustryList     []string                      `json:"industry_list"`
	BenchmarkWeights map[string]map[string]float64 `json:"benchmark_weights"`
}

// ExposureRequest 暴露分析请求
type ExposureRequest struct {
	Positions map[string]float64 `json:"positions"` // 股票代码->权重
	Benchmark string             `json:"benchmark"` // 基准指数
}

// ExposureResponse 暴露分析响应
type ExposureResponse struct {
	Exposures        []IndustryExposure `json:"exposures"`
	TotalActiveShare float64            `json:"total_active_share"`
	TopIndustries    []string           `json:"top_industries"`
}

// RotationRequest 轮动检测请求
type RotationRequest struct {
	LookbackDays int     `json:"lookback_days"`
	Threshold    float64 `json:"threshold"`
}

// CorrelationRequest 相关性请求
type CorrelationRequest struct {
	Period     string   `json:"period"` // 1m, 3m, 6m, 1y
	Industries []string `json:"industries,omitempty"`
}

// CorrelationResponse 相关性响应
type CorrelationResponse struct {
	Matrix     [][]float64 `json:"matrix"`
	Industries []string    `json:"industries"`
	Period     string      `json:"period"`
	Timestamp  time.Time   `json:"timestamp"`
}

package risk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading"
)

// PortfolioRisk 组合风险暴露管理
type PortfolioRisk struct {
	mu               sync.RWMutex
	config           *PortfolioRiskConfig
	positionManager  *trading.PositionManager
	industryExposure map[string]float64 // 行业暴露
	sectorExposure   map[string]float64 // 板块暴露
	symbolExposure   map[string]float64 // 个股暴露
	lastUpdate       time.Time
}

// PortfolioRiskConfig 组合风险配置
type PortfolioRiskConfig struct {
	MaxIndustryExposure float64 `yaml:"max_industry_exposure"` // 单一行业最大暴露比例
	MaxSectorExposure   float64 `yaml:"max_sector_exposure"`   // 单一板块最大暴露比例
	MaxSymbolExposure   float64 `yaml:"max_symbol_exposure"`   // 单一股票最大暴露比例
	ConcentrationAlert  float64 `yaml:"concentration_alert"`   // 集中度告警阈值
}

// NewPortfolioRisk 创建组合风险管理器
func NewPortfolioRisk(config PortfolioRiskConfig, positionManager *trading.PositionManager) *PortfolioRisk {
	return &PortfolioRisk{
		config:           &config,
		positionManager:  positionManager,
		industryExposure: make(map[string]float64),
		sectorExposure:   make(map[string]float64),
		symbolExposure:   make(map[string]float64),
	}
}

// CheckExposure 检查风险暴露
func (p *PortfolioRisk) CheckExposure(ctx context.Context) (*ExposureReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.positionManager == nil {
		return nil, fmt.Errorf("position manager not configured")
	}

	// 获取当前持仓
	positions := p.positionManager.GetAllPositions()

	// 计算总资产
	totalValue := 0.0
	for _, pos := range positions {
		totalValue += pos.MarketValue
	}

	if totalValue == 0 {
		return &ExposureReport{
			TotalValue:       0,
			IndustryExposure: p.industryExposure,
			SectorExposure:   p.sectorExposure,
			SymbolExposure:   p.symbolExposure,
		}, nil
	}

	// 更新暴露数据
	p.updateExposure(positions, totalValue)
	p.lastUpdate = time.Now()

	// 生成报告
	report := &ExposureReport{
		TotalValue:       totalValue,
		IndustryExposure: make(map[string]float64),
		SectorExposure:   make(map[string]float64),
		SymbolExposure:   make(map[string]float64),
		Alerts:           make([]ExposureAlert, 0),
		Timestamp:        p.lastUpdate,
	}

	// 复制数据
	for k, v := range p.industryExposure {
		report.IndustryExposure[k] = v
	}
	for k, v := range p.sectorExposure {
		report.SectorExposure[k] = v
	}
	for k, v := range p.symbolExposure {
		report.SymbolExposure[k] = v
	}

	// 检查告警
	report.Alerts = p.checkExposureAlerts(report)

	return report, nil
}

// updateExposure 更新暴露数据
func (p *PortfolioRisk) updateExposure(positions []*trading.PositionState, totalValue float64) {
	// 重置暴露数据
	p.industryExposure = make(map[string]float64)
	p.sectorExposure = make(map[string]float64)
	p.symbolExposure = make(map[string]float64)

	for _, pos := range positions {
		exposure := pos.MarketValue / totalValue

		// 个股暴露
		p.symbolExposure[pos.Symbol] = exposure

		// 行业暴露（简化：基于股票代码前缀）
		industry := p.getIndustryFromSymbol(pos.Symbol)
		p.industryExposure[industry] += exposure

		// 板块暴露（简化：基于股票代码前缀）
		sector := p.getSectorFromSymbol(pos.Symbol)
		p.sectorExposure[sector] += exposure
	}
}

// getIndustryFromSymbol 从股票代码获取行业（简化实现）
func (p *PortfolioRisk) getIndustryFromSymbol(symbol string) string {
	// 简化的行业分类逻辑
	// 实际应用中应该使用真实的行业分类数据
	if len(symbol) >= 6 {
		prefix := symbol[:3]
		switch prefix {
		case "600", "601", "603", "605":
			return "主板"
		case "000", "002", "003":
			return "中小板"
		case "300":
			return "创业板"
		case "688":
			return "科创板"
		default:
			return "其他"
		}
	}
	return "未知"
}

// getSectorFromSymbol 从股票代码获取板块（简化实现）
func (p *PortfolioRisk) getSectorFromSymbol(symbol string) string {
	// 简化的板块分类逻辑
	// 实际应用中应该使用真实的板块分类数据
	if len(symbol) >= 6 {
		prefix := symbol[:3]
		switch prefix {
		case "600":
			return "沪市主板"
		case "601", "603":
			return "沪市A股"
		case "000":
			return "深市主板"
		case "002":
			return "中小板"
		case "300":
			return "创业板"
		case "688":
			return "科创板"
		default:
			return "其他"
		}
	}
	return "未知板块"
}

// checkExposureAlerts 检查暴露告警
func (p *PortfolioRisk) checkExposureAlerts(report *ExposureReport) []ExposureAlert {
	var alerts []ExposureAlert

	// 检查行业集中度
	for industry, exposure := range report.IndustryExposure {
		if exposure > p.config.MaxIndustryExposure {
			alerts = append(alerts, ExposureAlert{
				Type:      "industry_concentration",
				Level:     "warning",
				Message:   fmt.Sprintf("行业暴露过高: %s (%.2f%%)", industry, exposure*100),
				Value:     exposure,
				Threshold: p.config.MaxIndustryExposure,
				Symbol:    "",
			})
		} else if exposure > p.config.ConcentrationAlert {
			alerts = append(alerts, ExposureAlert{
				Type:      "industry_concentration",
				Level:     "info",
				Message:   fmt.Sprintf("行业暴露告警: %s (%.2f%%)", industry, exposure*100),
				Value:     exposure,
				Threshold: p.config.ConcentrationAlert,
				Symbol:    "",
			})
		}
	}

	// 检查板块集中度
	for sector, exposure := range report.SectorExposure {
		if exposure > p.config.MaxSectorExposure {
			alerts = append(alerts, ExposureAlert{
				Type:      "sector_concentration",
				Level:     "warning",
				Message:   fmt.Sprintf("板块暴露过高: %s (%.2f%%)", sector, exposure*100),
				Value:     exposure,
				Threshold: p.config.MaxSectorExposure,
				Symbol:    "",
			})
		}
	}

	// 检查个股集中度
	for symbol, exposure := range report.SymbolExposure {
		if exposure > p.config.MaxSymbolExposure {
			alerts = append(alerts, ExposureAlert{
				Type:      "symbol_concentration",
				Level:     "error",
				Message:   fmt.Sprintf("个股暴露过高: %s (%.2f%%)", symbol, exposure*100),
				Value:     exposure,
				Threshold: p.config.MaxSymbolExposure,
				Symbol:    symbol,
			})
		}
	}

	return alerts
}

// GetCurrentExposure 获取当前暴露数据
func (p *PortfolioRisk) GetCurrentExposure() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"industry_exposure": p.industryExposure,
		"sector_exposure":   p.sectorExposure,
		"symbol_exposure":   p.symbolExposure,
		"last_update":       p.lastUpdate,
	}
}

// SetConfig 更新配置
func (p *PortfolioRisk) SetConfig(config PortfolioRiskConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = &config
	log.Printf("Portfolio risk config updated: max_industry=%.2f, max_symbol=%.2f",
		config.MaxIndustryExposure, config.MaxSymbolExposure)
}

// ExposureReport 暴露报告
type ExposureReport struct {
	TotalValue       float64            `json:"total_value"`
	IndustryExposure map[string]float64 `json:"industry_exposure"`
	SectorExposure   map[string]float64 `json:"sector_exposure"`
	SymbolExposure   map[string]float64 `json:"symbol_exposure"`
	Alerts           []ExposureAlert    `json:"alerts"`
	Timestamp        time.Time          `json:"timestamp"`
}

// ExposureAlert 暴露告警
type ExposureAlert struct {
	Type      string  `json:"type"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Symbol    string  `json:"symbol,omitempty"`
}

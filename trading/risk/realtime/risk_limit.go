package realtime

import (
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading"
)

// RiskLimitManager 风险限额管理器
type RiskLimitManager struct {
	limits     map[string]*RiskLimit
	limitsLock sync.RWMutex

	positionManager *trading.PositionManager
	riskManager     *trading.RiskManager

	enableAutoAdjust bool
	adjustFactor     float64
}

// LimitConfig 限额配置
type LimitConfig struct {
	Name              string  `json:"name"`
	Type              string  `json:"type"`
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
	EnableAutoAdjust  bool    `json:"enable_auto_adjust"`
	AdjustFactor      float64 `json:"adjust_factor"`
}

// NewRiskLimitManager 创建风险限额管理器
func NewRiskLimitManager(positionManager *trading.PositionManager, riskManager *trading.RiskManager) *RiskLimitManager {
	manager := &RiskLimitManager{
		limits:           make(map[string]*RiskLimit),
		positionManager:  positionManager,
		riskManager:      riskManager,
		enableAutoAdjust: true,
		adjustFactor:     0.1,
	}

	// 初始化默认限额
	manager.initDefaultLimits()

	return manager
}

// initDefaultLimits 初始化默认限额
func (m *RiskLimitManager) initDefaultLimits() {
	defaultLimits := []LimitConfig{
		{
			Name:              "max_single_position",
			Type:              "position",
			WarningThreshold:  0.25,
			CriticalThreshold: 0.30,
			EnableAutoAdjust:  true,
			AdjustFactor:      0.05,
		},
		{
			Name:              "max_total_positions",
			Type:              "position",
			WarningThreshold:  8.0,
			CriticalThreshold: 10.0,
			EnableAutoAdjust:  false,
		},
		{
			Name:              "max_drawdown",
			Type:              "portfolio",
			WarningThreshold:  0.15,
			CriticalThreshold: 0.20,
			EnableAutoAdjust:  true,
			AdjustFactor:      0.02,
		},
		{
			Name:              "daily_loss_limit",
			Type:              "portfolio",
			WarningThreshold:  0.08,
			CriticalThreshold: 0.10,
			EnableAutoAdjust:  false,
		},
		{
			Name:              "volatility_limit",
			Type:              "volatility",
			WarningThreshold:  0.25,
			CriticalThreshold: 0.30,
			EnableAutoAdjust:  true,
			AdjustFactor:      0.05,
		},
		{
			Name:              "concentration_limit",
			Type:              "position",
			WarningThreshold:  0.35,
			CriticalThreshold: 0.40,
			EnableAutoAdjust:  true,
			AdjustFactor:      0.05,
		},
	}

	for _, config := range defaultLimits {
		m.AddLimit(config)
	}
}

// AddLimit 添加限额
func (m *RiskLimitManager) AddLimit(config LimitConfig) error {
	m.limitsLock.Lock()
	defer m.limitsLock.Unlock()

	if _, ok := m.limits[config.Name]; ok {
		return fmt.Errorf("limit %s already exists", config.Name)
	}

	limit := &RiskLimit{
		Name:              config.Name,
		Type:              config.Type,
		WarningThreshold:  config.WarningThreshold,
		CriticalThreshold: config.CriticalThreshold,
		CurrentValue:      0.0,
	}

	m.limits[config.Name] = limit
	log.Printf("Added risk limit: %s (warning: %.2f, critical: %.2f)", config.Name, config.WarningThreshold, config.CriticalThreshold)

	return nil
}

// UpdateLimit 更新限额
func (m *RiskLimitManager) UpdateLimit(name string, warningThreshold, criticalThreshold float64) error {
	m.limitsLock.Lock()
	defer m.limitsLock.Unlock()

	limit, ok := m.limits[name]
	if !ok {
		return fmt.Errorf("limit %s not found", name)
	}

	oldWarning := limit.WarningThreshold
	oldCritical := limit.CriticalThreshold

	limit.WarningThreshold = warningThreshold
	limit.CriticalThreshold = criticalThreshold

	log.Printf("Updated risk limit %s: (%.2f, %.2f) -> (%.2f, %.2f)",
		name, oldWarning, oldCritical, warningThreshold, criticalThreshold)

	return nil
}

// RemoveLimit 删除限额
func (m *RiskLimitManager) RemoveLimit(name string) error {
	m.limitsLock.Lock()
	defer m.limitsLock.Unlock()

	if _, ok := m.limits[name]; !ok {
		return fmt.Errorf("limit %s not found", name)
	}

	delete(m.limits, name)
	log.Printf("Removed risk limit: %s", name)

	return nil
}

// GetLimit 获取限额
func (m *RiskLimitManager) GetLimit(name string) (*RiskLimit, error) {
	m.limitsLock.RLock()
	defer m.limitsLock.RUnlock()

	limit, ok := m.limits[name]
	if !ok {
		return nil, fmt.Errorf("limit %s not found", name)
	}

	limitCopy := *limit
	return &limitCopy, nil
}

// GetAllLimits 获取所有限额
func (m *RiskLimitManager) GetAllLimits() map[string]*RiskLimit {
	m.limitsLock.RLock()
	defer m.limitsLock.RUnlock()

	result := make(map[string]*RiskLimit)
	for k, v := range m.limits {
		limit := *v
		result[k] = &limit
	}
	return result
}

// UpdateCurrentValue 更新当前值
func (m *RiskLimitManager) UpdateCurrentValue(name string, value float64) error {
	m.limitsLock.Lock()
	defer m.limitsLock.Unlock()

	limit, ok := m.limits[name]
	if !ok {
		return fmt.Errorf("limit %s not found", name)
	}

	limit.CurrentValue = value
	return nil
}

// CheckViolations 检查违规
func (m *RiskLimitManager) CheckViolations() []RiskEvent {
	m.limitsLock.RLock()
	defer m.limitsLock.RUnlock()

	var violations []RiskEvent

	for name, limit := range m.limits {
		if limit.CurrentValue >= limit.CriticalThreshold {
			violations = append(violations, RiskEvent{
				ID:        generateEventID(),
				Type:      limit.Type + "_violation",
				Level:     RiskLevelCritical,
				Message:   fmt.Sprintf("Critical violation of limit %s: %.2f >= %.2f", name, limit.CurrentValue, limit.CriticalThreshold),
				Value:     limit.CurrentValue,
				Threshold: limit.CriticalThreshold,
				Timestamp: time.Now(),
				Metadata: map[string]string{
					"limit_name": name,
					"limit_type": limit.Type,
				},
			})
		} else if limit.CurrentValue >= limit.WarningThreshold {
			violations = append(violations, RiskEvent{
				ID:        generateEventID(),
				Type:      limit.Type + "_warning",
				Level:     RiskLevelHigh,
				Message:   fmt.Sprintf("Warning violation of limit %s: %.2f >= %.2f", name, limit.CurrentValue, limit.WarningThreshold),
				Value:     limit.CurrentValue,
				Threshold: limit.WarningThreshold,
				Timestamp: time.Now(),
				Metadata: map[string]string{
					"limit_name": name,
					"limit_type": limit.Type,
				},
			})
		}
	}

	return violations
}

// AutoAdjustThresholds 自动调整阈值
func (m *RiskLimitManager) AutoAdjustThresholds() {
	if !m.enableAutoAdjust {
		return
	}

	m.limitsLock.Lock()
	defer m.limitsLock.Unlock()

	for _, limit := range m.limits {
		// 如果当前值持续接近阈值，适当放宽阈值
		if limit.CurrentValue > limit.WarningThreshold*0.9 {
			newWarning := limit.WarningThreshold * (1 + m.adjustFactor)
			newCritical := limit.CriticalThreshold * (1 + m.adjustFactor)

			log.Printf("Auto-adjusting limit %s: (%.2f, %.2f) -> (%.2f, %.2f)",
				limit.Name, limit.WarningThreshold, limit.CriticalThreshold, newWarning, newCritical)

			limit.WarningThreshold = newWarning
			limit.CriticalThreshold = newCritical
		}
	}
}

// GetCurrentRiskLevel 获取当前风险级别
func (m *RiskLimitManager) GetCurrentRiskLevel() RiskLevel {
	violations := m.CheckViolations()

	for _, v := range violations {
		if v.Level == RiskLevelCritical {
			return RiskLevelCritical
		}
	}

	for _, v := range violations {
		if v.Level == RiskLevelHigh {
			return RiskLevelHigh
		}
	}

	if len(violations) > 0 {
		return RiskLevelMedium
	}

	return RiskLevelLow
}

// GetRiskSummary 获取风险摘要
func (m *RiskLimitManager) GetRiskSummary() map[string]interface{} {
	m.limitsLock.RLock()
	defer m.limitsLock.RUnlock()

	violations := m.CheckViolations()

	summary := map[string]interface{}{
		"total_limits":        len(m.limits),
		"total_violations":    len(violations),
		"critical_violations": 0,
		"warning_violations":  0,
		"risk_level":          m.GetCurrentRiskLevel().String(),
		"timestamp":           time.Now(),
	}

	for _, v := range violations {
		if v.Level == RiskLevelCritical {
			summary["critical_violations"] = summary["critical_violations"].(int) + 1
		} else if v.Level == RiskLevelHigh {
			summary["warning_violations"] = summary["warning_violations"].(int) + 1
		}
	}

	return summary
}

package providers

import (
	"context"
	"log"
	"sync"
	"time"
)

// DataProvider 数据提供者接口
type DataProvider interface {
	Name() string
	FetchTick(ctx context.Context, symbol string) (*Tick, error)
	FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error)
	HealthCheck() error
	Priority() int
}

// Tick 实时行情数据
type Tick struct {
	Symbol    string
	Name      string
	Price     float64
	Bid       float64
	Ask       float64
	Volume    int64
	Turnover  float64
	High      float64
	Low       float64
	Open      float64
	PreClose  float64
	Time      time.Time
	Change    float64
	ChangePct float64
}

// KLine K线数据
type KLine struct {
	Symbol    string
	Date      time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Turnover  float64
	Change    float64
	ChangePct float64
}

// ProviderManager 数据源管理器
type ProviderManager struct {
	providers           []DataProvider
	primary             DataProvider
	health              map[string]bool
	healthMu            sync.RWMutex
	healthCheckInterval time.Duration
	stopChan            chan struct{}
	mu                  sync.RWMutex
}

// NewProviderManager 创建数据源管理器
func NewProviderManager() *ProviderManager {
	return &ProviderManager{
		providers:           make([]DataProvider, 0),
		health:              make(map[string]bool),
		healthCheckInterval: 30 * time.Second,
		stopChan:            make(chan struct{}),
	}
}

// AddProvider 添加数据提供者
func (pm *ProviderManager) AddProvider(provider DataProvider) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.providers = append(pm.providers, provider)
	pm.health[provider.Name()] = true

	if pm.primary == nil || provider.Priority() > pm.primary.Priority() {
		pm.primary = provider
	}
}

// SetPrimaryProvider 设置主要数据提供者
func (pm *ProviderManager) SetPrimaryProvider(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, provider := range pm.providers {
		if provider.Name() == name {
			pm.primary = provider
			return nil
		}
	}

	return ErrProviderNotFound
}

// FetchTick 获取实时行情（自动切换数据源）
func (pm *ProviderManager) FetchTick(ctx context.Context, symbol string) (*Tick, error) {
	pm.mu.RLock()
	providers := make([]DataProvider, len(pm.providers))
	copy(providers, pm.providers)
	primary := pm.primary
	pm.mu.RUnlock()

	if primary != nil {
		tick, err := primary.FetchTick(ctx, symbol)
		if err == nil {
			return tick, nil
		}
		log.Printf("Primary provider %s failed: %v, trying fallback providers", primary.Name(), err)
	}

	for _, provider := range providers {
		if provider == primary {
			continue
		}

		if pm.isHealthy(provider.Name()) {
			tick, err := provider.FetchTick(ctx, symbol)
			if err == nil {
				log.Printf("Using fallback provider %s for %s", provider.Name(), symbol)
				return tick, nil
			}
			log.Printf("Provider %s failed: %v", provider.Name(), err)
		}
	}

	return nil, ErrAllProvidersFailed
}

// FetchKLines 获取K线数据（自动切换数据源）
func (pm *ProviderManager) FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error) {
	pm.mu.RLock()
	providers := make([]DataProvider, len(pm.providers))
	copy(providers, pm.providers)
	primary := pm.primary
	pm.mu.RUnlock()

	if primary != nil {
		klines, err := primary.FetchKLines(ctx, symbol, days)
		if err == nil && len(klines) > 0 {
			return klines, nil
		}
		log.Printf("Primary provider %s failed: %v, trying fallback providers", primary.Name(), err)
	}

	for _, provider := range providers {
		if provider == primary {
			continue
		}

		if pm.isHealthy(provider.Name()) {
			klines, err := provider.FetchKLines(ctx, symbol, days)
			if err == nil && len(klines) > 0 {
				log.Printf("Using fallback provider %s for %s", provider.Name(), symbol)
				return klines, nil
			}
			log.Printf("Provider %s failed: %v", provider.Name(), err)
		}
	}

	return nil, ErrAllProvidersFailed
}

// isHealthy 检查数据源是否健康
func (pm *ProviderManager) isHealthy(name string) bool {
	pm.healthMu.RLock()
	defer pm.healthMu.RUnlock()
	return pm.health[name]
}

// StartHealthChecks 启动健康检查
func (pm *ProviderManager) StartHealthChecks() {
	pm.mu.RLock()
	providers := make([]DataProvider, len(pm.providers))
	copy(providers, pm.providers)
	pm.mu.RUnlock()

	for _, provider := range providers {
		go pm.monitorProvider(provider)
	}
}

// monitorProvider 监控数据源健康状态
func (pm *ProviderManager) monitorProvider(provider DataProvider) {
	ticker := time.NewTicker(pm.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := provider.HealthCheck()
			pm.healthMu.Lock()
			if err != nil {
				pm.health[provider.Name()] = false
				log.Printf("Provider %s health check failed: %v", provider.Name(), err)
			} else {
				pm.health[provider.Name()] = true
			}
			pm.healthMu.Unlock()

		case <-pm.stopChan:
			return
		}
	}
}

// StopHealthChecks 停止健康检查
func (pm *ProviderManager) StopHealthChecks() {
	close(pm.stopChan)
}

// GetProvidersStatus 获取所有数据源状态
func (pm *ProviderManager) GetProvidersStatus() map[string]bool {
	pm.healthMu.RLock()
	defer pm.healthMu.RUnlock()

	status := make(map[string]bool)
	for name, healthy := range pm.health {
		status[name] = healthy
	}
	return status
}

// GetPrimaryProvider 获取当前主数据源
func (pm *ProviderManager) GetPrimaryProvider() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.primary == nil {
		return ""
	}
	return pm.primary.Name()
}

var (
	ErrProviderNotFound   = &ProviderError{Code: "provider_not_found", Message: "Data provider not found"}
	ErrAllProvidersFailed = &ProviderError{Code: "all_providers_failed", Message: "All data providers failed"}
)

// ProviderError 数据源错误
type ProviderError struct {
	Code    string
	Message string
}

func (e *ProviderError) Error() string {
	return e.Message
}

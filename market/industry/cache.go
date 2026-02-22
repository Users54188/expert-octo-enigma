// Package industry 提供行业数据缓存管理
package industry

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Cache 行业信息缓存管理器
type Cache struct {
	mapping     *IndustryMapping
	symbolMap   map[string]*IndustryInfo
	industryMap map[string][]*IndustryInfo
	sectorMap   map[string][]*IndustryInfo
	lastLoad    time.Time
	ttl         time.Duration
	mu          sync.RWMutex
	filePath    string
	autoReload  bool
	stopReload  chan struct{}
}

// NewCache 创建行业信息缓存
func NewCache(filePath string) *Cache {
	return &Cache{
		symbolMap:   make(map[string]*IndustryInfo),
		industryMap: make(map[string][]*IndustryInfo),
		sectorMap:   make(map[string][]*IndustryInfo),
		filePath:    filePath,
		ttl:         24 * time.Hour,
		stopReload:  make(chan struct{}),
	}
}

// Load 从文件加载行业映射数据
func (c *Cache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("读取行业数据文件失败: %w", err)
	}

	var mapping IndustryMapping
	if err := json.Unmarshal(data, &mapping); err != nil {
		return fmt.Errorf("解析行业数据失败: %w", err)
	}

	c.mapping = &mapping
	c.symbolMap = make(map[string]*IndustryInfo)
	c.industryMap = make(map[string][]*IndustryInfo)
	c.sectorMap = make(map[string][]*IndustryInfo)

	// 构建索引
	for i := range mapping.Data {
		info := &mapping.Data[i]
		c.symbolMap[info.Symbol] = info
		c.industryMap[info.SWIndustry] = append(c.industryMap[info.SWIndustry], info)
		c.sectorMap[info.SWSector] = append(c.sectorMap[info.SWSector], info)
	}

	c.lastLoad = time.Now()
	return nil
}

// StartAutoReload 启动自动重载
func (c *Cache) StartAutoReload(interval time.Duration) {
	c.autoReload = true
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := c.Load(); err != nil {
					// 静默处理错误，避免影响服务
					continue
				}
			case <-c.stopReload:
				return
			}
		}
	}()
}

// StopAutoReload 停止自动重载
func (c *Cache) StopAutoReload() {
	if c.autoReload {
		close(c.stopReload)
		c.autoReload = false
	}
}

// Reload 重新加载数据
func (c *Cache) Reload() error {
	return c.Load()
}

// IsExpired 检查缓存是否过期
func (c *Cache) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.lastLoad) > c.ttl
}

// GetStockIndustry 获取股票的行业信息
func (c *Cache) GetStockIndustry(symbol string) (*IndustryInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查缓存过期，异步重载
	if c.IsExpired() && !c.autoReload {
		go func() {
			_ = c.Reload()
		}()
	}

	info, exists := c.symbolMap[symbol]
	return info, exists
}

// GetAllStocks 获取所有股票的行业信息
func (c *Cache) GetAllStocks() []IndustryInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mapping == nil {
		return nil
	}

	result := make([]IndustryInfo, len(c.mapping.Data))
	copy(result, c.mapping.Data)
	return result
}

// GetIndustryList 获取所有行业列表
func (c *Cache) GetIndustryList() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mapping == nil {
		return nil
	}

	result := make([]string, len(c.mapping.IndustryList))
	copy(result, c.mapping.IndustryList)
	return result
}

// GetStocksByIndustry 获取指定行业的所有股票
func (c *Cache) GetStocksByIndustry(industry string) []*IndustryInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos, exists := c.industryMap[industry]
	if !exists {
		return nil
	}

	result := make([]*IndustryInfo, len(infos))
	copy(result, infos)
	return result
}

// GetStocksBySector 获取指定板块的所有股票
func (c *Cache) GetStocksBySector(sector string) []*IndustryInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	infos, exists := c.sectorMap[sector]
	if !exists {
		return nil
	}

	result := make([]*IndustryInfo, len(infos))
	copy(result, infos)
	return result
}

// GetStocksByMarketCap 获取指定市值分类的所有股票
func (c *Cache) GetStocksByMarketCap(marketCap string) []IndustryInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mapping == nil {
		return nil
	}

	var result []IndustryInfo
	for _, stock := range c.mapping.Data {
		if stock.MarketCap == marketCap {
			result = append(result, stock)
		}
	}
	return result
}

// GetBenchmarkWeights 获取基准权重
func (c *Cache) GetBenchmarkWeights(benchmark string) map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.mapping == nil || c.mapping.BenchmarkWeights == nil {
		return nil
	}

	weights, exists := c.mapping.BenchmarkWeights[benchmark]
	if !exists {
		return nil
	}

	// 返回副本
	result := make(map[string]float64, len(weights))
	for k, v := range weights {
		result[k] = v
	}
	return result
}

// GetLastUpdated 获取最后更新时间
func (c *Cache) GetLastUpdated() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastLoad
}

// SetTTL 设置缓存过期时间
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

// GetStats 获取缓存统计信息
func (c *Cache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"total_stocks":     len(c.symbolMap),
		"total_industries": len(c.industryMap),
		"total_sectors":    len(c.sectorMap),
		"last_load":        c.lastLoad,
		"is_expired":       c.IsExpired(),
	}
}

// 全局缓存实例
var (
	globalCache *Cache
	cacheOnce   sync.Once
	cacheErr    error
)

// GetGlobalCache 获取全局行业缓存（单例）
func GetGlobalCache(filePath string) (*Cache, error) {
	cacheOnce.Do(func() {
		globalCache = NewCache(filePath)
		cacheErr = globalCache.Load()
	})
	return globalCache, cacheErr
}

// ResetGlobalCache 重置全局缓存（用于测试）
func ResetGlobalCache() {
	globalCache = nil
	cacheOnce = sync.Once{}
}

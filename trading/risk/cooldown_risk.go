package risk

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading"
)

// CooldownRisk 交易冷却时间风险管理
type CooldownRisk struct {
	mu           sync.RWMutex
	config       *CooldownRiskConfig
	tradeHistory *trading.TradeHistory
	lastTrades   map[string]time.Time // 股票代码 -> 最后交易时间
	tradeCounts  map[string]int     // 股票代码 -> 交易次数
	blacklist    map[string]time.Time // 黑名单股票和解除时间
}

// CooldownRiskConfig 冷却风险配置
type CooldownRiskConfig struct {
	MinTradeInterval   time.Duration `yaml:"min_trade_interval"`   // 最小交易间隔
	MaxDailyTrades     int           `yaml:"max_daily_trades"`     // 每日最大交易次数
	MinOrderInterval   time.Duration `yaml:"min_order_interval"`   // 最小下单间隔
	MaxWeeklyTrades    int           `yaml:"max_weekly_trades"`    // 每周最大交易次数
	BlacklistDuration  time.Duration `yaml:"blacklist_duration"`   // 黑名单持续时间
	EnableCooldown     bool          `yaml:"enable_cooldown"`      // 启用冷却机制
}

// NewCooldownRisk 创建冷却风险管理器
func NewCooldownRisk(config CooldownRiskConfig, tradeHistory *trading.TradeHistory) *CooldownRisk {
	return &CooldownRisk{
		config:       &config,
		tradeHistory: tradeHistory,
		lastTrades:   make(map[string]time.Time),
		tradeCounts:  make(map[string]int),
		blacklist:    make(map[string]time.Time),
	}
}

// CheckTradeCooldown 检查交易冷却
func (c *CooldownRisk) CheckTradeCooldown(ctx context.Context, symbol string) (*CooldownCheckResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := &CooldownCheckResult{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Allowed:   true,
		Reasons:   make([]string, 0),
	}

	// 检查是否启用冷却机制
	if !c.config.EnableCooldown {
		return result, nil
	}

	// 检查黑名单
	if c.isBlacklisted(symbol) {
		result.Allowed = false
		result.Reasons = append(result.Reasons, fmt.Sprintf("股票 %s 在黑名单中", symbol))
		return result, nil
	}

	now := time.Now()

	// 检查最小交易间隔
	if lastTrade, exists := c.lastTrades[symbol]; exists {
		interval := now.Sub(lastTrade)
		if interval < c.config.MinTradeInterval {
			result.Allowed = false
			result.Reasons = append(result.Reasons, 
				fmt.Sprintf("最小交易间隔未满足: %v < %v", interval, c.config.MinTradeInterval))
		}
	}

	// 检查每日交易次数
	dailyTrades := c.getDailyTradeCount(symbol, now)
	if dailyTrades >= c.config.MaxDailyTrades {
		result.Allowed = false
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("今日交易次数已达上限: %d/%d", dailyTrades, c.config.MaxDailyTrades))
	}

	// 检查每周交易次数
	weeklyTrades := c.getWeeklyTradeCount(symbol, now)
	if weeklyTrades >= c.config.MaxWeeklyTrades {
		result.Allowed = false
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("本周交易次数已达上限: %d/%d", weeklyTrades, c.config.MaxWeeklyTrades))
	}

	// 检查全局冷却时间
	globalCooldown := c.getGlobalCooldownTime()
	if globalCooldown > 0 {
		result.Allowed = false
		result.Reasons = append(result.Reasons, 
			fmt.Sprintf("全局冷却中: %v", globalCooldown))
	}

	return result, nil
}

// RecordTrade 记录交易
func (c *CooldownRisk) RecordTrade(symbol string, tradeType string, price float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// 更新最后交易时间
	c.lastTrades[symbol] = now

	// 更新交易次数
	c.tradeCounts[symbol]++

	// 如果是连续亏损交易，加入黑名单
	// 这里简化实现，实际应该根据交易盈亏情况判断
	if c.shouldAddToBlacklist(symbol) {
		c.blacklist[symbol] = now.Add(c.config.BlacklistDuration)
		log.Printf("Added %s to blacklist until %v", symbol, c.blacklist[symbol])
	}

	log.Printf("Recorded trade for %s: %s at %.2f", symbol, tradeType, price)
}

// isBlacklisted 检查是否在黑名单中
func (c *CooldownRisk) isBlacklisted(symbol string) bool {
	blacklistTime, exists := c.blacklist[symbol]
	if !exists {
		return false
	}

	now := time.Now()
	if now.After(blacklistTime) {
		// 黑名单时间已过，移除
		delete(c.blacklist, symbol)
		return false
	}

	return true
}

// getDailyTradeCount 获取当日交易次数
func (c *CooldownRisk) getDailyTradeCount(symbol string, now time.Time) int {
	// 简化实现：使用内存中的计数
	// 实际应该查询数据库获取真实的历史交易记录
	
	// 检查是否跨日重置
	day := now.Format("2006-01-02")
	
	// 这里简化处理，实际应该根据日期重置计数
	return c.tradeCounts[symbol]
}

// getWeeklyTradeCount 获取本周交易次数
func (c *CooldownRisk) getWeeklyTradeCount(symbol string, now time.Time) int {
	// 简化实现：使用内存中的计数
	// 实际应该查询数据库获取真实的历史交易记录
	
	// 检查是否跨周重置
	week := getWeekString(now)
	
	// 这里简化处理，实际应该根据周重置计数
	return c.tradeCounts[symbol] / 7 // 简化计算
}

// getGlobalCooldownTime 获取全局冷却时间
func (c *CooldownRisk) getGlobalCooldownTime() time.Duration {
	// 简化实现：基于最近交易时间计算全局冷却
	// 实际应该根据系统状态和配置计算
	
	now := time.Now()
	var latestTime time.Time
	
	for _, lastTrade := range c.lastTrades {
		if lastTrade.After(latestTime) {
			latestTime = lastTrade
		}
	}

	if latestTime.IsZero() {
		return 0
	}

	elapsed := now.Sub(latestTime)
	if elapsed < c.config.MinOrderInterval {
		return c.config.MinOrderInterval - elapsed
	}

	return 0
}

// shouldAddToBlacklist 判断是否应该加入黑名单
func (c *CooldownRisk) shouldAddToBlacklist(symbol string) bool {
	// 简化实现：基于交易频率判断
	// 实际应该基于交易盈亏情况
	
	count := c.tradeCounts[symbol]
	
	// 如果在短时间内交易次数过多，加入黑名单
	if count > 10 {
		return true
	}
	
	return false
}

// GetCooldownStatus 获取冷却状态
func (c *CooldownRisk) GetCooldownStatus(symbol string) *CooldownStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	status := &CooldownStatus{
		Symbol:           symbol,
		Timestamp:        now,
		LastTradeTime:    c.lastTrades[symbol],
		TradeCount:       c.tradeCounts[symbol],
		DailyTrades:      c.getDailyTradeCount(symbol, now),
		WeeklyTrades:     c.getWeeklyTradeCount(symbol, now),
		Blacklisted:      c.isBlacklisted(symbol),
		CooldownRemaining: 0,
	}

	// 计算剩余冷却时间
	if lastTrade, exists := c.lastTrades[symbol]; exists {
		elapsed := now.Sub(lastTrade)
		if elapsed < c.config.MinTradeInterval {
			status.CooldownRemaining = c.config.MinTradeInterval - elapsed
		}
	}

	return status
}

// GetAllCooldownStatus 获取所有股票冷却状态
func (c *CooldownRisk) GetAllCooldownStatus() map[string]*CooldownStatus {
	status := make(map[string]*CooldownStatus)

	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()

	for symbol := range c.lastTrades {
		status[symbol] = c.GetCooldownStatus(symbol)
	}

	return status
}

// RemoveFromBlacklist 从黑名单移除
func (c *CooldownRisk) RemoveFromBlacklist(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.blacklist, symbol)
	log.Printf("Removed %s from blacklist", symbol)
}

// AddToBlacklist 添加到黑名单
func (c *CooldownRisk) AddToBlacklist(symbol string, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.blacklist[symbol] = time.Now().Add(duration)
	log.Printf("Added %s to blacklist for %v", symbol, duration)
}

// ResetCooldown 重置冷却状态
func (c *CooldownRisk) ResetCooldown(symbol string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.lastTrades, symbol)
	delete(c.tradeCounts, symbol)
	c.RemoveFromBlacklist(symbol)
	log.Printf("Reset cooldown for %s", symbol)
}

// ClearAllCooldowns 清除所有冷却状态
func (c *CooldownRisk) ClearAllCooldowns() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastTrades = make(map[string]time.Time)
	c.tradeCounts = make(map[string]int)
	c.blacklist = make(map[string]time.Time)
	log.Printf("Cleared all cooldown states")
}

// SetConfig 更新配置
func (c *CooldownRisk) SetConfig(config CooldownRiskConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = &config
	log.Printf("Cooldown risk config updated: interval=%v, max_daily=%d", 
		config.MinTradeInterval, config.MaxDailyTrades)
}

// GetConfig 获取配置
func (c *CooldownRisk) GetConfig() CooldownRiskConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return *c.config
}

// GetStats 获取统计信息
func (c *CooldownRisk) GetStats() *CooldownStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var latestTrade time.Time
	totalTrades := 0

	for _, lastTrade := range c.lastTrades {
		if lastTrade.After(latestTrade) {
			latestTrade = lastTrade
		}
	}

	for _, count := range c.tradeCounts {
		totalTrades += count
	}

	return &CooldownStats{
		TotalTrackedSymbols: len(c.lastTrades),
		BlacklistedSymbols: len(c.blacklist),
		TotalTrades:        totalTrades,
		LastTradeTime:      latestTrade,
		Enabled:            c.config.EnableCooldown,
	}
}

// CooldownCheckResult 冷却检查结果
type CooldownCheckResult struct {
	Symbol    string        `json:"symbol"`
	Timestamp time.Time     `json:"timestamp"`
	Allowed   bool          `json:"allowed"`
	Reasons   []string      `json:"reasons"`
}

// CooldownStatus 冷却状态
type CooldownStatus struct {
	Symbol            string        `json:"symbol"`
	Timestamp         time.Time     `json:"timestamp"`
	LastTradeTime     time.Time     `json:"last_trade_time"`
	TradeCount        int           `json:"trade_count"`
	DailyTrades       int           `json:"daily_trades"`
	WeeklyTrades      int           `json:"weekly_trades"`
	Blacklisted       bool          `json:"blacklisted"`
	CooldownRemaining time.Duration `json:"cooldown_remaining"`
}

// CooldownStats 冷却统计
type CooldownStats struct {
	TotalTrackedSymbols int           `json:"total_tracked_symbols"`
	BlacklistedSymbols  int           `json:"blacklisted_symbols"`
	TotalTrades         int           `json:"total_trades"`
	LastTradeTime       time.Time     `json:"last_trade_time"`
	Enabled             bool          `json:"enabled"`
}

// 工具函数
func getWeekString(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-%02d", year, week)
}
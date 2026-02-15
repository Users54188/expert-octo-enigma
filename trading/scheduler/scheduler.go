package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/market"
	"cloudquant/trading/strategies"
)

// Scheduler 策略调度器
type Scheduler struct {
	mu                 sync.RWMutex
	running            bool
	interval           time.Duration
	cronExpr           string
	enabled            bool
	lastExecution      time.Time
	executionCount     int64
	strategyManager    *strategies.StrategyManager
	marketProvider     *market.MarketProvider
	symbols            []string
	currentSymbolIndex int
	ticker             *time.Ticker
	ctx                context.Context
	cancel             context.CancelFunc
}

// NewScheduler 创建调度器
func NewScheduler(interval string) (*Scheduler, error) {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval format: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		interval:           duration,
		enabled:            true,
		ctx:                ctx,
		cancel:             cancel,
		symbols:            make([]string, 0),
		currentSymbolIndex: 0,
	}, nil
}

// SetStrategyManager 设置策略管理器
func (s *Scheduler) SetStrategyManager(manager *strategies.StrategyManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategyManager = manager
}

// SetMarketProvider 设置市场数据提供者
func (s *Scheduler) SetMarketProvider(provider *market.MarketProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.marketProvider = provider
}

// SetSymbols 设置监控的股票列表
func (s *Scheduler) SetSymbols(symbols []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbols = make([]string, len(symbols))
	copy(s.symbols, symbols)
	log.Printf("Scheduler configured with %d symbols: %v", len(symbols), symbols)
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	if s.strategyManager == nil {
		return fmt.Errorf("strategy manager not set")
	}

	if s.marketProvider == nil {
		return fmt.Errorf("market provider not set")
	}

	if len(s.symbols) == 0 {
		return fmt.Errorf("no symbols configured")
	}

	s.running = true
	s.ticker = time.NewTicker(s.interval)

	go s.runScheduler()

	log.Printf("Strategy scheduler started with interval: %s", s.interval)
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return fmt.Errorf("scheduler is not running")
	}

	s.running = false

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}

	s.cancel()
	log.Printf("Strategy scheduler stopped")
	return nil
}

// IsRunning 检查调度器是否运行中
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// SetEnabled 设置启用状态
func (s *Scheduler) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
	if enabled {
		log.Printf("Strategy scheduler enabled")
	} else {
		log.Printf("Strategy scheduler disabled")
	}
}

// IsEnabled 检查是否启用
func (s *Scheduler) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// GetStats 获取调度器统计信息
func (s *Scheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":         s.running,
		"enabled":         s.enabled,
		"interval":        s.interval.String(),
		"last_execution":  s.lastExecution,
		"execution_count": s.executionCount,
		"symbols_count":   len(s.symbols),
		"current_symbol":  s.getCurrentSymbol(),
		"cron_expression": s.cronExpr,
	}
}

// getCurrentSymbol 获取当前股票
func (s *Scheduler) getCurrentSymbol() string {
	if len(s.symbols) == 0 {
		return ""
	}
	return s.symbols[s.currentSymbolIndex%len(s.symbols)]
}

// runScheduler 运行调度器主循环
func (s *Scheduler) runScheduler() {
	defer func() {
		s.mu.Lock()
		s.running = false
		if s.ticker != nil {
			s.ticker.Stop()
			s.ticker = nil
		}
		s.mu.Unlock()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C:
			if !s.enabled {
				continue
			}

			// 执行策略调度
			s.executeCycle()
		}
	}
}

// executeCycle 执行一个调度周期
func (s *Scheduler) executeCycle() {
	startTime := time.Now()
	s.mu.Lock()
	s.executionCount++
	s.lastExecution = startTime
	s.mu.Unlock()

	log.Printf("Starting strategy execution cycle #%d", s.executionCount)

	// 轮询股票池
	symbol := s.getCurrentSymbol()
	if symbol == "" {
		log.Printf("No symbols configured for scheduler")
		return
	}

	// 更新股票索引
	s.mu.Lock()
	s.currentSymbolIndex++
	s.mu.Unlock()

	// 获取市场数据
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	marketData, err := s.getMarketData(ctx, symbol)
	if err != nil {
		log.Printf("Failed to get market data for %s: %v", symbol, err)
		return
	}

	// 执行策略
	result, err := s.executeStrategiesForSymbol(ctx, symbol, marketData)
	if err != nil {
		log.Printf("Strategy execution failed for %s: %v", symbol, err)
		return
	}

	// 处理策略结果
	if err := s.processStrategyResult(ctx, symbol, result); err != nil {
		log.Printf("Failed to process strategy result for %s: %v", symbol, err)
	}

	duration := time.Since(startTime)
	log.Printf("Strategy execution cycle #%d completed in %v", s.executionCount, duration)
}

// getMarketData 获取市场数据
func (s *Scheduler) getMarketData(ctx context.Context, symbol string) (*strategies.MarketData, error) {
	if s.marketProvider == nil {
		return nil, fmt.Errorf("market provider not set")
	}

	// 调用市场数据提供者的接口
	// 这里需要根据实际的market包接口进行调整
	// 暂时返回模拟数据
	return s.mockMarketData(symbol), nil
}

// mockMarketData 模拟市场数据
func (s *Scheduler) mockMarketData(symbol string) *strategies.MarketData {
	// 模拟市场数据生成
	return &strategies.MarketData{
		Symbol:        symbol,
		Open:          10.0,
		High:          10.5,
		Low:           9.8,
		Close:         10.2,
		Volume:        1000000,
		Amount:        10000000.0,
		Timestamp:     time.Now(),
		PreClose:      10.0,
		Change:        0.2,
		ChangePercent: 2.0,
	}
}

// executeStrategiesForSymbol 为特定股票执行策略
func (s *Scheduler) executeStrategiesForSymbol(ctx context.Context, symbol string, marketData *strategies.MarketData) (*strategies.StrategyExecutionResult, error) {
	if s.strategyManager == nil {
		return nil, fmt.Errorf("strategy manager not set")
	}

	// 执行策略
	result, err := s.strategyManager.ExecuteStrategies(ctx, marketData)
	if err != nil {
		return nil, fmt.Errorf("strategy execution failed: %v", err)
	}

	return result, nil
}

// processStrategyResult 处理策略执行结果
func (s *Scheduler) processStrategyResult(ctx context.Context, symbol string, result *strategies.StrategyExecutionResult) error {
	if result == nil {
		return nil
	}

	// 记录执行结果
	log.Printf("Strategy execution result for %s: %d signals, %d errors",
		symbol, len(result.Signals), len(result.Errors))

	// 处理错误
	for _, err := range result.Errors {
		log.Printf("Strategy error: %v", err)
	}

	// 处理信号
	if len(result.Signals) > 0 {
		log.Printf("Processing %d signals for %s", len(result.Signals), symbol)

		// 将信号发送给策略管理器处理
		if err := s.strategyManager.ProcessSignals(ctx, result.Signals); err != nil {
			return fmt.Errorf("failed to process signals: %v", err)
		}
	}

	return nil
}

// ExecuteNow 立即执行一次
func (s *Scheduler) ExecuteNow() error {
	if !s.enabled {
		return fmt.Errorf("scheduler is disabled")
	}

	log.Printf("Manual strategy execution triggered")
	s.executeCycle()
	return nil
}

// ExecuteSymbol 立即为指定股票执行策略
func (s *Scheduler) ExecuteSymbol(symbol string) error {
	if !s.enabled {
		return fmt.Errorf("scheduler is disabled")
	}

	log.Printf("Manual strategy execution for symbol: %s", symbol)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取市场数据
	marketData, err := s.getMarketData(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to get market data for %s: %v", symbol, err)
	}

	// 执行策略
	result, err := s.executeStrategiesForSymbol(ctx, symbol, marketData)
	if err != nil {
		return fmt.Errorf("strategy execution failed for %s: %v", symbol, err)
	}

	// 处理结果
	return s.processStrategyResult(ctx, symbol, result)
}

// GetNextExecutionTime 获取下次执行时间
func (s *Scheduler) GetNextExecutionTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running || !s.enabled {
		return time.Time{}
	}

	return s.lastExecution.Add(s.interval)
}

// SetInterval 设置执行间隔
func (s *Scheduler) SetInterval(interval string) error {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("invalid interval format: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.interval = duration

	// 如果正在运行，重新启动ticker
	if s.running && s.ticker != nil {
		s.ticker.Stop()
		s.ticker = time.NewTicker(duration)
	}

	log.Printf("Scheduler interval changed to: %s", interval)
	return nil
}

// SetCronExpression 设置cron表达式（预留接口）
func (s *Scheduler) SetCronExpression(expr string) error {
	// TODO: 实现cron表达式解析和调度
	// 目前仅保存表达式，后续可以基于cron库实现
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cronExpr = expr
	log.Printf("Scheduler cron expression set to: %s", expr)
	return nil
}

// GetStatus 获取详细状态信息
func (s *Scheduler) GetStatus() map[string]interface{} {
	stats := s.GetStats()

	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"scheduler":      stats,
		"current_symbol": s.getCurrentSymbol(),
		"next_execution": s.GetNextExecutionTime(),
		"uptime":         time.Since(s.lastExecution).String(),
	}

	return status
}

// ForceStop 强制停止（不优雅关闭）
func (s *Scheduler) ForceStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	s.enabled = false

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}

	s.cancel()
	log.Printf("Strategy scheduler force stopped")
}

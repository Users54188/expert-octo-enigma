package strategies

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"cloudquant/trading"
)

// SignalCombination  信号组合方法
type SignalCombination string

const (
	VoteCombination    SignalCombination = "vote"     // 投票法
	WeightedCombination SignalCombination = "weighted" // 加权法
	PriorityCombination SignalCombination = "priority" // 优先级法
)

// StrategyManager 策略管理器
type StrategyManager struct {
	loader         *StrategyLoader
	combination    SignalCombination
	mu             sync.RWMutex
	riskManager    *trading.RiskManager
	positionManager *trading.PositionManager
	orderExecutor  *trading.OrderExecutor
	signalHandler  *trading.SignalHandler
	lastExecution  time.Time
	executionCount int64
}

// NewStrategyManager 创建策略管理器
func NewStrategyManager(
	loader *StrategyLoader,
	combination SignalCombination,
) *StrategyManager {
	return &StrategyManager{
		loader:      loader,
		combination: combination,
	}
}

// SetTradingComponents 设置交易组件
func (m *StrategyManager) SetTradingComponents(
	riskManager *trading.RiskManager,
	positionManager *trading.PositionManager,
	orderExecutor *trading.OrderExecutor,
	signalHandler *trading.SignalHandler,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.riskManager = riskManager
	m.positionManager = positionManager
	m.orderExecutor = orderExecutor
	m.signalHandler = signalHandler
}

// ExecuteStrategies 执行所有策略
func (m *StrategyManager) ExecuteStrategies(ctx context.Context, marketData *MarketData) (*StrategyExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()
	m.executionCount++
	m.lastExecution = startTime

	// 获取所有启用的策略
	enabledStrategies := m.loader.GetEnabledStrategies()
	if len(enabledStrategies) == 0 {
		return &StrategyExecutionResult{
			Timestamp: startTime,
			Error:     fmt.Errorf("no enabled strategies"),
		}, nil
	}

	// 并行执行所有策略
	signals := make(chan *Signal, len(enabledStrategies))
	errCh := make(chan error, len(enabledStrategies))
	var wg sync.WaitGroup

	for name, strategy := range enabledStrategies {
		wg.Add(1)
		go func(name string, strategy Strategy) {
			defer wg.Done()

			// 执行策略生成信号
			result, err := m.executeSingleStrategy(ctx, strategy, marketData)
			if err != nil {
				errCh <- fmt.Errorf("strategy %s failed: %v", name, err)
				return
			}

			// 处理策略结果
			for _, signal := range result.Signals {
				signal.Metadata["strategy_name"] = name
				signals <- signal
			}
		}(name, strategy)
	}

	wg.Wait()
	close(signals)
	close(errCh)

	// 收集错误
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	// 合并信号
	combinedSignals, err := m.combineSignals(signals, marketData)
	if err != nil {
		return &StrategyExecutionResult{
			Timestamp: startTime,
			Duration:  time.Since(startTime).Milliseconds(),
			Errors:    errors,
			Error:     err,
		}, err
	}

	return &StrategyExecutionResult{
		Timestamp:    startTime,
		Duration:     time.Since(startTime).Milliseconds(),
		Signals:      combinedSignals,
		StrategyCount: len(enabledStrategies),
		Errors:       errors,
	}, nil
}

// executeSingleStrategy 执行单个策略
func (m *StrategyManager) executeSingleStrategy(
	ctx context.Context,
	strategy Strategy,
	marketData *MarketData,
) (*StrategyResult, error) {
	startTime := time.Now()

	// 生成信号
	signal, err := strategy.GenerateSignal(ctx, marketData)
	if err != nil {
		return &StrategyResult{
			Duration: time.Since(startTime).Milliseconds(),
			Error:    err,
		}, err
	}

	// 验证信号
	if signal != nil {
		if err := ValidateSignal(signal); err != nil {
			return &StrategyResult{
				Duration: time.Since(startTime).Milliseconds(),
				Error:    err,
			}, err
		}

		// 添加策略信息到信号
		signal.Metadata["strategy_weight"] = strategy.GetWeight()
		signal.Metadata["strategy_name"] = strategy.GetName()
	}

	signals := make([]*Signal, 0, 1)
	if signal != nil {
		signals = append(signals, signal)
	}

	return &StrategyResult{
		Signals:    signals,
		Score:      0.0, // 可以根据策略表现计算
		Confidence: 0.0, // 可以根据信号强度计算
		Duration:   time.Since(startTime).Milliseconds(),
		Error:      nil,
	}, nil
}

// combineSignals 合并多个策略的信号
func (m *StrategyManager) combineSignals(signals <-chan *Signal, marketData *MarketData) ([]*Signal, error) {
	// 收集所有信号
	var allSignals []*Signal
	for signal := range signals {
		allSignals = append(allSignals, signal)
	}

	if len(allSignals) == 0 {
		return nil, nil
	}

	// 按股票分组
	signalsBySymbol := make(map[string][]*Signal)
	for _, signal := range allSignals {
		signalsBySymbol[signal.SignalType] = append(signalsBySymbol[signal.SignalType], signal)
	}

	// 根据组合方法合并信号
	var combined []*Signal

	switch m.combination {
	case VoteCombination:
		combined = m.combineByVote(signalsBySymbol, marketData)
	case WeightedCombination:
		combined = m.combineByWeight(signalsBySymbol, marketData)
	case PriorityCombination:
		combined = m.combineByPriority(signalsBySymbol, marketData)
	default:
		return allSignals, nil // 默认返回所有信号
	}

	return combined, nil
}

// combineByVote 投票法合并信号
func (m *StrategyManager) combineByVote(signalsBySymbol map[string][]*Signal, marketData *MarketData) []*Signal {
	var combined []*Signal

	// 按信号类型统计
	buySignals := len(signalsBySymbol["buy"])
	sellSignals := len(signalsBySymbol["sell"])
	totalSignals := buySignals + sellSignals

	// 简单多数决
	if totalSignals == 0 {
		return nil
	}

	var finalSignal *Signal
	var signalType string

	if buySignals > sellSignals {
		signalType = "buy"
	} else if sellSignals > buySignals {
		signalType = "sell"
	} else {
		signalType = "hold" // 平票
	}

	if signalType != "hold" {
		finalSignal = NewSignal(
			marketData.Symbol,
			signalType,
			float64(max(buySignals, sellSignals))/float64(totalSignals),
			marketData.Close,
		)
		finalSignal.Reason = fmt.Sprintf("Vote: %d buy, %d sell", buySignals, sellSignals)
	}

	if finalSignal != nil {
		combined = append(combined, finalSignal)
	}

	return combined
}

// combineByWeight 加权法合并信号
func (m *StrategyManager) combineByWeight(signalsBySymbol map[string][]*Signal, marketData *MarketData) []*Signal {
	// 计算加权得分
	var weightedScore float64
	var totalWeight float64

	for _, signals := range signalsBySymbol {
		for _, signal := range signals {
			weight, ok := signal.Metadata["strategy_weight"].(float64)
			if !ok {
				weight = 0.5 // 默认权重
			}

			var score float64
			switch signal.SignalType {
			case "buy":
				score = 1.0
			case "sell":
				score = -1.0
			default:
				score = 0.0
			}

			weightedScore += score * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return nil
	}

	normalizedScore := weightedScore / totalWeight

	// 设定阈值
	threshold := 0.1
	if abs(normalizedScore) < threshold {
		return nil // 信号太弱，不执行
	}

	var signalType string
	if normalizedScore > 0 {
		signalType = "buy"
	} else {
		signalType = "sell"
	}

	signal := NewSignal(
		marketData.Symbol,
		signalType,
		abs(normalizedScore),
		marketData.Close,
	)
	signal.Reason = fmt.Sprintf("Weighted: %.3f", normalizedScore)

	return []*Signal{signal}
}

// combineByPriority 优先级法合并信号
func (m *StrategyManager) combineByPriority(signalsBySymbol map[string][]*Signal, marketData *MarketData) []*Signal {
	// 收集所有信号，包含优先级信息
	type PrioritySignal struct {
		Signal    *Signal
		Priority  int
		Weight    float64
	}

	var prioritySignals []PrioritySignal

	for _, signals := range signalsBySymbol {
		for _, signal := range signals {
			priority, _ := signal.Metadata["strategy_priority"].(int)
			if priority == 0 {
				priority = 5 // 默认优先级
			}

			weight, _ := signal.Metadata["strategy_weight"].(float64)
			if weight == 0 {
				weight = 0.5
			}

			prioritySignals = append(prioritySignals, PrioritySignal{
				Signal:   signal,
				Priority: priority,
				Weight:   weight,
			})
		}
	}

	// 按优先级排序
	sort.Slice(prioritySignals, func(i, j int) bool {
		return prioritySignals[i].Priority < prioritySignals[j].Priority
	})

	// 取最高优先级的信号
	if len(prioritySignals) > 0 {
		bestSignal := prioritySignals[0].Signal
		bestSignal.Reason = fmt.Sprintf("Priority: rank %d from %d signals",
			1, len(prioritySignals))
		return []*Signal{bestSignal}
	}

	return nil
}

// GetStats 获取执行统计
func (m *StrategyManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"last_execution":   m.lastExecution,
		"execution_count":  m.executionCount,
		"strategy_count":   m.loader.GetStrategyCount(),
		"enabled_count":    m.loader.GetEnabledStrategyCount(),
		"combination_type": m.combination,
	}
}

// SetCombinationMethod 设置信号组合方法
func (m *StrategyManager) SetCombinationMethod(method SignalCombination) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.combination = method
	log.Printf("Strategy combination method changed to: %s", method)
}

// GetCombinationMethod 获取信号组合方法
func (m *StrategyManager) GetCombinationMethod() SignalCombination {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.combination
}

// StrategyExecutionResult 策略执行结果
type StrategyExecutionResult struct {
	Timestamp     time.Time     `json:"timestamp"`
	Duration      int64         `json:"duration"`      // 执行时间(毫秒)
	Signals       []*Signal     `json:"signals"`      // 合并后的信号
	StrategyCount int           `json:"strategy_count"` // 策略数量
	Errors        []error       `json:"errors"`       // 执行错误
	Error         error         `json:"error"`        // 整体错误
}

// HasErrors 检查是否有错误
func (r *StrategyExecutionResult) HasErrors() bool {
	return len(r.Errors) > 0 || r.Error != nil
}

// GetSuccessCount 获取成功策略数量
func (r *StrategyExecutionResult) GetSuccessCount() int {
	return r.StrategyCount - len(r.Errors)
}

// ProcessSignals 处理策略信号
func (m *StrategyManager) ProcessSignals(ctx context.Context, signals []*Signal) error {
	if m.signalHandler == nil {
		return fmt.Errorf("signal handler not set")
	}

	for _, signal := range signals {
		// 验证信号
		if err := ValidateSignal(signal); err != nil {
			log.Printf("Invalid signal from strategy: %v", err)
			continue
		}

		// 检查风险
		if m.riskManager != nil {
			// 可以在这里进行信号级别的风险检查
		}

		// 发送到信号处理器
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		// 根据信号类型进行处理
		switch signal.SignalType {
		case "buy":
			// 处理买入信号
			_ = m.signalHandler.HandleBuySignal(ctxWithTimeout, signal.Symbol, signal)
		case "sell":
			// 处理卖出信号
			_ = m.signalHandler.HandleSellSignal(ctxWithTimeout, signal.Symbol, signal)
		case "hold":
			// 持仓信号，不需要特殊处理
			log.Printf("Hold signal for %s: %s", signal.Symbol, signal.Reason)
		}
	}

	return nil
}

// 工具函数
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
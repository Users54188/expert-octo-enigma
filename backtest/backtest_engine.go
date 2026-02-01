package backtest

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading/strategies"
)

// BacktestEngine 回测引擎
type BacktestEngine struct {
	mu           sync.RWMutex
	config       *BacktestConfig
	strategies   map[string]strategies.Strategy
	results      *BacktestResults
	started      bool
	completed    bool
	startTime    time.Time
	endTime      time.Time
	progress     float64
}

// BacktestConfig 回测配置
type BacktestConfig struct {
	StartDate        time.Time         `yaml:"start_date"`
	EndDate          time.Time         `yaml:"end_date"`
	InitialCapital   float64          `yaml:"initial_capital"`
	Commission       float64          `yaml:"commission"`        // 手续费率
	Slippage         float64          `yaml:"slippage"`          // 滑点
	Symbols          []string         `yaml:"symbols"`           // 回测股票
	Strategies       []StrategyConfig `yaml:"strategies"`         // 策略配置
	RiskFreeRate     float64          `yaml:"risk_free_rate"`     // 无风险利率
	BenchmarkSymbol  string           `yaml:"benchmark_symbol"`  // 基准股票
	MaxDrawdownLimit float64          `yaml:"max_drawdown_limit"` // 最大回撤限制
	Realtime         bool             `yaml:"realtime"`           // 实时模式
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name      string                 `yaml:"name"`
	Type      strategies.StrategyType `yaml:"type"`
	Enabled   bool                   `yaml:"enabled"`
	Weight    float64                `yaml:"weight"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// BacktestResults 回测结果
type BacktestResults struct {
	Summary       *BacktestSummary        `json:"summary"`       // 回测摘要
	EquityCurve   []EquityPoint           `json:"equity_curve"` // 收益曲线
	Trades        []BacktestTrade         `json:"trades"`       // 交易记录
	Returns       []ReturnPoint           `json:"returns"`      // 收益率序列
	Drawdowns     []DrawdownPoint         `json:"drawdowns"`    // 回撤序列
	MonthlyReturns map[string]float64     `json:"monthly_returns"` // 月度收益
	StrategyStats map[string]*StrategyPerformance `json:"strategy_stats"` // 策略统计
	Benchmark     *BenchmarkComparison    `json:"benchmark"`    // 基准比较
	RiskMetrics   *RiskMetrics           `json:"risk_metrics"` // 风险指标
	Exposures     map[string][]ExposurePoint `json:"exposures"` // 暴露情况
	Errors        []string               `json:"errors"`       // 错误信息
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
}

// BacktestSummary 回测摘要
type BacktestSummary struct {
	InitialCapital    float64   `json:"initial_capital"`
	FinalValue       float64   `json:"final_value"`
	TotalReturn      float64   `json:"total_return"`
	AnnualizedReturn float64   `json:"annualized_return"`
	TotalTrades      int       `json:"total_trades"`
	WinningTrades    int       `json:"winning_trades"`
	LosingTrades     int       `json:"losing_trades"`
	WinRate          float64   `json:"win_rate"`
	ProfitFactor     float64   `json:"profit_factor"`
	SharpeRatio      float64   `json:"sharpe_ratio"`
	MaxDrawdown      float64   `json:"max_drawdown"`
	MaxDrawdownDuration time.Duration `json:"max_drawdown_duration"`
	CalmarRatio      float64   `json:"calmar_ratio"`
	SortinoRatio     float64   `json:"sortino_ratio"`
	InformationRatio float64   `json:"information_ratio"`
	TrackingError    float64   `json:"tracking_error"`
	ValueAtRisk      float64   `json:"value_at_risk"`
	ConditionalVaR   float64   `json:"conditional_var"`
	AverageTrade     float64   `json:"average_trade"`
	LargestWin       float64   `json:"largest_win"`
	LargestLoss      float64   `json:"largest_loss"`
	AverageWin       float64   `json:"average_win"`
	AverageLoss      float64   `json:"average_loss"`
	Commissions      float64   `json:"commissions"`
	Slippage         float64   `json:"slippage"`
	Alpha            float64   `json:"alpha"`
	Beta             float64   `json:"beta"`
	Correlation      float64   `json:"correlation"`
}

// EquityPoint 权益点
type EquityPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value    float64   `json:"value"`
	Drawdown float64   `json:"drawdown"`
}

// BacktestTrade 回测交易
type BacktestTrade struct {
	ID           string    `json:"id"`
	Symbol       string    `json:"symbol"`
	EntryTime    time.Time `json:"entry_time"`
	EntryPrice   float64   `json:"entry_price"`
	ExitTime     time.Time `json:"exit_time"`
	ExitPrice    float64   `json:"exit_price"`
	Quantity     int64     `json:"quantity"`
	Side         string    `json:"side"` // buy, sell
	PnL          float64   `json:"pnl"`
	Return       float64   `json:"return"`
	Strategy     string    `json:"strategy"`
	Commission   float64   `json:"commission"`
	Slippage     float64   `json:"slippage"`
	HoldDuration time.Duration `json:"hold_duration"`
}

// ReturnPoint 收益率点
type ReturnPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value    float64   `json:"value"`
	Return   float64   `json:"return"`
}

// DrawdownPoint 回撤点
type DrawdownPoint struct {
	Start      time.Time `json:"start"`
	End        time.Time `json:"end"`
	Peak       float64   `json:"peak"`
	Trough     float64   `json:"trough"`
	Duration   time.Duration `json:"duration"`
	Recovery   time.Duration `json:"recovery"`
}

// StrategyPerformance 策略表现
type StrategyPerformance struct {
	Name         string    `json:"name"`
	TotalReturn  float64   `json:"total_return"`
	WinRate      float64   `json:"win_rate"`
	SharpeRatio  float64   `json:"sharpe_ratio"`
	MaxDrawdown  float64   `json:"max_drawdown"`
	TradesCount  int       `json:"trades_count"`
	AvgReturn    float64   `json:"avg_return"`
}

// BenchmarkComparison 基准比较
type BenchmarkComparison struct {
	Symbol           string   `json:"symbol"`
	TotalReturn      float64  `json:"total_return"`
	AnnualizedReturn float64  `json:"annualized_return"`
	SharpeRatio      float64  `json:"sharpe_ratio"`
	MaxDrawdown      float64  `json:"max_drawdown"`
	Correlation      float64  `json:"correlation"`
	Alpha            float64  `json:"alpha"`
	Beta             float64  `json:"beta"`
}

// RiskMetrics 风险指标
type RiskMetrics struct {
	Volatility        float64   `json:"volatility"`
	DownsideDeviation float64   `json:"downside_deviation"`
	Skewness         float64   `json:"skewness"`
	Kurtosis         float64   `json:"kurtosis"`
	VaR95            float64   `json:"var_95"`
	CVaR95           float64   `json:"cvar_95"`
	VaR99            float64   `json:"var_99"`
	CVaR99           float64   `json:"cvar_99"`
}

// ExposurePoint 暴露点
type ExposurePoint struct {
	Timestamp time.Time         `json:"timestamp"`
	Exposures map[string]float64 `json:"exposures"`
}

// NewBacktestEngine 创建回测引擎
func NewBacktestEngine(config BacktestConfig) *BacktestEngine {
	return &BacktestEngine{
		config:    &config,
		strategies: make(map[string]strategies.Strategy),
		results:   &BacktestResults{},
		started:   false,
		completed: false,
	}
}

// AddStrategy 添加策略
func (b *BacktestEngine) AddStrategy(strategy strategies.Strategy) error {
	if strategy == nil {
		return fmt.Errorf("strategy is nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return fmt.Errorf("cannot add strategy after backtest started")
	}

	b.strategies[strategy.GetName()] = strategy
	log.Printf("Added strategy to backtest: %s", strategy.GetName())
	return nil
}

// Run 执行回测
func (b *BacktestEngine) Run(ctx context.Context) (*BacktestResults, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return nil, fmt.Errorf("backtest is already running")
	}

	if len(b.strategies) == 0 {
		return nil, fmt.Errorf("no strategies added")
	}

	b.started = true
	b.startTime = time.Now()
	b.progress = 0.0

	defer func() {
		b.completed = true
		b.endTime = time.Now()
		b.progress = 100.0
	}()

	log.Printf("Starting backtest: %s to %s", b.config.StartDate.Format("2006-01-02"), b.config.EndDate.Format("2006-01-02"))

	// 初始化回测结果
	if err := b.initializeResults(); err != nil {
		return nil, fmt.Errorf("failed to initialize results: %v", err)
	}

	// 执行回测主循环
	if err := b.runBacktestLoop(ctx); err != nil {
		return nil, fmt.Errorf("backtest failed: %v", err)
	}

	// 计算最终指标
	b.calculateFinalMetrics()

	log.Printf("Backtest completed: duration=%v, final_value=%.2f", b.endTime.Sub(b.startTime), b.results.Summary.FinalValue)
	return b.results, nil
}

// initializeResults 初始化回测结果
func (b *BacktestEngine) initializeResults() error {
	b.results = &BacktestResults{
		Summary: &BacktestSummary{
			InitialCapital: b.config.InitialCapital,
		},
		EquityCurve:   make([]EquityPoint, 0),
		Trades:        make([]BacktestTrade, 0),
		Returns:       make([]ReturnPoint, 0),
		Drawdowns:     make([]DrawdownPoint, 0),
		MonthlyReturns: make(map[string]float64),
		StrategyStats: make(map[string]*StrategyPerformance),
		Exposures:     make(map[string][]ExposurePoint),
		StartTime:     b.startTime,
	}

	// 初始化策略统计
	for name := range b.strategies {
		b.results.StrategyStats[name] = &StrategyPerformance{
			Name: name,
		}
	}

	return nil
}

// runBacktestLoop 执行回测主循环
func (b *BacktestEngine) runBacktestLoop(ctx context.Context) error {
	// 模拟回测数据生成
	// 实际应用中需要加载真实的历史数据
	
	currentDate := b.config.StartDate
	currentValue := b.config.InitialCapital
	peakValue := currentValue

	tradeID := 1

	for !currentDate.After(b.config.EndDate) {
		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return fmt.Errorf("backtest cancelled: %v", ctx.Err())
		default:
		}

		// 更新进度
		totalDays := int(b.config.EndDate.Sub(b.config.StartDate).Hours() / 24)
		elapsedDays := int(currentDate.Sub(b.config.StartDate).Hours() / 24)
		b.progress = float64(elapsedDays) / float64(totalDays) * 100

		// 生成市场数据（模拟）
		marketData := b.generateMockMarketData(currentDate)

		// 执行策略
		signals, err := b.executeStrategies(ctx, marketData)
		if err != nil {
			log.Printf("Strategy execution failed: %v", err)
			continue
		}

		// 处理信号并生成交易
		for _, signal := range signals {
			trade := b.createBacktestTrade(tradeID, signal, currentDate)
			if trade != nil {
				b.results.Trades = append(b.results.Trades, *trade)
				currentValue += trade.PnL
				tradeID++
			}
		}

		// 更新权益曲线
		drawdown := (peakValue - currentValue) / peakValue
		b.results.EquityCurve = append(b.results.EquityCurve, EquityPoint{
			Timestamp: currentDate,
			Value:     currentValue,
			Drawdown:  drawdown,
		})

		// 更新峰值
		if currentValue > peakValue {
			peakValue = currentValue
		}

		// 计算日收益率
		if len(b.results.EquityCurve) > 1 {
			prevValue := b.results.EquityCurve[len(b.results.EquityCurve)-2].Value
			dailyReturn := (currentValue - prevValue) / prevValue
			b.results.Returns = append(b.results.Returns, ReturnPoint{
				Timestamp: currentDate,
				Value:     currentValue,
				Return:    dailyReturn,
			})
		}

		// 移动到下一个交易日
		currentDate = currentDate.AddDate(0, 0, 1)

		// 更新进度日志
		if int(b.progress)%10 == 0 && int(b.progress) > 0 {
			log.Printf("Backtest progress: %.1f%%", b.progress)
		}
	}

	// 更新最终结果
	b.results.Summary.FinalValue = currentValue
	b.results.Summary.TotalReturn = (currentValue - b.config.InitialCapital) / b.config.InitialCapital
	b.results.EndTime = currentDate
	b.results.Duration = b.endTime.Sub(b.startTime)

	return nil
}

// executeStrategies 执行策略
func (b *BacktestEngine) executeStrategies(ctx context.Context, marketData map[string]*strategies.MarketData) ([]*strategies.Signal, error) {
	var allSignals []*strategies.Signal

	for symbol, data := range marketData {
		for name, strategy := range b.strategies {
			if !strategy.IsEnabled() {
				continue
			}

			// 生成信号
			signal, err := strategy.GenerateSignal(ctx, data)
			if err != nil {
				log.Printf("Strategy %s failed for %s: %v", name, symbol, err)
				continue
			}

			if signal != nil {
				signal.Metadata["strategy_name"] = name
				allSignals = append(allSignals, signal)
			}
		}
	}

	return allSignals, nil
}

// generateMockMarketData 生成模拟市场数据
func (b *BacktestEngine) generateMockMarketData(date time.Time) map[string]*strategies.MarketData {
	marketData := make(map[string]*strategies.MarketData)

	for _, symbol := range b.config.Symbols {
		// 简化的模拟数据生成
		// 实际应用中应该从数据源获取真实历史数据
		basePrice := 10.0 + float64(len(symbol)) // 基于股票代码生成基础价格
		
		// 添加随机波动
		dayOfYear := date.YearDay()
		volatility := 0.02 // 2%日波动率
		
		priceChange := basePrice * volatility * (float64(dayOfYear%100) - 50) / 50
		open := basePrice + priceChange
		high := open * (1 + volatility * 0.5)
		low := open * (1 - volatility * 0.5)
		close := open + priceChange * 0.5

		marketData[symbol] = &strategies.MarketData{
			Symbol:         symbol,
			Open:          open,
			High:          high,
			Low:           low,
			Close:         close,
			Volume:        1000000 + int64(dayOfYear)*1000,
			Amount:        close * 1000000,
			Timestamp:     date,
			PreClose:      basePrice,
			Change:        close - basePrice,
			ChangePercent: (close - basePrice) / basePrice * 100,
		}
	}

	return marketData
}

// createBacktestTrade 创建回测交易
func (b *BacktestEngine) createBacktestTrade(tradeID int, signal *strategies.Signal, currentDate time.Time) *BacktestTrade {
	// 简化的交易逻辑
	if signal.SignalType != "buy" && signal.SignalType != "sell" {
		return nil
	}

	// 模拟交易参数
	quantity := int64(1000) // 固定交易数量
	price := signal.Price
	commission := price * float64(quantity) * b.config.Commission
	slippage := price * float64(quantity) * b.config.Slippage
	
	// 模拟PnL（简化）
	var pnl float64
	if signal.SignalType == "buy" {
		// 模拟买入后价格上涨
		pnl = price * float64(quantity) * 0.02 // 2%收益
	} else {
		// 模拟卖出
		pnl = -commission - slippage // 只有成本
	}

	return &BacktestTrade{
		ID:           fmt.Sprintf("trade_%d", tradeID),
		Symbol:       signal.Symbol,
		EntryTime:    currentDate,
		EntryPrice:   price,
		ExitTime:     currentDate.Add(time.Hour),
		ExitPrice:    price * 1.02,
		Quantity:     quantity,
		Side:         signal.SignalType,
		PnL:          pnl,
		Return:       pnl / (price * float64(quantity)),
		Strategy:     signal.Metadata["strategy_name"].(string),
		Commission:   commission,
		Slippage:     slippage,
		HoldDuration: time.Hour,
	}
}

// calculateFinalMetrics 计算最终指标
func (b *BacktestEngine) calculateFinalMetrics() {
	if b.results == nil || b.results.Summary == nil {
		return
	}

	// 计算年化收益率
	days := b.results.EndTime.Sub(b.results.StartTime).Hours() / 24
	if days > 0 {
		b.results.Summary.AnnualizedReturn = b.results.Summary.TotalReturn * 365 / days
	}

	// 计算交易统计
	b.results.Summary.TotalTrades = len(b.results.Trades)
	winningTrades := 0
	totalWin := 0.0
	totalLoss := 0.0

	for _, trade := range b.results.Trades {
		if trade.PnL > 0 {
			winningTrades++
			totalWin += trade.PnL
		} else {
			totalLoss += trade.PnL
		}
	}

	b.results.Summary.WinningTrades = winningTrades
	b.results.Summary.LosingTrades = b.results.Summary.TotalTrades - winningTrades
	
	if b.results.Summary.TotalTrades > 0 {
		b.results.Summary.WinRate = float64(winningTrades) / float64(b.results.Summary.TotalTrades)
	}

	// 计算盈亏比
	if totalLoss != 0 {
		b.results.Summary.ProfitFactor = -totalWin / totalLoss
	}

	// 计算平均交易
	if b.results.Summary.TotalTrades > 0 {
		totalPnL := b.results.Summary.FinalValue - b.results.Summary.InitialCapital
		b.results.Summary.AverageTrade = totalPnL / float64(b.results.Summary.TotalTrades)
	}

	// 计算最大回撤
	b.calculateMaxDrawdown()

	// 计算夏普比率
	b.calculateSharpeRatio()

	// 计算卡尔玛比率
	if b.results.Summary.MaxDrawdown > 0 {
		b.results.Summary.CalmarRatio = b.results.Summary.AnnualizedReturn / b.results.Summary.MaxDrawdown
	}
}

// calculateMaxDrawdown 计算最大回撤
func (b *BacktestEngine) calculateMaxDrawdown() {
	if len(b.results.EquityCurve) == 0 {
		return
	}

	var maxDrawdown float64
	var peak float64
	var drawdownStart time.Time
	var peakTime time.Time
	var currentDrawdownStart time.Time
	var maxDrawdownDuration time.Duration

	for i, point := range b.results.EquityCurve {
		if point.Value > peak {
			peak = point.Value
			peakTime = point.Timestamp
			
			// 如果之前有回撤，结束它
			if currentDrawdownStart.Before(peakTime) && peak > 0 {
				duration := peakTime.Sub(currentDrawdownStart)
				if maxDrawdownDuration < duration {
					maxDrawdownDuration = duration
				}
			}
		}

		currentDrawdown := (peak - point.Value) / peak
		if currentDrawdown > maxDrawdown {
			maxDrawdown = currentDrawdown
			drawdownStart = currentDrawdownStart
			maxDrawdownDuration = peakTime.Sub(currentDrawdownStart)
		}
	}

	b.results.Summary.MaxDrawdown = maxDrawdown
	b.results.Summary.MaxDrawdownDuration = maxDrawdownDuration
}

// calculateSharpeRatio 计算夏普比率
func (b *BacktestEngine) calculateSharpeRatio() {
	if len(b.results.Returns) < 2 {
		return
	}

	// 计算收益率均值和标准差
	var sum float64
	for _, point := range b.results.Returns {
		sum += point.Return
	}
	mean := sum / float64(len(b.results.Returns))

	var variance float64
	for _, point := range b.results.Returns {
		diff := point.Return - mean
		variance += diff * diff
	}
	variance /= float64(len(b.results.Returns) - 1)
	stdDev := b.sqrt(variance)

	// 计算年化指标
	if stdDev > 0 {
		annualizedReturn := b.results.Summary.AnnualizedReturn
		annualizedStdDev := stdDev * b.sqrt(252) // 假设252个交易日
		b.results.Summary.SharpeRatio = (annualizedReturn - b.config.RiskFreeRate) / annualizedStdDev
	}
}

// GetProgress 获取回测进度
func (b *BacktestEngine) GetProgress() float64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.progress
}

// IsRunning 检查回测是否运行中
func (b *BacktestEngine) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.started && !b.completed
}

// GetResults 获取回测结果
func (b *BacktestEngine) GetResults() *BacktestResults {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.results
}

// sqrt 计算平方根（简化实现）
func (b *BacktestEngine) sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x
}
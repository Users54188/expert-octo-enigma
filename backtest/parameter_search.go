package backtest

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ParameterSearch 参数优化器
type ParameterSearch struct {
	mu        sync.RWMutex
	config    *SearchConfig
	engine    *BacktestEngine
	results   map[string]*OptimizationResult
	started   bool
	completed bool
	progress  float64
}

// SearchConfig 搜索配置
type SearchConfig struct {
	Method          string                     `yaml:"method"`           // 优化方法: grid_search, random_search, bayesian
	Metric          string                     `yaml:"metric"`           // 优化目标: sharpe_ratio, total_return, max_drawdown
	MaxIterations   int                        `yaml:"max_iterations"`   // 最大迭代次数
	MinSamples      int                        `yaml:"min_samples"`      // 最小样本数
	Parameters      map[string]ParameterConfig `yaml:"parameters"`       // 参数配置
	Constraints     map[string]Constraint      `yaml:"constraints"`      // 约束条件
	Parallel        bool                       `yaml:"parallel"`         // 是否并行
	MaxWorkers      int                        `yaml:"max_workers"`      // 最大工作协程数
	RandomSeed      int64                      `yaml:"random_seed"`      // 随机种子
	Timeout         time.Duration              `yaml:"timeout"`          // 超时时间
	EarlyStopping   bool                       `yaml:"early_stopping"`   // 早停机制
	Patience        int                        `yaml:"patience"`         // 早停耐心值
	ValidationSplit float64                    `yaml:"validation_split"` // 验证集比例
}

// ParameterConfig 参数配置
type ParameterConfig struct {
	Name   string        `yaml:"name"`   // 参数名称
	Type   string        `yaml:"type"`   // 参数类型: int, float, string
	Min    interface{}   `yaml:"min"`    // 最小值
	Max    interface{}   `yaml:"max"`    // 最大值
	Step   interface{}   `yaml:"step"`   // 步长
	Values []interface{} `yaml:"values"` // 特定值列表
}

// Constraint 约束条件
type Constraint struct {
	Expression string  `yaml:"expression"` // 约束表达式
	Weight     float64 `yaml:"weight"`     // 权重
}

// OptimizationResult 优化结果
type OptimizationResult struct {
	Parameters         map[string]interface{} `json:"parameters"`          // 最优参数
	Metric             float64                `json:"metric"`              // 优化指标值
	BacktestResults    *BacktestResults       `json:"backtest_results"`    // 回测结果
	Rank               int                    `json:"rank"`                // 排名
	StdError           float64                `json:"std_error"`           // 标准误差
	ConfidenceInterval [2]float64             `json:"confidence_interval"` // 置信区间
	Duration           time.Duration          `json:"duration"`            // 优化耗时
	Iterations         int                    `json:"iterations"`          // 迭代次数
	Timestamp          time.Time              `json:"timestamp"`
}

// SearchIteration 搜索迭代
type SearchIteration struct {
	ID              int                    `json:"id"`
	Parameters      map[string]interface{} `json:"parameters"`
	Metric          float64                `json:"metric"`
	BacktestResults *BacktestResults       `json:"backtest_results"`
	Duration        time.Duration          `json:"duration"`
	Timestamp       time.Time              `json:"timestamp"`
	Status          string                 `json:"status"` // running, completed, failed
	Error           string                 `json:"error,omitempty"`
}

// ParameterSpace 参数空间
type ParameterSpace struct {
	Dimensions []ParameterDimension `json:"dimensions"`
	TotalSize  int                  `json:"total_size"`
}

// ParameterDimension 参数维度
type ParameterDimension struct {
	Name   string        `json:"name"`
	Type   string        `json:"type"`
	Values []interface{} `json:"values"`
}

// NewParameterSearch 创建参数优化器
func NewParameterSearch(config SearchConfig, engine *BacktestEngine) *ParameterSearch {
	return &ParameterSearch{
		config:   &config,
		engine:   engine,
		results:  make(map[string]*OptimizationResult),
		progress: 0.0,
	}
}

// Optimize 执行参数优化
func (p *ParameterSearch) Optimize(ctx context.Context) (*OptimizationResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil, fmt.Errorf("parameter search is already running")
	}

	p.started = true
	p.completed = false

	defer func() {
		p.completed = true
	}()

	log.Printf("Starting parameter optimization: method=%s, metric=%s", p.config.Method, p.config.Metric)

	// 构建参数空间
	parameterSpace, err := p.buildParameterSpace()
	if err != nil {
		return nil, fmt.Errorf("failed to build parameter space: %v", err)
	}

	log.Printf("Parameter space built: %d combinations", parameterSpace.TotalSize)

	// 执行优化搜索
	var bestResult *OptimizationResult
	var allResults []SearchIteration

	switch p.config.Method {
	case "grid_search":
		bestResult, allResults, err = p.gridSearch(ctx, parameterSpace)
	case "random_search":
		bestResult, allResults, err = p.randomSearch(ctx, parameterSpace)
	default:
		return nil, fmt.Errorf("unsupported optimization method: %s", p.config.Method)
	}

	if err != nil {
		return nil, fmt.Errorf("optimization failed: %v", err)
	}

	// 存储所有结果
	p.storeResults(allResults)

	log.Printf("Parameter optimization completed: best_metric=%.4f, iterations=%d",
		bestResult.Metric, len(allResults))

	return bestResult, nil
}

// buildParameterSpace 构建参数空间
func (p *ParameterSearch) buildParameterSpace() (*ParameterSpace, error) {
	var dimensions []ParameterDimension
	totalSize := 1

	for paramName, paramConfig := range p.config.Parameters {
		dimension, size := p.buildParameterDimension(paramName, paramConfig)
		dimensions = append(dimensions, dimension)
		totalSize *= size

		// 检查参数空间是否过大
		if totalSize > 100000 {
			log.Printf("Large parameter space detected: %d combinations", totalSize)
		}
	}

	return &ParameterSpace{
		Dimensions: dimensions,
		TotalSize:  totalSize,
	}, nil
}

// buildParameterDimension 构建参数维度
func (p *ParameterSearch) buildParameterDimension(name string, config ParameterConfig) (ParameterDimension, int) {
	var values []interface{}

	switch config.Type {
	case "int":
		if config.Values != nil {
			values = config.Values
		} else {
			values = p.generateIntValues(config.Min.(int), config.Max.(int), config.Step.(int))
		}
	case "float":
		if config.Values != nil {
			values = config.Values
		} else {
			values = p.generateFloatValues(config.Min.(float64), config.Max.(float64), config.Step.(float64))
		}
	case "string":
		values = config.Values
	default:
		values = []interface{}{config.Min}
	}

	dimension := ParameterDimension{
		Name:   name,
		Type:   config.Type,
		Values: values,
	}

	return dimension, len(values)
}

// generateIntValues 生成整数值序列
func (p *ParameterSearch) generateIntValues(min, max, step int) []interface{} {
	var values []interface{}

	for i := min; i <= max; i += step {
		values = append(values, i)
	}

	return values
}

// generateFloatValues 生成浮点值序列
func (p *ParameterSearch) generateFloatValues(min, max, step float64) []interface{} {
	var values []interface{}

	for i := min; i <= max; i += step {
		values = append(values, i)
	}

	return values
}

// gridSearch 网格搜索
func (p *ParameterSearch) gridSearch(ctx context.Context, space *ParameterSpace) (*OptimizationResult, []SearchIteration, error) {
	startTime := time.Now()
	var iterations []SearchIteration
	var bestResult *OptimizationResult

	// 生成所有参数组合
	combinations := p.generateParameterCombinations(space, p.config.MaxIterations)

	log.Printf("Grid search: %d parameter combinations", len(combinations))

	iterationID := 1
	for _, params := range combinations {
		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("parameter search cancelled: %v", ctx.Err())
		default:
		}

		iterationStart := time.Now()

		// 执行回测
		backtestResults, err := p.runBacktestWithParams(params)
		if err != nil {
			log.Printf("Backtest failed for parameters %v: %v", params, err)
			iterations = append(iterations, SearchIteration{
				ID:         iterationID,
				Parameters: params,
				Metric:     -999, // 失败
				Duration:   time.Since(iterationStart),
				Timestamp:  startTime,
				Status:     "failed",
				Error:      err.Error(),
			})
			iterationID++
			continue
		}

		// 计算优化指标
		metric, err := p.calculateOptimizationMetric(backtestResults)
		if err != nil {
			log.Printf("Failed to calculate metric for parameters %v: %v", params, err)
			iterations = append(iterations, SearchIteration{
				ID:         iterationID,
				Parameters: params,
				Metric:     -999,
				Duration:   time.Since(iterationStart),
				Timestamp:  startTime,
				Status:     "failed",
				Error:      fmt.Sprintf("metric calculation failed: %v", err),
			})
			iterationID++
			continue
		}

		iteration := SearchIteration{
			ID:              iterationID,
			Parameters:      params,
			Metric:          metric,
			BacktestResults: backtestResults,
			Duration:        time.Since(iterationStart),
			Timestamp:       startTime,
			Status:          "completed",
		}

		iterations = append(iterations, iteration)

		// 更新最佳结果
		if bestResult == nil || p.isBetterResult(metric, bestResult.Metric) {
			bestResult = &OptimizationResult{
				Parameters:      params,
				Metric:          metric,
				BacktestResults: backtestResults,
				Duration:        time.Since(startTime),
				Iterations:      iterationID,
				Timestamp:       startTime,
			}
		}

		// 更新进度
		p.progress = float64(iterationID) / float64(len(combinations)) * 100
		iterationID++

		log.Printf("Grid search progress: %.1f%% (best_metric=%.4f)", p.progress, bestResult.Metric)
	}

	return bestResult, iterations, nil
}

// randomSearch 随机搜索
func (p *ParameterSearch) randomSearch(ctx context.Context, space *ParameterSpace) (*OptimizationResult, []SearchIteration, error) {
	startTime := time.Now()
	var iterations []SearchIteration
	var bestResult *OptimizationResult

	log.Printf("Random search: %d iterations", p.config.MaxIterations)

	for i := 1; i <= p.config.MaxIterations; i++ {
		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("parameter search cancelled: %v", ctx.Err())
		default:
		}

		iterationStart := time.Now()

		// 随机生成参数组合
		params := p.generateRandomParameters(space)

		// 执行回测
		backtestResults, err := p.runBacktestWithParams(params)
		if err != nil {
			log.Printf("Backtest failed for random parameters: %v", err)
			iterations = append(iterations, SearchIteration{
				ID:         i,
				Parameters: params,
				Metric:     -999,
				Duration:   time.Since(iterationStart),
				Timestamp:  startTime,
				Status:     "failed",
				Error:      err.Error(),
			})
			continue
		}

		// 计算优化指标
		metric, err := p.calculateOptimizationMetric(backtestResults)
		if err != nil {
			log.Printf("Failed to calculate metric: %v", err)
			iterations = append(iterations, SearchIteration{
				ID:         i,
				Parameters: params,
				Metric:     -999,
				Duration:   time.Since(iterationStart),
				Timestamp:  startTime,
				Status:     "failed",
				Error:      fmt.Sprintf("metric calculation failed: %v", err),
			})
			continue
		}

		iteration := SearchIteration{
			ID:              i,
			Parameters:      params,
			Metric:          metric,
			BacktestResults: backtestResults,
			Duration:        time.Since(iterationStart),
			Timestamp:       startTime,
			Status:          "completed",
		}

		iterations = append(iterations, iteration)

		// 更新最佳结果
		if bestResult == nil || p.isBetterResult(metric, bestResult.Metric) {
			bestResult = &OptimizationResult{
				Parameters:      params,
				Metric:          metric,
				BacktestResults: backtestResults,
				Duration:        time.Since(startTime),
				Iterations:      i,
				Timestamp:       startTime,
			}
		}

		// 更新进度
		p.progress = float64(i) / float64(p.config.MaxIterations) * 100

		log.Printf("Random search progress: %.1f%% (best_metric=%.4f)", p.progress, bestResult.Metric)
	}

	return bestResult, iterations, nil
}

// generateParameterCombinations 生成参数组合
func (p *ParameterSearch) generateParameterCombinations(space *ParameterSpace, maxCombinations int) []map[string]interface{} {
	if len(space.Dimensions) == 0 {
		return []map[string]interface{}{}
	}

	var combinations []map[string]interface{}

	// 递归生成所有组合
	p.generateCombinationsRecursive(space.Dimensions, 0, make(map[string]interface{}), &combinations, maxCombinations)

	return combinations
}

// generateCombinationsRecursive 递归生成参数组合
func (p *ParameterSearch) generateCombinationsRecursive(
	dimensions []ParameterDimension,
	index int,
	current map[string]interface{},
	combinations *[]map[string]interface{},
	maxCombinations int,
) {
	if len(*combinations) >= maxCombinations {
		return
	}

	if index == len(dimensions) {
		// 复制当前组合
		combo := make(map[string]interface{})
		for k, v := range current {
			combo[k] = v
		}
		*combinations = append(*combinations, combo)
		return
	}

	dimension := dimensions[index]
	for _, value := range dimension.Values {
		current[dimension.Name] = value
		p.generateCombinationsRecursive(dimensions, index+1, current, combinations, maxCombinations)
	}
}

// generateRandomParameters 生成随机参数
func (p *ParameterSearch) generateRandomParameters(space *ParameterSpace) map[string]interface{} {
	params := make(map[string]interface{})

	for _, dimension := range space.Dimensions {
		if len(dimension.Values) == 0 {
			continue
		}

		// 简单随机选择
		index := int(time.Now().UnixNano()) % len(dimension.Values)
		params[dimension.Name] = dimension.Values[index]
	}

	return params
}

// runBacktestWithParams 使用指定参数运行回测
func (p *ParameterSearch) runBacktestWithParams(params map[string]interface{}) (*BacktestResults, error) {
	// 创建回测配置副本
	config := *p.engine.config

	// 应用参数到策略
	if err := p.applyParametersToStrategies(params); err != nil {
		return nil, fmt.Errorf("failed to apply parameters: %v", err)
	}

	// 创建新的回测引擎
	engine := NewBacktestEngine(config)

	// 复制策略
	for _, strategy := range p.engine.strategies {
		if err := engine.AddStrategy(strategy); err != nil {
			return nil, fmt.Errorf("failed to add strategy: %v", err)
		}
	}

	// 执行回测
	ctx := context.Background()
	results, err := engine.Run(ctx)
	if err != nil {
		return nil, fmt.Errorf("backtest failed: %v", err)
	}

	return results, nil
}

// applyParametersToStrategies 应用参数到策略
func (p *ParameterSearch) applyParametersToStrategies(params map[string]interface{}) error {
	for name, strategy := range p.engine.strategies {
		// 查找策略相关参数
		strategyParams := make(map[string]interface{})
		for paramName, value := range params {
			if len(paramName) > len(name)+1 && paramName[:len(name)+1] == name+"." {
				cleanParamName := paramName[len(name)+1:]
				strategyParams[cleanParamName] = value
			}
		}

		// 应用参数
		if len(strategyParams) > 0 {
			if err := strategy.UpdateParameters(strategyParams); err != nil {
				log.Printf("Failed to update parameters for strategy %s: %v", name, err)
			}
		}
	}

	return nil
}

// calculateOptimizationMetric 计算优化指标
func (p *ParameterSearch) calculateOptimizationMetric(results *BacktestResults) (float64, error) {
	if results == nil || results.Summary == nil {
		return 0, fmt.Errorf("invalid backtest results")
	}

	summary := results.Summary

	switch p.config.Metric {
	case "sharpe_ratio":
		return summary.SharpeRatio, nil
	case "total_return":
		return summary.TotalReturn, nil
	case "annualized_return":
		return summary.AnnualizedReturn, nil
	case "max_drawdown":
		return -summary.MaxDrawdown, nil // 负值，最小回撤越好
	case "calmar_ratio":
		return summary.CalmarRatio, nil
	case "sortino_ratio":
		return summary.SortinoRatio, nil
	case "information_ratio":
		return summary.InformationRatio, nil
	case "profit_factor":
		return summary.ProfitFactor, nil
	case "win_rate":
		return summary.WinRate, nil
	case "total_trades":
		return float64(summary.TotalTrades), nil
	default:
		return summary.SharpeRatio, nil // 默认使用夏普比率
	}
}

// isBetterResult 判断是否为更好的结果
func (p *ParameterSearch) isBetterResult(newMetric, currentMetric float64) bool {
	switch p.config.Metric {
	case "max_drawdown":
		return newMetric > currentMetric // 回撤越小越好（已经是负值）
	case "total_trades":
		return newMetric < currentMetric // 交易次数越少越好（避免过度交易）
	default:
		return newMetric > currentMetric // 其他指标越大越好
	}
}

// storeResults 存储结果
func (p *ParameterSearch) storeResults(iterations []SearchIteration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, iteration := range iterations {
		if iteration.Status == "completed" {
			key := fmt.Sprintf("result_%d", i)
			p.results[key] = &OptimizationResult{
				Parameters:      iteration.Parameters,
				Metric:          iteration.Metric,
				BacktestResults: iteration.BacktestResults,
				Rank:            i + 1,
				Duration:        iteration.Duration,
				Iterations:      i + 1,
				Timestamp:       iteration.Timestamp,
			}
		}
	}
}

// GetBestResult 获取最佳结果
func (p *ParameterSearch) GetBestResult() *OptimizationResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *OptimizationResult
	for _, result := range p.results {
		if best == nil || p.isBetterResult(result.Metric, best.Metric) {
			best = result
		}
	}

	return best
}

// GetAllResults 获取所有结果
func (p *ParameterSearch) GetAllResults() map[string]*OptimizationResult {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*OptimizationResult)
	for k, v := range p.results {
		result[k] = v
	}

	return result
}

// GetTopResults 获取前N个结果
func (p *ParameterSearch) GetTopResults(n int) []*OptimizationResult {
	results := make([]*OptimizationResult, 0, len(p.results))

	for _, result := range p.results {
		results = append(results, result)
	}

	// 排序
	if p.config.Metric == "max_drawdown" {
		// 回撤越小越好
		quickSort(results, func(a, b *OptimizationResult) bool {
			return a.Metric > b.Metric
		})
	} else {
		// 其他指标越大越好
		quickSort(results, func(a, b *OptimizationResult) bool {
			return a.Metric < b.Metric
		})
	}

	if len(results) > n {
		results = results[:n]
	}

	return results
}

// GetProgress 获取优化进度
func (p *ParameterSearch) GetProgress() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.progress
}

// IsRunning 检查是否正在运行
func (p *ParameterSearch) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.started && !p.completed
}

// GetParameterSpace 获取参数空间信息
func (p *ParameterSearch) GetParameterSpace() *ParameterSpace {
	space, _ := p.buildParameterSpace()
	return space
}

// GetStats 获取优化统计
func (p *ParameterSearch) GetStats() *SearchStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var completed int
	var failed int
	var totalDuration time.Duration

	for _, result := range p.results {
		if result.BacktestResults != nil {
			completed++
			totalDuration += result.Duration
		}
	}
	failed = len(p.results) - completed

	return &SearchStats{
		TotalResults: len(p.results),
		Completed:    completed,
		Failed:       failed,
		Progress:     p.progress,
		AvgDuration:  time.Duration(0),
		BestMetric:   p.GetBestResult().Metric,
		Started:      p.started,
		IsCompleted:  p.completed,
	}
}

// SearchStats 搜索统计
type SearchStats struct {
	TotalResults int           `json:"total_results"`
	Completed    int           `json:"completed"`
	Failed       int           `json:"failed"`
	Progress     float64       `json:"progress"`
	AvgDuration  time.Duration `json:"avg_duration"`
	BestMetric   float64       `json:"best_metric"`
	Started      bool          `json:"started"`
	IsCompleted  bool          `json:"is_completed"`
}

// 简单的快速排序实现
func quickSort(arr []*OptimizationResult, less func(a, b *OptimizationResult) bool) {
	if len(arr) <= 1 {
		return
	}

	pivot := arr[len(arr)/2]
	left, right := 0, len(arr)-1

	// 移动pivot到右侧
	arr[len(arr)/2], arr[right] = arr[right], arr[len(arr)/2]

	// 分区
	for i := range arr {
		if less(arr[i], pivot) {
			arr[i], arr[left] = arr[left], arr[i]
			left++
		}
	}

	// 移动pivot到正确位置
	arr[left], arr[right] = arr[right], arr[left]

	// 递归排序
	quickSort(arr[:left], less)
	quickSort(arr[left+1:], less)
}

package monitoring

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
	MetricTypeSummary   MetricType = "summary"
)

// Metric 指标
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Labels    map[string]string      `json:"labels,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Help      string                 `json:"help,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MetricsCollector 指标收集器
type MetricsCollector struct {
	metrics     map[string][]*Metric
	metricsLock sync.RWMutex

	startTime time.Time
}

// NewMetricsCollector 创建指标收集器
func NewMetricsCollector() *MetricsCollector {
	collector := &MetricsCollector{
		metrics:   make(map[string][]*Metric),
		startTime: time.Now(),
	}

	// 启动系统指标收集
	go collector.collectSystemMetrics()

	return collector
}

// RecordMetric 记录指标
func (mc *MetricsCollector) RecordMetric(metric *Metric) {
	mc.metricsLock.Lock()
	defer mc.metricsLock.Unlock()

	metric.Timestamp = time.Now()

	if _, ok := mc.metrics[metric.Name]; !ok {
		mc.metrics[metric.Name] = make([]*Metric, 0)
	}

	mc.metrics[metric.Name] = append(mc.metrics[metric.Name], metric)

	// 限制历史大小（保留最近1000个）
	if len(mc.metrics[metric.Name]) > 1000 {
		mc.metrics[metric.Name] = mc.metrics[metric.Name][100:]
	}
}

// GetMetric 获取指标
func (mc *MetricsCollector) GetMetric(name string) ([]*Metric, error) {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()

	metrics, ok := mc.metrics[name]
	if !ok {
		return nil, fmt.Errorf("metric %s not found", name)
	}

	// 返回副本
	result := make([]*Metric, len(metrics))
	for i, m := range metrics {
		metricCopy := *m
		result[i] = &metricCopy
	}

	return result, nil
}

// GetAllMetrics 获取所有指标
func (mc *MetricsCollector) GetAllMetrics() map[string][]*Metric {
	mc.metricsLock.RLock()
	defer mc.metricsLock.RUnlock()

	result := make(map[string][]*Metric)
	for name, metrics := range mc.metrics {
		metricCopy := make([]*Metric, len(metrics))
		for i, m := range metrics {
			m := *m
			metricCopy[i] = &m
		}
		result[name] = metricCopy
	}

	return result
}

// GetMetricSummary 获取指标摘要
func (mc *MetricsCollector) GetMetricSummary(name string) (map[string]interface{}, error) {
	metrics, err := mc.GetMetric(name)
	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return map[string]interface{}{
			"count": 0,
		}, nil
	}

	summary := map[string]interface{}{
		"count":     len(metrics),
		"latest":    metrics[len(metrics)-1].Value,
		"earliest":  metrics[0].Value,
		"min":       metrics[0].Value,
		"max":       metrics[0].Value,
		"timestamp": metrics[len(metrics)-1].Timestamp,
	}

	sum := 0.0
	for _, m := range metrics {
		sum += m.Value
		if m.Value < summary["min"].(float64) {
			summary["min"] = m.Value
		}
		if m.Value > summary["max"].(float64) {
			summary["max"] = m.Value
		}
	}

	summary["average"] = sum / float64(len(metrics))
	summary["name"] = name

	return summary, nil
}

// collectSystemMetrics 收集系统指标
func (mc *MetricsCollector) collectSystemMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mc.collectCPUMetrics()
		mc.collectMemoryMetrics()
		mc.collectGoroutineMetrics()
	}
}

// collectCPUMetrics 收集CPU指标
func (mc *MetricsCollector) collectCPUMetrics() {
	// CPU使用率（简化版）
	mc.RecordMetric(&Metric{
		Name:  "system_cpu_usage",
		Type:  MetricTypeGauge,
		Value: 0.0,
		Help:  "System CPU usage percentage",
	})
}

// collectMemoryMetrics 收集内存指标
func (mc *MetricsCollector) collectMemoryMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	mc.RecordMetric(&Metric{
		Name:  "memory_heap_alloc",
		Type:  MetricTypeGauge,
		Value: float64(m.HeapAlloc),
		Help:  "Memory heap allocated in bytes",
	})

	mc.RecordMetric(&Metric{
		Name:  "memory_heap_sys",
		Type:  MetricTypeGauge,
		Value: float64(m.HeapSys),
		Help:  "Memory heap system bytes",
	})

	mc.RecordMetric(&Metric{
		Name:  "memory_gc_count",
		Type:  MetricTypeCounter,
		Value: float64(m.NumGC),
		Help:  "Number of garbage collections",
	})
}

// collectGoroutineMetrics 收集协程指标
func (mc *MetricsCollector) collectGoroutineMetrics() {
	mc.RecordMetric(&Metric{
		Name:  "system_goroutines",
		Type:  MetricTypeGauge,
		Value: float64(runtime.NumGoroutine()),
		Help:  "Number of goroutines",
	})
}

// IncrCounter 增加计数器
func (mc *MetricsCollector) IncrCounter(name string, value float64, labels map[string]string) {
	metric := &Metric{
		Name:   name,
		Type:   MetricTypeCounter,
		Value:  value,
		Labels: labels,
	}
	mc.RecordMetric(metric)
}

// SetGauge 设置仪表
func (mc *MetricsCollector) SetGauge(name string, value float64, labels map[string]string) {
	metric := &Metric{
		Name:   name,
		Type:   MetricTypeGauge,
		Value:  value,
		Labels: labels,
	}
	mc.RecordMetric(metric)
}

// RecordHistogram 记录直方图
func (mc *MetricsCollector) RecordHistogram(name string, value float64, labels map[string]string, buckets []float64) {
	// 记录原始值
	metric := &Metric{
		Name:   name,
		Type:   MetricTypeHistogram,
		Value:  value,
		Labels: labels,
		Metadata: map[string]interface{}{
			"buckets": buckets,
		},
	}
	mc.RecordMetric(metric)
}

// ExportPrometheus 导出Prometheus格式
func (mc *MetricsCollector) ExportPrometheus() string {
	var output string

	metrics := mc.GetAllMetrics()
	for name, metricList := range metrics {
		if len(metricList) == 0 {
			continue
		}

		metric := metricList[len(metricList)-1]
		help := metric.Help
		if help == "" {
			help = fmt.Sprintf("Metric %s", name)
		}

		output += fmt.Sprintf("# HELP %s %s\n", name, help)
		output += fmt.Sprintf("# TYPE %s %s\n", name, metric.Type)

		// 输出最新值
		labelsStr := ""
		if len(metric.Labels) > 0 {
			labels := make([]string, 0, len(metric.Labels))
			for k, v := range metric.Labels {
				labels = append(labels, fmt.Sprintf(`%s="%s"`, k, v))
			}
			labelsStr = fmt.Sprintf("{%s}", fmt.Sprintf("%s", labels))
		}
		output += fmt.Sprintf("%s%s %f %d\n", name, labelsStr, metric.Value, metric.Timestamp.Unix())
	}

	return output
}

// ExportJSON 导出JSON格式
func (mc *MetricsCollector) ExportJSON() (string, error) {
	metrics := mc.GetAllMetrics()

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetUptime 获取运行时间
func (mc *MetricsCollector) GetUptime() time.Duration {
	return time.Since(mc.startTime)
}

// GetSystemStats 获取系统统计
func (mc *MetricsCollector) GetSystemStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]interface{}{
		"uptime":     mc.GetUptime().String(),
		"goroutines": runtime.NumGoroutine(),
		"memory": map[string]interface{}{
			"alloc":        m.Alloc,
			"total_alloc":  m.TotalAlloc,
			"sys":          m.Sys,
			"heap_alloc":   m.HeapAlloc,
			"heap_sys":     m.HeapSys,
			"heap_inuse":   m.HeapInuse,
			"heap_idle":    m.HeapIdle,
			"heap_objects": m.HeapObjects,
			"gc_count":     m.NumGC,
			"gc_pause_ns":  m.PauseTotalNs,
		},
		"num_cpu":      runtime.NumCPU(),
		"num_cgo_call": runtime.NumCgoCall(),
	}
}

// BusinessMetrics 业务指标
type BusinessMetrics struct {
	metricsLock sync.RWMutex

	tradeCount    int64
	tradeVolume   float64
	tradeAmount   float64
	orderCount    int64
	positionCount int
	strategyStats map[string]*StrategyStat
}

// StrategyStat 策略统计
type StrategyStat struct {
	Name        string  `json:"name"`
	SignalCount int64   `json:"signal_count"`
	TradeCount  int64   `json:"trade_count"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
	MaxDrawdown float64 `json:"max_drawdown"`
}

// NewBusinessMetrics 创建业务指标
func NewBusinessMetrics() *BusinessMetrics {
	return &BusinessMetrics{
		strategyStats: make(map[string]*StrategyStat),
	}
}

// RecordTrade 记录交易
func (bm *BusinessMetrics) RecordTrade(volume, amount float64) {
	bm.metricsLock.Lock()
	defer bm.metricsLock.Unlock()

	bm.tradeCount++
	bm.tradeVolume += volume
	bm.tradeAmount += amount
}

// RecordOrder 记录订单
func (bm *BusinessMetrics) RecordOrder() {
	bm.metricsLock.Lock()
	defer bm.metricsLock.Unlock()

	bm.orderCount++
}

// SetPositionCount 设置持仓数
func (bm *BusinessMetrics) SetPositionCount(count int) {
	bm.metricsLock.Lock()
	defer bm.metricsLock.Unlock()

	bm.positionCount = count
}

// RecordStrategySignal 记录策略信号
func (bm *BusinessMetrics) RecordStrategySignal(strategyName string) {
	bm.metricsLock.Lock()
	defer bm.metricsLock.Unlock()

	if _, ok := bm.strategyStats[strategyName]; !ok {
		bm.strategyStats[strategyName] = &StrategyStat{Name: strategyName}
	}
	bm.strategyStats[strategyName].SignalCount++
}

// RecordStrategyTrade 记录策略交易
func (bm *BusinessMetrics) RecordStrategyTrade(strategyName string, pnl float64) {
	bm.metricsLock.Lock()
	defer bm.metricsLock.Unlock()

	if _, ok := bm.strategyStats[strategyName]; !ok {
		bm.strategyStats[strategyName] = &StrategyStat{Name: strategyName}
	}

	stat := bm.strategyStats[strategyName]
	stat.TradeCount++
	stat.TotalPnL += pnl

	// 更新胜率（简化版）
	// 实际应该基于历史交易计算
}

// GetBusinessStats 获取业务统计
func (bm *BusinessMetrics) GetBusinessStats() map[string]interface{} {
	bm.metricsLock.RLock()
	defer bm.metricsLock.RUnlock()

	strategies := make([]*StrategyStat, 0, len(bm.strategyStats))
	for _, stat := range bm.strategyStats {
		statCopy := *stat
		strategies = append(strategies, &statCopy)
	}

	return map[string]interface{}{
		"trade_count":    bm.tradeCount,
		"trade_volume":   bm.tradeVolume,
		"trade_amount":   bm.tradeAmount,
		"order_count":    bm.orderCount,
		"position_count": bm.positionCount,
		"strategy_stats": strategies,
	}
}

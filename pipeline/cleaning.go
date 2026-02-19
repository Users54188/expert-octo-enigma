package pipeline

import (
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// CleaningRule 清洗规则
type CleaningRule interface {
	Apply(*DataPoint) (*DataPoint, error)
	Name() string
}

// QualityIssue 质量问题
type QualityIssue struct {
	Type      string    `json:"type"`
	Severity  string    `json:"severity"` // low, medium, high
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
}

// DataCleaner 数据清洗器
type DataCleaner struct {
	rules      []CleaningRule
	issues     []QualityIssue
	issuesLock sync.RWMutex

	stats     CleaningStats
	statsLock sync.RWMutex
}

// CleaningStats 清洗统计
type CleaningStats struct {
	TotalProcessed int64            `json:"total_processed"`
	Passed         int64            `json:"passed"`
	Rejected       int64            `json:"rejected"`
	Corrected      int64            `json:"corrected"`
	Issues         map[string]int64 `json:"issues"`
	LastClean      time.Time        `json:"last_clean"`
}

// NewDataCleaner 创建数据清洗器
func NewDataCleaner() *DataCleaner {
	cleaner := &DataCleaner{
		rules:  make([]CleaningRule, 0),
		issues: make([]QualityIssue, 0),
		stats: CleaningStats{
			Issues: make(map[string]int64),
		},
	}

	// 添加默认规则
	cleaner.AddRule(NewPriceValidationRule())
	cleaner.AddRule(NewVolumeValidationRule())
	cleaner.AddRule(NewTimestampValidationRule())
	cleaner.AddRule(NewOutlierDetectionRule())
	cleaner.AddRule(NewDuplicateDetectionRule())

	return cleaner
}

// AddRule 添加清洗规则
func (dc *DataCleaner) AddRule(rule CleaningRule) {
	dc.rules = append(dc.rules, rule)
	log.Printf("Added cleaning rule: %s", rule.Name())
}

// Clean 清洗数据
func (dc *DataCleaner) Clean(points []*DataPoint) ([]*DataPoint, []QualityIssue) {
	var cleaned []*DataPoint
	var issues []QualityIssue

	dc.statsLock.Lock()
	defer dc.statsLock.Unlock()

	for _, point := range points {
		dc.stats.TotalProcessed++

		originalPoint := *point
		var pointIssues []QualityIssue

		// 应用所有规则
		for _, rule := range dc.rules {
			cleanedPoint, err := rule.Apply(point)
			if err != nil {
				issue := QualityIssue{
					Type:      rule.Name(),
					Severity:  "high",
					Message:   err.Error(),
					Timestamp: time.Now(),
					Symbol:    point.Symbol,
				}
				pointIssues = append(pointIssues, issue)
				dc.recordIssue(rule.Name())
				continue
			}

			if cleanedPoint != nil {
				point = cleanedPoint
			}
		}

		// 处理问题
		if len(pointIssues) > 0 {
			dc.stats.Rejected++
			issues = append(issues, pointIssues...)
			dc.issuesLock.Lock()
			dc.issues = append(dc.issues, pointIssues...)
			dc.issuesLock.Unlock()
		} else {
			// 检查是否有修正
			if !dc.isEqual(&originalPoint, point) {
				dc.stats.Corrected++
			}
			dc.stats.Passed++
			cleaned = append(cleaned, point)
		}
	}

	dc.stats.LastClean = time.Now()

	return cleaned, issues
}

// isEqual 检查两个数据点是否相等
func (dc *DataCleaner) isEqual(a, b *DataPoint) bool {
	return a.Symbol == b.Symbol &&
		a.Timestamp == b.Timestamp &&
		a.Open == b.Open &&
		a.High == b.High &&
		a.Low == b.Low &&
		a.Close == b.Close &&
		a.Volume == b.Volume
}

// recordIssue 记录问题
func (dc *DataCleaner) recordIssue(issueType string) {
	dc.stats.Issues[issueType]++
}

// GetStats 获取统计信息
func (dc *DataCleaner) GetStats() CleaningStats {
	dc.statsLock.RLock()
	defer dc.statsLock.RUnlock()

	return dc.stats
}

// GetIssues 获取问题列表
func (dc *DataCleaner) GetIssues(limit int) []QualityIssue {
	dc.issuesLock.RLock()
	defer dc.issuesLock.RUnlock()

	if limit <= 0 || limit > len(dc.issues) {
		limit = len(dc.issues)
	}

	issues := make([]QualityIssue, limit)
	copy(issues, dc.issues[len(dc.issues)-limit:])
	return issues
}

// ClearIssues 清空问题列表
func (dc *DataCleaner) ClearIssues() {
	dc.issuesLock.Lock()
	defer dc.issuesLock.Unlock()

	dc.issues = make([]QualityIssue, 0)
}

// ============ 清洗规则实现 ============

// PriceValidationRule 价格验证规则
type PriceValidationRule struct {
	MinPrice     float64
	MaxPrice     float64
	MinChangePct float64
	MaxChangePct float64
}

func NewPriceValidationRule() *PriceValidationRule {
	return &PriceValidationRule{
		MinPrice:     0.01,
		MaxPrice:     1000000.0,
		MinChangePct: -0.2, // -20%
		MaxChangePct: 0.2,  // +20%
	}
}

func (r *PriceValidationRule) Name() string {
	return "price_validation"
}

func (r *PriceValidationRule) Apply(point *DataPoint) (*DataPoint, error) {
	// 检查价格是否在合理范围内
	if point.Close < r.MinPrice || point.Close > r.MaxPrice {
		return nil, fmt.Errorf("price %.2f out of range [%.2f, %.2f]", point.Close, r.MinPrice, r.MaxPrice)
	}

	// 检查高低价
	if point.High < point.Low {
		return nil, fmt.Errorf("high price %.2f less than low price %.2f", point.High, point.Low)
	}

	if point.Close < point.Low || point.Close > point.High {
		return nil, fmt.Errorf("close price %.2f outside range [%.2f, %.2f]", point.Close, point.Low, point.High)
	}

	// 检查价格跳变
	changePct := (point.Close - point.Open) / point.Open
	if changePct < r.MinChangePct || changePct > r.MaxChangePct {
		// 标记但允许通过，记录为警告
		return point, nil
	}

	return point, nil
}

// VolumeValidationRule 成交量验证规则
type VolumeValidationRule struct {
	MinVolume float64
	MaxVolume float64
}

func NewVolumeValidationRule() *VolumeValidationRule {
	return &VolumeValidationRule{
		MinVolume: 0,
		MaxVolume: 1e15,
	}
}

func (r *VolumeValidationRule) Name() string {
	return "volume_validation"
}

func (r *VolumeValidationRule) Apply(point *DataPoint) (*DataPoint, error) {
	if point.Volume < r.MinVolume || point.Volume > r.MaxVolume {
		return nil, fmt.Errorf("volume %.2f out of range [%.2f, %.2f]", point.Volume, r.MinVolume, r.MaxVolume)
	}

	// 成交量不能为负
	if point.Amount < 0 {
		return nil, fmt.Errorf("amount %.2f is negative", point.Amount)
	}

	return point, nil
}

// TimestampValidationRule 时间戳验证规则
type TimestampValidationRule struct {
	MaxFutureSeconds int64
}

func NewTimestampValidationRule() *TimestampValidationRule {
	return &TimestampValidationRule{
		MaxFutureSeconds: 300, // 允许未来5分钟（时钟差异）
	}
}

func (r *TimestampValidationRule) Name() string {
	return "timestamp_validation"
}

func (r *TimestampValidationRule) Apply(point *DataPoint) (*DataPoint, error) {
	now := time.Now().Unix()

	// 检查时间戳是否太旧（1970年之前）
	if point.Timestamp < 0 {
		return nil, fmt.Errorf("timestamp %d is invalid", point.Timestamp)
	}

	// 检查时间戳是否在未来
	if point.Timestamp > now+int64(r.MaxFutureSeconds) {
		return nil, fmt.Errorf("timestamp %d is too far in the future", point.Timestamp)
	}

	return point, nil
}

// OutlierDetectionRule 异常值检测规则
type OutlierDetectionRule struct {
	StdDevThreshold float64
}

func NewOutlierDetectionRule() *OutlierDetectionRule {
	return &OutlierDetectionRule{
		StdDevThreshold: 3.0, // 3个标准差
	}
}

func (r *OutlierDetectionRule) Name() string {
	return "outlier_detection"
}

func (r *OutlierDetectionRule) Apply(point *DataPoint) (*DataPoint, error) {
	// 简化版：这里只是占位
	// 实际应该基于历史数据计算统计量

	return point, nil
}

// DuplicateDetectionRule 重复检测规则
type DuplicateDetectionRule struct {
	seenMap map[string]struct{}
	mu      sync.Mutex
}

func NewDuplicateDetectionRule() *DuplicateDetectionRule {
	return &DuplicateDetectionRule{
		seenMap: make(map[string]struct{}),
	}
}

func (r *DuplicateDetectionRule) Name() string {
	return "duplicate_detection"
}

func (r *DuplicateDetectionRule) Apply(point *DataPoint) (*DataPoint, error) {
	key := fmt.Sprintf("%s_%d", point.Symbol, point.Timestamp)

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.seenMap[key]; exists {
		return nil, fmt.Errorf("duplicate data point: %s at %d", point.Symbol, point.Timestamp)
	}

	r.seenMap[key] = struct{}{}
	return point, nil
}

// StatisticalCorrector 统计修正器
type StatisticalCorrector struct {
	windowSize int
}

func NewStatisticalCorrector(windowSize int) *StatisticalCorrector {
	if windowSize == 0 {
		windowSize = 20
	}
	return &StatisticalCorrector{
		windowSize: windowSize,
	}
}

// CorrectOutliers 修正异常值
func (sc *StatisticalCorrector) CorrectOutliers(points []*DataPoint) []*DataPoint {
	if len(points) == 0 {
		return points
	}

	// 提取收盘价
	prices := make([]float64, len(points))
	for i, p := range points {
		prices[i] = p.Close
	}

	// 计算均值和标准差
	mean, stdDev := sc.calculateMeanStdDev(prices)

	// 修正异常值
	for _, p := range points {
		zScore := math.Abs((p.Close - mean) / stdDev)
		if zScore > 3.0 {
			// 用中位数替换
			median := sc.calculateMedian(prices)
			p.Close = median
		}
	}

	return points
}

// calculateMeanStdDev 计算均值和标准差
func (sc *StatisticalCorrector) calculateMeanStdDev(values []float64) (float64, float64) {
	n := len(values)
	if n == 0 {
		return 0, 0
	}

	// 计算均值
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(n)

	// 计算标准差
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(n)
	stdDev := math.Sqrt(variance)

	return mean, stdDev
}

// calculateMedian 计算中位数
func (sc *StatisticalCorrector) calculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0
	}
	return sorted[n/2]
}

// FillMissing 填充缺失数据
func (dc *DataCleaner) FillMissing(points []*DataPoint) []*DataPoint {
	if len(points) == 0 {
		return points
	}

	// 简单的前向填充
	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		curr := points[i]

		// 检查是否需要填充
		if curr.Open == 0 && curr.Close > 0 {
			curr.Open = prev.Close
		}
		if curr.High == 0 || curr.High < curr.Close {
			curr.High = math.Max(curr.Close, prev.High)
		}
		if curr.Low == 0 || curr.Low > curr.Close {
			curr.Low = math.Min(curr.Close, prev.Low)
		}
		if curr.Volume == 0 {
			curr.Volume = prev.Volume
		}
	}

	return points
}

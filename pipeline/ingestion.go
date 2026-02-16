package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/market"
)

// DataPoint 数据点
type DataPoint struct {
	Symbol    string                 `json:"symbol"`
	Timestamp int64                  `json:"timestamp"`
	Open      float64                `json:"open"`
	High      float64                `json:"high"`
	Low       float64                `json:"low"`
	Close     float64                `json:"close"`
	Volume    float64                `json:"volume"`
	Amount    float64                `json:"amount"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

// IngestionConfig 数据摄取配置
type IngestionConfig struct {
	BatchSize         int           `json:"batch_size"`
	BatchTimeout      time.Duration `json:"batch_timeout"`
	CheckInterval     time.Duration `json:"check_interval"`
	EnableIncremental bool          `json:"enable_incremental"`
	MaxRetries        int           `json:"max_retries"`
}

// DataIngester 数据摄取器
type DataIngester struct {
	config   IngestionConfig
	provider *market.MarketProvider
	storage  DataStorage

	batchBuffer map[string][]*DataPoint
	bufferLock  sync.RWMutex

	progress     map[string]int64 // symbol -> last timestamp
	progressLock sync.RWMutex

	stopChan chan struct{}
	wg       sync.WaitGroup

	stats     IngestionStats
	statsLock sync.RWMutex
}

// IngestionStats 摄取统计
type IngestionStats struct {
	TotalPoints      int64            `json:"total_points"`
	FailedPoints     int64            `json:"failed_points"`
	BatchesProcessed int64            `json:"batches_processed"`
	LastIngestion    time.Time        `json:"last_ingestion"`
	Symbols          map[string]int64 `json:"symbols"`
}

// DataStorage 数据存储接口
type DataStorage interface {
	SaveBatch(ctx context.Context, points []*DataPoint) error
	GetLastTimestamp(ctx context.Context, symbol string) (int64, error)
}

// NewDataIngester 创建数据摄取器
func NewDataIngester(config IngestionConfig, provider *market.MarketProvider, storage DataStorage) *DataIngester {
	if config.BatchSize == 0 {
		config.BatchSize = 1000
	}
	if config.BatchTimeout == 0 {
		config.BatchTimeout = 5 * time.Second
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 1 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &DataIngester{
		config:      config,
		provider:    provider,
		storage:     storage,
		batchBuffer: make(map[string][]*DataPoint),
		progress:    make(map[string]int64),
		stopChan:    make(chan struct{}),
		stats: IngestionStats{
			Symbols: make(map[string]int64),
		},
	}
}

// Start 启动数据摄取
func (di *DataIngester) Start(symbols []string) error {
	log.Printf("Starting data ingestion for %d symbols", len(symbols))

	// 初始化进度
	for _, symbol := range symbols {
		if di.config.EnableIncremental {
			if lastTime, err := di.storage.GetLastTimestamp(context.Background(), symbol); err == nil {
				di.progressLock.Lock()
				di.progress[symbol] = lastTime
				di.progressLock.Unlock()
				log.Printf("Loaded progress for %s: %d", symbol, lastTime)
			}
		}
		di.batchBuffer[symbol] = make([]*DataPoint, 0, di.config.BatchSize)
	}

	di.wg.Add(1)
	go di.runIngestionLoop(symbols)

	return nil
}

// Stop 停止数据摄取
func (di *DataIngester) Stop() {
	log.Println("Stopping data ingestion...")
	close(di.stopChan)

	// 刷新所有缓冲区
	di.flushAllBatches(context.Background())

	di.wg.Wait()
	log.Println("Data ingestion stopped")
}

// runIngestionLoop 运行摄取循环
func (di *DataIngester) runIngestionLoop(symbols []string) {
	defer di.wg.Done()

	ticker := time.NewTicker(di.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-di.stopChan:
			return
		case <-ticker.C:
			for _, symbol := range symbols {
				if err := di.ingestSymbol(context.Background(), symbol); err != nil {
					log.Printf("Failed to ingest %s: %v", symbol, err)
				}
			}
		}
	}
}

// ingestSymbol 摄取单个标的
func (di *DataIngester) ingestSymbol(ctx context.Context, symbol string) error {
	// 获取最后时间戳
	lastTime := int64(0)
	if di.config.EnableIncremental {
		di.progressLock.RLock()
		lastTime = di.progress[symbol]
		di.progressLock.RUnlock()
	}

	// 从数据源获取数据
	data, err := di.fetchData(ctx, symbol, lastTime)
	if err != nil {
		return fmt.Errorf("fetch data failed: %w", err)
	}

	if len(data) == 0 {
		return nil
	}

	// 添加到缓冲区
	di.addToBuffer(symbol, data)

	// 检查是否需要刷新
	if di.shouldFlush(symbol) {
		if err := di.flushBuffer(ctx, symbol); err != nil {
			return fmt.Errorf("flush buffer failed: %w", err)
		}
	}

	return nil
}

// fetchData 获取数据
func (di *DataIngester) fetchData(ctx context.Context, symbol string, since int64) ([]*DataPoint, error) {
	// 这里应该调用实际的市场数据API
	// 简化版：生成模拟数据

	var points []*DataPoint
	now := time.Now().Unix()

	// 如果有最后时间戳，获取增量数据
	if since > 0 {
		// 获取最近的数据点
		for i := 0; i < 10; i++ {
			timestamp := now - int64(i)*60
			if timestamp <= since {
				continue
			}
			points = append(points, di.generateDataPoint(symbol, timestamp))
		}
	} else {
		// 获取历史数据（这里简化为最近100个点）
		for i := 0; i < 100; i++ {
			timestamp := now - int64(i)*60
			points = append(points, di.generateDataPoint(symbol, timestamp))
		}
	}

	return points, nil
}

// generateDataPoint 生成数据点（模拟）
func (di *DataIngester) generateDataPoint(symbol string, timestamp int64) *DataPoint {
	basePrice := 100.0
	price := basePrice + float64(timestamp%100)/10.0

	return &DataPoint{
		Symbol:    symbol,
		Timestamp: timestamp,
		Open:      price * 0.99,
		High:      price * 1.01,
		Low:       price * 0.98,
		Close:     price,
		Volume:    1000000.0 + float64(timestamp%10000)*100,
		Amount:    (1000000.0 + float64(timestamp%10000)*100) * price,
	}
}

// addToBuffer 添加到缓冲区
func (di *DataIngester) addToBuffer(symbol string, points []*DataPoint) {
	di.bufferLock.Lock()
	defer di.bufferLock.Unlock()

	di.batchBuffer[symbol] = append(di.batchBuffer[symbol], points...)

	// 更新统计
	di.statsLock.Lock()
	di.stats.TotalPoints += int64(len(points))
	di.stats.Symbols[symbol] += int64(len(points))
	di.statsLock.Unlock()
}

// shouldFlush 检查是否需要刷新
func (di *DataIngester) shouldFlush(symbol string) bool {
	di.bufferLock.RLock()
	defer di.bufferLock.RUnlock()

	buffer := di.batchBuffer[symbol]
	return len(buffer) >= di.config.BatchSize
}

// flushBuffer 刷新缓冲区
func (di *DataIngester) flushBuffer(ctx context.Context, symbol string) error {
	di.bufferLock.Lock()
	buffer := di.batchBuffer[symbol]
	di.batchBuffer[symbol] = make([]*DataPoint, 0, di.config.BatchSize)
	di.bufferLock.Unlock()

	if len(buffer) == 0 {
		return nil
	}

	// 保存数据
	for retry := 0; retry < di.config.MaxRetries; retry++ {
		if err := di.storage.SaveBatch(ctx, buffer); err != nil {
			if retry == di.config.MaxRetries-1 {
				// 最后一次失败
				di.statsLock.Lock()
				di.stats.FailedPoints += int64(len(buffer))
				di.statsLock.Unlock()
				return err
			}
			time.Sleep(time.Duration(retry+1) * time.Second)
			continue
		}
		break
	}

	// 更新进度
	if di.config.EnableIncremental && len(buffer) > 0 {
		lastPoint := buffer[len(buffer)-1]
		di.progressLock.Lock()
		di.progress[symbol] = lastPoint.Timestamp
		di.progressLock.Unlock()
	}

	// 更新统计
	di.statsLock.Lock()
	di.stats.BatchesProcessed++
	di.stats.LastIngestion = time.Now()
	di.statsLock.Unlock()

	log.Printf("Flushed %d points for %s", len(buffer), symbol)

	return nil
}

// flushAllBatches 刷新所有缓冲区
func (di *DataIngester) flushAllBatches(ctx context.Context) {
	di.bufferLock.RLock()
	symbols := make([]string, 0, len(di.batchBuffer))
	for symbol := range di.batchBuffer {
		symbols = append(symbols, symbol)
	}
	di.bufferLock.RUnlock()

	for _, symbol := range symbols {
		if err := di.flushBuffer(ctx, symbol); err != nil {
			log.Printf("Failed to flush buffer for %s: %v", symbol, err)
		}
	}
}

// GetStats 获取统计信息
func (di *DataIngester) GetStats() IngestionStats {
	di.statsLock.RLock()
	defer di.statsLock.RUnlock()

	return di.stats
}

// GetProgress 获取进度
func (di *DataIngester) GetProgress(symbol string) (int64, error) {
	di.progressLock.RLock()
	defer di.progressLock.RUnlock()

	lastTime, ok := di.progress[symbol]
	if !ok {
		return 0, fmt.Errorf("progress not found for %s", symbol)
	}

	return lastTime, nil
}

// IngestManual 手动摄取数据
func (di *DataIngester) IngestManual(ctx context.Context, points []*DataPoint) error {
	if len(points) == 0 {
		return nil
	}

	// 按标的分组
	batches := make(map[string][]*DataPoint)
	for _, point := range points {
		batches[point.Symbol] = append(batches[point.Symbol], point)
	}

	// 处理每个批次
	for symbol, batch := range batches {
		di.addToBuffer(symbol, batch)
		if err := di.flushBuffer(ctx, symbol); err != nil {
			return fmt.Errorf("failed to save batch for %s: %w", symbol, err)
		}
	}

	return nil
}

// ToJSON 转换为JSON
func (dp *DataPoint) ToJSON() (string, error) {
	data, err := json.Marshal(dp)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

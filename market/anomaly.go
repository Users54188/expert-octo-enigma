package market

import (
	"math"
	"sync"
	"time"
)

type AnomalyDetector struct {
	mu                  sync.RWMutex
	priceHistory        map[string][]float64
	volumeHistory       map[string][]int64
	maxHistoryLength    int
	priceJumpThreshold  float64
	volumeAnomalyFactor float64
	latencyThreshold    time.Duration
	anomalyCallbacks    []func(AnomalyEvent)
}

type AnomalyEvent struct {
	Type        string                 `json:"type"`
	Symbol      string                 `json:"symbol"`
	Timestamp   time.Time              `json:"timestamp"`
	Description string                 `json:"description"`
	Details     map[string]interface{} `json:"details"`
}

const (
	AnomalyTypePriceJump       = "price_jump"
	AnomalyTypeVolumeSpike     = "volume_spike"
	AnomalyTypeDataDelay       = "data_delay"
	AnomalyTypeUnusualMovement = "unusual_movement"
)

func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		priceHistory:        make(map[string][]float64),
		volumeHistory:       make(map[string][]int64),
		maxHistoryLength:    100,
		priceJumpThreshold:  0.05,
		volumeAnomalyFactor: 3.0,
		latencyThreshold:    10 * time.Second,
		anomalyCallbacks:    make([]func(AnomalyEvent), 0),
	}
}

func (ad *AnomalyDetector) SetPriceJumpThreshold(threshold float64) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.priceJumpThreshold = threshold
}

func (ad *AnomalyDetector) SetVolumeAnomalyFactor(factor float64) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.volumeAnomalyFactor = factor
}

func (ad *AnomalyDetector) AddCallback(callback func(AnomalyEvent)) {
	ad.mu.Lock()
	defer ad.mu.Unlock()
	ad.anomalyCallbacks = append(ad.anomalyCallbacks, callback)
}

func (ad *AnomalyDetector) ProcessTick(symbol string, price float64, volume int64, timestamp time.Time) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	ad.updatePriceHistory(symbol, price)
	ad.updateVolumeHistory(symbol, volume)

	ad.detectPriceJump(symbol, price, timestamp)
	ad.detectVolumeSpike(symbol, volume, timestamp)
	ad.detectUnusualMovement(symbol, timestamp)
}

func (ad *AnomalyDetector) updatePriceHistory(symbol string, price float64) {
	if _, exists := ad.priceHistory[symbol]; !exists {
		ad.priceHistory[symbol] = make([]float64, 0)
	}

	ad.priceHistory[symbol] = append(ad.priceHistory[symbol], price)

	if len(ad.priceHistory[symbol]) > ad.maxHistoryLength {
		ad.priceHistory[symbol] = ad.priceHistory[symbol][1:]
	}
}

func (ad *AnomalyDetector) updateVolumeHistory(symbol string, volume int64) {
	if _, exists := ad.volumeHistory[symbol]; !exists {
		ad.volumeHistory[symbol] = make([]int64, 0)
	}

	ad.volumeHistory[symbol] = append(ad.volumeHistory[symbol], volume)

	if len(ad.volumeHistory[symbol]) > ad.maxHistoryLength {
		ad.volumeHistory[symbol] = ad.volumeHistory[symbol][1:]
	}
}

func (ad *AnomalyDetector) detectPriceJump(symbol string, price float64, timestamp time.Time) {
	history := ad.priceHistory[symbol]
	if len(history) < 2 {
		return
	}

	prevPrice := history[len(history)-2]
	priceChange := math.Abs(price-prevPrice) / prevPrice

	if priceChange > ad.priceJumpThreshold {
		ad.triggerAnomaly(AnomalyEvent{
			Type:        AnomalyTypePriceJump,
			Symbol:      symbol,
			Timestamp:   timestamp,
			Description: "价格剧烈波动",
			Details: map[string]interface{}{
				"current_price":  price,
				"previous_price": prevPrice,
				"change_percent": priceChange * 100,
				"threshold":      ad.priceJumpThreshold * 100,
			},
		})
	}
}

func (ad *AnomalyDetector) detectVolumeSpike(symbol string, volume int64, timestamp time.Time) {
	history := ad.volumeHistory[symbol]
	if len(history) < 10 {
		return
	}

	recent := history[len(history)-10:]
	sum := int64(0)
	for _, v := range recent {
		sum += v
	}
	avgVolume := float64(sum) / float64(len(recent))

	if avgVolume > 0 && float64(volume)/avgVolume > ad.volumeAnomalyFactor {
		ad.triggerAnomaly(AnomalyEvent{
			Type:        AnomalyTypeVolumeSpike,
			Symbol:      symbol,
			Timestamp:   timestamp,
			Description: "成交量异常放大",
			Details: map[string]interface{}{
				"current_volume": volume,
				"average_volume": avgVolume,
				"ratio":          float64(volume) / avgVolume,
			},
		})
	}
}

func (ad *AnomalyDetector) detectUnusualMovement(symbol string, timestamp time.Time) {
	priceHistory := ad.priceHistory[symbol]
	if len(priceHistory) < 20 {
		return
	}

	recent := priceHistory[len(priceHistory)-20:]

	sum := 0.0
	for _, price := range recent {
		sum += price
	}
	mean := sum / float64(len(recent))

	variance := 0.0
	for _, price := range recent {
		diff := price - mean
		variance += diff * diff
	}
	variance /= float64(len(recent))
	stdDev := math.Sqrt(variance)

	currentPrice := priceHistory[len(priceHistory)-1]
	zScore := math.Abs(currentPrice-mean) / stdDev

	if zScore > 3.0 {
		direction := "上涨"
		if currentPrice < mean {
			direction = "下跌"
		}

		ad.triggerAnomaly(AnomalyEvent{
			Type:        AnomalyTypeUnusualMovement,
			Symbol:      symbol,
			Timestamp:   timestamp,
			Description: "异常价格波动",
			Details: map[string]interface{}{
				"current_price": currentPrice,
				"mean_price":    mean,
				"std_dev":       stdDev,
				"z_score":       zScore,
				"direction":     direction,
			},
		})
	}
}

func (ad *AnomalyDetector) CheckDataDelay(symbol string, dataTime time.Time, expectedInterval time.Duration) {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	now := time.Now()
	delay := now.Sub(dataTime)

	if delay > ad.latencyThreshold {
		ad.triggerAnomaly(AnomalyEvent{
			Type:        AnomalyTypeDataDelay,
			Symbol:      symbol,
			Timestamp:   now,
			Description: "数据延迟",
			Details: map[string]interface{}{
				"data_time":         dataTime,
				"current_time":      now,
				"delay":             delay.Seconds(),
				"expected_interval": expectedInterval.Seconds(),
			},
		})
	}
}

func (ad *AnomalyDetector) triggerAnomaly(event AnomalyEvent) {
	for _, callback := range ad.anomalyCallbacks {
		go callback(event)
	}
}

func (ad *AnomalyDetector) GetSymbolStats(symbol string) map[string]interface{} {
	ad.mu.RLock()
	defer ad.mu.RUnlock()

	priceHistory, hasPrice := ad.priceHistory[symbol]
	volumeHistory, hasVolume := ad.volumeHistory[symbol]

	stats := make(map[string]interface{})

	if hasPrice && len(priceHistory) > 0 {
		sum := 0.0
		min := priceHistory[0]
		max := priceHistory[0]
		for _, price := range priceHistory {
			sum += price
			if price < min {
				min = price
			}
			if price > max {
				max = price
			}
		}
		stats["avg_price"] = sum / float64(len(priceHistory))
		stats["min_price"] = min
		stats["max_price"] = max
		stats["current_price"] = priceHistory[len(priceHistory)-1]
		stats["price_history_length"] = len(priceHistory)
	}

	if hasVolume && len(volumeHistory) > 0 {
		sum := int64(0)
		for _, vol := range volumeHistory {
			sum += vol
		}
		stats["avg_volume"] = float64(sum) / float64(len(volumeHistory))
		stats["current_volume"] = volumeHistory[len(volumeHistory)-1]
		stats["volume_history_length"] = len(volumeHistory)
	}

	return stats
}

func (ad *AnomalyDetector) ClearHistory(symbol string) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	delete(ad.priceHistory, symbol)
	delete(ad.volumeHistory, symbol)
}

func (ad *AnomalyDetector) ClearAllHistory() {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	ad.priceHistory = make(map[string][]float64)
	ad.volumeHistory = make(map[string][]int64)
}

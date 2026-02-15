package market

import "time"

type Tick struct {
	Symbol    string    `json:"symbol"`
	Close     float64   `json:"price"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Open      float64   `json:"open"`
	Volume    int64     `json:"volume"`
	Timestamp time.Time `json:"timestamp"`
}

type Indicator struct {
	MA5       float64   `json:"ma5"`
	MA20      float64   `json:"ma20"`
	RSI       float64   `json:"rsi"`
	MACD      float64   `json:"macd"`
	Timestamp time.Time `json:"timestamp"`
}

type KLine struct {
	Symbol     string    `json:"symbol"`
	Close      float64   `json:"close"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	Open       float64   `json:"open"`
	Volume     int64     `json:"volume"`
	Timestamp  time.Time `json:"timestamp"`
	Indicators Indicator `json:"indicators"`
}

// MarketProvider 市场数据提供者
type MarketProvider struct {
	// 可以添加缓存、连接池等
}

// GetMarketData 获取市场数据（简化实现）
func (p *MarketProvider) GetMarketData(symbol string) (*Tick, error) {
	return FetchTick(symbol)
}

// GetHistoricalData 获取历史数据
func (p *MarketProvider) GetHistoricalData(symbol string, days int) ([]KLine, error) {
	return FetchHistoricalData(symbol, days)
}

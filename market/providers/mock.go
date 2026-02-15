package providers

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

type MockProvider struct {
	basePrices map[string]float64
	mu         sync.RWMutex
	rand       *rand.Rand
}

func NewMockProvider() *MockProvider {
	mp := &MockProvider{
		basePrices: make(map[string]float64),
		rand:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}

	mp.initBasePrices()
	return mp
}

func (mp *MockProvider) Name() string {
	return "mock"
}

func (mp *MockProvider) Priority() int {
	return 0
}

func (mp *MockProvider) FetchTick(ctx context.Context, symbol string) (*Tick, error) {
	mp.mu.RLock()
	basePrice, exists := mp.basePrices[symbol]
	mp.mu.RUnlock()

	if !exists {
		basePrice = 10.0 + mp.rand.Float64()*90.0
		mp.mu.Lock()
		mp.basePrices[symbol] = basePrice
		mp.mu.Unlock()
	}

	changePercent := (mp.rand.Float64() - 0.5) * 0.1
	price := basePrice * (1 + changePercent)

	bid := price * (1 - mp.rand.Float64()*0.005)
	ask := price * (1 + mp.rand.Float64()*0.005)

	volume := int64(mp.rand.Float64() * 10000000)
	turnover := price * float64(volume)

	high := price * (1 + mp.rand.Float64()*0.02)
	low := price * (1 - mp.rand.Float64()*0.02)
	open := basePrice
	preClose := basePrice

	change := price - preClose
	changePct := (change / preClose) * 100

	return &Tick{
		Symbol:    symbol,
		Name:      getMockStockName(symbol),
		Price:     price,
		Bid:       bid,
		Ask:       ask,
		Volume:    volume,
		Turnover:  turnover,
		High:      high,
		Low:       low,
		Open:      open,
		PreClose:  preClose,
		Time:      time.Now(),
		Change:    change,
		ChangePct: changePct,
	}, nil
}

func (mp *MockProvider) FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error) {
	mp.mu.RLock()
	basePrice, exists := mp.basePrices[symbol]
	mp.mu.RUnlock()

	if !exists {
		basePrice = 10.0 + mp.rand.Float64()*90.0
		mp.mu.Lock()
		mp.basePrices[symbol] = basePrice
		mp.mu.Unlock()
	}

	var klines []KLine
	currentPrice := basePrice

	for i := days; i >= 1; i-- {
		date := time.Now().AddDate(0, 0, -i)

		changePercent := (mp.rand.Float64() - 0.48) * 0.08
		open := currentPrice * (1 + changePercent*0.3)
		close := currentPrice * (1 + changePercent)
		high := math.Max(open, close) * (1 + mp.rand.Float64()*0.02)
		low := math.Min(open, close) * (1 - mp.rand.Float64()*0.02)

		volume := int64(mp.rand.Float64() * 10000000)
		turnover := close * float64(volume)

		change := close - open
		changePct := (change / open) * 100

		klines = append(klines, KLine{
			Symbol:    symbol,
			Date:      date,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Turnover:  turnover,
			Change:    change,
			ChangePct: changePct,
		})

		currentPrice = close
	}

	return klines, nil
}

func (mp *MockProvider) HealthCheck() error {
	return nil
}

func (mp *MockProvider) initBasePrices() {
	stockPrices := map[string]float64{
		"sh600000": 7.50,
		"sh601398": 5.20,
		"sh600519": 1800.00,
		"sh600036": 32.00,
		"sz000858": 150.00,
		"sh601318": 45.00,
		"sz000001": 12.50,
		"sh600030": 20.00,
		"sh601166": 16.50,
		"sh600887": 30.00,
		"sz000333": 65.00,
		"sh601888": 85.00,
		"sh600276": 45.00,
		"sz300750": 180.00,
		"sz000651": 35.00,
		"sh601012": 22.00,
		"sh600031": 16.00,
		"sh600028": 6.50,
		"sh600019": 6.20,
		"sz002594": 250.00,
		"sh601336": 32.00,
		"sz300059": 15.00,
		"sh600585": 26.00,
		"sz000963": 42.00,
		"sh601766": 7.50,
		"sz002415": 32.00,
		"sh601668": 5.80,
		"sh600690": 22.00,
		"sz300760": 280.00,
		"sh601985": 6.50,
		"sh600900": 24.00,
	}

	mp.mu.Lock()
	for symbol, price := range stockPrices {
		mp.basePrices[symbol] = price
	}
	mp.mu.Unlock()
}

func getMockStockName(symbol string) string {
	names := map[string]string{
		"sh600000": "浦发银行",
		"sh601398": "工商银行",
		"sh600519": "贵州茅台",
		"sh600036": "招商银行",
		"sz000858": "五粮液",
		"sh601318": "中国平安",
		"sz000001": "平安银行",
		"sh600030": "中信证券",
		"sh601166": "兴业银行",
		"sh600887": "伊利股份",
		"sz000333": "美的集团",
		"sh601888": "中国中免",
		"sh600276": "恒瑞医药",
		"sz300750": "宁德时代",
		"sz000651": "格力电器",
		"sh601012": "隆基绿能",
		"sh600031": "三一重工",
		"sh600028": "中国石化",
		"sh600019": "宝钢股份",
		"sz002594": "比亚迪",
		"sh601336": "新华保险",
		"sz300059": "东方财富",
		"sh600585": "海螺水泥",
		"sz000963": "华东医药",
		"sh601766": "中国中车",
		"sz002415": "海康威视",
		"sh601668": "中国建筑",
		"sh600690": "海尔智家",
		"sz300760": "迈瑞医疗",
		"sh601985": "中国核电",
		"sh600900": "长江电力",
	}

	if name, exists := names[symbol]; exists {
		return name
	}
	return "模拟股票"
}

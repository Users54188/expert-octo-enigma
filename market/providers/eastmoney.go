package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type EastmoneyProvider struct {
	client *http.Client
}

func NewEastmoneyProvider() *EastmoneyProvider {
	return &EastmoneyProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (ep *EastmoneyProvider) Name() string {
	return "eastmoney"
}

func (ep *EastmoneyProvider) Priority() int {
	return 2
}

func (ep *EastmoneyProvider) FetchTick(ctx context.Context, symbol string) (*Tick, error) {
	marketCode := convertToEastmoneySymbol(symbol)
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f43,f44,f45,f46,f47,f48,f49,f50,f51,f52,f57,f58,f60,f107,f152", marketCode)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	resp, err := ep.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			F43  *float64 `json:"f43"`
			F44  *float64 `json:"f44"`
			F45  *float64 `json:"f45"`
			F46  *float64 `json:"f46"`
			F47  *float64 `json:"f47"`
			F48  *float64 `json:"f48"`
			F50  *int64   `json:"f50"`
			F51  *float64 `json:"f51"`
			F57  *int64   `json:"f57"`
			F60  *float64 `json:"f60"`
			F107 *float64 `json:"f107"`
			F152 *float64 `json:"f152"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	data := result.Data
	if data.F60 == nil || data.F43 == nil {
		return nil, fmt.Errorf("invalid data format")
	}

	price := *data.F60
	preClose := *data.F43
	open := getFloat(data.F44)
	high := getFloat(data.F45)
	low := getFloat(data.F46)
	volume := getInt64(data.F47)
	turnover := getFloat(data.F48)
	bid := getFloat(data.F51)

	change := price - preClose
	changePct := 0.0
	if preClose > 0 {
		changePct = (change / preClose) * 100
	}

	return &Tick{
		Symbol:    symbol,
		Price:     price,
		Bid:       bid,
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

func (ep *EastmoneyProvider) FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error) {
	marketCode := convertToEastmoneySymbol(symbol)
	url := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline?secid=%s&fields1=f1,f2,f3,f4,f5&fields2=f51,f52,f53,f54,f55,f56,f57,f58&klt=101&fqt=1&beg=0&end=20500101&lmt=%d", marketCode, days)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	resp, err := ep.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var klines []KLine
	for _, klineStr := range result.Data.Klines {
		parts := strings.Split(klineStr, ",")
		if len(parts) >= 7 {
			date, _ := time.Parse("2006-01-02", parts[0])
			open, _ := strconv.ParseFloat(parts[1], 64)
			close, _ := strconv.ParseFloat(parts[2], 64)
			high, _ := strconv.ParseFloat(parts[3], 64)
			low, _ := strconv.ParseFloat(parts[4], 64)
			volume, _ := strconv.ParseInt(parts[5], 10, 64)
			turnover, _ := strconv.ParseFloat(parts[6], 64)

			change := close - open
			changePct := 0.0
			if open > 0 {
				changePct = (change / open) * 100
			}

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
		}
	}

	return klines, nil
}

func (ep *EastmoneyProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ep.FetchTick(ctx, "sh600000")
	return err
}

func convertToEastmoneySymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	if strings.HasPrefix(symbol, "SH") {
		return "1." + strings.TrimPrefix(symbol, "SH")
	}
	if strings.HasPrefix(symbol, "SZ") {
		return "0." + strings.TrimPrefix(symbol, "SZ")
	}
	return symbol
}

func getFloat(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

func getInt64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

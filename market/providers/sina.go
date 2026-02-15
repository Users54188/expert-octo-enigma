package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type SinaProvider struct {
	client *http.Client
}

func NewSinaProvider() *SinaProvider {
	return &SinaProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (sp *SinaProvider) Name() string {
	return "sina"
}

func (sp *SinaProvider) Priority() int {
	return 3
}

func (sp *SinaProvider) FetchTick(ctx context.Context, symbol string) (*Tick, error) {
	sinaSymbol := convertToSinaSymbol(symbol)
	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", sinaSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := sp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dataStr := string(body)
	if strings.Contains(dataStr, "var hq_str_") {
		start := strings.Index(dataStr, "=") + 2
		end := strings.Index(dataStr, ";")
		if start >= 2 && end > start {
			data := dataStr[start:end]
			parts := strings.Split(data, ",")

			if len(parts) >= 32 {
				price, _ := strconv.ParseFloat(parts[3], 64)
				preClose, _ := strconv.ParseFloat(parts[2], 64)
				open, _ := strconv.ParseFloat(parts[1], 64)
				high, _ := strconv.ParseFloat(parts[4], 64)
				low, _ := strconv.ParseFloat(parts[5], 64)
				volume, _ := strconv.ParseInt(parts[8], 10, 64)
				bid, _ := strconv.ParseFloat(parts[6], 64)
				ask, _ := strconv.ParseFloat(parts[7], 64)
				dateStr := parts[30] + " " + parts[31]
				date, _ := time.ParseInLocation("2006-01-02 15:04:05", dateStr, time.Local)

				change := price - preClose
				changePct := 0.0
				if preClose > 0 {
					changePct = (change / preClose) * 100
				}

				return &Tick{
					Symbol:    symbol,
					Name:      strings.TrimSpace(parts[0]),
					Price:     price,
					Bid:       bid,
					Ask:       ask,
					Volume:    volume,
					High:      high,
					Low:       low,
					Open:      open,
					PreClose:  preClose,
					Turnover:  change * float64(volume),
					Time:      date,
					Change:    change,
					ChangePct: changePct,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to parse tick data")
}

func (sp *SinaProvider) FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error) {
	sinaSymbol := convertToSinaSymbol(symbol)
	url := fmt.Sprintf("https://finance.sina.com.cn/realstock/company/%s/kline.js?callback=dummy&scale=240&ma=no&datalen=%d", sinaSymbol, days)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := sp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dataStr := string(body)
	if strings.Contains(dataStr, "dummy(") {
		start := strings.Index(dataStr, "(") + 1
		end := strings.LastIndex(dataStr, ")")
		if start > 0 && end > start {
			jsonStr := dataStr[start:end]
			var klineData [][]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &klineData); err != nil {
				return nil, err
			}

			var klines []KLine
			for _, item := range klineData {
				if len(item) >= 7 {
					date, _ := time.Parse("2006-01-02", item[0].(string))
					open, _ := item[1].(json.Number).Float64()
					high, _ := item[2].(json.Number).Float64()
					low, _ := item[3].(json.Number).Float64()
					close, _ := item[4].(json.Number).Float64()
					volume, _ := item[5].(json.Number).Int64()

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
						Turnover:  change * float64(volume),
						Change:    change,
						ChangePct: changePct,
					})
				}
			}

			return klines, nil
		}
	}

	return nil, fmt.Errorf("failed to parse kline data")
}

func (sp *SinaProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := sp.FetchTick(ctx, "sh600000")
	return err
}

func convertToSinaSymbol(symbol string) string {
	symbol = strings.ToLower(symbol)
	if strings.HasPrefix(symbol, "sh") {
		return "sh" + strings.TrimPrefix(symbol, "sh")
	}
	if strings.HasPrefix(symbol, "sz") {
		return "sz" + strings.TrimPrefix(symbol, "sz")
	}
	return symbol
}

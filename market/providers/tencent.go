package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type TencentProvider struct {
	client *http.Client
}

func NewTencentProvider() *TencentProvider {
	return &TencentProvider{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (tp *TencentProvider) Name() string {
	return "tencent"
}

func (tp *TencentProvider) Priority() int {
	return 1
}

func (tp *TencentProvider) FetchTick(ctx context.Context, symbol string) (*Tick, error) {
	tencentSymbol := convertToTencentSymbol(symbol)
	url := fmt.Sprintf("https://qt.gtimg.cn/q=%s", tencentSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := tp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dataStr := string(body)
	if strings.Contains(dataStr, "v_") {
		start := strings.Index(dataStr, "\"") + 1
		end := strings.LastIndex(dataStr, "\"")
		if start > 0 && end > start {
			data := dataStr[start:end]
			parts := strings.Split(data, "~")

			if len(parts) >= 40 {
				name := strings.TrimSpace(parts[1])
				price, _ := strconv.ParseFloat(parts[3], 64)
				preClose, _ := strconv.ParseFloat(parts[4], 64)
				open, _ := strconv.ParseFloat(parts[5], 64)
				volume, _ := strconv.ParseInt(parts[6], 10, 64)
				bid, _ := strconv.ParseFloat(parts[9], 64)
				ask, _ := strconv.ParseFloat(parts[19], 64)
				high, _ := strconv.ParseFloat(parts[33], 64)
				low, _ := strconv.ParseFloat(parts[34], 64)

				change := price - preClose
				changePct := 0.0
				if preClose > 0 {
					changePct = (change / preClose) * 100
				}

				return &Tick{
					Symbol:    symbol,
					Name:      name,
					Price:     price,
					Bid:       bid,
					Ask:       ask,
					Volume:    volume,
					Turnover:  change * float64(volume),
					High:      high,
					Low:       low,
					Open:      open,
					PreClose:  preClose,
					Time:      time.Now(),
					Change:    change,
					ChangePct: changePct,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to parse tick data")
}

func (tp *TencentProvider) FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error) {
	tencentSymbol := convertToTencentSymbol(symbol)
	url := fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s,day,%s,,%d,qfq", tencentSymbol, time.Now().Format("20060102"), days)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := tp.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dataStr := string(body)
	var klines []KLine

	if strings.Contains(dataStr, "data") {
		lines := strings.Split(dataStr, "\n")
		for _, line := range lines {
			if strings.Contains(line, "day") {
				parts := strings.Split(line, "~")
				if len(parts) >= 7 {
					date, _ := time.ParseInLocation("2006-01-02", parts[0], time.Local)
					open, _ := strconv.ParseFloat(parts[1], 64)
					close, _ := strconv.ParseFloat(parts[2], 64)
					high, _ := strconv.ParseFloat(parts[3], 64)
					low, _ := strconv.ParseFloat(parts[4], 64)
					volume, _ := strconv.ParseInt(parts[5], 10, 64)

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
		}
	}

	return klines, nil
}

func (tp *TencentProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := tp.FetchTick(ctx, "sh600000")
	return err
}

func convertToTencentSymbol(symbol string) string {
	symbol = strings.ToLower(symbol)
	if strings.HasPrefix(symbol, "sh") {
		return "sh" + strings.TrimPrefix(symbol, "sh")
	}
	if strings.HasPrefix(symbol, "sz") {
		return "sz" + strings.TrimPrefix(symbol, "sz")
	}
	return symbol
}

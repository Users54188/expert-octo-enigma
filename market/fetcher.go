package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// FetchTick fetches the latest price for a single stock symbol from Sina API
func FetchTick(symbol string) (*Tick, error) {
	url := fmt.Sprintf("http://hq.sinajs.cn/list=%s", symbol)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Referer", "http://finance.sina.com.cn")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	utf8Reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(utf8Reader)
	if err != nil {
		return nil, err
	}

	line := string(body)
	parts := strings.Split(line, "\"")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid response from sina api")
	}

	data := strings.Split(parts[1], ",")
	if len(data) < 30 {
		return nil, fmt.Errorf("unexpected data format from sina api")
	}

	open, _ := strconv.ParseFloat(data[1], 64)
	high, _ := strconv.ParseFloat(data[4], 64)
	low, _ := strconv.ParseFloat(data[5], 64)
	curr, _ := strconv.ParseFloat(data[3], 64)
	volume, _ := strconv.ParseInt(data[8], 10, 64)
	
	date := data[30]
	timeStr := data[31]
	
	timestamp, _ := time.ParseInLocation("2006-01-02 15:04:05", date+" "+timeStr, time.Local)

	return &Tick{
		Symbol:    symbol,
		Open:      open,
		High:      high,
		Low:       low,
		Close:     curr,
		Volume:    volume,
		Timestamp: timestamp,
	}, nil
}

type sinaKLine struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

// FetchHistoricalData fetches historical K-line data for a symbol
func FetchHistoricalData(symbol string, days int) ([]KLine, error) {
	// scale=240 is daily
	url := fmt.Sprintf("http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=240&ma=no&datalen=%d", symbol, days)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var sinaData []sinaKLine
	if err := json.NewDecoder(resp.Body).Decode(&sinaData); err != nil {
		return nil, err
	}

	klines := make([]KLine, len(sinaData))
	for i, d := range sinaData {
		open, _ := strconv.ParseFloat(d.Open, 64)
		high, _ := strconv.ParseFloat(d.High, 64)
		low, _ := strconv.ParseFloat(d.Low, 64)
		close, _ := strconv.ParseFloat(d.Close, 64)
		volume, _ := strconv.ParseInt(d.Volume, 10, 64)
		
		var timestamp time.Time
		if len(d.Day) > 10 {
			timestamp, _ = time.ParseInLocation("2006-01-02 15:04:05", d.Day, time.Local)
		} else {
			timestamp, _ = time.ParseInLocation("2006-01-02", d.Day, time.Local)
		}

		klines[i] = KLine{
			Symbol:    symbol,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Timestamp: timestamp,
		}
	}

	return klines, nil
}

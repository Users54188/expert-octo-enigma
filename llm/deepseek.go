package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloudquant/market"
)

type DeepSeekAnalyzer struct {
	apiKey    string
	model     string
	client    *http.Client
	baseURL   string
	maxTokens int
}

type AnalysisResult struct {
	Symbol string `json:"symbol"`
	Trend  string `json:"trend"`
	Risk   string `json:"risk"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

func NewDeepSeekAnalyzer(apiKey, model string, timeout time.Duration, maxTokens int) *DeepSeekAnalyzer {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &DeepSeekAnalyzer{
		apiKey:    apiKey,
		model:     model,
		client:    &http.Client{Timeout: timeout},
		baseURL:   "https://api.deepseek.com/chat/completions",
		maxTokens: maxTokens,
	}
}

func (d *DeepSeekAnalyzer) Analyze(ctx context.Context, kline market.KLine, indicator market.Indicator) (*AnalysisResult, error) {
	if d == nil || d.client == nil {
		return nil, errors.New("deepseek analyzer not configured")
	}
	if d.apiKey == "" {
		return nil, errors.New("deepseek api key is required")
	}
	if d.model == "" {
		d.model = "deepseek-chat"
	}

	prompt := fmt.Sprintf(`你是一个A股量化交易分析师。基于以下数据分析市场：

股票代码: %s
当前价格: %.4f
成交量: %d

技术指标:
- MA5: %.4f, MA20: %.4f
- RSI(14): %.4f (超买>70, 超卖<30)
- MACD: %.4f

请分析：
1. 短期趋势判断（看涨/看跌/震荡）
2. 风险等级（低/中/高）
3. 建议操作（买入/卖出/观望）
4. 理由（20字以内）

仅返回JSON格式：
{
  "trend": "看涨|看跌|震荡",
  "risk": "低|中|高",
  "action": "买入|卖出|观望",
  "reason": "..."
}
`, kline.Symbol, kline.Close, kline.Volume, indicator.MA5, indicator.MA20, indicator.RSI, indicator.MACD)

	requestBody := deepSeekRequest{
		Model: d.model,
		Messages: []deepSeekMessage{{
			Role:    "user",
			Content: prompt,
		}},
		MaxTokens:  d.maxTokens,
		Temperature: 0.2,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.apiKey))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr deepSeekErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("deepseek api error: %s", apiErr.Error.Message)
		}
		return nil, fmt.Errorf("deepseek api returned status %d", resp.StatusCode)
	}

	var apiResp deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}
	if len(apiResp.Choices) == 0 {
		return nil, errors.New("deepseek api returned empty response")
	}
	content := apiResp.Choices[0].Message.Content
	result, err := parseAnalysisResult(content)
	if err != nil {
		return nil, err
	}
	result.Symbol = kline.Symbol
	return result, nil
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepSeekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Temperature float64           `json:"temperature,omitempty"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message deepSeekMessage `json:"message"`
	} `json:"choices"`
}

type deepSeekErrorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func parseAnalysisResult(content string) (*AnalysisResult, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var result AnalysisResult
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

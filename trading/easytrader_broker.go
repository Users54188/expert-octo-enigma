package trading

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// EasyTraderBroker 实现easytrader的HTTP客户端
type EasyTraderBroker struct {
	baseURL    string
	httpClient *http.Client
	brokerType string
	connected  bool
	mu         sync.RWMutex
}

// EasyTraderResponse easytrader服务响应结构
type EasyTraderResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

// NewEasyTraderBroker 创建easytrader客户端
func NewEasyTraderBroker(serviceURL, brokerType string) *EasyTraderBroker {
	return &EasyTraderBroker{
		baseURL: serviceURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		brokerType: brokerType,
		connected:  false,
	}
}

// Login 登录券商
func (b *EasyTraderBroker) Login(ctx context.Context, username, password, exePath string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	reqBody := map[string]interface{}{
		"broker_type": b.brokerType,
		"username":    username,
		"password":    password,
		"exe_path":    exePath,
	}

	resp, err := b.post(ctx, "/login", reqBody)
	if err != nil {
		return fmt.Errorf("登录请求失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("登录失败: %s", resp.Message)
	}

	b.connected = true
	return nil
}

// Logout 登出券商
func (b *EasyTraderBroker) Logout(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, err := b.get(ctx, "/logout")
	if err != nil {
		return fmt.Errorf("登出请求失败: %w", err)
	}

	b.connected = false
	return nil
}

// Buy 买入股票
func (b *EasyTraderBroker) Buy(ctx context.Context, symbol string, price float64, amount int) (string, error) {
	if !b.IsConnected() {
		return "", ErrNotConnected
	}

	reqBody := map[string]interface{}{
		"symbol": symbol,
		"price":  price,
		"amount": amount,
	}

	resp, err := b.post(ctx, "/buy", reqBody)
	if err != nil {
		return "", fmt.Errorf("买入请求失败: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("买入失败: %s", resp.Message)
	}

	// 从响应中解析order_id
	if data, ok := resp.Data.(map[string]interface{}); ok {
		if orderID, ok := data["order_id"].(string); ok {
			return orderID, nil
		}
	}

	return "", nil
}

// Sell 卖出股票
func (b *EasyTraderBroker) Sell(ctx context.Context, symbol string, price float64, amount int) (string, error) {
	if !b.IsConnected() {
		return "", ErrNotConnected
	}

	reqBody := map[string]interface{}{
		"symbol": symbol,
		"price":  price,
		"amount": amount,
	}

	resp, err := b.post(ctx, "/sell", reqBody)
	if err != nil {
		return "", fmt.Errorf("卖出请求失败: %w", err)
	}

	if !resp.Success {
		return "", fmt.Errorf("卖出失败: %s", resp.Message)
	}

	if data, ok := resp.Data.(map[string]interface{}); ok {
		if orderID, ok := data["order_id"].(string); ok {
			return orderID, nil
		}
	}

	return "", nil
}

// Cancel 撤销委托
func (b *EasyTraderBroker) Cancel(ctx context.Context, orderID string) error {
	if !b.IsConnected() {
		return ErrNotConnected
	}

	reqBody := map[string]interface{}{
		"order_id": orderID,
	}

	resp, err := b.post(ctx, "/cancel", reqBody)
	if err != nil {
		return fmt.Errorf("撤单请求失败: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("撤单失败: %s", resp.Message)
	}

	return nil
}

// GetBalance 获取账户余额
func (b *EasyTraderBroker) GetBalance(ctx context.Context) (*Balance, error) {
	if !b.IsConnected() {
		return nil, ErrNotConnected
	}

	resp, err := b.get(ctx, "/balance")
	if err != nil {
		return nil, fmt.Errorf("获取余额失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取余额失败: %s", resp.Message)
	}

	// 解析余额数据
	var balance Balance
	if data, ok := resp.Data.(map[string]interface{}); ok {
		if val, ok := data["total_assets"].(float64); ok {
			balance.TotalAssets = val
		}
		if val, ok := data["cash"].(float64); ok {
			balance.Cash = val
		}
		if val, ok := data["market_value"].(float64); ok {
			balance.MarketValue = val
		}
		if val, ok := data["total_profit"].(float64); ok {
			balance.TotalProfit = val
		}
		if val, ok := data["available_cash"].(float64); ok {
			balance.AvailableCash = val
		}
		if val, ok := data["frozen_cash"].(float64); ok {
			balance.FrozenCash = val
		}
		balance.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
	}

	return &balance, nil
}

// GetPositions 获取持仓
func (b *EasyTraderBroker) GetPositions(ctx context.Context) ([]Position, error) {
	if !b.IsConnected() {
		return nil, ErrNotConnected
	}

	resp, err := b.get(ctx, "/portfolio")
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取持仓失败: %s", resp.Message)
	}

	// 解析持仓数据
	var positions []Position
	if data, ok := resp.Data.([]interface{}); ok {
		for _, item := range data {
			if pos, ok := item.(map[string]interface{}); ok {
				position := Position{}
				if val, ok := pos["证券代码"].(string); ok {
					position.Symbol = val
				}
				if val, ok := pos["证券名称"].(string); ok {
					position.Name = val
				}
				if val, ok := pos["持仓数量"]; ok {
					position.Amount = int(val.(float64))
				}
				if val, ok := pos["可用数量"]; ok {
					position.Available = int(val.(float64))
				}
				if val, ok := pos["成本价"].(float64); ok {
					position.CostPrice = val
				}
				if val, ok := pos["当前价"].(float64); ok {
					position.CurrentPrice = val
				}
				if val, ok := pos["市值"].(float64); ok {
					position.MarketValue = val
				}
				if val, ok := pos["盈亏"].(float64); ok {
					position.Profit = val
				}
				if val, ok := pos["盈亏比例"].(float64); ok {
					position.ProfitPercent = val
				}
				position.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
				positions = append(positions, position)
			}
		}
	}

	return positions, nil
}

// GetOrders 获取当日委托
func (b *EasyTraderBroker) GetOrders(ctx context.Context) ([]Order, error) {
	if !b.IsConnected() {
		return nil, ErrNotConnected
	}

	resp, err := b.get(ctx, "/orders")
	if err != nil {
		return nil, fmt.Errorf("获取委托失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取委托失败: %s", resp.Message)
	}

	return b.parseOrders(resp.Data)
}

// GetTodayTrades 获取当日成交
func (b *EasyTraderBroker) GetTodayTrades(ctx context.Context) ([]Trade, error) {
	if !b.IsConnected() {
		return nil, ErrNotConnected
	}

	resp, err := b.get(ctx, "/today_trades")
	if err != nil {
		return nil, fmt.Errorf("获取成交失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("获取成交失败: %s", resp.Message)
	}

	return b.parseTrades(resp.Data)
}

// IsConnected 检查连接状态
func (b *EasyTraderBroker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// post 发送POST请求
func (b *EasyTraderBroker) post(ctx context.Context, path string, body interface{}) (*EasyTraderResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+path, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return b.parseResponse(resp.Body)
}

// get 发送GET请求
func (b *EasyTraderBroker) get(ctx context.Context, path string) (*EasyTraderResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.baseURL+path, nil)
	if err != nil {
		return nil, err
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return b.parseResponse(resp.Body)
}

// parseResponse 解析响应
func (b *EasyTraderBroker) parseResponse(body io.Reader) (*EasyTraderResponse, error) {
	var resp EasyTraderResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// parseOrders 解析委托数据
func (b *EasyTraderBroker) parseOrders(data interface{}) ([]Order, error) {
	var orders []Order
	if dataSlice, ok := data.([]interface{}); ok {
		for _, item := range dataSlice {
			if orderMap, ok := item.(map[string]interface{}); ok {
				order := Order{}
				if val, ok := orderMap["委托编号"].(string); ok {
					order.OrderID = val
				}
				if val, ok := orderMap["证券代码"].(string); ok {
					order.Symbol = val
				}
				if val, ok := orderMap["证券名称"].(string); ok {
					order.Name = val
				}
				if val, ok := orderMap["操作"].(string); ok {
					order.Type = val
				}
				if val, ok := orderMap["价格"].(float64); ok {
					order.Price = val
				}
				if val, ok := orderMap["数量"]; ok {
					order.Amount = int(val.(float64))
				}
				if val, ok := orderMap["成交数量"]; ok {
					order.FilledAmount = int(val.(float64))
				}
				if val, ok := orderMap["状态"].(string); ok {
					order.Status = val
				}
				if timeStr, ok := orderMap["委托时间"].(string); ok {
					order.OrderTime, _ = time.Parse("2006-01-02 15:04:05", timeStr)
				}
				orders = append(orders, order)
			}
		}
	}
	return orders, nil
}

// parseTrades 解析成交数据
func (b *EasyTraderBroker) parseTrades(data interface{}) ([]Trade, error) {
	var trades []Trade
	if dataSlice, ok := data.([]interface{}); ok {
		for _, item := range dataSlice {
			if tradeMap, ok := item.(map[string]interface{}); ok {
				trade := Trade{}
				if val, ok := tradeMap["成交编号"].(string); ok {
					trade.TradeID = val
				}
				if val, ok := tradeMap["委托编号"].(string); ok {
					trade.OrderID = val
				}
				if val, ok := tradeMap["证券代码"].(string); ok {
					trade.Symbol = val
				}
				if val, ok := tradeMap["证券名称"].(string); ok {
					trade.Name = val
				}
				if val, ok := tradeMap["操作"].(string); ok {
					trade.Type = val
				}
				if val, ok := tradeMap["成交价"].(float64); ok {
					trade.Price = val
				}
				if val, ok := tradeMap["成交数量"]; ok {
					trade.Amount = int(val.(float64))
				}
				if timeStr, ok := tradeMap["成交时间"].(string); ok {
					trade.TradeTime, _ = time.Parse("2006-01-02 15:04:05", timeStr)
				}
				if val, ok := tradeMap["手续费"].(float64); ok {
					trade.Commission = val
				}
				trades = append(trades, trade)
			}
		}
	}
	return trades, nil
}

package trading

import (
	"context"
	"time"
)

// Broker 定义券商接口
type Broker interface {
	// Login 登录券商客户端
	Login(ctx context.Context, username, password, exePath string) error

	// Logout 登出券商客户端
	Logout(ctx context.Context) error

	// Buy 买入股票
	Buy(ctx context.Context, symbol string, price float64, amount int) (string, error)

	// Sell 卖出股票
	Sell(ctx context.Context, symbol string, price float64, amount int) (string, error)

	// Cancel 撤销委托
	Cancel(ctx context.Context, orderID string) error

	// GetBalance 获取账户余额
	GetBalance(ctx context.Context) (*Balance, error)

	// GetPositions 获取持仓
	GetPositions(ctx context.Context) ([]Position, error)

	// GetOrders 获取当日委托
	GetOrders(ctx context.Context) ([]Order, error)

	// GetTodayTrades 获取当日成交
	GetTodayTrades(ctx context.Context) ([]Trade, error)

	// IsConnected 检查连接状态
	IsConnected() bool
}

// Balance 账户余额信息
type Balance struct {
	TotalAssets   float64 `json:"total_assets"`   // 总资产
	Cash          float64 `json:"cash"`           // 可用资金
	MarketValue   float64 `json:"market_value"`   // 持仓市值
	TotalProfit   float64 `json:"total_profit"`   // 总盈亏
	AvailableCash float64 `json:"available_cash"` // 可取资金
	FrozenCash    float64 `json:"frozen_cash"`    // 冻结资金
	UpdateTime    string  `json:"update_time"`    // 更新时间
}

// Position 持仓信息
type Position struct {
	Symbol        string  `json:"symbol"`         // 股票代码
	Name          string  `json:"name"`           // 股票名称
	Amount        int     `json:"amount"`         // 持仓数量
	Available     int     `json:"available"`      // 可用数量
	CostPrice     float64 `json:"cost_price"`     // 成本价
	CurrentPrice  float64 `json:"current_price"`  // 当前价
	MarketValue   float64 `json:"market_value"`   // 市值
	Profit        float64 `json:"profit"`         // 盈亏
	ProfitPercent float64 `json:"profit_percent"` // 盈亏比例
	UpdateTime    string  `json:"update_time"`    // 更新时间
}

// Order 委托信息
type Order struct {
	OrderID      string    `json:"order_id"`      // 委托编号
	Symbol       string    `json:"symbol"`        // 股票代码
	Name         string    `json:"name"`          // 股票名称
	Type         string    `json:"type"`          // 买卖方向: buy/sell
	Price        float64   `json:"price"`         // 委托价格
	Amount       int       `json:"amount"`        // 委托数量
	FilledAmount int       `json:"filled_amount"` // 成交数量
	Status       string    `json:"status"`        // 状态: 已报/已撤/部分成交/已成交
	OrderTime    time.Time `json:"order_time"`    // 委托时间
	Message      string    `json:"message"`       // 委托信息
}

// Trade 成交信息
type Trade struct {
	TradeID    string    `json:"trade_id"`   // 成交编号
	OrderID    string    `json:"order_id"`   // 委托编号
	Symbol     string    `json:"symbol"`     // 股票代码
	Name       string    `json:"name"`       // 股票名称
	Type       string    `json:"type"`       // 买卖方向: buy/sell
	Price      float64   `json:"price"`      // 成交价格
	Amount     int       `json:"amount"`     // 成交数量
	TradeTime  time.Time `json:"trade_time"` // 成交时间
	Commission float64   `json:"commission"` // 手续费
}

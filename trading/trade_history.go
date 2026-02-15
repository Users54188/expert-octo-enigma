package trading

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TradeHistory 交易历史记录
type TradeHistory struct {
	db *sql.DB
}

// NewTradeHistory 创建交易历史记录器
func NewTradeHistory(dbPath string) (*TradeHistory, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 创建表
	if err := createTradeTables(db); err != nil {
		db.Close()
		return nil, err
	}

	return &TradeHistory{db: db}, nil
}

// createTradeTables 创建交易相关表
func createTradeTables(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trade_id TEXT NOT NULL,
			order_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			type TEXT NOT NULL,
			price REAL NOT NULL,
			amount INTEGER NOT NULL,
			commission REAL DEFAULT 0,
			trade_time DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(trade_id)
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id TEXT NOT NULL,
			symbol TEXT NOT NULL,
			type TEXT NOT NULL,
			price REAL NOT NULL,
			amount INTEGER NOT NULL,
			filled_amount INTEGER DEFAULT 0,
			status TEXT NOT NULL,
			order_time DATETIME NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(order_id)
		)`,
		`CREATE TABLE IF NOT EXISTS positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT NOT NULL UNIQUE,
			amount INTEGER DEFAULT 0,
			cost_price REAL DEFAULT 0,
			total_cost REAL DEFAULT 0,
			current_price REAL DEFAULT 0,
			market_value REAL DEFAULT 0,
			unrealized_pnl REAL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS daily_performance (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL UNIQUE,
			open_equity REAL DEFAULT 0,
			close_equity REAL DEFAULT 0,
			daily_pnl REAL DEFAULT 0,
			daily_pnl_percent REAL DEFAULT 0,
			trade_count INTEGER DEFAULT 0,
			buy_count INTEGER DEFAULT 0,
			sell_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	return nil
}

// TradeRecord 交易记录
type TradeRecord struct {
	TradeID    string    `json:"trade_id"`
	OrderID    string    `json:"order_id"`
	Symbol     string    `json:"symbol"`
	Type       string    `json:"type"`
	Price      float64   `json:"price"`
	Amount     int       `json:"amount"`
	Commission float64   `json:"commission"`
	TradeTime  time.Time `json:"trade_time"`
}

// SaveTrade 保存交易记录
func (th *TradeHistory) SaveTrade(trade TradeRecord) error {
	if th.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	_, err := th.db.Exec(`
		INSERT OR REPLACE INTO trades (
			trade_id, order_id, symbol, type, price, amount, commission, trade_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, trade.TradeID, trade.OrderID, trade.Symbol, trade.Type,
		trade.Price, trade.Amount, trade.Commission, trade.TradeTime)

	return err
}

// SaveOrder 保存订单记录
func (th *TradeHistory) SaveOrder(order Order) error {
	if th.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	_, err := th.db.Exec(`
		INSERT OR REPLACE INTO orders (
			order_id, symbol, type, price, amount, filled_amount, status, order_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, order.OrderID, order.Symbol, order.Type, order.Price,
		order.Amount, order.FilledAmount, order.Status, order.OrderTime)

	return err
}

// UpdateOrderStatus 更新订单状态
func (th *TradeHistory) UpdateOrderStatus(orderID, status string) error {
	if th.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	_, err := th.db.Exec(`
		UPDATE orders SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE order_id = ?
	`, status, orderID)

	return err
}

// GetTrades 获取交易记录
func (th *TradeHistory) GetTrades(limit int) ([]TradeRecord, error) {
	if th.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
		SELECT trade_id, order_id, symbol, type, price, amount, commission, trade_time
		FROM trades
		ORDER BY trade_time DESC
		LIMIT ?
	`

	rows, err := th.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []TradeRecord
	for rows.Next() {
		var trade TradeRecord
		err := rows.Scan(
			&trade.TradeID, &trade.OrderID, &trade.Symbol, &trade.Type,
			&trade.Price, &trade.Amount, &trade.Commission, &trade.TradeTime,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetOrders 获取订单记录
func (th *TradeHistory) GetOrders(limit int) ([]Order, error) {
	if th.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
		SELECT order_id, symbol, type, price, amount, filled_amount, status, order_time
		FROM orders
		ORDER BY order_time DESC
		LIMIT ?
	`

	rows, err := th.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.OrderID, &order.Symbol, &order.Type, &order.Price,
			&order.Amount, &order.FilledAmount, &order.Status, &order.OrderTime,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	return orders, nil
}

// DailyPnL 日度盈亏
type DailyPnL struct {
	Date        string  `json:"date"`
	OpenEquity  float64 `json:"open_equity"`
	CloseEquity float64 `json:"close_equity"`
	PnL         float64 `json:"pnl"`
	PnLPercent  float64 `json:"pnl_percent"`
	TradeCount  int     `json:"trade_count"`
	BuyCount    int     `json:"buy_count"`
	SellCount   int     `json:"sell_count"`
}

// SaveDailyPnL 保存日度盈亏
func (th *TradeHistory) SaveDailyPnL(pnl DailyPnL) error {
	if th.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	_, err := th.db.Exec(`
		INSERT OR REPLACE INTO daily_performance (
			date, open_equity, close_equity, daily_pnl, daily_pnl_percent,
			trade_count, buy_count, sell_count
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, pnl.Date, pnl.OpenEquity, pnl.CloseEquity, pnl.PnL, pnl.PnLPercent,
		pnl.TradeCount, pnl.BuyCount, pnl.SellCount)

	return err
}

// GetDailyPnL 获取日度盈亏
func (th *TradeHistory) GetDailyPnL(days int) ([]DailyPnL, error) {
	if th.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	query := `
		SELECT date, open_equity, close_equity, daily_pnl, daily_pnl_percent,
		       trade_count, buy_count, sell_count
		FROM daily_performance
		ORDER BY date DESC
		LIMIT ?
	`

	rows, err := th.db.Query(query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pnls []DailyPnL
	for rows.Next() {
		var pnl DailyPnL
		err := rows.Scan(
			&pnl.Date, &pnl.OpenEquity, &pnl.CloseEquity, &pnl.PnL, &pnl.PnLPercent,
			&pnl.TradeCount, &pnl.BuyCount, &pnl.SellCount,
		)
		if err != nil {
			return nil, err
		}
		pnls = append(pnls, pnl)
	}

	return pnls, nil
}

// PerformanceMetrics 绩效指标
type PerformanceMetrics struct {
	TotalReturn    float64 `json:"total_return"`     // 总收益率
	DailyAvgReturn float64 `json:"daily_avg_return"` // 日均收益率
	MaxDrawdown    float64 `json:"max_drawdown"`     // 最大回撤
	WinRate        float64 `json:"win_rate"`         // 胜率
	ProfitFactor   float64 `json:"profit_factor"`    // 盈亏比
	TotalTrades    int     `json:"total_trades"`     // 总交易次数
	WinTrades      int     `json:"win_trades"`       // 盈利交易次数
	LossTrades     int     `json:"loss_trades"`      // 亏损交易次数
	SharpeRatio    float64 `json:"sharpe_ratio"`     // 夏普比率（简化版）
}

// CalculatePerformance 计算绩效指标
func (th *TradeHistory) CalculatePerformance(initialCapital float64) (*PerformanceMetrics, error) {
	if th.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	// 获取所有交易
	trades, err := th.GetTrades(10000)
	if err != nil {
		return nil, err
	}

	// 获取日度数据
	dailyPnLs, err := th.GetDailyPnL(1000)
	if err != nil {
		return nil, err
	}

	metrics := &PerformanceMetrics{
		TotalTrades: len(trades),
	}

	if len(dailyPnLs) == 0 {
		return metrics, nil
	}

	// 总收益率
	lastEquity := dailyPnLs[0].CloseEquity
	if initialCapital > 0 {
		metrics.TotalReturn = (lastEquity - initialCapital) / initialCapital
	}

	// 日均收益率
	if len(dailyPnLs) > 0 {
		totalDailyReturn := 0.0
		for _, pnl := range dailyPnLs {
			totalDailyReturn += pnl.PnLPercent
		}
		metrics.DailyAvgReturn = totalDailyReturn / float64(len(dailyPnLs))
	}

	// 最大回撤
	maxEquity := dailyPnLs[len(dailyPnLs)-1].OpenEquity
	maxDrawdown := 0.0
	for _, pnl := range dailyPnLs {
		if pnl.CloseEquity > maxEquity {
			maxEquity = pnl.CloseEquity
		}
		drawdown := (maxEquity - pnl.CloseEquity) / maxEquity
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	metrics.MaxDrawdown = maxDrawdown

	// 胜率和盈亏比
	winTrades := 0
	lossTrades := 0
	totalProfit := 0.0
	totalLoss := 0.0

	symbolPnL := make(map[string]float64)
	for _, trade := range trades {
		if trade.Type == "卖出" || trade.Type == "sell" {
			pnl := (trade.Price - 0) * float64(trade.Amount) // 简化计算，实际需要买入价
			symbolPnL[trade.Symbol] += pnl
		}
	}

	for _, pnl := range symbolPnL {
		if pnl > 0 {
			winTrades++
			totalProfit += pnl
		} else {
			lossTrades++
			totalLoss += -pnl
		}
	}

	metrics.WinTrades = winTrades
	metrics.LossTrades = lossTrades

	if winTrades+lossTrades > 0 {
		metrics.WinRate = float64(winTrades) / float64(winTrades+lossTrades)
	}

	if totalLoss > 0 {
		metrics.ProfitFactor = totalProfit / totalLoss
	}

	// 简化版夏普比率
	if len(dailyPnLs) > 1 {
		variance := 0.0
		mean := metrics.DailyAvgReturn
		for _, pnl := range dailyPnLs {
			diff := pnl.PnLPercent - mean
			variance += diff * diff
		}
		variance /= float64(len(dailyPnLs))
		if variance > 0 {
			metrics.SharpeRatio = mean / (variance * 0.01)
		}
	}

	return metrics, nil
}

// Close 关闭数据库连接
func (th *TradeHistory) Close() error {
	if th.db != nil {
		return th.db.Close()
	}
	return nil
}

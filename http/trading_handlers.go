package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"cloudquant/trading"
)

var (
	tradeHistory      *trading.TradeHistory
	brokerConnector   *trading.BrokerConnector
	riskManager       *trading.RiskManager
	positionManager   *trading.PositionManager
	orderExecutor     *trading.OrderExecutor
	signalHandler     *trading.SignalHandler
	autoTradeEnabled  bool
	autoTradeStopChan chan struct{}
)

// SetTradingComponents 设置交易组件
func SetTradingComponents(th *trading.TradeHistory, bc *trading.BrokerConnector,
	rm *trading.RiskManager, pm *trading.PositionManager, oe *trading.OrderExecutor, sh *trading.SignalHandler) {
	tradeHistory = th
	brokerConnector = bc
	riskManager = rm
	positionManager = pm
	orderExecutor = oe
	signalHandler = sh
}

// RegisterTradingHandlers 注册交易相关的路由
func RegisterTradingHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/trading/portfolio", handlePortfolio)
	mux.HandleFunc("GET /api/trading/balance", handleBalance)
	mux.HandleFunc("POST /api/trading/buy", handleBuy)
	mux.HandleFunc("POST /api/trading/sell", handleSell)
	mux.HandleFunc("POST /api/trading/cancel", handleCancel)
	mux.HandleFunc("GET /api/trading/orders", handleOrders)
	mux.HandleFunc("GET /api/trading/trades", handleTrades)
	mux.HandleFunc("GET /api/trading/performance", handlePerformance)
	mux.HandleFunc("GET /api/trading/daily_pnl", handleDailyPnL)
	mux.HandleFunc("GET /api/trading/risk", handleRisk)
	mux.HandleFunc("POST /api/trading/auto_trade/start", handleAutoTradeStart)
	mux.HandleFunc("POST /api/trading/auto_trade/stop", handleAutoTradeStop)
	mux.HandleFunc("GET /api/trading/auto_trade/status", handleAutoTradeStatus)
}

// handlePortfolio 处理投资组合请求
func handlePortfolio(w http.ResponseWriter, r *http.Request) {
	if positionManager == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	// 同步持仓
	if err := positionManager.SyncPositions(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	summary := positionManager.GetPositionSummary()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    summary,
	})
}

// handleBalance 处理账户余额请求
func handleBalance(w http.ResponseWriter, r *http.Request) {
	if brokerConnector == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	balance, err := brokerConnector.GetCachedBalance()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    balance,
	})
}

// handleBuy 处理买入请求
func handleBuy(w http.ResponseWriter, r *http.Request) {
	if orderExecutor == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Symbol string  `json:"symbol"`
		Price  float64 `json:"price"`
		Amount float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" || req.Price <= 0 || req.Amount <= 0 {
		http.Error(w, "缺少必要参数", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	orderID, err := orderExecutor.ExecuteBuy(ctx, req.Symbol, req.Price, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"order_id": orderID,
	})
}

// handleSell 处理卖出请求
func handleSell(w http.ResponseWriter, r *http.Request) {
	if orderExecutor == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Symbol   string  `json:"symbol"`
		Price    float64 `json:"price"`
		Quantity int     `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.Symbol == "" || req.Price <= 0 || req.Quantity <= 0 {
		http.Error(w, "缺少必要参数", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	orderID, err := orderExecutor.ExecuteSell(ctx, req.Symbol, req.Price, req.Quantity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"order_id": orderID,
	})
}

// handleCancel 处理撤单请求
func handleCancel(w http.ResponseWriter, r *http.Request) {
	if orderExecutor == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	if req.OrderID == "" {
		http.Error(w, "缺少订单ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := orderExecutor.ExecuteCancel(ctx, req.OrderID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

// handleOrders 处理订单历史请求
func handleOrders(w http.ResponseWriter, r *http.Request) {
	if brokerConnector == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	_, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	orders, err := brokerConnector.GetCachedOrders()
	if err != nil {
		// 尝试从数据库读取
		if tradeHistory != nil {
			records, dbErr := tradeHistory.GetOrders(limit)
			if dbErr != nil {
				http.Error(w, dbErr.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data":    records,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if limit < len(orders) {
		orders = orders[:limit]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    orders,
	})
}

// handleTrades 处理成交记录请求
func handleTrades(w http.ResponseWriter, r *http.Request) {
	if tradeHistory == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	trades, err := tradeHistory.GetTrades(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    trades,
	})
}

// handlePerformance 处理绩效统计请求
func handlePerformance(w http.ResponseWriter, r *http.Request) {
	if tradeHistory == nil || riskManager == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	initialCapital := riskManager.GetRiskMetrics().InitialCapital

	metrics, err := tradeHistory.CalculatePerformance(initialCapital)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    metrics,
	})
}

// handleDailyPnL 处理日度盈亏请求
func handleDailyPnL(w http.ResponseWriter, r *http.Request) {
	if tradeHistory == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	pnls, err := tradeHistory.GetDailyPnL(days)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    pnls,
	})
}

// handleRisk 处理风险指标请求
func handleRisk(w http.ResponseWriter, r *http.Request) {
	if riskManager == nil {
		http.Error(w, "交易服务未初始化", http.StatusServiceUnavailable)
		return
	}

	metrics := riskManager.GetRiskMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    metrics,
	})
}

// handleAutoTradeStart 处理启动自动交易
func handleAutoTradeStart(w http.ResponseWriter, r *http.Request) {
	if autoTradeEnabled {
		http.Error(w, "自动交易已在运行", http.StatusBadRequest)
		return
	}

	autoTradeStopChan = make(chan struct{})
	autoTradeEnabled = true

	// 启动自动交易协程
	go runAutoTrade()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "自动交易已启动",
	})
}

// handleAutoTradeStop 处理停止自动交易
func handleAutoTradeStop(w http.ResponseWriter, r *http.Request) {
	if !autoTradeEnabled {
		http.Error(w, "自动交易未运行", http.StatusBadRequest)
		return
	}

	close(autoTradeStopChan)
	autoTradeEnabled = false

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "自动交易已停止",
	})
}

// handleAutoTradeStatus 处理自动交易状态
func handleAutoTradeStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"enabled": autoTradeEnabled,
	})
}

// runAutoTrade 运行自动交易
func runAutoTrade() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			executeAutoTradeCycle()
		case <-autoTradeStopChan:
			return
		}
	}
}

// executeAutoTradeCycle 执行自动交易周期
func executeAutoTradeCycle() {
	if signalHandler == nil || orderExecutor == nil {
		return
	}

	// 1. 同步持仓
	if positionManager != nil {
		_ = positionManager.SyncPositions()
	}

	// 2. 检查止损
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if riskManager != nil {
		stopLossSymbols, err := riskManager.CheckPositionLoss(ctx)
		if err == nil {
			for _, symbol := range stopLossSymbols {
				// 获取当前价格
				price := 0.0
				if pos, err := positionManager.GetPosition(symbol); err == nil {
					price = pos.CurrentPrice
					_ = orderExecutor.ExecuteStopLoss(ctx, symbol, price)
				}
			}
		}
	}

	// 3. 更新日度盈亏
	if riskManager != nil {
		_, _ = riskManager.UpdateDailyPnL(ctx)
	}

	// 4. 同步成交记录
	if orderExecutor != nil {
		_ = orderExecutor.SyncTrades(ctx)
	}
}

// GetAIAndMLSignals 获取AI和ML信号（示例实现，实际应该调用相应的API）
func GetAIAndMLSignals(symbol string) (trading.AISignal, trading.MLSignal, error) {
	// 这里应该调用实际的AI和ML服务获取信号
	// 示例：返回默认信号
	aiSignal := trading.CreateHoldSignal(symbol, 0.7)
	mlSignal := trading.CreateMLBuySignal(symbol, 0.6)

	return aiSignal, mlSignal, nil
}

// ProcessSignalForSymbol 处理单个股票的信号
func ProcessSignalForSymbol(symbol string, price float64, amount float64) error {
	if signalHandler == nil || orderExecutor == nil {
		return fmt.Errorf("交易组件未初始化")
	}

	// 获取信号
	aiSignal, mlSignal, err := GetAIAndMLSignals(symbol)
	if err != nil {
		return fmt.Errorf("获取信号失败: %w", err)
	}

	// 处理信号
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	signal, err := signalHandler.ProcessSignal(ctx, aiSignal, mlSignal)
	if err != nil {
		return err
	}

	// 执行信号
	if signal.Action != "hold" {
		_, err = signalHandler.ExecuteSignal(ctx, signal, price, amount)
	}

	return err
}

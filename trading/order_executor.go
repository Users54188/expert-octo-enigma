package trading

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// OrderExecutor 订单执行引擎
type OrderExecutor struct {
	connector    *BrokerConnector
	riskManager  *RiskManager
	positionMgr  *PositionManager
	tradeHistory *TradeHistory
	mu           sync.RWMutex
}

// NewOrderExecutor 创建订单执行器
func NewOrderExecutor(
	connector *BrokerConnector,
	riskManager *RiskManager,
	positionMgr *PositionManager,
	tradeHistory *TradeHistory,
) *OrderExecutor {
	return &OrderExecutor{
		connector:    connector,
		riskManager:  riskManager,
		positionMgr:  positionMgr,
		tradeHistory: tradeHistory,
	}
}

// ExecuteBuy 执行买入
func (oe *OrderExecutor) ExecuteBuy(ctx context.Context, symbol string, price float64, amount float64) (string, error) {
	// 1. 风险检查
	orderReq := OrderRequest{
		Type:   OrderTypeBuy,
		Symbol: symbol,
		Price:  price,
		Amount: int(amount),
	}

	if err := oe.riskManager.CheckBeforeOrder(ctx, orderReq); err != nil {
		return "", fmt.Errorf("风险检查失败: %w", err)
	}

	// 2. 计算下单数量（按手数）
	quantity := orderReq.CalculateQuantity()
	if quantity <= 0 {
		return "", fmt.Errorf("下单数量不足: 金额 %.2f, 价格 %.2f", amount, price)
	}

	// 3. 下单
	broker := oe.connector.GetBroker()
	orderID, err := broker.Buy(ctx, symbol, price, quantity)
	if err != nil {
		return "", fmt.Errorf("买入失败: %w", err)
	}

	log.Printf("买入订单提交: %s, 价格: %.2f, 数量: %d, 订单ID: %s", symbol, price, quantity, orderID)

	// 4. 记录订单
	if oe.tradeHistory != nil {
		oe.recordOrder(Order{
			OrderID:   orderID,
			Symbol:    symbol,
			Type:      OrderTypeBuy,
			Price:     price,
			Amount:    quantity,
			OrderTime: time.Now(),
			Status:    "已报",
		})
	}

	return orderID, nil
}

// ExecuteSell 执行卖出
func (oe *OrderExecutor) ExecuteSell(ctx context.Context, symbol string, price float64, quantity int) (string, error) {
	// 1. 检查持仓
	posState, err := oe.positionMgr.GetPosition(symbol)
	if err != nil {
		return "", fmt.Errorf("未找到持仓: %w", err)
	}

	if quantity > posState.Available {
		return "", fmt.Errorf("可用持仓不足: 持有 %d, 可用 %d, 卖出 %d", posState.Amount, posState.Available, quantity)
	}

	// 2. 下单
	broker := oe.connector.GetBroker()
	orderID, err := broker.Sell(ctx, symbol, price, quantity)
	if err != nil {
		return "", fmt.Errorf("卖出失败: %w", err)
	}

	log.Printf("卖出订单提交: %s, 价格: %.2f, 数量: %d, 订单ID: %s", symbol, price, quantity, orderID)

	// 3. 记录订单
	if oe.tradeHistory != nil {
		oe.recordOrder(Order{
			OrderID:   orderID,
			Symbol:    symbol,
			Type:      OrderTypeSell,
			Price:     price,
			Amount:    quantity,
			OrderTime: time.Now(),
			Status:    "已报",
		})
	}

	return orderID, nil
}

// ExecuteCancel 执行撤单
func (oe *OrderExecutor) ExecuteCancel(ctx context.Context, orderID string) error {
	broker := oe.connector.GetBroker()

	err := broker.Cancel(ctx, orderID)
	if err != nil {
		return fmt.Errorf("撤单失败: %w", err)
	}

	log.Printf("撤单成功: %s", orderID)

	// 更新订单状态
	if oe.tradeHistory != nil {
		if err := oe.tradeHistory.UpdateOrderStatus(orderID, "已撤"); err != nil {
			log.Printf("更新订单状态失败: %v", err)
		}
	}

	return nil
}

// ExecuteStopLoss 执行止损
func (oe *OrderExecutor) ExecuteStopLoss(ctx context.Context, symbol string, currentPrice float64) error {
	// 获取持仓
	posState, err := oe.positionMgr.GetPosition(symbol)
	if err != nil {
		return fmt.Errorf("未找到持仓: %w", err)
	}

	// 全部卖出止损
	_, err = oe.ExecuteSell(ctx, symbol, currentPrice, posState.Amount)
	if err != nil {
		return fmt.Errorf("止损卖出失败: %w", err)
	}

	log.Printf("止损执行成功: %s, 价格: %.2f, 数量: %d", symbol, currentPrice, posState.Amount)
	return nil
}

// recordOrder 记录订单
func (oe *OrderExecutor) recordOrder(order Order) {
	// 这里简单记录，实际应该保存到数据库
	log.Printf("订单记录: %+v", order)
}

// CheckOrderStatus 检查订单状态
func (oe *OrderExecutor) CheckOrderStatus(ctx context.Context, orderID string) (*Order, error) {
	broker := oe.connector.GetBroker()
	orders, err := broker.GetOrders(ctx)
	if err != nil {
		return nil, err
	}

	for _, order := range orders {
		if order.OrderID == orderID {
			return &order, nil
		}
	}

	return nil, fmt.Errorf("未找到订单: %s", orderID)
}

// SyncTrades 同步成交记录
func (oe *OrderExecutor) SyncTrades(ctx context.Context) error {
	broker := oe.connector.GetBroker()
	trades, err := broker.GetTodayTrades(ctx)
	if err != nil {
		return err
	}

	// 更新持仓和记录交易
	for _, trade := range trades {
		// 更新持仓
		_ = oe.positionMgr.UpdatePosition(trade)

		// 记录交易历史
		if oe.tradeHistory != nil {
			_ = oe.tradeHistory.SaveTrade(TradeRecord{
				TradeID:    trade.TradeID,
				OrderID:    trade.OrderID,
				Symbol:     trade.Symbol,
				Type:       trade.Type,
				Price:      trade.Price,
				Volume:     int64(trade.Amount),
				TradeTime:  trade.TradeTime,
				Commission: trade.Commission,
			})
		}
	}

	log.Printf("同步 %d 条成交记录", len(trades))
	return nil
}

// GetPendingOrders 获取待成交订单
func (oe *OrderExecutor) GetPendingOrders(ctx context.Context) ([]Order, error) {
	broker := oe.connector.GetBroker()
	orders, err := broker.GetOrders(ctx)
	if err != nil {
		return nil, err
	}

	var pending []Order
	for _, order := range orders {
		if order.Status == "已报" || order.Status == "部分成交" {
			pending = append(pending, order)
		}
	}

	return pending, nil
}

// CancelAllOrders 取消所有待成交订单
func (oe *OrderExecutor) CancelAllOrders(ctx context.Context) error {
	pending, err := oe.GetPendingOrders(ctx)
	if err != nil {
		return err
	}

	for _, order := range pending {
		if err := oe.ExecuteCancel(ctx, order.OrderID); err != nil {
			log.Printf("撤单失败: %s, %v", order.OrderID, err)
		}
	}

	log.Printf("已取消 %d 个待成交订单", len(pending))
	return nil
}

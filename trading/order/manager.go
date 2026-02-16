package order

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"cloudquant/trading"
)

// OrderStatus 订单状态
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusSubmitted OrderStatus = "submitted"
	OrderStatusPartial   OrderStatus = "partial"
	OrderStatusFilled    OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
	OrderStatusRejected  OrderStatus = "rejected"
	OrderStatusFailed    OrderStatus = "failed"
)

// OrderSide 订单方向
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderType 订单类型
type OrderType string

const (
	OrderTypeMarket    OrderType = "market"
	OrderTypeLimit     OrderType = "limit"
	OrderTypeStop      OrderType = "stop"
	OrderTypeStopLimit OrderType = "stop_limit"
)

// Order 订单
type Order struct {
	ID             string            `json:"id"`
	Symbol         string            `json:"symbol"`
	Side           OrderSide         `json:"side"`
	Type           OrderType         `json:"type"`
	Quantity       float64           `json:"quantity"`
	Price          float64           `json:"price,omitempty"`
	StopPrice      float64           `json:"stop_price,omitempty"`
	Status         OrderStatus       `json:"status"`
	FilledQuantity float64           `json:"filled_quantity"`
	AvgPrice       float64           `json:"avg_price"`
	CreateTime     time.Time         `json:"create_time"`
	UpdateTime     time.Time         `json:"update_time"`
	SubmitTime     time.Time         `json:"submit_time,omitempty"`
	FillTime       time.Time         `json:"fill_time,omitempty"`
	ErrorMessage   string            `json:"error_message,omitempty"`
	ParentOrderID  string            `json:"parent_order_id,omitempty"`
	ChildOrders    []string          `json:"child_orders,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// OrderManager 订单管理器
type OrderManager struct {
	orders     map[string]*Order
	ordersLock sync.RWMutex

	brokerConnector *trading.BrokerConnector
	orderExecutor   *trading.OrderExecutor
	riskManager     *trading.RiskManager
	positionManager *trading.PositionManager

	orderChan chan *Order
	stopChan  chan struct{}
	wg        sync.WaitGroup

	maxPendingOrders int
	orderTimeout     time.Duration
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxPendingOrders int
	OrderTimeout     time.Duration
	EnableRouting    bool
}

// NewOrderManager 创建订单管理器
func NewOrderManager(
	brokerConnector *trading.BrokerConnector,
	orderExecutor *trading.OrderExecutor,
	riskManager *trading.RiskManager,
	positionManager *trading.PositionManager,
	config ManagerConfig,
) *OrderManager {
	if config.MaxPendingOrders == 0 {
		config.MaxPendingOrders = 100
	}
	if config.OrderTimeout == 0 {
		config.OrderTimeout = 30 * time.Second
	}

	return &OrderManager{
		orders:           make(map[string]*Order),
		brokerConnector:  brokerConnector,
		orderExecutor:    orderExecutor,
		riskManager:      riskManager,
		positionManager:  positionManager,
		orderChan:        make(chan *Order, config.MaxPendingOrders),
		stopChan:         make(chan struct{}),
		maxPendingOrders: config.MaxPendingOrders,
		orderTimeout:     config.OrderTimeout,
	}
}

// Start 启动订单管理器
func (m *OrderManager) Start() error {
	log.Println("Starting order manager...")

	m.wg.Add(1)
	go m.processOrders()

	return nil
}

// Stop 停止订单管理器
func (m *OrderManager) Stop() {
	log.Println("Stopping order manager...")
	close(m.stopChan)
	m.wg.Wait()
	log.Println("Order manager stopped")
}

// processOrders 处理订单
func (m *OrderManager) processOrders() {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		case order := <-m.orderChan:
			if err := m.executeOrder(context.Background(), order); err != nil {
				log.Printf("Failed to execute order %s: %v", order.ID, err)
			}
		}
	}
}

// SubmitOrder 提交订单
func (m *OrderManager) SubmitOrder(ctx context.Context, order *Order) (string, error) {
	// 验证订单
	if err := m.validateOrder(order); err != nil {
		return "", fmt.Errorf("order validation failed: %w", err)
	}

	// 风险检查
	if err := m.checkRisk(ctx, order); err != nil {
		return "", fmt.Errorf("risk check failed: %w", err)
	}

	// 生成订单ID
	if order.ID == "" {
		order.ID = generateOrderID()
	}

	// 设置状态
	order.Status = OrderStatusPending
	order.CreateTime = time.Now()
	order.UpdateTime = time.Now()

	// 保存订单
	m.ordersLock.Lock()
	m.orders[order.ID] = order
	m.ordersLock.Unlock()

	// 发送到处理队列
	select {
	case m.orderChan <- order:
		log.Printf("Order %s queued for execution", order.ID)
		return order.ID, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("order queue full, timeout waiting")
	}
}

// executeOrder 执行订单
func (m *OrderManager) executeOrder(ctx context.Context, order *Order) error {
	// 更新状态为已提交
	m.updateOrderStatus(order.ID, OrderStatusSubmitted, "")
	order.SubmitTime = time.Now()

	// 执行订单
	// 这里应该调用实际的订单执行逻辑
	// 简化版：直接标记为已成交

	time.Sleep(100 * time.Millisecond) // 模拟执行延迟

	// 模拟成交
	m.updateOrderStatus(order.ID, OrderStatusFilled, "")
	order.FilledQuantity = order.Quantity
	order.AvgPrice = order.Price
	order.FillTime = time.Now()

	log.Printf("Order %s executed successfully: %s %s %.2f @ %.2f",
		order.ID, order.Side, order.Symbol, order.Quantity, order.Price)

	return nil
}

// CancelOrder 取消订单
func (m *OrderManager) CancelOrder(ctx context.Context, orderID string) error {
	m.ordersLock.Lock()
	defer m.ordersLock.Unlock()

	order, ok := m.orders[orderID]
	if !ok {
		return fmt.Errorf("order %s not found", orderID)
	}

	if order.Status == OrderStatusFilled || order.Status == OrderStatusCancelled {
		return fmt.Errorf("order %s already %s", orderID, order.Status)
	}

	order.Status = OrderStatusCancelled
	order.UpdateTime = time.Now()

	log.Printf("Order %s cancelled", orderID)

	return nil
}

// GetOrder 获取订单
func (m *OrderManager) GetOrder(orderID string) (*Order, error) {
	m.ordersLock.RLock()
	defer m.ordersLock.RUnlock()

	order, ok := m.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	orderCopy := *order
	return &orderCopy, nil
}

// GetOrders 获取订单列表
func (m *OrderManager) GetOrders(filter OrderFilter) []*Order {
	m.ordersLock.RLock()
	defer m.ordersLock.RUnlock()

	var orders []*Order

	for _, order := range m.orders {
		if filter.Match(order) {
			orderCopy := *order
			orders = append(orders, &orderCopy)
		}
	}

	return orders
}

// validateOrder 验证订单
func (m *OrderManager) validateOrder(order *Order) error {
	if order.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	if order.Side != OrderSideBuy && order.Side != OrderSideSell {
		return fmt.Errorf("invalid side: %s", order.Side)
	}

	if order.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}

	switch order.Type {
	case OrderTypeLimit:
		if order.Price <= 0 {
			return fmt.Errorf("price is required for limit order")
		}
	case OrderTypeStop, OrderTypeStopLimit:
		if order.StopPrice <= 0 {
			return fmt.Errorf("stop price is required for stop order")
		}
	case OrderTypeMarket:
		// Market orders don't require price
	default:
		return fmt.Errorf("invalid order type: %s", order.Type)
	}

	return nil
}

// checkRisk 检查风险
func (m *OrderManager) checkRisk(ctx context.Context, order *Order) error {
	if m.riskManager == nil {
		return nil
	}

	// 这里应该调用风险管理器进行风险检查
	// 简化版：只做基本检查

	return nil
}

// updateOrderStatus 更新订单状态
func (m *OrderManager) updateOrderStatus(orderID string, status OrderStatus, errorMsg string) {
	m.ordersLock.Lock()
	defer m.ordersLock.Unlock()

	if order, ok := m.orders[orderID]; ok {
		order.Status = status
		order.UpdateTime = time.Now()
		if errorMsg != "" {
			order.ErrorMessage = errorMsg
		}
	}
}

// GetPendingOrders 获取待处理订单
func (m *OrderManager) GetPendingOrders() []*Order {
	return m.GetOrders(OrderFilter{Status: OrderStatusPending})
}

// GetActiveOrders 获取活跃订单
func (m *OrderManager) GetActiveOrders() []*Order {
	activeStatuses := []OrderStatus{
		OrderStatusPending,
		OrderStatusSubmitted,
		OrderStatusPartial,
	}

	var orders []*Order
	for _, status := range activeStatuses {
		orders = append(orders, m.GetOrders(OrderFilter{Status: status})...)
	}

	return orders
}

// GetOrderStats 获取订单统计
func (m *OrderManager) GetOrderStats() map[string]interface{} {
	m.ordersLock.RLock()
	defer m.ordersLock.RUnlock()

	stats := map[string]interface{}{
		"total":     len(m.orders),
		"pending":   0,
		"submitted": 0,
		"partial":   0,
		"filled":    0,
		"cancelled": 0,
		"rejected":  0,
		"failed":    0,
	}

	for _, order := range m.orders {
		switch order.Status {
		case OrderStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case OrderStatusSubmitted:
			stats["submitted"] = stats["submitted"].(int) + 1
		case OrderStatusPartial:
			stats["partial"] = stats["partial"].(int) + 1
		case OrderStatusFilled:
			stats["filled"] = stats["filled"].(int) + 1
		case OrderStatusCancelled:
			stats["cancelled"] = stats["cancelled"].(int) + 1
		case OrderStatusRejected:
			stats["rejected"] = stats["rejected"].(int) + 1
		case OrderStatusFailed:
			stats["failed"] = stats["failed"].(int) + 1
		}
	}

	return stats
}

// OrderFilter 订单过滤器
type OrderFilter struct {
	Symbol  string
	Side    OrderSide
	Status  OrderStatus
	Type    OrderType
	MinTime time.Time
	MaxTime time.Time
}

// Match 检查是否匹配
func (f *OrderFilter) Match(order *Order) bool {
	if f.Symbol != "" && order.Symbol != f.Symbol {
		return false
	}
	if f.Side != "" && order.Side != f.Side {
		return false
	}
	if f.Status != "" && order.Status != f.Status {
		return false
	}
	if f.Type != "" && order.Type != f.Type {
		return false
	}
	if !f.MinTime.IsZero() && order.CreateTime.Before(f.MinTime) {
		return false
	}
	if !f.MaxTime.IsZero() && order.CreateTime.After(f.MaxTime) {
		return false
	}
	return true
}

// generateOrderID 生成订单ID
func generateOrderID() string {
	return fmt.Sprintf("ord_%d", time.Now().UnixNano())
}

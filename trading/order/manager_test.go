package order

import (
	"context"
	"testing"
	"time"
)

func TestNewOrderManager(t *testing.T) {
	config := ManagerConfig{
		MaxPendingOrders: 100,
		OrderTimeout:     30 * time.Second,
	}

	mgr := NewOrderManager(nil, nil, nil, nil, config)
	if mgr == nil {
		t.Fatal("NewOrderManager returned nil")
	}

	if mgr.maxPendingOrders != 100 {
		t.Errorf("Expected maxPendingOrders 100, got %d", mgr.maxPendingOrders)
	}
}

func TestOrderManager_SubmitOrder(t *testing.T) {
	config := ManagerConfig{
		MaxPendingOrders: 100,
		OrderTimeout:     30 * time.Second,
	}

	mgr := NewOrderManager(nil, nil, nil, nil, config)
	if err := mgr.Start(); err != nil {
		t.Fatalf("failed to start order manager: %v", err)
	}
	defer mgr.Stop()

	order := &Order{
		Symbol:   "sh600000",
		Side:     OrderSideBuy,
		Type:     OrderTypeLimit,
		Quantity: 1000,
		Price:    10.50,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orderID, err := mgr.SubmitOrder(ctx, order)
	if err != nil {
		t.Fatalf("SubmitOrder failed: %v", err)
	}

	if orderID == "" {
		t.Error("OrderID is empty")
	}

	// 验证订单状态
	retrievedOrder, err := mgr.GetOrder(orderID)
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}

	if retrievedOrder.Symbol != order.Symbol {
		t.Errorf("Expected symbol %s, got %s", order.Symbol, retrievedOrder.Symbol)
	}
}

func TestOrderManager_ValidateOrder(t *testing.T) {
	config := ManagerConfig{}
	mgr := NewOrderManager(nil, nil, nil, nil, config)

	tests := []struct {
		name    string
		order   *Order
		wantErr bool
	}{
		{
			name: "valid buy order",
			order: &Order{
				Symbol:   "sh600000",
				Side:     OrderSideBuy,
				Type:     OrderTypeLimit,
				Quantity: 1000,
				Price:    10.50,
			},
			wantErr: false,
		},
		{
			name: "invalid symbol",
			order: &Order{
				Side:     OrderSideBuy,
				Type:     OrderTypeLimit,
				Quantity: 1000,
				Price:    10.50,
			},
			wantErr: true,
		},
		{
			name: "invalid side",
			order: &Order{
				Symbol:   "sh600000",
				Side:     "invalid",
				Type:     OrderTypeLimit,
				Quantity: 1000,
				Price:    10.50,
			},
			wantErr: true,
		},
		{
			name: "negative quantity",
			order: &Order{
				Symbol:   "sh600000",
				Side:     OrderSideBuy,
				Type:     OrderTypeLimit,
				Quantity: -100,
				Price:    10.50,
			},
			wantErr: true,
		},
		{
			name: "limit order without price",
			order: &Order{
				Symbol:   "sh600000",
				Side:     OrderSideBuy,
				Type:     OrderTypeLimit,
				Quantity: 1000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.validateOrder(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrderManager_GetOrders(t *testing.T) {
	config := ManagerConfig{
		MaxPendingOrders: 100,
		OrderTimeout:     30 * time.Second,
	}

	mgr := NewOrderManager(nil, nil, nil, nil, config)

	orders := []*Order{
		{
			Symbol:   "sh600000",
			Side:     OrderSideBuy,
			Type:     OrderTypeLimit,
			Quantity: 1000,
			Price:    10.50,
			Status:   OrderStatusPending,
		},
		{
			Symbol:   "sh601398",
			Side:     OrderSideSell,
			Type:     OrderTypeLimit,
			Quantity: 500,
			Price:    5.20,
			Status:   OrderStatusFilled,
		},
	}

	// 添加订单到管理器
	for _, order := range orders {
		mgr.orders[generateOrderID()] = order
	}

	// 测试获取所有订单
	allOrders := mgr.GetOrders(OrderFilter{})
	if len(allOrders) != 2 {
		t.Errorf("Expected 2 orders, got %d", len(allOrders))
	}

	// 测试按状态过滤
	pendingOrders := mgr.GetOrders(OrderFilter{Status: OrderStatusPending})
	if len(pendingOrders) != 1 {
		t.Errorf("Expected 1 pending order, got %d", len(pendingOrders))
	}

	// 测试按标的过滤
	symbolOrders := mgr.GetOrders(OrderFilter{Symbol: "sh600000"})
	if len(symbolOrders) != 1 {
		t.Errorf("Expected 1 order for sh600000, got %d", len(symbolOrders))
	}
}

func TestOrderManager_CancelOrder(t *testing.T) {
	config := ManagerConfig{}
	mgr := NewOrderManager(nil, nil, nil, nil, config)

	order := &Order{
		Symbol:   "sh600000",
		Side:     OrderSideBuy,
		Type:     OrderTypeLimit,
		Quantity: 1000,
		Price:    10.50,
		Status:   OrderStatusPending,
	}

	orderID := generateOrderID()
	mgr.orders[orderID] = order

	ctx := context.Background()
	err := mgr.CancelOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("CancelOrder failed: %v", err)
	}

	// 验证订单状态
	retrievedOrder, err := mgr.GetOrder(orderID)
	if err != nil {
		t.Fatalf("GetOrder failed: %v", err)
	}

	if retrievedOrder.Status != OrderStatusCancelled {
		t.Errorf("Expected status %s, got %s", OrderStatusCancelled, retrievedOrder.Status)
	}
}

func TestOrderManager_GetOrderStats(t *testing.T) {
	config := ManagerConfig{}
	mgr := NewOrderManager(nil, nil, nil, nil, config)

	// 添加测试订单
	mgr.orders["ord1"] = &Order{Status: OrderStatusPending}
	mgr.orders["ord2"] = &Order{Status: OrderStatusFilled}
	mgr.orders["ord3"] = &Order{Status: OrderStatusCancelled}

	stats := mgr.GetOrderStats()

	if stats["total"].(int) != 3 {
		t.Errorf("Expected total 3, got %d", stats["total"])
	}

	if stats["pending"].(int) != 1 {
		t.Errorf("Expected pending 1, got %d", stats["pending"])
	}

	if stats["filled"].(int) != 1 {
		t.Errorf("Expected filled 1, got %d", stats["filled"])
	}

	if stats["cancelled"].(int) != 1 {
		t.Errorf("Expected cancelled 1, got %d", stats["cancelled"])
	}
}

func BenchmarkOrderManager_SubmitOrder(b *testing.B) {
	config := ManagerConfig{
		MaxPendingOrders: 10000,
		OrderTimeout:     30 * time.Second,
	}

	mgr := NewOrderManager(nil, nil, nil, nil, config)
	if err := mgr.Start(); err != nil {
		b.Fatalf("failed to start order manager: %v", err)
	}
	defer mgr.Stop()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order := &Order{
			Symbol:   "sh600000",
			Side:     OrderSideBuy,
			Type:     OrderTypeLimit,
			Quantity: 1000,
			Price:    10.50,
		}
		if _, err := mgr.SubmitOrder(ctx, order); err != nil {
			b.Fatalf("failed to submit order: %v", err)
		}
	}
}

package order

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ExecutionAlgorithm 执行算法类型
type ExecutionAlgorithm string

const (
	AlgoMarket  ExecutionAlgorithm = "market"  // 市价执行
	AlgoTWAP    ExecutionAlgorithm = "twap"    // 时间加权平均价
	AlgoVWAP    ExecutionAlgorithm = "vwap"    // 成交量加权平均价
	AlgoIceberg ExecutionAlgorithm = "iceberg" // 冰山算法
	AlgoPOV     ExecutionAlgorithm = "pov"     // 参与率算法
)

// AlgoConfig 算法配置
type AlgoConfig struct {
	Type          ExecutionAlgorithm `json:"type"`
	Duration      time.Duration      `json:"duration"`
	SliceCount    int                `json:"slice_count"`
	Participation float64            `json:"participation"` // 0-1
	MinSliceSize  float64            `json:"min_slice_size"`
}

// SliceExecution 执行分片
type SliceExecution struct {
	OrderID      string      `json:"order_id"`
	SliceIndex   int         `json:"slice_index"`
	TotalSlices  int         `json:"total_slices"`
	Quantity     float64     `json:"quantity"`
	Price        float64     `json:"price"`
	Status       OrderStatus `json:"status"`
	ExecuteTime  time.Time   `json:"execute_time"`
	CompleteTime time.Time   `json:"complete_time"`
}

// ExecutionEngine 执行引擎
type ExecutionEngine struct {
	orders     map[string]*Order
	slices     map[string][]*SliceExecution
	ordersLock sync.RWMutex

	orderMgr   *OrderManager
	marketData func(symbol string) (*LiquidityInfo, error)
}

// NewExecutionEngine 创建执行引擎
func NewExecutionEngine(orderMgr *OrderManager, marketData func(string) (*LiquidityInfo, error)) *ExecutionEngine {
	return &ExecutionEngine{
		orders:     make(map[string]*Order),
		slices:     make(map[string][]*SliceExecution),
		orderMgr:   orderMgr,
		marketData: marketData,
	}
}

// ExecuteWithAlgorithm 使用算法执行订单
func (e *ExecutionEngine) ExecuteWithAlgorithm(ctx context.Context, order *Order, config AlgoConfig) error {
	switch config.Type {
	case AlgoMarket:
		return e.executeMarket(ctx, order)
	case AlgoTWAP:
		return e.executeTWAP(ctx, order, config)
	case AlgoVWAP:
		return e.executeVWAP(ctx, order, config)
	case AlgoIceberg:
		return e.executeIceberg(ctx, order, config)
	case AlgoPOV:
		return e.executePOV(ctx, order, config)
	default:
		return fmt.Errorf("unsupported algorithm: %s", config.Type)
	}
}

// executeMarket 市价执行
func (e *ExecutionEngine) executeMarket(ctx context.Context, order *Order) error {
	log.Printf("Executing order %s with market algorithm", order.ID)

	_, err := e.orderMgr.SubmitOrder(ctx, order)
	return err
}

// executeTWAP 时间加权平均价执行
func (e *ExecutionEngine) executeTWAP(ctx context.Context, order *Order, config AlgoConfig) error {
	log.Printf("Executing order %s with TWAP algorithm (duration: %v, slices: %d)",
		order.ID, config.Duration, config.SliceCount)

	if config.SliceCount == 0 {
		config.SliceCount = 10
	}

	// 计算每个分片的大小
	sliceQuantity := order.Quantity / float64(config.SliceCount)
	sliceInterval := config.Duration / time.Duration(config.SliceCount)

	// 保存原始订单
	e.ordersLock.Lock()
	e.orders[order.ID] = order
	e.ordersLock.Unlock()

	// 执行分片
	for i := 0; i < config.SliceCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sliceOrder := *order
			sliceOrder.ID = generateOrderID()
			sliceOrder.ParentOrderID = order.ID
			sliceOrder.Quantity = sliceQuantity

			// 最后一笔调整数量
			if i == config.SliceCount-1 {
				sliceOrder.Quantity = order.Quantity - (float64(config.SliceCount-1) * sliceQuantity)
			}

			// 记录分片
			slice := &SliceExecution{
				OrderID:     order.ID,
				SliceIndex:  i,
				TotalSlices: config.SliceCount,
				Quantity:    sliceOrder.Quantity,
				Status:      OrderStatusPending,
				ExecuteTime: time.Now(),
			}

			e.ordersLock.Lock()
			e.slices[order.ID] = append(e.slices[order.ID], slice)
			e.ordersLock.Unlock()

			// 提交分片订单
			if _, err := e.orderMgr.SubmitOrder(ctx, &sliceOrder); err != nil {
				log.Printf("Failed to submit slice %d: %v", i, err)
				slice.Status = OrderStatusFailed
			} else {
				slice.Status = OrderStatusSubmitted
			}

			// 等待下一个分片
			if i < config.SliceCount-1 {
				time.Sleep(sliceInterval)
			}
		}
	}

	log.Printf("TWAP execution completed for order %s", order.ID)
	return nil
}

// executeVWAP 成交量加权平均价执行
func (e *ExecutionEngine) executeVWAP(ctx context.Context, order *Order, config AlgoConfig) error {
	log.Printf("Executing order %s with VWAP algorithm", order.ID)

	if config.SliceCount == 0 {
		config.SliceCount = 10
	}

	// 获取流动性信息
	liq, err := e.getLiquidity(order.Symbol)
	if err != nil {
		log.Printf("Failed to get liquidity for VWAP: %v", err)
		return e.executeMarket(ctx, order)
	}

	// 保存原始订单
	e.ordersLock.Lock()
	e.orders[order.ID] = order
	e.ordersLock.Unlock()

	// 根据流动性分配分片大小
	var totalLiquidity float64
	if order.Side == OrderSideBuy {
		totalLiquidity = liq.AskVolume
	} else {
		totalLiquidity = liq.BidVolume
	}

	sliceCount := config.SliceCount
	sliceQuantity := order.Quantity / float64(sliceCount)

	// 执行分片
	for i := 0; i < sliceCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			sliceOrder := *order
			sliceOrder.ID = generateOrderID()
			sliceOrder.ParentOrderID = order.ID
			sliceOrder.Quantity = sliceQuantity

			// 最后一笔调整数量
			if i == sliceCount-1 {
				sliceOrder.Quantity = order.Quantity - (float64(sliceCount-1) * sliceQuantity)
			}

			// 根据流动性调整价格
			if order.Type == OrderTypeLimit && e.marketData != nil {
				newLiq, err := e.getLiquidity(order.Symbol)
				if err == nil {
					if order.Side == OrderSideBuy {
						sliceOrder.Price = newLiq.AskPrice
					} else {
						sliceOrder.Price = newLiq.BidPrice
					}
				}
			}

			// 记录分片
			slice := &SliceExecution{
				OrderID:     order.ID,
				SliceIndex:  i,
				TotalSlices: sliceCount,
				Quantity:    sliceOrder.Quantity,
				Price:       sliceOrder.Price,
				Status:      OrderStatusPending,
				ExecuteTime: time.Now(),
			}

			e.ordersLock.Lock()
			e.slices[order.ID] = append(e.slices[order.ID], slice)
			e.ordersLock.Unlock()

			// 提交分片订单
			if _, err := e.orderMgr.SubmitOrder(ctx, &sliceOrder); err != nil {
				log.Printf("Failed to submit VWAP slice %d: %v", i, err)
				slice.Status = OrderStatusFailed
			} else {
				slice.Status = OrderStatusSubmitted
			}

			// 等待一段时间再执行下一笔
			time.Sleep(500 * time.Millisecond)
		}
	}

	log.Printf("VWAP execution completed for order %s", order.ID)
	return nil
}

// executeIceberg 冰山算法执行
func (e *ExecutionEngine) executeIceberg(ctx context.Context, order *Order, config AlgoConfig) error {
	log.Printf("Executing order %s with iceberg algorithm", order.ID)

	if config.SliceCount == 0 {
		config.SliceCount = 5
	}

	if config.MinSliceSize == 0 {
		config.MinSliceSize = 100.0
	}

	sliceQuantity := config.MinSliceSize
	if order.Quantity < config.MinSliceSize {
		return e.executeMarket(ctx, order)
	}

	// 保存原始订单
	e.ordersLock.Lock()
	e.orders[order.ID] = order
	e.ordersLock.Unlock()

	// 执行分片
	remaining := order.Quantity
	for i := 0; remaining > 0; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			currentSlice := sliceQuantity
			if remaining < currentSlice*2 {
				currentSlice = remaining
			}

			sliceOrder := *order
			sliceOrder.ID = generateOrderID()
			sliceOrder.ParentOrderID = order.ID
			sliceOrder.Quantity = currentSlice
			sliceOrder.Type = OrderTypeLimit

			// 设置价格
			if order.Type == OrderTypeLimit {
				sliceOrder.Price = order.Price
			}

			// 记录分片
			slice := &SliceExecution{
				OrderID:     order.ID,
				SliceIndex:  i,
				TotalSlices: -1, // 未知总数
				Quantity:    currentSlice,
				Price:       sliceOrder.Price,
				Status:      OrderStatusPending,
				ExecuteTime: time.Now(),
			}

			e.ordersLock.Lock()
			e.slices[order.ID] = append(e.slices[order.ID], slice)
			e.ordersLock.Unlock()

			// 提交分片订单
			if _, err := e.orderMgr.SubmitOrder(ctx, &sliceOrder); err != nil {
				log.Printf("Failed to submit iceberg slice %d: %v", i, err)
				slice.Status = OrderStatusFailed
			} else {
				slice.Status = OrderStatusSubmitted
				remaining -= currentSlice
			}

			// 等待执行完成
			time.Sleep(1 * time.Second)
		}
	}

	log.Printf("Iceberg execution completed for order %s", order.ID)
	return nil
}

// executePOV 参与率算法执行
func (e *ExecutionEngine) executePOV(ctx context.Context, order *Order, config AlgoConfig) error {
	log.Printf("Executing order %s with POV algorithm (participation: %.2f%%)",
		order.ID, config.Participation*100)

	if config.Participation <= 0 || config.Participation > 1 {
		config.Participation = 0.1 // 默认10%
	}

	// 保存原始订单
	e.ordersLock.Lock()
	e.orders[order.ID] = order
	e.ordersLock.Unlock()

	// 持续执行直到完成或取消
	remaining := order.Quantity
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// 获取当前市场成交量
			liq, err := e.getLiquidity(order.Symbol)
			if err != nil {
				log.Printf("Failed to get liquidity: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 计算当前可执行量
			var marketVolume float64
			if order.Side == OrderSideBuy {
				marketVolume = liq.AskVolume
			} else {
				marketVolume = liq.BidVolume
			}

			sliceQuantity := marketVolume * config.Participation
			if sliceQuantity > remaining {
				sliceQuantity = remaining
			}

			if sliceQuantity < 100 { // 最小交易量
				time.Sleep(1 * time.Second)
				continue
			}

			// 创建分片订单
			sliceOrder := *order
			sliceOrder.ID = generateOrderID()
			sliceOrder.ParentOrderID = order.ID
			sliceOrder.Quantity = sliceQuantity

			// 设置价格
			if e.marketData != nil {
				newLiq, _ := e.getLiquidity(order.Symbol)
				if order.Side == OrderSideBuy {
					sliceOrder.Price = newLiq.AskPrice
				} else {
					sliceOrder.Price = newLiq.BidPrice
				}
			}

			// 记录分片
			slice := &SliceExecution{
				OrderID:     order.ID,
				SliceIndex:  len(e.slices[order.ID]),
				TotalSlices: -1,
				Quantity:    sliceQuantity,
				Price:       sliceOrder.Price,
				Status:      OrderStatusPending,
				ExecuteTime: time.Now(),
			}

			e.ordersLock.Lock()
			e.slices[order.ID] = append(e.slices[order.ID], slice)
			e.ordersLock.Unlock()

			// 提交分片订单
			_, err = e.orderMgr.SubmitOrder(ctx, &sliceOrder)
			if err != nil {
				log.Printf("Failed to submit POV slice: %v", err)
				slice.Status = OrderStatusFailed
			} else {
				slice.Status = OrderStatusSubmitted
				remaining -= sliceQuantity
			}

			time.Sleep(1 * time.Second)
		}
	}

	log.Printf("POV execution completed for order %s", order.ID)
	return nil
}

// getLiquidity 获取流动性
func (e *ExecutionEngine) getLiquidity(symbol string) (*LiquidityInfo, error) {
	if e.marketData == nil {
		return nil, fmt.Errorf("market data provider not configured")
	}
	return e.marketData(symbol)
}

// GetExecutionStatus 获取执行状态
func (e *ExecutionEngine) GetExecutionStatus(orderID string) ([]*SliceExecution, error) {
	e.ordersLock.RLock()
	defer e.ordersLock.RUnlock()

	slices, ok := e.slices[orderID]
	if !ok {
		return nil, fmt.Errorf("no execution found for order %s", orderID)
	}

	// 返回副本
	result := make([]*SliceExecution, len(slices))
	for i, s := range slices {
		sliceCopy := *s
		result[i] = &sliceCopy
	}

	return result, nil
}

// CalculateExecutionPrice 计算执行均价
func (e *ExecutionEngine) CalculateExecutionPrice(orderID string) (float64, error) {
	slices, err := e.GetExecutionStatus(orderID)
	if err != nil {
		return 0, err
	}

	var totalValue float64
	var totalQty float64

	for _, slice := range slices {
		if slice.Status == OrderStatusFilled {
			totalValue += slice.Price * slice.Quantity
			totalQty += slice.Quantity
		}
	}

	if totalQty == 0 {
		return 0, fmt.Errorf("no filled slices")
	}

	return totalValue / totalQty, nil
}

// CalculateCompletionRate 计算完成率
func (e *ExecutionEngine) CalculateCompletionRate(orderID string) (float64, error) {
	e.ordersLock.RLock()
	defer e.ordersLock.RUnlock()

	order, ok := e.orders[orderID]
	if !ok {
		return 0, fmt.Errorf("order not found: %s", orderID)
	}

	slices := e.slices[orderID]
	var filledQty float64

	for _, slice := range slices {
		if slice.Status == OrderStatusFilled {
			filledQty += slice.Quantity
		}
	}

	return filledQty / order.Quantity, nil
}

// EstimateExecutionTime 估算执行时间
func (e *ExecutionEngine) EstimateExecutionTime(order *Order, config AlgoConfig) time.Duration {
	switch config.Type {
	case AlgoMarket:
		return 1 * time.Second
	case AlgoTWAP, AlgoVWAP:
		if config.Duration > 0 {
			return config.Duration
		}
		return 10 * time.Minute
	case AlgoIceberg:
		slices := int(math.Ceil(order.Quantity / config.MinSliceSize))
		return time.Duration(slices) * time.Second
	case AlgoPOV:
		return 30 * time.Minute
	default:
		return 1 * time.Minute
	}
}

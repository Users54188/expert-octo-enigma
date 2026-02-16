package order

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
)

// RouteStrategy 路由策略
type RouteStrategy string

const (
	RouteStrategyDirect    RouteStrategy = "direct"    // 直连券商
	RouteStrategyBestBid   RouteStrategy = "best_bid"  // 最优买价
	RouteStrategyBestAsk   RouteStrategy = "best_ask"  // 最优卖价
	RouteStrategySpread    RouteStrategy = "spread"    // 价差优先
	RouteStrategyLiquidity RouteStrategy = "liquidity" // 流动性优先
)

// RouteConfig 路由配置
type RouteConfig struct {
	Strategy       RouteStrategy `json:"strategy"`
	SplitOrders    bool          `json:"split_orders"`
	MaxSplits      int           `json:"max_splits"`
	MinSplitSize   float64       `json:"min_split_size"`
	CheckLiquidity bool          `json:"check_liquidity"`
}

// LiquidityInfo 流动性信息
type LiquidityInfo struct {
	Symbol    string  `json:"symbol"`
	BidPrice  float64 `json:"bid_price"`
	BidVolume float64 `json:"bid_volume"`
	AskPrice  float64 `json:"ask_price"`
	AskVolume float64 `json:"ask_volume"`
	Spread    float64 `json:"spread"`
	SpreadPct float64 `json:"spread_pct"`
	Timestamp int64   `json:"timestamp"`
}

// OrderRouter 订单路由器
type OrderRouter struct {
	config         RouteConfig
	liquidityCache map[string]*LiquidityInfo
	cacheLock      sync.RWMutex

	marketData func(symbol string) (*LiquidityInfo, error)
}

// NewOrderRouter 创建订单路由器
func NewOrderRouter(config RouteConfig, marketData func(string) (*LiquidityInfo, error)) *OrderRouter {
	if config.MaxSplits == 0 {
		config.MaxSplits = 5
	}
	if config.MinSplitSize == 0 {
		config.MinSplitSize = 100.0
	}

	return &OrderRouter{
		config:         config,
		liquidityCache: make(map[string]*LiquidityInfo),
		marketData:     marketData,
	}
}

// RouteOrder 路由订单
func (r *OrderRouter) RouteOrder(ctx context.Context, order *Order) ([]*Order, error) {
	// 获取流动性信息
	liq, err := r.getLiquidityInfo(ctx, order.Symbol)
	if err != nil {
		log.Printf("Failed to get liquidity info for %s: %v", order.Symbol, err)
		// 如果获取失败，使用直连策略
		return []*Order{order}, nil
	}

	// 根据策略路由
	switch r.config.Strategy {
	case RouteStrategyDirect:
		return r.routeDirect(order)
	case RouteStrategyBestBid, RouteStrategyBestAsk:
		return r.routeBestPrice(order, liq)
	case RouteStrategySpread:
		return r.routeSpread(order, liq)
	case RouteStrategyLiquidity:
		return r.routeLiquidity(order, liq)
	default:
		return []*Order{order}, nil
	}
}

// routeDirect 直连路由
func (r *OrderRouter) routeDirect(order *Order) ([]*Order, error) {
	return []*Order{order}, nil
}

// routeBestPrice 最优价格路由
func (r *OrderRouter) routeBestPrice(order *Order, liq *LiquidityInfo) ([]*Order, error) {
	if order.Type != OrderTypeMarket {
		return []*Order{order}, nil
	}

	// 根据订单方向设置价格
	if order.Side == OrderSideBuy {
		order.Price = liq.AskPrice
	} else {
		order.Price = liq.BidPrice
	}

	order.Type = OrderTypeLimit

	log.Printf("Routed order %s at best price: %.2f", order.ID, order.Price)
	return []*Order{order}, nil
}

// routeSpread 价差路由
func (r *OrderRouter) routeSpread(order *Order, liq *LiquidityInfo) ([]*Order, error) {
	// 检查价差是否合理
	if liq.SpreadPct > 0.02 { // 价差超过2%
		log.Printf("Large spread detected for %s: %.2f%%, routing direct", order.Symbol, liq.SpreadPct*100)
		return []*Order{order}, nil
	}

	return r.routeBestPrice(order, liq)
}

// routeLiquidity 流动性路由
func (r *OrderRouter) routeLiquidity(order *Order, liq *LiquidityInfo) ([]*Order, error) {
	// 如果不启用拆单，直接返回
	if !r.config.SplitOrders {
		return []*Order{order}, nil
	}

	// 获取可用流动性
	var availableLiquidity float64
	if order.Side == OrderSideBuy {
		availableLiquidity = liq.AskVolume
	} else {
		availableLiquidity = liq.BidVolume
	}

	// 如果订单量小于流动性，直接返回
	if order.Quantity <= availableLiquidity {
		return []*Order{order}, nil
	}

	// 计算拆分数量
	splitCount := int(math.Ceil(order.Quantity / availableLiquidity))
	if splitCount > r.config.MaxSplits {
		splitCount = r.config.MaxSplits
	}

	// 计算每笔数量
	splitQuantity := order.Quantity / float64(splitCount)
	if splitQuantity < r.config.MinSplitSize {
		log.Printf("Split quantity %.2f below minimum %.2f, routing direct", splitQuantity, r.config.MinSplitSize)
		return []*Order{order}, nil
	}

	// 创建拆分订单
	var splitOrders []*Order
	for i := 0; i < splitCount; i++ {
		splitOrder := *order
		splitOrder.ID = generateOrderID()
		splitOrder.ParentOrderID = order.ID
		splitOrder.Quantity = splitQuantity

		// 最后一笔调整数量
		if i == splitCount-1 {
			splitOrder.Quantity = order.Quantity - (float64(splitCount-1) * splitQuantity)
		}

		splitOrders = append(splitOrders, &splitOrder)
	}

	log.Printf("Routed order %s into %d splits", order.ID, len(splitOrders))
	return splitOrders, nil
}

// getLiquidityInfo 获取流动性信息
func (r *OrderRouter) getLiquidityInfo(ctx context.Context, symbol string) (*LiquidityInfo, error) {
	// 先从缓存获取
	r.cacheLock.RLock()
	liq, ok := r.liquidityCache[symbol]
	r.cacheLock.RUnlock()

	if ok {
		return liq, nil
	}

	// 从市场数据获取
	if r.marketData == nil {
		return nil, fmt.Errorf("market data provider not configured")
	}

	liq, err := r.marketData(symbol)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	r.cacheLock.Lock()
	r.liquidityCache[symbol] = liq
	r.cacheLock.Unlock()

	return liq, nil
}

// UpdateLiquidity 更新流动性
func (r *OrderRouter) UpdateLiquidity(liq *LiquidityInfo) {
	r.cacheLock.Lock()
	defer r.cacheLock.Unlock()

	r.liquidityCache[liq.Symbol] = liq
}

// ClearCache 清空缓存
func (r *OrderRouter) ClearCache() {
	r.cacheLock.Lock()
	defer r.cacheLock.Unlock()

	r.liquidityCache = make(map[string]*LiquidityInfo)
}

// GetLiquidity 获取流动性
func (r *OrderRouter) GetLiquidity(symbol string) (*LiquidityInfo, error) {
	r.cacheLock.RLock()
	defer r.cacheLock.RUnlock()

	liq, ok := r.liquidityCache[symbol]
	if !ok {
		return nil, fmt.Errorf("liquidity info not found for %s", symbol)
	}

	liqCopy := *liq
	return &liqCopy, nil
}

// EstimateSlippage 估算滑点
func (r *OrderRouter) EstimateSlippage(order *Order) (float64, error) {
	liq, err := r.getLiquidityInfo(context.Background(), order.Symbol)
	if err != nil {
		return 0, err
	}

	// 简化版滑点估算
	var availableLiquidity float64
	if order.Side == OrderSideBuy {
		availableLiquidity = liq.AskVolume
	} else {
		availableLiquidity = liq.BidVolume
	}

	if order.Quantity <= availableLiquidity {
		return 0.0, nil
	}

	// 超过流动性的部分估算滑点
	excessRatio := (order.Quantity - availableLiquidity) / availableLiquidity
	return excessRatio * 0.001, nil // 0.1% per excess
}

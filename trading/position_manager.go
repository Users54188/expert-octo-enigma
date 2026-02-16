package trading

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// PositionManager 持仓管理器
type PositionManager struct {
	connector *BrokerConnector
	positions map[string]*PositionState
	mu        sync.RWMutex
}

// PositionState 持仓状态（包含成本计算）
type PositionState struct {
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Amount        int       `json:"amount"`
	Available     int       `json:"available"`
	CostPrice     float64   `json:"cost_price"`
	TotalCost     float64   `json:"total_cost"`
	CurrentPrice  float64   `json:"current_price"`
	MarketValue   float64   `json:"market_value"`
	UnrealizedPnL float64   `json:"unrealized_pnl"`
	RealizedPnL   float64   `json:"realized_pnl"`
	UpdateTime    time.Time `json:"update_time"`
}

// NewPositionManager 创建持仓管理器
func NewPositionManager(connector *BrokerConnector) *PositionManager {
	pm := &PositionManager{
		connector: connector,
		positions: make(map[string]*PositionState),
	}

	// 初始加载持仓
	pm.syncPositions()

	return pm
}

// SyncPositions 同步持仓
func (pm *PositionManager) SyncPositions() error {
	return pm.syncPositions()
}

// syncPositions 内部同步方法
func (pm *PositionManager) syncPositions() error {
	positions, err := pm.connector.GetCachedPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 清空现有持仓
	pm.positions = make(map[string]*PositionState)

	// 更新持仓
	for _, pos := range positions {
		pm.positions[pos.Symbol] = &PositionState{
			Symbol:        pos.Symbol,
			Name:          pos.Name,
			Amount:        pos.Amount,
			Available:     pos.Available,
			CostPrice:     pos.CostPrice,
			TotalCost:     float64(pos.Amount) * pos.CostPrice,
			CurrentPrice:  pos.CurrentPrice,
			MarketValue:   pos.MarketValue,
			UnrealizedPnL: pos.Profit,
			UpdateTime:    time.Now(),
		}
	}

	log.Printf("同步持仓完成，共 %d 只股票", len(pm.positions))
	return nil
}

// GetPosition 获取单个持仓
func (pm *PositionManager) GetPosition(symbol string) (*PositionState, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pos, ok := pm.positions[symbol]; ok {
		return pos, nil
	}

	return nil, fmt.Errorf("未找到持仓: %s", symbol)
}

// GetAllPositions 获取所有持仓
func (pm *PositionManager) GetAllPositions() []*PositionState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*PositionState, 0, len(pm.positions))
	for _, pos := range pm.positions {
		result = append(result, pos)
	}

	return result
}

// HasPosition 检查是否有持仓
func (pm *PositionManager) HasPosition(symbol string) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	_, ok := pm.positions[symbol]
	return ok
}

// UpdatePosition 更新持仓（用于成交后）
func (pm *PositionManager) UpdatePosition(trade Trade) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	symbol := trade.Symbol

	// 获取当前价格
	currentPrice := trade.Price

	switch trade.Type {
	case "买入", "buy":
		// 买入：新增持仓或增加数量
		cost := float64(trade.Amount) * trade.Price

		if pos, ok := pm.positions[symbol]; ok {
			// 重新计算成本价
			totalAmount := pos.Amount + trade.Amount
			newCostPrice := (pos.TotalCost + cost) / float64(totalAmount)

			pos.Amount = totalAmount
			pos.TotalCost += cost
			pos.CostPrice = newCostPrice
			pos.CurrentPrice = currentPrice
			pos.MarketValue = float64(pos.Amount) * currentPrice
			pos.UnrealizedPnL = pos.MarketValue - pos.TotalCost
			pos.UpdateTime = time.Now()
		} else {
			// 新建持仓
			pm.positions[symbol] = &PositionState{
				Symbol:        symbol,
				Name:          trade.Name,
				Amount:        trade.Amount,
				CostPrice:     trade.Price,
				TotalCost:     cost,
				CurrentPrice:  currentPrice,
				MarketValue:   float64(trade.Amount) * currentPrice,
				UnrealizedPnL: 0,
				UpdateTime:    time.Now(),
			}
		}

	case "卖出", "sell":
		// 卖出：减少持仓或清仓
		if pos, ok := pm.positions[symbol]; ok {
			sellValue := float64(trade.Amount) * trade.Price
			costPerShare := pos.TotalCost / float64(pos.Amount)
			soldCost := float64(trade.Amount) * costPerShare

			// 计算已实现盈亏
			profit := sellValue - soldCost
			pos.RealizedPnL += profit

			// 减少持仓
			pos.Amount -= trade.Amount
			pos.TotalCost -= soldCost

			if pos.Amount <= 0 {
				// 清仓，删除持仓
				delete(pm.positions, symbol)
				log.Printf("持仓 %s 已清仓，实现盈亏: %.2f", symbol, pos.RealizedPnL)
			} else {
				// 部分卖出，更新市值
				pos.CurrentPrice = currentPrice
				pos.MarketValue = float64(pos.Amount) * currentPrice
				pos.UnrealizedPnL = pos.MarketValue - pos.TotalCost
				pos.UpdateTime = time.Now()
			}
		}
	}

	return nil
}

// GetTotalMarketValue 获取总持仓市值
func (pm *PositionManager) GetTotalMarketValue() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	total := 0.0
	for _, pos := range pm.positions {
		total += pos.MarketValue
	}

	return total
}

// GetTotalUnrealizedPnL 获取总浮动盈亏
func (pm *PositionManager) GetTotalUnrealizedPnL() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	total := 0.0
	for _, pos := range pm.positions {
		total += pos.UnrealizedPnL
	}

	return total
}

// GetTotalRealizedPnL 获取总已实现盈亏
func (pm *PositionManager) GetTotalRealizedPnL() float64 {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	total := 0.0
	for _, pos := range pm.positions {
		total += pos.RealizedPnL
	}

	return total
}

// GetPositionCount 获取持仓数量
func (pm *PositionManager) GetPositionCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return len(pm.positions)
}

// RefreshPrices 刷新持仓价格
func (pm *PositionManager) RefreshPrices(priceMap map[string]float64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for symbol, price := range priceMap {
		if pos, ok := pm.positions[symbol]; ok {
			pos.CurrentPrice = price
			pos.MarketValue = float64(pos.Amount) * price
			pos.UnrealizedPnL = pos.MarketValue - pos.TotalCost
			pos.UpdateTime = time.Now()
		}
	}

	return nil
}

// CalculatePositionValue 计算持仓价值
func (pm *PositionManager) CalculatePositionValue(symbol string) (float64, error) {
	pos, err := pm.GetPosition(symbol)
	if err != nil {
		return 0, err
	}

	return pos.MarketValue, nil
}

// GetPositionSummary 获取持仓摘要
func (pm *PositionManager) GetPositionSummary() PositionSummary {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	summary := PositionSummary{
		PositionCount:      len(pm.positions),
		TotalMarketValue:   0,
		TotalUnrealizedPnL: 0,
		TotalRealizedPnL:   0,
		Positions:          make([]*PositionState, 0, len(pm.positions)),
	}

	for _, pos := range pm.positions {
		summary.TotalMarketValue += pos.MarketValue
		summary.TotalUnrealizedPnL += pos.UnrealizedPnL
		summary.TotalRealizedPnL += pos.RealizedPnL
		summary.Positions = append(summary.Positions, pos)
	}

	return summary
}

// PositionSummary 持仓摘要
type PositionSummary struct {
	PositionCount      int              `json:"position_count"`
	TotalMarketValue   float64          `json:"total_market_value"`
	TotalUnrealizedPnL float64          `json:"total_unrealized_pnl"`
	TotalRealizedPnL   float64          `json:"total_realized_pnl"`
	Positions          []*PositionState `json:"positions"`
}

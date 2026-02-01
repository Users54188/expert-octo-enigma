package trading

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// RiskManager 风险管理器
type RiskManager struct {
	config      RiskConfig
	connector   *BrokerConnector
	tradeHistory *TradeHistory
	mu          sync.RWMutex
	dailyPnL    float64
	dailyStartEquity float64
	emergencyStop bool
}

// RiskConfig 风险配置
type RiskConfig struct {
	InitialCapital    float64 `yaml:"initial_capital" json:"initial_capital"`       // 初始资金
	MaxSinglePosition float64 `yaml:"max_single_position" json:"max_single_position"` // 单只股票最大仓位比例
	MaxPositions      int     `yaml:"max_positions" json:"max_positions"`           // 最大持仓数量
	MaxDailyLoss      float64 `yaml:"max_daily_loss" json:"max_daily_loss"`           // 单日最大亏损比例
	MinOrderAmount    float64 `yaml:"min_order_amount" json:"min_order_amount"`       // 最小下单金额
	StopLossPercent   float64 `yaml:"stop_loss_percent" json:"stop_loss_percent"`     // 单只股票止损比例
}

// DefaultRiskConfig 默认风险配置
var DefaultRiskConfig = RiskConfig{
	InitialCapital:    100.0,    // 100元初始资金
	MaxSinglePosition: 0.3,      // 单只股票最多30%
	MaxPositions:      3,        // 最多3只股票
	MaxDailyLoss:      0.1,      // 单日亏损10%全部平仓
	MinOrderAmount:    100.0,    // 最小下单金额100元
	StopLossPercent:   0.05,     // 单只股票亏损5%止损
}

// NewRiskManager 创建风险管理器
func NewRiskManager(config RiskConfig, connector *BrokerConnector, tradeHistory *TradeHistory) *RiskManager {
	if config.InitialCapital == 0 {
		config = DefaultRiskConfig
	}

	rm := &RiskManager{
		config:       config,
		connector:    connector,
		tradeHistory: tradeHistory,
		dailyPnL:     0,
		emergencyStop: false,
	}

	// 初始化当日初始权益
	rm.initDailyEquity()

	return rm
}

// initDailyEquity 初始化当日权益
func (rm *RiskManager) initDailyEquity() {
	balance, err := rm.connector.GetCachedBalance()
	if err != nil {
		log.Printf("获取初始余额失败: %v", err)
		rm.dailyStartEquity = rm.config.InitialCapital
	} else {
		rm.dailyStartEquity = balance.TotalAssets
	}
	log.Printf("当日初始权益: %.2f", rm.dailyStartEquity)
}

// CheckBeforeOrder 订单前风险检查
func (rm *RiskManager) CheckBeforeOrder(ctx context.Context, order OrderRequest) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 紧急停止检查
	if rm.emergencyStop {
		return ErrEmergencyStop
	}

	// 检查单日亏损
	if err := rm.checkDailyLoss(ctx); err != nil {
		return err
	}

	// 检查最小下单金额
	if order.Amount < rm.config.MinOrderAmount {
		return fmt.Errorf("%w: 订单金额 %.2f 小于最小金额 %.2f", ErrMinOrderAmount, order.Amount, rm.config.MinOrderAmount)
	}

	// 买单检查
	if order.Type == OrderTypeBuy {
		if err := rm.checkBuyOrder(ctx, order); err != nil {
			return err
		}
	}

	return nil
}

// checkBuyOrder 检查买单
func (rm *RiskManager) checkBuyOrder(ctx context.Context, order OrderRequest) error {
	// 获取当前余额
	balance, err := rm.connector.GetCachedBalance()
	if err != nil {
		return fmt.Errorf("获取余额失败: %w", err)
	}

	// 检查可用资金
	if order.Amount > balance.AvailableCash {
		return fmt.Errorf("%w: 可用资金 %.2f 不足", ErrInsufficientCash, balance.AvailableCash)
	}

	// 检查单只股票最大仓位
	maxSingleAmount := rm.config.InitialCapital * rm.config.MaxSinglePosition
	if order.Amount > maxSingleAmount {
		return fmt.Errorf("%w: 订单金额 %.2f 超过单只股票最大金额 %.2f", ErrMaxPositionExceeded, order.Amount, maxSingleAmount)
	}

	// 获取当前持仓
	positions, err := rm.connector.GetCachedPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	// 检查是否已有该股票持仓
	for _, pos := range positions {
		if pos.Symbol == order.Symbol {
			// 已有持仓，检查买入后是否超限
			currentValue := float64(pos.Amount) * pos.CurrentPrice
			if currentValue+order.Amount > maxSingleAmount {
				return fmt.Errorf("%w: 买入后总金额 %.2f 超过单只股票最大金额 %.2f", ErrMaxPositionExceeded, currentValue+order.Amount, maxSingleAmount)
			}
			return nil
		}
	}

	// 检查最大持仓数量
	if len(positions) >= rm.config.MaxPositions {
		return fmt.Errorf("%w: 当前持仓 %d 只，已达最大持仓数量 %d", ErrMaxPositionsExceeded, len(positions), rm.config.MaxPositions)
	}

	return nil
}

// checkDailyLoss 检查单日亏损
func (rm *RiskManager) checkDailyLoss(ctx context.Context) error {
	balance, err := rm.connector.GetCachedBalance()
	if err != nil {
		return fmt.Errorf("获取余额失败: %w", err)
	}

	// 计算当日盈亏
	dailyProfit := balance.TotalAssets - rm.dailyStartEquity
	lossPercent := 0.0
	if rm.dailyStartEquity > 0 {
		lossPercent = dailyProfit / rm.dailyStartEquity
	}

	// 如果亏损超过阈值，触发紧急平仓
	if lossPercent < -rm.config.MaxDailyLoss {
		rm.mu.Lock()
		rm.emergencyStop = true
		rm.mu.Unlock()

		log.Printf("警告：单日亏损 %.2f%% 超过阈值 %.2f%%，触发紧急平仓", lossPercent*100, rm.config.MaxDailyLoss*100)

		// 异步执行紧急平仓
		go rm.emergencyClosePositions()

		return ErrDailyLossExceeded
	}

	return nil
}

// emergencyClosePositions 紧急平仓
func (rm *RiskManager) emergencyClosePositions() {
	log.Println("开始紧急平仓...")

	positions, err := rm.connector.GetCachedPositions()
	if err != nil {
		log.Printf("获取持仓失败: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	for _, pos := range positions {
		// 使用当前价卖出
		_, err := rm.connector.GetBroker().Sell(ctx, pos.Symbol, pos.CurrentPrice, pos.Amount)
		if err != nil {
			log.Printf("紧急卖出 %s 失败: %v", pos.Symbol, err)
		} else {
			log.Printf("紧急卖出 %s, 数量: %d, 价格: %.2f", pos.Symbol, pos.Amount, pos.CurrentPrice)
		}
	}

	log.Println("紧急平仓完成")
}

// CheckPositionLoss 检查持仓止损
func (rm *RiskManager) CheckPositionLoss(ctx context.Context) ([]string, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.emergencyStop {
		return nil, ErrEmergencyStop
	}

	positions, err := rm.connector.GetCachedPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	var stopLossSymbols []string

	for _, pos := range positions {
		// 计算盈亏比例
		profitPercent := pos.ProfitPercent / 100.0

		// 如果亏损超过止损比例，触发止损
		if profitPercent < -rm.config.StopLossPercent {
			stopLossSymbols = append(stopLossSymbols, pos.Symbol)
			log.Printf("止损触发: %s 盈亏 %.2f%%, 阈值 %.2f%%", pos.Symbol, profitPercent*100, rm.config.StopLossPercent*100)
		}
	}

	return stopLossSymbols, nil
}

// UpdateDailyPnL 更新当日盈亏
func (rm *RiskManager) UpdateDailyPnL(ctx context.Context) (float64, error) {
	balance, err := rm.connector.GetCachedBalance()
	if err != nil {
		return 0, fmt.Errorf("获取余额失败: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.dailyPnL = balance.TotalAssets - rm.dailyStartEquity

	// 保存日度盈亏
	if rm.tradeHistory != nil {
		date := time.Now().Format("2006-01-02")
		_ = rm.tradeHistory.SaveDailyPnL(DailyPnL{
			Date:     date,
			OpenEquity: rm.dailyStartEquity,
			CloseEquity: balance.TotalAssets,
			PnL:      rm.dailyPnL,
			PnLPercent: rm.dailyPnL / rm.dailyStartEquity,
		})
	}

	return rm.dailyPnL, nil
}

// GetRiskMetrics 获取风险指标
func (rm *RiskManager) GetRiskMetrics() RiskMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	balance, _ := rm.connector.GetCachedBalance()
	positions, _ := rm.connector.GetCachedPositions()

	metrics := RiskMetrics{
		InitialCapital: rm.config.InitialCapital,
		CurrentEquity: 0,
		DailyPnL:       rm.dailyPnL,
		DailyPnLPercent: 0,
		PositionCount:  len(positions),
		EmergencyStop:  rm.emergencyStop,
	}

	if balance != nil {
		metrics.CurrentEquity = balance.TotalAssets
		if rm.dailyStartEquity > 0 {
			metrics.DailyPnLPercent = rm.dailyPnL / rm.dailyStartEquity
		}
	}

	return metrics
}

// ResetDaily 重置当日状态（新交易日）
func (rm *RiskManager) ResetDaily() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.emergencyStop = false
	rm.initDailyEquity()
	rm.dailyPnL = 0

	log.Println("重置当日风险状态")
}

// SetEmergencyStop 设置紧急停止
func (rm *RiskManager) SetEmergencyStop(stop bool) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.emergencyStop = stop

	if stop {
		log.Println("手动触发紧急停止")
	} else {
		log.Println("解除紧急停止")
	}
}

// RiskMetrics 风险指标
type RiskMetrics struct {
	InitialCapital  float64 `json:"initial_capital"`
	CurrentEquity   float64 `json:"current_equity"`
	DailyPnL        float64 `json:"daily_pnl"`
	DailyPnLPercent float64 `json:"daily_pnl_percent"`
	PositionCount   int     `json:"position_count"`
	EmergencyStop   bool    `json:"emergency_stop"`
}

// 订单类型
const (
	OrderTypeBuy  = "buy"
	OrderTypeSell = "sell"
)

// OrderRequest 订单请求
type OrderRequest struct {
	Type   string  `json:"type"`
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Amount int     `json:"amount"` // 金额（元）
}

// 计算订单数量
func (o *OrderRequest) CalculateQuantity() int {
	if o.Price <= 0 {
		return 0
	}
	return int(o.Amount / o.Price / 100) * 100 // 按手数（100股）下单
}

var (
	// ErrEmergencyStop 紧急停止错误
	ErrEmergencyStop = fmt.Errorf("紧急停止已触发")
	// ErrMinOrderAmount 最小下单金额错误
	ErrMinOrderAmount = fmt.Errorf("订单金额不足")
	// ErrInsufficientCash 资金不足错误
	ErrInsufficientCash = fmt.Errorf("资金不足")
	// ErrMaxPositionExceeded 超过最大仓位错误
	ErrMaxPositionExceeded = fmt.Errorf("超过单只股票最大仓位")
	// ErrMaxPositionsExceeded 超过最大持仓数量错误
	ErrMaxPositionsExceeded = fmt.Errorf("超过最大持仓数量")
	// ErrDailyLossExceeded 超过单日最大亏损错误
	ErrDailyLossExceeded = fmt.Errorf("超过单日最大亏损")
)

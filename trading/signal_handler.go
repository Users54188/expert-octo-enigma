package trading

import (
	"context"
	"fmt"
	"log"
	"time"
)

// SignalHandler 信号处理器，融合AI和ML信号进行交易决策
type SignalHandler struct {
	aiThreshold   float64
	mlConfidence  float64
	riskManager   *RiskManager
	positionMgr   *PositionManager
	orderExecutor *OrderExecutor
}

// AISignal AI分析信号
type AISignal struct {
	Symbol   string  `json:"symbol"`
	Trend    string  `json:"trend"`     // 上升/下降/震荡
	Risk     string  `json:"risk"`      // 高/中/低
	Action   string  `json:"action"`    // 买入/卖出/持有
	Reason   string  `json:"reason"`
	Confidence float64 `json:"confidence"` // 0-1
	Timestamp time.Time `json:"timestamp"`
}

// MLSignal ML预测信号
type MLSignal struct {
	Symbol     string    `json:"symbol"`
	Label      int       `json:"label"`      // 0=卖出, 1=持有, 2=买入
	Confidence float64   `json:"confidence"`  // 0-1
	Timestamp  time.Time `json:"timestamp"`
}

// TradingSignal 交易信号（融合后的信号）
type TradingSignal struct {
	Symbol       string    `json:"symbol"`
	Action       string    `json:"action"`      // buy/sell/hold
	Confidence   float64   `json:"confidence"`  // 综合置信度
	AIAction     string    `json:"ai_action"`
	AIConfidence float64   `json:"ai_confidence"`
	MLLabel      int       `json:"ml_label"`
	MLConfidence float64   `json:"ml_confidence"`
	Reason       string    `json:"reason"`
	Timestamp    time.Time `json:"timestamp"`
}

// NewSignalHandler 创建信号处理器
func NewSignalHandler(
	aiThreshold, mlConfidence float64,
	riskManager *RiskManager,
	positionMgr *PositionManager,
	orderExecutor *OrderExecutor,
) *SignalHandler {
	return &SignalHandler{
		aiThreshold:   aiThreshold,
		mlConfidence:  mlConfidence,
		riskManager:   riskManager,
		positionMgr:   positionMgr,
		orderExecutor: orderExecutor,
	}
}

// ProcessSignal 处理AI和ML信号，生成交易决策
func (sh *SignalHandler) ProcessSignal(ctx context.Context, aiSignal AISignal, mlSignal MLSignal) (*TradingSignal, error) {
	// 1. 评估AI信号
	aiScore := sh.evaluateAISignal(aiSignal)

	// 2. 评估ML信号
	mlScore := sh.evaluateMLSignal(mlSignal)

	// 3. 融合信号
	signal := sh.fuseSignals(aiSignal, mlSignal, aiScore, mlScore)

	// 4. 应用风险过滤
	signal = sh.applyRiskFilter(ctx, signal)

	return signal, nil
}

// evaluateAISignal 评估AI信号
func (sh *SignalHandler) evaluateAISignal(signal AISignal) float64 {
	score := signal.Confidence

	// 根据风险等级调整评分
	switch signal.Risk {
	case "高":
		score *= 0.5
	case "中":
		score *= 0.8
	case "低":
		score *= 1.0
	}

	// 根据趋势调整
	if signal.Trend == "上升" && signal.Action == "买入" {
		score *= 1.1
	} else if signal.Trend == "下降" && signal.Action == "卖出" {
		score *= 1.1
	}

	// 限制在0-1之间
	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}

	return score
}

// evaluateMLSignal 评估ML信号
func (sh *SignalHandler) evaluateMLSignal(signal MLSignal) float64 {
	// 将label转换为动作权重
	var weight float64
	switch signal.Label {
	case 2: // 买入
		weight = 1.0
	case 1: // 持有
		weight = 0.5
	case 0: // 卖出
		weight = -1.0
	default:
		weight = 0
	}

	return weight * signal.Confidence
}

// fuseSignals 融合AI和ML信号
func (sh *SignalHandler) fuseSignals(aiSignal AISignal, mlSignal MLSignal, aiScore, mlScore float64) *TradingSignal {
	// 计算综合得分
	totalScore := aiScore + mlScore

	// 确定动作
	var action string
	var confidence float64

	if totalScore > 1.0 {
		action = "buy"
		confidence = (totalScore - 1.0) / 2.0
	} else if totalScore < -0.5 {
		action = "sell"
		confidence = (-totalScore - 0.5) / 2.0
	} else {
		action = "hold"
		confidence = 1.0 - (totalScore + 0.5)
	}

	// 限制置信度
	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0 {
		confidence = 0
	}

	// 生成理由
	reason := fmt.Sprintf("AI: %s(%s,%.2f) + ML: label%d(%.2f) = %s(%.2f)",
		aiSignal.Action, aiSignal.Trend, aiScore,
		mlSignal.Label, mlScore,
		action, confidence)

	return &TradingSignal{
		Symbol:       aiSignal.Symbol,
		Action:       action,
		Confidence:   confidence,
		AIAction:     aiSignal.Action,
		AIConfidence: aiSignal.Confidence,
		MLLabel:      mlSignal.Label,
		MLConfidence: mlSignal.Confidence,
		Reason:       reason,
		Timestamp:    time.Now(),
	}
}

// applyRiskFilter 应用风险过滤
func (sh *SignalHandler) applyRiskFilter(ctx context.Context, signal *TradingSignal) *TradingSignal {
	// 检查紧急停止
	if sh.riskManager != nil {
		metrics := sh.riskManager.GetRiskMetrics()
		if metrics.EmergencyStop {
			signal.Action = "hold"
			signal.Reason += " [紧急停止]"
			return signal
		}
	}

	// 检查持仓
	if signal.Action == "buy" {
		if sh.positionMgr.HasPosition(signal.Symbol) {
			// 已有持仓，考虑是否加仓
			pos, _ := sh.positionMgr.GetPosition(signal.Symbol)
			if pos.UnrealizedPnL > 0 {
				// 盈利状态，可以考虑加仓
				signal.Reason += " [持仓盈利,考虑加仓]"
			} else {
				// 亏损状态，不追加
				signal.Action = "hold"
				signal.Reason += " [持仓亏损,不加仓]"
			}
		}
	}

	// 检查止损
	if signal.Action == "sell" && sh.positionMgr.HasPosition(signal.Symbol) {
		pos, _ := sh.positionMgr.GetPosition(signal.Symbol)
		if pos.UnrealizedPnL < 0 {
			// 亏损状态，优先执行止损
			signal.Action = "sell"
			signal.Reason += " [止损优先]"
		}
	}

	return signal
}

// ExecuteSignal 执行交易信号
func (sh *SignalHandler) ExecuteSignal(ctx context.Context, signal *TradingSignal, price float64, amount float64) (string, error) {
	if signal == nil {
		return "", fmt.Errorf("信号为空")
	}

	log.Printf("执行交易信号: %s - 动作: %s, 价格: %.2f, 置信度: %.2f, 原因: %s",
		signal.Symbol, signal.Action, price, signal.Confidence, signal.Reason)

	switch signal.Action {
	case "buy":
		// 检查置信度是否达到阈值
		if signal.Confidence < sh.aiThreshold {
			return "", fmt.Errorf("买入置信度 %.2f 低于阈值 %.2f", signal.Confidence, sh.aiThreshold)
		}
		return sh.orderExecutor.ExecuteBuy(ctx, signal.Symbol, price, amount)

	case "sell":
		// 获取持仓数量
		if !sh.positionMgr.HasPosition(signal.Symbol) {
			return "", fmt.Errorf("无持仓，无法卖出")
		}
		pos, _ := sh.positionMgr.GetPosition(signal.Symbol)
		return sh.orderExecutor.ExecuteSell(ctx, signal.Symbol, price, pos.Amount)

	case "hold":
		return "", nil // 不操作

	default:
		return "", fmt.Errorf("未知的交易动作: %s", signal.Action)
	}
}

// CreateBuySignal 创建买入信号（用于测试）
func CreateBuySignal(symbol string, confidence float64) AISignal {
	return AISignal{
		Symbol:     symbol,
		Trend:      "上升",
		Risk:       "低",
		Action:     "买入",
		Reason:     "技术面强势突破",
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}

// CreateSellSignal 创建卖出信号（用于测试）
func CreateSellSignal(symbol string, confidence float64) AISignal {
	return AISignal{
		Symbol:     symbol,
		Trend:      "下降",
		Risk:       "中",
		Action:     "卖出",
		Reason:     "技术面走弱",
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}

// CreateHoldSignal 创建持有信号（用于测试）
func CreateHoldSignal(symbol string, confidence float64) AISignal {
	return AISignal{
		Symbol:     symbol,
		Trend:      "震荡",
		Risk:       "中",
		Action:     "持有",
		Reason:     "方向不明确",
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}

// CreateMLBuySignal 创建ML买入信号（用于测试）
func CreateMLBuySignal(symbol string, confidence float64) MLSignal {
	return MLSignal{
		Symbol:     symbol,
		Label:      2, // 买入
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}

// CreateMLSellSignal 创建ML卖出信号（用于测试）
func CreateMLSellSignal(symbol string, confidence float64) MLSignal {
	return MLSignal{
		Symbol:     symbol,
		Label:      0, // 卖出
		Confidence: confidence,
		Timestamp:  time.Now(),
	}
}

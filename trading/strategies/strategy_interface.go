package strategies

import (
    "context"
    "time"

    "cloudquant/trading"
)

// Strategy 策略接口 - 定义所有策略必须实现的方法
type Strategy interface {
    // Init 策略初始化
    Init(ctx context.Context, symbol string, config map[string]interface{}) error

    // GenerateSignal 生成交易信号
    GenerateSignal(ctx context.Context, marketData *MarketData) (*Signal, error)

    // OnTrade 交易执行回调
    OnTrade(ctx context.Context, trade *trading.TradeRecord) error

    // OnDailyClose 每日收盘回调
    OnDailyClose(ctx context.Context, date time.Time) error

    // GetName 获取策略名称
    GetName() string

    // GetWeight 获取策略权重
    GetWeight() float64

    // SetWeight 设置策略权重
    SetWeight(weight float64)

    // GetParameters 获取策略参数
    GetParameters() map[string]interface{}

    // UpdateParameters 更新策略参数
    UpdateParameters(params map[string]interface{}) error

    // IsEnabled 检查策略是否启用
    IsEnabled() bool

    // SetEnabled 设置策略启用状态
    SetEnabled(enabled bool)
}

// Signal 交易信号结构
type Signal struct {
    Symbol     string    `json:"symbol"`     // 股票代码
    SignalType string    `json:"signal_type"` // 信号类型: buy, sell, hold
    Strength   float64   `json:"strength"`    // 信号强度: 0-1
    Price      float64   `json:"price"`      // 当前价格
    TargetPrice float64  `json:"target_price"` // 目标价格
    StopLoss   float64   `json:"stop_loss"`   // 止损价格
    Timestamp  time.Time `json:"timestamp"`   // 信号时间
    Reason     string    `json:"reason"`     // 信号原因
    Metadata   map[string]interface{} `json:"metadata"` // 额外信息
}

// MarketData 市场数据结构
type MarketData struct {
    Symbol      string    `json:"symbol"`
    Open        float64   `json:"open"`
    High        float64   `json:"high"`
    Low         float64   `json:"low"`
    Close       float64   `json:"close"`
    Volume      int64     `json:"volume"`
    Amount      float64   `json:"amount"`
    Timestamp   time.Time `json:"timestamp"`
    PreClose    float64   `json:"pre_close"`
    Change      float64   `json:"change"`
    ChangePercent float64 `json:"change_percent"`
}

// StrategyResult 策略执行结果
type StrategyResult struct {
    Signals    []*Signal `json:"signals"`    // 生成的信号
    Score      float64   `json:"score"`      // 策略评分
    Confidence float64   `json:"confidence"` // 策略置信度
    Duration   int64     `json:"duration"`   // 执行时间(毫秒)
    Error      error     `json:"error"`      // 执行错误
}

// StrategyType 策略类型
type StrategyType string

const (
    MA       StrategyType = "ma"       // 均线策略
    RSI      StrategyType = "rsi"      // RSI策略
    AI       StrategyType = "ai"       // AI策略
    ML       StrategyType = "ml"       // 机器学习策略
    CUSTOM   StrategyType = "custom"   // 自定义策略
)

// BaseStrategy 策略基类，实现通用功能
type BaseStrategy struct {
    name       string
    weight     float64
    enabled    bool
    parameters map[string]interface{}
    createdAt  time.Time
}

// NewBaseStrategy 创建基础策略
func NewBaseStrategy(name string, weight float64) *BaseStrategy {
    return &BaseStrategy{
        name:       name,
        weight:     weight,
        enabled:    true,
        parameters: make(map[string]interface{}),
        createdAt:  time.Now(),
    }
}

// GetName 实现Strategy接口
func (b *BaseStrategy) GetName() string {
    return b.name
}

// GetWeight 实现Strategy接口
func (b *BaseStrategy) GetWeight() float64 {
    return b.weight
}

// SetWeight 实现Strategy接口
func (b *BaseStrategy) SetWeight(weight float64) {
    b.weight = weight
}

// IsEnabled 实现Strategy接口
func (b *BaseStrategy) IsEnabled() bool {
    return b.enabled
}

// SetEnabled 实现Strategy接口
func (b *BaseStrategy) SetEnabled(enabled bool) {
    b.enabled = enabled
}

// GetParameters 实现Strategy接口
func (b *BaseStrategy) GetParameters() map[string]interface{} {
    return b.parameters
}

// UpdateParameters 实现Strategy接口
func (b *BaseStrategy) UpdateParameters(params map[string]interface{}) error {
    b.parameters = make(map[string]interface{})
    for k, v := range params {
        b.parameters[k] = v
    }
    return nil
}

// Init 基础初始化
func (b *BaseStrategy) Init(ctx context.Context, symbol string, config map[string]interface{}) error {
    if config != nil {
        b.parameters = make(map[string]interface{})
        for k, v := range config {
            b.parameters[k] = v
        }
    }
    return nil
}

// OnTrade 基础交易回调
func (b *BaseStrategy) OnTrade(ctx context.Context, trade *trading.TradeRecord) error {
    // 默认实现：记录交易日志
    return nil
}

// OnDailyClose 基础收盘回调
func (b *BaseStrategy) OnDailyClose(ctx context.Context, date time.Time) error {
    // 默认实现：进行日终处理
    return nil
}

// NewSignal 创建新信号
func NewSignal(symbol, signalType string, strength, price float64) *Signal {
    return &Signal{
        Symbol:     symbol,
        SignalType: signalType,
        Strength:   strength,
        Price:      price,
        Timestamp:  time.Now(),
        Metadata:   make(map[string]interface{}),
    }
}

// ValidateSignal 验证信号有效性
func ValidateSignal(signal *Signal) error {
    if signal.Symbol == "" {
        return ErrInvalidSymbol
    }
    if signal.SignalType != "buy" && signal.SignalType != "sell" && signal.SignalType != "hold" {
        return ErrInvalidSignalType
    }
    if signal.Strength < 0 || signal.Strength > 1 {
        return ErrInvalidStrength
    }
    if signal.Price <= 0 {
        return ErrInvalidPrice
    }
    return nil
}

// 错误定义
var (
    ErrInvalidSymbol     = NewStrategyError("invalid symbol")
    ErrInvalidSignalType = NewStrategyError("invalid signal type")
    ErrInvalidStrength   = NewStrategyError("invalid strength")
    ErrInvalidPrice      = NewStrategyError("invalid price")
)

// StrategyError 策略错误
type StrategyError struct {
    message string
}

func NewStrategyError(msg string) *StrategyError {
    return &StrategyError{message: msg}
}

func (e *StrategyError) Error() string {
    return e.message
}
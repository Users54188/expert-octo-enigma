package trading

import (
    "context"
    "errors"
    "fmt"
    "log"
    "sync"
    "time"
)

var (
    // ErrNotConnected 券商未连接错误
    ErrNotConnected = errors.New("券商未连接")
    // ErrInvalidBrokerType 无效的券商类型
    ErrInvalidBrokerType = errors.New("无效的券商类型")
)

// BrokerConnector 券商连接器，管理券商连接的生命周期
type BrokerConnector struct {
    broker       Broker
    config       BrokerConfig
    retryConfig  RetryConfig
    healthCheck  *time.Ticker
    stopHealth   chan struct{}
    mu           sync.RWMutex
    initialized  bool
}

// BrokerConfig 券商配置
type BrokerConfig struct {
    Type     string `yaml:"type" json:"type"`             // 券商类型: easytrader
    Service  string `yaml:"service" json:"service"`       // 服务地址
    Broker   string `yaml:"broker" json:"broker"`         // 具体券商: ht, yh, yjb
    Username string `yaml:"username" json:"username"`     // 用户名
    Password string `yaml:"password" json:"password"`     // 密码
    ExePath  string `yaml:"exe_path" json:"exe_path"`    // 客户端路径
}

// RetryConfig 重试配置
type RetryConfig struct {
    MaxRetries    int           `yaml:"max_retries" json:"max_retries"`
    RetryInterval time.Duration `yaml:"retry_interval" json:"retry_interval"`
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
    MaxRetries:    3,
    RetryInterval: 5 * time.Second,
}

// NewBrokerConnector 创建券商连接器
func NewBrokerConnector(config BrokerConfig) (*BrokerConnector, error) {
    connector := &BrokerConnector{
        config:      config,
        retryConfig: DefaultRetryConfig,
        stopHealth:  make(chan struct{}),
    }

    // 根据类型创建broker实例
    if err := connector.initBroker(); err != nil {
        return nil, err
    }

    connector.initialized = true
    return connector, nil
}

// initBroker 初始化券商实例
func (bc *BrokerConnector) initBroker() error {
    switch bc.config.Type {
    case "easytrader":
        broker := NewEasyTraderBroker(bc.config.Service, bc.config.Broker)
        bc.mu.Lock()
        bc.broker = broker
        bc.mu.Unlock()
        return nil
    default:
        return fmt.Errorf("%w: %s", ErrInvalidBrokerType, bc.config.Type)
    }
}

// Connect 连接券商
func (bc *BrokerConnector) Connect() error {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    if bc.broker == nil {
        if err := bc.initBroker(); err != nil {
            return err
        }
    }

    // 登录
    ctx, cancel := contextWithTimeout(60 * time.Second)
    defer cancel()

    err := bc.retryOperation(func() error {
        return bc.broker.Login(ctx, bc.config.Username, bc.config.Password, bc.config.ExePath)
    }, "登录券商")

    if err != nil {
        return fmt.Errorf("连接券商失败: %w", err)
    }

    // 启动健康检查
    bc.startHealthCheck()

    log.Printf("成功连接券商: %s (%s)", bc.config.Broker, bc.config.Service)
    return nil
}

// Disconnect 断开连接
func (bc *BrokerConnector) Disconnect() error {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    // 停止健康检查
    bc.stopHealthCheck()

    if bc.broker != nil {
        ctx, cancel := contextWithTimeout(10 * time.Second)
        defer cancel()
        if err := bc.broker.Logout(ctx); err != nil {
            log.Printf("登出券商失败: %v", err)
        }
    }

    log.Println("已断开券商连接")
    return nil
}

// Reconnect 重新连接
func (bc *BrokerConnector) Reconnect() error {
    log.Println("尝试重新连接券商...")
    bc.Disconnect()
    return bc.Connect()
}

// GetBroker 获取broker实例
func (bc *BrokerConnector) GetBroker() Broker {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    return bc.broker
}

// IsConnected 检查连接状态
func (bc *BrokerConnector) IsConnected() bool {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    if bc.broker == nil {
        return false
    }
    return bc.broker.IsConnected()
}

// startHealthCheck 启动健康检查
func (bc *BrokerConnector) startHealthCheck() {
    bc.stopHealth = make(chan struct{})
    bc.healthCheck = time.NewTicker(30 * time.Second)

    go func() {
        for {
            select {
            case <-bc.healthCheck.C:
                bc.checkHealth()
            case <-bc.stopHealth:
                return
            }
        }
    }()
}

// stopHealthCheck 停止健康检查
func (bc *BrokerConnector) stopHealthCheck() {
    if bc.healthCheck != nil {
        bc.healthCheck.Stop()
    }
    if bc.stopHealth != nil {
        close(bc.stopHealth)
    }
}

// checkHealth 检查健康状态
func (bc *BrokerConnector) checkHealth() {
    if !bc.IsConnected() {
        log.Println("券商连接断开，尝试重新连接...")
        if err := bc.Reconnect(); err != nil {
            log.Printf("重新连接失败: %v", err)
        }
    }
}

// retryOperation 带重试的操作
func (bc *BrokerConnector) retryOperation(op func() error, operationName string) error {
    var lastErr error

    for i := 0; i < bc.retryConfig.MaxRetries; i++ {
        if i > 0 {
            log.Printf("%s 重试 %d/%d...", operationName, i, bc.retryConfig.MaxRetries)
            time.Sleep(bc.retryConfig.RetryInterval)
        }

        err := op()
        if err == nil {
            return nil
        }

        lastErr = err
        log.Printf("%s 失败: %v", operationName, err)
    }

    return fmt.Errorf("%s 失败，已重试 %d 次: %w", operationName, bc.retryConfig.MaxRetries, lastErr)
}

// contextWithTimeout 创建带超时的context
func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), timeout)
}

// GetCachedBalance 获取缓存的余额信息
func (bc *BrokerConnector) GetCachedBalance() (*Balance, error) {
    broker := bc.GetBroker()
    if broker == nil {
        return nil, ErrNotConnected
    }

    ctx, cancel := contextWithTimeout(10 * time.Second)
    defer cancel()

    return broker.GetBalance(ctx)
}

// GetCachedPositions 获取缓存的持仓信息
func (bc *BrokerConnector) GetCachedPositions() ([]Position, error) {
    broker := bc.GetBroker()
    if broker == nil {
        return nil, ErrNotConnected
    }

    ctx, cancel := contextWithTimeout(10 * time.Second)
    defer cancel()

    return broker.GetPositions(ctx)
}

// GetCachedOrders 获取缓存的委托信息
func (bc *BrokerConnector) GetCachedOrders() ([]Order, error) {
    broker := bc.GetBroker()
    if broker == nil {
        return nil, ErrNotConnected
    }

    ctx, cancel := contextWithTimeout(10 * time.Second)
    defer cancel()

    return broker.GetOrders(ctx)
}

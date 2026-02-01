package main

import (
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "cloudquant/db"
    qhttp "cloudquant/http"
    "cloudquant/llm"
    "cloudquant/ml"
    "cloudquant/trading"
    "gopkg.in/yaml.v2"
)

type Config struct {
    Symbols  []string `yaml:"symbols"`
    Database struct {
        Path string `yaml:"path"`
    } `yaml:"database"`
    Http struct {
        Port int `yaml:"port"`
    } `yaml:"http"`
    Log struct {
        Level string `yaml:"level"`
    } `yaml:"log"`
    LLM struct {
        Provider  string        `yaml:"provider"`
        APIKey    string        `yaml:"api_key"`
        Model     string        `yaml:"model"`
        Timeout   time.Duration `yaml:"timeout"`
        MaxTokens int           `yaml:"max_tokens"`
    } `yaml:"llm"`
    ML struct {
        ModelType    string `yaml:"model_type"`
        ModelPath    string `yaml:"model_path"`
        MaxTreeDepth int    `yaml:"max_tree_depth"`
        TrainInterval string `yaml:"train_interval"`
        Features     struct {
            LookbackDays  int `yaml:"lookback_days"`
            LookaheadDays int `yaml:"lookahead_days"`
        } `yaml:"features"`
        Training struct {
            MinDataPoints int     `yaml:"min_data_points"`
            TestRatio     float64 `yaml:"test_ratio"`
        } `yaml:"training"`
    } `yaml:"ml"`
    Trading struct {
        Broker struct {
            Type     string `yaml:"type"`
            Service  string `yaml:"service_url"`
            Broker   string `yaml:"broker_type"`
            Username string `yaml:"username"`
            Password string `yaml:"password"`
            ExePath  string `yaml:"exe_path"`
        } `yaml:"broker"`
        Risk struct {
            InitialCapital    float64 `yaml:"initial_capital"`
            MaxSinglePosition float64 `yaml:"max_single_position"`
            MaxPositions      int     `yaml:"max_positions"`
            MaxDailyLoss      float64 `yaml:"max_daily_loss"`
            MinOrderAmount    float64 `yaml:"min_order_amount"`
            StopLossPercent   float64 `yaml:"stop_loss_percent"`
        } `yaml:"risk"`
        AutoTrade struct {
            Enabled       bool    `yaml:"enabled"`
            CheckInterval string  `yaml:"check_interval"`
            AIThreshold   float64 `yaml:"ai_threshold"`
            MLConfidence  float64 `yaml:"ml_confidence"`
        } `yaml:"auto_trade"`
    } `yaml:"trading"`
}

func main() {
    // 1. Load config
    config, err := loadConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // 2. Initialize database
    if err := db.InitDB(config.Database.Path); err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    log.Printf("Database initialized at %s", config.Database.Path)

    initializeServices(config)

    // 3. Start HTTP server
    server := qhttp.NewServer(config.Http.Port)
    go func() {
        if err := server.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("HTTP server failed: %v", err)
        }
    }()

    // 4. Handle graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    log.Println("Shutting down...")

    if err := server.Stop(); err != nil {
        log.Printf("Server forced to shutdown: %v", err)
    }

    log.Println("Exiting")
}

func loadConfig(path string) (*Config, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var config Config
    if err := yaml.NewDecoder(file).Decode(&config); err != nil {
        return nil, err
    }
    return &config, nil
}

func initializeServices(config *Config) {
    if config == nil {
        return
    }
    analyzer := llm.NewDeepSeekAnalyzer(config.LLM.APIKey, config.LLM.Model, config.LLM.Timeout, config.LLM.MaxTokens)
    qhttp.SetAnalyzer(analyzer)

    if config.ML.ModelType != "" && config.ML.ModelPath != "" {
        if model, err := ml.LoadModel(config.ML.ModelType, config.ML.ModelPath); err == nil {
            qhttp.SetModelProvider(model)
        }
    }

    qhttp.SetTrainingConfig(qhttp.TrainingConfig{
        Symbol:       firstSymbol(config.Symbols),
        Days:         500,
        ModelType:    config.ML.ModelType,
        ModelPath:    config.ML.ModelPath,
        MaxTreeDepth: config.ML.MaxTreeDepth,
        TestRatio:    config.ML.Training.TestRatio,
    })

    // 初始化交易系统
    initializeTradingSystem(config)
}

func firstSymbol(symbols []string) string {
    if len(symbols) == 0 {
        return ""
    }
    return symbols[0]
}

func initializeTradingSystem(config *Config) {
    // 如果配置了券商，则初始化交易系统
    if config.Trading.Broker.Type != "" && config.Trading.Broker.Service != "" {
        log.Println("Initializing trading system...")

        // 1. 创建交易历史记录器
        tradeHistory, err := trading.NewTradeHistory(config.Database.Path)
        if err != nil {
            log.Printf("Failed to initialize trade history: %v", err)
            return
        }

        // 2. 创建券商连接器
        brokerConfig := trading.BrokerConfig{
            Type:     config.Trading.Broker.Type,
            Service:  config.Trading.Broker.Service,
            Broker:   config.Trading.Broker.Broker,
            Username: config.Trading.Broker.Username,
            Password: config.Trading.Broker.Password,
            ExePath:  config.Trading.Broker.ExePath,
        }

        brokerConnector, err := trading.NewBrokerConnector(brokerConfig)
        if err != nil {
            log.Printf("Failed to create broker connector: %v", err)
            return
        }

        // 3. 尝试连接券商
        if config.Trading.Broker.Username != "" && config.Trading.Broker.Password != "" {
            if err := brokerConnector.Connect(); err != nil {
                log.Printf("Failed to connect to broker: %v (trading will be disabled)", err)
            } else {
                log.Printf("Successfully connected to broker: %s", config.Trading.Broker.Broker)
            }
        } else {
            log.Println("Broker credentials not configured, trading disabled")
        }

        // 4. 创建风险管理器
        riskConfig := trading.RiskConfig{
            InitialCapital:    config.Trading.Risk.InitialCapital,
            MaxSinglePosition: config.Trading.Risk.MaxSinglePosition,
            MaxPositions:      config.Trading.Risk.MaxPositions,
            MaxDailyLoss:      config.Trading.Risk.MaxDailyLoss,
            MinOrderAmount:    config.Trading.Risk.MinOrderAmount,
            StopLossPercent:   config.Trading.Risk.StopLossPercent,
        }
        riskManager := trading.NewRiskManager(riskConfig, brokerConnector, tradeHistory)

        // 5. 创建持仓管理器
        positionManager := trading.NewPositionManager(brokerConnector)

        // 6. 创建订单执行器
        orderExecutor := trading.NewOrderExecutor(brokerConnector, riskManager, positionManager, tradeHistory)

        // 7. 创建信号处理器
        signalHandler := trading.NewSignalHandler(
            config.Trading.AutoTrade.AIThreshold,
            config.Trading.AutoTrade.MLConfidence,
            riskManager,
            positionManager,
            orderExecutor,
        )

        // 8. 设置HTTP处理器
        qhttp.SetTradingComponents(tradeHistory, brokerConnector, riskManager, positionManager, orderExecutor, signalHandler)

        log.Println("Trading system initialized")

        // 9. 启动自动交易（如果启用）
        if config.Trading.AutoTrade.Enabled && brokerConnector.IsConnected() {
            log.Println("Auto trading enabled, starting monitor...")
            go startRiskMonitor(riskManager, orderExecutor)
        }
    }
}

func startRiskMonitor(riskManager *trading.RiskManager, orderExecutor *trading.OrderExecutor) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        // 检查止损
        ctx, cancel := contextWithTimeout(30 * time.Second)
        stopLossSymbols, err := riskManager.CheckPositionLoss(ctx)
        cancel()

        if err == nil && len(stopLossSymbols) > 0 {
            log.Printf("Stop loss triggered for %d symbols", len(stopLossSymbols))
            for _, symbol := range stopLossSymbols {
                ctx, cancel := contextWithTimeout(30 * time.Second)
                _ = orderExecutor.SyncTrades(ctx)
                cancel()
            }
        }
    }
}

func contextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), timeout)
}

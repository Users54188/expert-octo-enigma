package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloudquant/backtest"
	"cloudquant/db"
	qhttp "cloudquant/http"
	"cloudquant/llm"
	"cloudquant/market"
	"cloudquant/ml"
	"cloudquant/monitoring"
	"cloudquant/trading"
	"cloudquant/trading/portfolio"
	"cloudquant/trading/risk"
	"cloudquant/trading/scheduler"
	"cloudquant/trading/strategies"
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
		ModelType     string `yaml:"model_type"`
		ModelPath     string `yaml:"model_path"`
		MaxTreeDepth  int    `yaml:"max_tree_depth"`
		TrainInterval string `yaml:"train_interval"`
		Features      struct {
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
		Strategies []StrategyConfig `yaml:"strategies"`
		Scheduler  struct {
			Enabled        bool   `yaml:"enabled"`
			Interval       string `yaml:"interval"`
			CronExpression string `yaml:"cron_expression"`
		} `yaml:"scheduler"`
		PortfolioRisk struct {
			MaxIndustryExposure float64 `yaml:"max_industry_exposure"`
			MaxSectorExposure   float64 `yaml:"max_sector_exposure"`
			MaxSymbolExposure   float64 `yaml:"max_symbol_exposure"`
			ConcentrationAlert  float64 `yaml:"concentration_alert"`
		} `yaml:"portfolio_risk"`
		VolatilityRisk struct {
			MaxVolatility       float64 `yaml:"max_volatility"`
			VolatilityThreshold float64 `yaml:"volatility_threshold"`
			LookbackPeriod      int     `yaml:"lookback_period"`
			AdjustmentFactor    float64 `yaml:"adjustment_factor"`
		} `yaml:"volatility_risk"`
		CooldownRisk struct {
			MinTradeInterval  time.Duration `yaml:"min_trade_interval"`
			MaxDailyTrades    int           `yaml:"max_daily_trades"`
			MinOrderInterval  time.Duration `yaml:"min_order_interval"`
			MaxWeeklyTrades   int           `yaml:"max_weekly_trades"`
			BlacklistDuration time.Duration `yaml:"blacklist_duration"`
			EnableCooldown    bool          `yaml:"enable_cooldown"`
		} `yaml:"cooldown_risk"`
		AIRisk struct {
			Enabled           bool          `yaml:"enabled"`
			AnalysisInterval  time.Duration `yaml:"analysis_interval"`
			CacheExpiry       time.Duration `yaml:"cache_expiry"`
			RiskThreshold     float64       `yaml:"risk_threshold"`
			AutoAlert         bool          `yaml:"auto_alert"`
			DeepLearning      bool          `yaml:"deep_learning"`
			SentimentAnalysis bool          `yaml:"sentiment_analysis"`
			NewsAnalysis      bool          `yaml:"news_analysis"`
		} `yaml:"ai_risk"`
		Portfolio struct {
			RebalanceFrequency time.Duration `yaml:"rebalance_frequency"`
			MaxTurnover        float64       `yaml:"max_turnover"`
			MinPositionWeight  float64       `yaml:"min_position_weight"`
			MaxPositionWeight  float64       `yaml:"max_position_weight"`
			TargetReturn       float64       `yaml:"target_return"`
			RiskFreeRate       float64       `yaml:"risk_free_rate"`
		} `yaml:"portfolio"`
		Optimizer struct {
			Method          string  `yaml:"method"`
			RiskFreeRate    float64 `yaml:"risk_free_rate"`
			LookbackPeriod  int     `yaml:"lookback_period"`
			MinWeight       float64 `yaml:"min_weight"`
			MaxWeight       float64 `yaml:"max_weight"`
			RebalancePeriod int     `yaml:"rebalance_period"`
		} `yaml:"optimizer"`
	} `yaml:"trading"`
	Monitoring struct {
		WebSocket struct {
			Enabled        bool `yaml:"enabled"`
			Port           int  `yaml:"port"`
			MaxConnections int  `yaml:"max_connections"`
		} `yaml:"websocket"`
		Alerts struct {
			Enabled  bool `yaml:"enabled"`
			Channels struct {
				Email struct {
					Enabled  bool   `yaml:"enabled"`
					SMTPHost string `yaml:"smtp_host"`
					SMTPPort int    `yaml:"smtp_port"`
					Username string `yaml:"username"`
					Password string `yaml:"password"`
					To       string `yaml:"to"`
				} `yaml:"email"`
				Feishu struct {
					Enabled   bool   `yaml:"enabled"`
					Webhook   string `yaml:"webhook"`
					RateLimit struct {
						MaxPerHour int           `yaml:"max_per_hour"`
						MaxPerDay  int           `yaml:"max_per_day"`
						Cooldown   time.Duration `yaml:"cooldown"`
					} `yaml:"rate_limit"`
				} `yaml:"feishu"`
				Dingding struct {
					Enabled   bool   `yaml:"enabled"`
					Webhook   string `yaml:"webhook"`
					RateLimit struct {
						MaxPerHour int           `yaml:"max_per_hour"`
						MaxPerDay  int           `yaml:"max_per_day"`
						Cooldown   time.Duration `yaml:"cooldown"`
					} `yaml:"rate_limit"`
				} `yaml:"dingding"`
			} `yaml:"channels"`
		} `yaml:"alerts"`
	} `yaml:"monitoring"`
	Backtest struct {
		Enabled       bool `yaml:"enabled"`
		DefaultConfig struct {
			StartDate        time.Time `yaml:"start_date"`
			EndDate          time.Time `yaml:"end_date"`
			InitialCapital   float64   `yaml:"initial_capital"`
			Commission       float64   `yaml:"commission"`
			Slippage         float64   `yaml:"slippage"`
			RiskFreeRate     float64   `yaml:"risk_free_rate"`
			MaxDrawdownLimit float64   `yaml:"max_drawdown_limit"`
			Realtime         bool      `yaml:"realtime"`
		} `yaml:"default_config"`
		ParameterSearch struct {
			Method        string `yaml:"method"`
			Metric        string `yaml:"metric"`
			MaxIterations int    `yaml:"max_iterations"`
			MinSamples    int    `yaml:"min_samples"`
			Parallel      bool   `yaml:"parallel"`
			MaxWorkers    int    `yaml:"max_workers"`
			EarlyStopping bool   `yaml:"early_stopping"`
			Patience      int    `yaml:"patience"`
		} `yaml:"parameter_search"`
	} `yaml:"backtest"`
}

// StrategyConfig 策略配置
type StrategyConfig struct {
	Name       string                 `yaml:"name"`
	Type       string                 `yaml:"type"`
	Enabled    bool                   `yaml:"enabled"`
	Weight     float64                `yaml:"weight"`
	Priority   int                    `yaml:"priority"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// 全局组件变量
var (
	strategyLoader   *strategies.StrategyLoader
	strategyManager  *strategies.StrategyManager
	scheduler        *scheduler.Scheduler
	monitor          *monitoring.RealtimeMonitor
	alertSystem      *monitoring.AlertSystem
	portfolioManager *portfolio.PortfolioManager
	optimizer        *portfolio.PortfolioOptimizer
	backtestEngine   *backtest.BacktestEngine
	parameterSearch  *backtest.ParameterSearch
	llmAnalyzer      *llm.DeepSeekAnalyzer

	// 传统交易组件
	tradeHistory    *trading.TradeHistory
	brokerConnector *trading.BrokerConnector
	riskManager     *trading.RiskManager
	positionManager *trading.PositionManager
	orderExecutor   *trading.OrderExecutor
	signalHandler   *trading.SignalHandler

	// 风险管理组件
	portfolioRisk  *risk.PortfolioRisk
	volatilityRisk *risk.VolatilityRisk
	cooldownRisk   *risk.CooldownRisk
	aiRisk         *risk.AIRisk

	// 市场数据提供者（使用market包的现有功能）
	marketProvider = &market.MarketProvider{}
)

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

	// 1. 初始化基础服务
	llmAnalyzer = llm.NewDeepSeekAnalyzer(config.LLM.APIKey, config.LLM.Model, config.LLM.Timeout, config.LLM.MaxTokens)
	qhttp.SetAnalyzer(llmAnalyzer)

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

	// 2. 初始化多策略框架
	initializeMultiStrategySystem(config)

	// 3. 初始化监控系统
	initializeMonitoringSystem(config)

	// 4. 初始化传统交易系统（保持向后兼容）
	initializeLegacyTradingSystem(config)

	// 5. 初始化组合管理系统
	initializePortfolioSystem(config)

	// 6. 初始化回测系统
	initializeBacktestSystem(config)
}

func firstSymbol(symbols []string) string {
	if len(symbols) == 0 {
		return ""
	}
	return symbols[0]
}

// initializeMultiStrategySystem 初始化多策略框架
func initializeMultiStrategySystem(config *Config) {
	log.Println("Initializing multi-strategy system...")

	// 1. 创建策略加载器
	strategyLoader = strategies.NewStrategyLoader()

	// 2. 转换配置格式
	var strategyConfigs []strategies.StrategyConfig
	for _, config := range config.Trading.Strategies {
		strategyConfigs = append(strategyConfigs, strategies.StrategyConfig{
			Name:       config.Name,
			Type:       strategies.StrategyType(config.Type),
			Enabled:    config.Enabled,
			Weight:     config.Weight,
			Parameters: config.Parameters,
			Priority:   config.Priority,
		})
	}

	// 3. 加载策略
	if err := strategyLoader.LoadStrategies(strategyConfigs); err != nil {
		log.Printf("Failed to load strategies: %v", err)
		return
	}

	// 4. 创建策略管理器
	strategyManager = strategies.NewStrategyManager(strategyLoader, strategies.WeightedCombination)

	// 5. 创建调度器
	if scheduler, err := scheduler.NewScheduler(config.Trading.Scheduler.Interval); err != nil {
		log.Printf("Failed to create scheduler: %v", err)
	} else {
		scheduler.SetStrategyManager(strategyManager)
		scheduler.SetSymbols(config.Symbols)

		// 如果启用调度器，启动它
		if config.Trading.Scheduler.Enabled {
			if err := scheduler.Start(); err != nil {
				log.Printf("Failed to start scheduler: %v", err)
			} else {
				log.Println("Strategy scheduler started")
			}
		}
	}

	log.Println("Multi-strategy system initialized")
}

// initializeMonitoringSystem 初始化监控系统
func initializeMonitoringSystem(config *Config) {
	log.Println("Initializing monitoring system...")

	// 1. 创建实时监控器
	monitor = monitoring.NewRealtimeMonitor()
	if err := monitor.Start(); err != nil {
		log.Printf("Failed to start monitor: %v", err)
		return
	}

	// 2. 创建告警系统
	alertSystem = monitoring.NewAlertSystem()
	if err := alertSystem.Start(); err != nil {
		log.Printf("Failed to start alert system: %v", err)
	}

	// 3. 配置告警渠道
	configureAlertChannels(config)

	// 4. 设置告警系统到监控器
	monitor.SetAlertSystem(alertSystem)

	log.Println("Monitoring system initialized")
}

// configureAlertChannels 配置告警渠道
func configureAlertChannels(config *Config) {
	if !config.Monitoring.Alerts.Enabled {
		return
	}

	// 配置飞书
	if config.Monitoring.Alerts.Channels.Feishu.Enabled {
		channel := &monitoring.AlertChannel{
			Type:    "feishu",
			Enabled: true,
			Settings: map[string]interface{}{
				"webhook": config.Monitoring.Alerts.Channels.Feishu.Webhook,
			},
			Filters: []monitoring.AlertFilter{
				{Field: "level", Operator: "equals", Value: "error"},
				{Field: "level", Operator: "equals", Value: "critical"},
			},
			RateLimit: monitoring.RateLimit{
				MaxPerHour: config.Monitoring.Alerts.Channels.Feishu.RateLimit.MaxPerHour,
				MaxPerDay:  config.Monitoring.Alerts.Channels.Feishu.RateLimit.MaxPerDay,
				Cooldown:   config.Monitoring.Alerts.Channels.Feishu.RateLimit.Cooldown,
			},
		}

		if err := alertSystem.AddChannel("feishu", channel); err != nil {
			log.Printf("Failed to add feishu channel: %v", err)
		}
	}

	// 配置钉钉
	if config.Monitoring.Alerts.Channels.Dingding.Enabled {
		channel := &monitoring.AlertChannel{
			Type:    "dingding",
			Enabled: true,
			Settings: map[string]interface{}{
				"webhook": config.Monitoring.Alerts.Channels.Dingding.Webhook,
			},
			Filters: []monitoring.AlertFilter{
				{Field: "level", Operator: "equals", Value: "warning"},
				{Field: "level", Operator: "equals", Value: "error"},
				{Field: "level", Operator: "equals", Value: "critical"},
			},
			RateLimit: monitoring.RateLimit{
				MaxPerHour: config.Monitoring.Alerts.Channels.Dingding.RateLimit.MaxPerHour,
				MaxPerDay:  config.Monitoring.Alerts.Channels.Dingding.RateLimit.MaxPerDay,
				Cooldown:   config.Monitoring.Alerts.Channels.Dingding.RateLimit.Cooldown,
			},
		}

		if err := alertSystem.AddChannel("dingding", channel); err != nil {
			log.Printf("Failed to add dingding channel: %v", err)
		}
	}
}

// initializePortfolioSystem 初始化组合管理系统
func initializePortfolioSystem(config *Config) {
	log.Println("Initializing portfolio management system...")

	if positionManager == nil || riskManager == nil {
		log.Println("Trading components not configured, skipping position-based portfolio managers")
	} else {
		// 2. 创建组合风险管理器
		portfolioRiskConfig := risk.PortfolioRiskConfig{
			MaxIndustryExposure: config.Trading.PortfolioRisk.MaxIndustryExposure,
			MaxSectorExposure:   config.Trading.PortfolioRisk.MaxSectorExposure,
			MaxSymbolExposure:   config.Trading.PortfolioRisk.MaxSymbolExposure,
			ConcentrationAlert:  config.Trading.PortfolioRisk.ConcentrationAlert,
		}
		portfolioRisk = risk.NewPortfolioRisk(portfolioRiskConfig, positionManager)

		// 3. 创建波动率风险管理器
		volatilityRiskConfig := risk.VolatilityRiskConfig{
			MaxVolatility:       config.Trading.VolatilityRisk.MaxVolatility,
			VolatilityThreshold: config.Trading.VolatilityRisk.VolatilityThreshold,
			LookbackPeriod:      config.Trading.VolatilityRisk.LookbackPeriod,
			AdjustmentFactor:    config.Trading.VolatilityRisk.AdjustmentFactor,
		}
		volatilityRisk = risk.NewVolatilityRisk(volatilityRiskConfig, positionManager)

		// 5. 创建AI风险管理器
		aiRiskConfig := risk.AIRiskConfig{
			Enabled:           config.Trading.AIRisk.Enabled,
			AnalysisInterval:  config.Trading.AIRisk.AnalysisInterval,
			CacheExpiry:       config.Trading.AIRisk.CacheExpiry,
			RiskThreshold:     config.Trading.AIRisk.RiskThreshold,
			AutoAlert:         config.Trading.AIRisk.AutoAlert,
			DeepLearning:      config.Trading.AIRisk.DeepLearning,
			SentimentAnalysis: config.Trading.AIRisk.SentimentAnalysis,
			NewsAnalysis:      config.Trading.AIRisk.NewsAnalysis,
		}
		aiRisk = risk.NewAIRisk(aiRiskConfig, llmAnalyzer, positionManager)

		// 6. 创建组合管理器
		portfolioConfig := portfolio.PortfolioConfig{
			RebalanceFrequency: config.Trading.Portfolio.RebalanceFrequency,
			MaxTurnover:        config.Trading.Portfolio.MaxTurnover,
			MinPositionWeight:  config.Trading.Portfolio.MinPositionWeight,
			MaxPositionWeight:  config.Trading.Portfolio.MaxPositionWeight,
			TargetReturn:       config.Trading.Portfolio.TargetReturn,
			RiskFreeRate:       config.Trading.Portfolio.RiskFreeRate,
		}
		portfolioManager = portfolio.NewPortfolioManager(portfolioConfig, positionManager, riskManager)
	}

	// 4. 创建交易冷却风险管理器
	if tradeHistory == nil {
		log.Println("Trade history not configured, cooldown risk disabled")
	} else {
		cooldownRiskConfig := risk.CooldownRiskConfig{
			MinTradeInterval:  config.Trading.CooldownRisk.MinTradeInterval,
			MaxDailyTrades:    config.Trading.CooldownRisk.MaxDailyTrades,
			MinOrderInterval:  config.Trading.CooldownRisk.MinOrderInterval,
			MaxWeeklyTrades:   config.Trading.CooldownRisk.MaxWeeklyTrades,
			BlacklistDuration: config.Trading.CooldownRisk.BlacklistDuration,
			EnableCooldown:    config.Trading.CooldownRisk.EnableCooldown,
		}
		cooldownRisk = risk.NewCooldownRisk(cooldownRiskConfig, tradeHistory)
	}

	// 7. 创建组合优化器
	optimizerConfig := portfolio.OptimizerConfig{
		Method:          config.Trading.Optimizer.Method,
		RiskFreeRate:    config.Trading.Optimizer.RiskFreeRate,
		LookbackPeriod:  config.Trading.Optimizer.LookbackPeriod,
		MinWeight:       config.Trading.Optimizer.MinWeight,
		MaxWeight:       config.Trading.Optimizer.MaxWeight,
		RebalancePeriod: config.Trading.Optimizer.RebalancePeriod,
	}
	optimizer = portfolio.NewPortfolioOptimizer(optimizerConfig)

	log.Println("Portfolio management system initialized")
}

// initializeBacktestSystem 初始化回测系统
func initializeBacktestSystem(config *Config) {
	if !config.Backtest.Enabled {
		log.Println("Backtest system disabled")
		return
	}

	log.Println("Initializing backtest system...")

	// 1. 创建回测引擎
	backtestConfig := backtest.BacktestConfig{
		StartDate:        config.Backtest.DefaultConfig.StartDate,
		EndDate:          config.Backtest.DefaultConfig.EndDate,
		InitialCapital:   config.Backtest.DefaultConfig.InitialCapital,
		Commission:       config.Backtest.DefaultConfig.Commission,
		Slippage:         config.Backtest.DefaultConfig.Slippage,
		Symbols:          config.Symbols,
		RiskFreeRate:     config.Backtest.DefaultConfig.RiskFreeRate,
		MaxDrawdownLimit: config.Backtest.DefaultConfig.MaxDrawdownLimit,
		Realtime:         config.Backtest.DefaultConfig.Realtime,
	}

	// 转换策略配置
	for _, strategyConfig := range config.Trading.Strategies {
		backtestConfig.Strategies = append(backtestConfig.Strategies, backtest.StrategyConfig{
			Name:       strategyConfig.Name,
			Type:       strategies.StrategyType(strategyConfig.Type),
			Enabled:    strategyConfig.Enabled,
			Weight:     strategyConfig.Weight,
			Parameters: strategyConfig.Parameters,
		})
	}

	backtestEngine = backtest.NewBacktestEngine(backtestConfig)

	// 2. 创建参数搜索器
	searchConfig := backtest.SearchConfig{
		Method:        config.Backtest.ParameterSearch.Method,
		Metric:        config.Backtest.ParameterSearch.Metric,
		MaxIterations: config.Backtest.ParameterSearch.MaxIterations,
		MinSamples:    config.Backtest.ParameterSearch.MinSamples,
		Parallel:      config.Backtest.ParameterSearch.Parallel,
		MaxWorkers:    config.Backtest.ParameterSearch.MaxWorkers,
		EarlyStopping: config.Backtest.ParameterSearch.EarlyStopping,
		Patience:      config.Backtest.ParameterSearch.Patience,
		Parameters:    make(map[string][]backtest.ParameterConfig),
	}

	parameterSearch = backtest.NewParameterSearch(searchConfig, backtestEngine)

	log.Println("Backtest system initialized")
}

// initializeLegacyTradingSystem 初始化传统交易系统（保持向后兼容）
func initializeLegacyTradingSystem(config *Config) {
	// 如果配置了券商，则初始化交易系统
	if config.Trading.Broker.Type != "" && config.Trading.Broker.Service != "" {
		log.Println("Initializing legacy trading system...")
		var err error

		// 1. 创建交易历史记录器
		tradeHistory, err = trading.NewTradeHistory(config.Database.Path)
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

		brokerConnector, err = trading.NewBrokerConnector(brokerConfig)
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
		riskManager = trading.NewRiskManager(riskConfig, brokerConnector, tradeHistory)

		// 5. 创建持仓管理器
		positionManager = trading.NewPositionManager(brokerConnector)

		// 6. 创建订单执行器
		orderExecutor = trading.NewOrderExecutor(brokerConnector, riskManager, positionManager, tradeHistory)

		// 7. 创建信号处理器
		signalHandler = trading.NewSignalHandler(
			config.Trading.AutoTrade.AIThreshold,
			config.Trading.AutoTrade.MLConfidence,
			riskManager,
			positionManager,
			orderExecutor,
		)

		// 8. 设置HTTP处理器
		qhttp.SetTradingComponents(tradeHistory, brokerConnector, riskManager, positionManager, orderExecutor, signalHandler)

		// 9. 连接多策略系统到传统交易系统
		if strategyManager != nil {
			strategyManager.SetTradingComponents(riskManager, positionManager, orderExecutor, signalHandler)
		}

		log.Println("Legacy trading system initialized")

		// 10. 启动自动交易（如果启用）
		if config.Trading.AutoTrade.Enabled && brokerConnector.IsConnected() {
			log.Println("Auto trading enabled, starting monitor...")
			go startRiskMonitor(riskManager, orderExecutor)
		}
	}
}

// 现有的函数保持不变
func initializeTradingSystem(config *Config) {
	// 这个函数现在由 initializeLegacyTradingSystem 替代
	initializeLegacyTradingSystem(config)
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

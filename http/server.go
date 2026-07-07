// Package http 提供HTTP服务器功能
package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"cloudquant/monitoring"
)

// Server HTTP服务器
type Server struct {
	server *http.Server
	config ServerConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port           int
	Timeout        time.Duration
	MaxConnections int
	AllowedOrigins []string
	APIKey         string
}

// DefaultServerConfig 默认服务器配置
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Port:           8080,
		Timeout:        30 * time.Second,
		MaxConnections: 100,
		AllowedOrigins: []string{"*"},
	}
}

// NewServer 创建HTTP服务器
func NewServer(config ServerConfig) *Server {
	mux := http.NewServeMux()

	// 注册所有处理器
	RegisterHandlers(mux)
	RegisterDashboardRoutes(mux)
	RegisterAPIHandlers(mux)

	// 注册WebSocket端点
	hub := monitoring.NewWebSocketHub()
	go hub.Start()
	mux.HandleFunc("/api/ws/dashboard", hub.HandleWebSocket)

	// 创建中间件链（基础链，不含认证）
	chain := Chain(
		RecoveryMiddleware,                    // 1. 恢复中间件（最先执行，捕获panic）
		LoggerMiddleware,                      // 2. 日志中间件
		SecurityHeadersMiddleware,             // 3. 安全头中间件
		CORSMiddleware(config.AllowedOrigins), // 4. CORS中间件
		TimeoutMiddleware(config.Timeout),     // 5. 超时中间件
		RequestSizeMiddleware(10 << 20),       // 6. 10MB请求大小限制
	)

	// 包装处理器
	handler := chain(mux)

	// 如果配置了API Key，对交易路由和Dashboard路由添加认证中间件
	if config.APIKey != "" {
		authChain := Chain(
			RecoveryMiddleware,
			LoggerMiddleware,
			SecurityHeadersMiddleware,
			CORSMiddleware(config.AllowedOrigins),
			TimeoutMiddleware(config.Timeout),
			RequestSizeMiddleware(10 << 20),
			AuthMiddleware(func(token string) bool {
				return token == config.APIKey
			}),
		)
		tradingMux := http.NewServeMux()
		RegisterTradingHandlers(tradingMux)
		// 将交易路由挂载到带认证的处理器下
		mux.Handle("POST /api/trading/", authChain(tradingMux))
		mux.Handle("GET /api/trading/", authChain(tradingMux))

		// 将Dashboard路由挂载到带认证的处理器下
		dashboardMux := http.NewServeMux()
		RegisterDashboardRoutes(dashboardMux)
		mux.Handle("/api/dashboard/", authChain(dashboardMux))
		mux.Handle("/api/performance/", authChain(dashboardMux))
	} else {
		// 未配置API Key时，路由不加认证（仅限开发环境）
		RegisterTradingHandlers(mux)
		RegisterDashboardRoutes(mux)
	}

	// 对分析API添加速率限制（每秒10个请求）
	rateLimitedChain := Chain(
		RecoveryMiddleware,
		LoggerMiddleware,
		SecurityHeadersMiddleware,
		CORSMiddleware(config.AllowedOrigins),
		TimeoutMiddleware(config.Timeout),
		RequestSizeMiddleware(10 << 20),
		RateLimitMiddleware(10),
	)
	analysisMux := http.NewServeMux()
	analysisMux.HandleFunc("GET /api/analysis/{symbol}", handleAnalysis)
	analysisMux.HandleFunc("GET /api/analysis/batch", handleBatchAnalysis)
	analysisMux.HandleFunc("POST /api/train", handleTrain)
	mux.Handle("/api/analysis/", rateLimitedChain(analysisMux))
	mux.Handle("/api/train", rateLimitedChain(analysisMux))

	return &Server{
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", config.Port),
			Handler:           handler,
			ReadTimeout:       config.Timeout,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      config.Timeout,
			IdleTimeout:       120 * time.Second,
		},
		config: config,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/api/ws/dashboard", s.server.Addr)

	// #nosec G114 -- HTTP server for local dashboard is intentional
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Shutting down HTTP server...")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	return nil
}

// Addr 返回服务器地址
func (s *Server) Addr() string {
	return s.server.Addr
}

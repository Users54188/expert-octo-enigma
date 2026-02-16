// Package http 提供HTTP服务器功能
package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
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
	RegisterTradingHandlers(mux)
	RegisterDashboardRoutes(mux)
	RegisterAPIHandlers(mux)

	// 创建中间件链
	chain := Chain(
		RecoveryMiddleware,                    // 1. 恢复中间件（最先执行，捕获panic）
		LoggerMiddleware,                      // 2. 日志中间件
		SecurityHeadersMiddleware,             // 3. 安全头中间件
		CORSMiddleware(config.AllowedOrigins), // 4. CORS中间件
		TimeoutMiddleware(config.Timeout),     // 5. 超时中间件
		GzipMiddleware,                        // 6. Gzip压缩中间件
	)

	// 包装处理器
	handler := chain(mux)

	return &Server{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.Port),
			Handler:      handler,
			ReadTimeout:  config.Timeout,
			WriteTimeout: config.Timeout,
			IdleTimeout:  120 * time.Second,
		},
		config: config,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/api/ws/dashboard", s.server.Addr)

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

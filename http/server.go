package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	server *http.Server
}

func NewServer(port int) *Server {
	mux := http.NewServeMux()
	RegisterHandlers(mux)
	RegisterTradingHandlers(mux)
	RegisterDashboardRoutes(mux)

	return &Server{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
	}
}

func (s *Server) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

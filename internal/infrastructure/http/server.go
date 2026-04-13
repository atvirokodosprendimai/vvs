package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Server struct {
	srv *http.Server
}

func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0, // SSE needs no write timeout
			IdleTimeout:  120 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	log.Printf("Server starting on %s", s.srv.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

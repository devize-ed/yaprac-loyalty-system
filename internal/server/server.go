package server

import (
	"context"
	"errors"
	"fmt"
	"loyaltySys/internal/config"
	"loyaltySys/internal/handlers"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Server is a struct that contains the HTTP server and the configuration.
type Server struct {
	*http.Server
	cfg    *config.Config
	logger *zap.SugaredLogger
}

// NewServer creates a new server with the given configuration, handler, and logger.
func NewServer(cfg *config.Config, h *handlers.Handler, logger *zap.SugaredLogger) *Server {
	return &Server{
		Server: &http.Server{
			Addr:    cfg.Host,
			Handler: h.NewRouter(),
		},
		cfg:    cfg,
		logger: logger,
	}
}

// Start starts the server and listens for incoming requests.
func (s *Server) Start(ctx context.Context) error {
	// Start the HTTP server in a goroutine.
	go func() {
		// Setart the server and listen for incoming requests.
		s.logger.Infof("HTTP server listening on %s", s.cfg.Host)
		if err := s.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			s.logger.Errorf("listen error: %w", err)
		} else {
			s.logger.Debug("HTTP server closed")
		}
	}()

	// Wait for the context to be done.
	<-ctx.Done()
	s.logger.Infof("stopping signal received, shutting down server...")

	// create a context with a timeout.
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server.
	if err := s.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("error shutting down the server: %w", err)
	}
	return nil
}

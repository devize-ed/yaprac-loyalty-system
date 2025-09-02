package server

import (
	"context"
	"loyaltySys/internal/config"
	"loyaltySys/internal/handlers"
	cfg "loyaltySys/internal/service/server/config"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_Start(t *testing.T) {
	cfg := &config.Config{ServerConfig: cfg.ServerConfig{Host: "127.0.0.1:0"}}
	h := &handlers.Handler{}
	logger := zap.NewNop().Sugar()

	s := NewServer(cfg, h, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(500*time.Millisecond, cancel)

	start := time.Now()
	err := s.Start(ctx)
	assert.NoError(t, err, "Start should return cleanly after context cancellation")

	assert.GreaterOrEqual(t, time.Since(start), 250*time.Millisecond)
}

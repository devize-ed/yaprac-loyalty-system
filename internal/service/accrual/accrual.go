package accrual

import (
	"context"
	"fmt"
	"loyaltySys/internal/db"
	"loyaltySys/internal/models"
	"loyaltySys/internal/service/accrual/config"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

// Storage interface for the accrual service
type Storage interface {
	GetUnprocessedOrders(ctx context.Context) ([]*models.Order, error)
	UpdateOrder(ctx context.Context, order *models.Order) error
}

// NewStorage creates a new storage
func NewStorage(ctx context.Context, dsn string, logger *zap.SugaredLogger) Storage {
	db, err := db.NewDB(ctx, dsn, logger)
	if err != nil {
		logger.Fatal("failed to create storage", err)
		return nil
	}
	return db
}

// AccrualService is the accrual service
type AccrualService struct {
	req     *resty.Request
	cfg     config.AccrualConfig
	storage Storage
	logger  *zap.SugaredLogger
}

// NewAccrualService creates a new accrual service
func NewAccrualService(accrualURL string, storage Storage, cfg config.AccrualConfig, logger *zap.SugaredLogger) *AccrualService {
	return &AccrualService{
		req:     newRequest(accrualURL),
		cfg:     cfg,
		storage: storage,
		logger:  logger,
	}
}

func newRequest(accrualURL string) *resty.Request {
	client := resty.New()
	client.SetBaseURL(accrualURL)
	req := client.R()
	return req
}

// Start starts the accrual service
func (s *AccrualService) Start(ctx context.Context) error {
	// Start the accrual service in a goroutine.
	t := time.NewTicker(s.cfg.Timeout + 120*time.Millisecond)
	go func() {
		for {
			select {
			case <-ctx.Done():
				t.Stop()
				s.logger.Info("accrual service stopped")
				return
			case <-t.C:
				s.logger.Info("process orders")
				err := s.processOrders(ctx)
				if err != nil {
					s.logger.Errorf("failed to process orders: %v", err)
					continue
				}
			}
		}
	}()
	return nil
}

func (s *AccrualService) processOrders(ctx context.Context) error {
	orders, err := s.storage.GetUnprocessedOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed orders: %w", err)
	}
	s.createRequesters(ctx, orders)
	return nil
}

func (s *AccrualService) createRequesters(ctx context.Context, orders []*models.Order) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	for _, order := range orders {
		go func(order *models.Order) {
			select {
			case <-ctxTimeout.Done():
				s.logger.Errorf("Failed to get accrual for order, timeout %s: %w", order.Number, ctxTimeout.Err())
				return
			default:
				s.getAccrual(ctxTimeout, order)
			}
		}(order)
	}
	return nil
}

func (s *AccrualService) getAccrual(ctx context.Context, order *models.Order) error {
	req := newRequest(s.cfg.AccrualAddr)
	req.SetPathParam("order_number", order.Number).SetContext(ctx)
	resp, err := req.Get("/api/orders/{order_number}")
	if err != nil {
		return fmt.Errorf("failed to get accrual for order: %w", err)
	}
	if resp.IsError() {
		return fmt.Errorf("failed to get accrual for order: %s", resp.String())
	}
	return nil
}

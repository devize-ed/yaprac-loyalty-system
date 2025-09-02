package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"loyaltySys/internal/db"
	"loyaltySys/internal/models"
	"loyaltySys/internal/service/accrual/config"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
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
	req       *resty.Request
	cfg       config.AccrualConfig
	storage   Storage
	logger    *zap.SugaredLogger
	sendAfter atomic.Uint32
	wg        sync.WaitGroup
	errCh     chan error
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
func (s *AccrualService) Start(ctx context.Context) {
	// Start the accrual service in a goroutine.
	t := time.NewTicker(time.Second*time.Duration(s.cfg.Timeout) + 120*time.Millisecond)
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
					s.logger.Errorf("failed to process orders: %w", err)
					continue
				}
			}
		}
	}()
	s.logger.Info("accrual service started")
}

func (s *AccrualService) processOrders(ctx context.Context) error {
	s.errCh = make(chan error, 100)
	orders, err := s.storage.GetUnprocessedOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed orders: %w", err)
	}

	s.createRequesters(ctx, orders)
	s.wg.Wait()
	close(s.errCh)
	for err := range s.errCh {
		return fmt.Errorf("failed to get accrual for order: %w", err)
	}

	return nil
}

func (s *AccrualService) createRequesters(ctx context.Context, orders []*models.Order) {
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*time.Duration(s.cfg.Timeout))
	defer cancel()
	var wait bool
	if a := s.sendAfter.Load(); a > 0 {
		s.logger.Infof("waiting for %d seconds", a)
		s.sendAfter.Store(0)
		time.Sleep(time.Second * time.Duration(a))
		wait = true
	}
	for i, order := range orders {
		s.wg.Add(1)
		go func(orderNum string) {
			defer s.wg.Done()

			select {
			case <-ctxTimeout.Done():
				s.errCh <- fmt.Errorf("failed to get accrual for order, timeout %s: %w", orderNum, ctxTimeout.Err())
				return
			default:
				if wait {
					time.Sleep(time.Millisecond * time.Duration(i))
					wait = false
				}
				err := s.getAccrual(ctxTimeout, orderNum)
				if err != nil {
					s.errCh <- fmt.Errorf("failed to get accrual for order: %w", err)
				}
			}
		}(order.Number)
	}
}

func (s *AccrualService) getAccrual(ctx context.Context, orderNum string) error {
	req := newRequest(s.cfg.AccrualAddr)
	req.SetPathParam("order_number", orderNum).SetContext(ctx)
	resp, err := req.Get("/api/orders/{order_number}")
	if err != nil {
		return fmt.Errorf("failed to get accrual for order: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusTooManyRequests:
		s.logger.Errorf("Too many requests: %s", resp.String())
		retryAfter, err := strconv.Atoi(resp.Header().Get("Retry-After"))
		if err != nil {
			return fmt.Errorf("failed to convert retry-after to int: %w", err)
		}
		s.sendAfter.Store(uint32(retryAfter))
		return fmt.Errorf("failed to get accrual for order: %s", resp.String())
	case http.StatusNoContent:
		return fmt.Errorf("the order is not registered in the accrual system: %s", resp.String())
	case http.StatusInternalServerError:
		return fmt.Errorf("failed to get accrual for order: %s", resp.String())
	}

	gotOrder := &models.Order{}
	err = json.Unmarshal(resp.Body(), gotOrder)
	if err != nil {
		return fmt.Errorf("failed to unmarshal accrual for order: %w", err)
	}

	if gotOrder.Status == models.StatusProcessed || gotOrder.Status == models.StatusInvalid {
		err := s.storage.UpdateOrder(ctx, gotOrder)
		if err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}
	}
	return nil
}

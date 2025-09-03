package accrual

import (
	"context"
	"encoding/json"
	"errors"
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
	client  *resty.Client
	cfg     config.AccrualConfig
	storage Storage

	logger *zap.SugaredLogger

	sendAfter atomic.Uint32
	wg        sync.WaitGroup
	errCh     chan error
}

type accrualResp struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// NewAccrualService creates a new accrual service
func NewAccrualService(accrualURL string, storage Storage, cfg config.AccrualConfig, logger *zap.SugaredLogger) *AccrualService {
	client := resty.New().
		SetBaseURL(accrualURL).
		SetTimeout(time.Duration(cfg.Timeout) * time.Second)

	return &AccrualService{
		client:  client,
		cfg:     cfg,
		storage: storage,
		logger:  logger,
	}
}

// Start starts the accrual service
func (s *AccrualService) Start(ctx context.Context) {
	t := time.NewTicker(time.Second*time.Duration(s.cfg.Timeout) + 120*time.Millisecond)
	go func() {
		defer t.Stop()
		s.logger.Info("accrual service started")
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("accrual service stopped")
				return
			case <-t.C:
				if err := s.processOrders(ctx); err != nil {
					s.logger.Errorf("failed to process orders: %v", err)
				}
			}
		}
	}()
}

func (s *AccrualService) processOrders(ctx context.Context) error {
	orders, err := s.storage.GetUnprocessedOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed orders: %w", err)
	}
	if len(orders) == 0 {
		return nil
	}

	if a := s.sendAfter.Swap(0); a > 0 {
		s.logger.Infof("respecting Retry-After: sleeping %d seconds", a)
		time.Sleep(time.Duration(a) * time.Second)
	}

	// create error channel
	s.errCh = make(chan error, len(orders))

	// create requesters
	for _, order := range orders {
		s.wg.Add(1)
		orderNum := order.Number
		go func() {
			defer s.wg.Done()

			reqCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Timeout)*time.Second)
			defer cancel()

			if err := s.getAccrual(reqCtx, orderNum); err != nil {
				select {
				case s.errCh <- fmt.Errorf("order %s: %w", orderNum, err):
				default:
					s.logger.Warnf("error channel is full; dropping error for order %s: %v", orderNum, err)
				}
			}
		}()
	}

	s.wg.Wait()
	close(s.errCh)

	// collect errors
	var joined error
	for err := range s.errCh {
		joined = errors.Join(joined, err)
	}
	return joined
}

func (s *AccrualService) getAccrual(ctx context.Context, orderNum string) error {
	resp, err := s.client.R().
		SetContext(ctx).
		SetPathParam("order_number", orderNum).
		Get("/api/orders/{order_number}")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("request timeout: %w", err)
		}
		return fmt.Errorf("http request failed: %w", err)
	}

	switch resp.StatusCode() {
	case http.StatusTooManyRequests:
		retryAfter, convErr := strconv.Atoi(resp.Header().Get("Retry-After"))
		if convErr != nil {
			return fmt.Errorf("429 without valid Retry-After: %w", convErr)
		}
		s.sendAfter.Store(uint32(retryAfter))
		return fmt.Errorf("too many requests, retry-after=%d", retryAfter)

	case http.StatusNoContent:
		return fmt.Errorf("order not registered in accrual system")

	case http.StatusInternalServerError:
		return fmt.Errorf("accrual service 500")
	}

	r := &accrualResp{}
	if err := json.Unmarshal(resp.Body(), &r); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	gotOrder := &models.Order{
		Number: r.Order,
		Status: models.OrderStatus(r.Status),
	}
	if r.Accrual != nil {
		gotOrder.Accrual = *r.Accrual
	}

	if gotOrder.Status == models.StatusProcessed || gotOrder.Status == models.StatusInvalid {
		if err := s.storage.UpdateOrder(ctx, gotOrder); err != nil {
			return fmt.Errorf("update order: %w", err)
		}
	}
	return nil
}

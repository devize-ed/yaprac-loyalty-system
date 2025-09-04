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

// accrualResp is the structure to store the response from the accrual system
type accrualResp struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// NewAccrualService creates a new accrual service
func NewAccrualService(accrualURL string, storage Storage, cfg config.AccrualConfig, logger *zap.SugaredLogger) *AccrualService {
	// create a new client
	client := resty.New().
		SetBaseURL(accrualURL).
		SetTimeout(time.Duration(cfg.Timeout) * time.Second)

	// create a new accrual service
	return &AccrualService{
		client:  client,
		cfg:     cfg,
		storage: storage,
		logger:  logger,
	}
}

// Start starts the accrual service
func (s *AccrualService) Start(ctx context.Context) {
	// create a new ticker
	t := time.NewTicker(time.Second*time.Duration(s.cfg.Timeout) + 120*time.Millisecond)
	// create a new goroutine to process the orders
	go func() {
		defer t.Stop()
		s.logger.Info("accrual service started")
		// process the orders
		for {
			select {
			// stop the accrual service
			case <-ctx.Done():
				s.logger.Info("accrual service stopped")
				return
			// process the orders on ticker signal
			case <-t.C:
				if err := s.processOrders(ctx); err != nil {
					s.logger.Errorf("failed to process orders: %v", err)
				}
			}
		}
	}()
}

// processOrders loads the unprocessed orders and sending requests to the accrual system
func (s *AccrualService) processOrders(ctx context.Context) error {
	// get the unprocessed orders
	orders, err := s.storage.GetUnprocessedOrders(ctx)
	if err != nil {
		return fmt.Errorf("failed to get unprocessed orders: %w", err)
	}
	// if there are no unprocessed orders, return nil
	if len(orders) == 0 {
		return nil
	}

	// if there is a Retry-After, sleep for the duration
	if a := s.sendAfter.Swap(0); a > 0 {
		s.logger.Infof("respecting Retry-After: sleeping %d seconds", a)
		time.Sleep(time.Duration(a) * time.Second)
	}

	// create error channel
	s.errCh = make(chan error, len(orders))

	// create requesters
	for _, order := range orders {
		// add a new goroutine to process the order
		s.wg.Add(1)
		// get the order number
		orderNum := order.Number
		// create a new goroutine to process the order
		go func() {
			defer s.wg.Done()

			// create a new context with timeout
			reqCtx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Timeout)*time.Second)
			defer cancel()

			// get the accrual for the order
			if err := s.getAccrual(reqCtx, orderNum); err != nil {
				// send the error to the error channel
				s.errCh <- fmt.Errorf("order %s: %w", orderNum, err)
			}
		}()
	}

	// wait for all the goroutines to finish
	s.wg.Wait()
	// close the error channel
	close(s.errCh)

	// collect errors
	var joined error
	for err := range s.errCh {
		joined = errors.Join(joined, err)
	}
	return joined
}

// getAccrual sends a request to the accrual system to get the accrual for the order
func (s *AccrualService) getAccrual(ctx context.Context, orderNum string) error {
	// send a request to the accrual system to get the accrual for the order
	resp, err := s.client.R().
		SetContext(ctx).
		SetPathParam("order_number", orderNum).
		Get("/api/orders/{order_number}")
	if err != nil {
		// if the request timed out or was canceled, return an error
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("request timeout: %w", err)
		}
		return fmt.Errorf("http request failed: %w", err)
	}

	switch resp.StatusCode() {
	// if the request is a too many requests, return an error
	case http.StatusTooManyRequests:
		// get the Retry-After header
		retryAfter, convErr := strconv.Atoi(resp.Header().Get("Retry-After"))
		if convErr != nil {
			// if the Retry-After header is not valid, return an error
			return fmt.Errorf("429 without valid Retry-After: %w", convErr)
		}
		// store the Retry-After header
		s.sendAfter.Store(uint32(retryAfter))
		return fmt.Errorf("too many requests, retry-after=%d", retryAfter)

	case http.StatusNoContent:
		// if the order is not registered in the accrual system, return an error
		return fmt.Errorf("order not registered in accrual system")

	case http.StatusInternalServerError:
		// if the accrual service is returning a 500, return an error
		return fmt.Errorf("accrual service 500")
	}

	// unmarshal the response
	r := &accrualResp{}
	if err := json.Unmarshal(resp.Body(), &r); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	// create a new order
	gotOrder := &models.Order{
		Number: r.Order,
		Status: models.OrderStatus(r.Status),
	}
	// if the accrual is not nil, set the accrual
	if r.Accrual != nil {
		gotOrder.Accrual = *r.Accrual
	}

	// if the order is processed or invalid, update the order
	if gotOrder.Status == models.StatusProcessed || gotOrder.Status == models.StatusInvalid {
		if err := s.storage.UpdateOrder(ctx, gotOrder); err != nil {
			return fmt.Errorf("update order: %w", err)
		}
	}
	return nil
}

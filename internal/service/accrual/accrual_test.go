//go:build mock_tests
// +build mock_tests

package accrual

import (
	"context"
	"encoding/json"
	"loyaltySys/internal/models"
	"loyaltySys/internal/service/accrual/config"
	"loyaltySys/internal/service/accrual/mocks"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestAccrualService_Start(t *testing.T) {
	type fields struct {
		client    *resty.Client
		cfg       config.AccrualConfig
		storage   Storage
		logger    *zap.SugaredLogger
		sendAfter atomic.Uint32
		wg        sync.WaitGroup
		errCh     chan error
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "successful_start",
			fields: fields{
				client:    resty.New(),
				cfg:       config.AccrualConfig{Timeout: 0, AccrualAddr: "http://localhost:8080"},
				storage:   nil,
				logger:    zap.NewNop().Sugar(),
				sendAfter: atomic.Uint32{},
				wg:        sync.WaitGroup{},
				errCh:     make(chan error),
			},
			args: args{
				ctx: context.Background(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AccrualService{
				client:    tt.fields.client,
				cfg:       tt.fields.cfg,
				storage:   tt.fields.storage,
				logger:    tt.fields.logger,
				sendAfter: tt.fields.sendAfter,
				wg:        tt.fields.wg,
				errCh:     tt.fields.errCh,
			}

			// use mock storage to avoid real DB dependency
			mockStorage := mocks.NewStorage(t)
			mockStorage.EXPECT().GetUnprocessedOrders(mock.Anything).Return(make([]models.Order, 0), nil)
			s.storage = mockStorage
			s.Start(tt.args.ctx)
			time.Sleep(300 * time.Millisecond)
		})
	}
}

func TestAccrualService_processOrders(t *testing.T) {
	type fields struct {
		client    *resty.Client
		cfg       config.AccrualConfig
		storage   Storage
		logger    *zap.SugaredLogger
		sendAfter atomic.Uint32
		wg        sync.WaitGroup
		errCh     chan error
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "no_unprocessed_orders",
			fields: fields{
				client: resty.New(),
				cfg:    config.AccrualConfig{Timeout: 0},
				storage: func() Storage {
					m := mocks.NewStorage(t)
					m.EXPECT().GetUnprocessedOrders(mock.Anything).Return([]models.Order{}, nil)
					return m
				}(),
				logger: zap.NewNop().Sugar(),
			},
			args:    args{ctx: context.Background()},
			wantErr: false,
		},
		{
			name: "successful_update",
			fields: func() fields {
				handler := http.NewServeMux()
				handler.HandleFunc("/api/orders/123", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"order": "123", "status": "PROCESSED", "accrual": 12.5})
				})
				srv := httptest.NewServer(handler)
				t.Cleanup(srv.Close)

				m := mocks.NewStorage(t)
				m.EXPECT().GetUnprocessedOrders(mock.Anything).Return([]models.Order{{Number: "123"}}, nil)
				m.EXPECT().UpdateOrder(mock.Anything, mock.MatchedBy(func(o *models.Order) bool {
					return o.Number == "123" && o.Status == models.StatusProcessed && o.Accrual == 12.5
				})).Return(nil)

				return fields{
					client:  resty.New().SetBaseURL(srv.URL),
					cfg:     config.AccrualConfig{Timeout: 1, AccrualAddr: srv.URL},
					storage: m,
					logger:  zap.NewNop().Sugar(),
				}
			}(),
			args:    args{ctx: context.Background()},
			wantErr: false,
		},
		{
			name: "return_errors",
			fields: func() fields {
				handler := http.NewServeMux()
				handler.HandleFunc("/api/orders/ok", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"order": "ok", "status": "PROCESSED", "accrual": 1})
				})
				handler.HandleFunc("/api/orders/err", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				})
				srv := httptest.NewServer(handler)
				t.Cleanup(srv.Close)

				m := mocks.NewStorage(t)
				m.EXPECT().GetUnprocessedOrders(mock.Anything).Return([]models.Order{{Number: "ok"}, {Number: "err"}}, nil)
				m.EXPECT().UpdateOrder(mock.Anything, mock.MatchedBy(func(o *models.Order) bool { return o.Number == "ok" && o.Status == models.StatusProcessed })).Return(nil)

				return fields{
					client:  resty.New().SetBaseURL(srv.URL),
					cfg:     config.AccrualConfig{Timeout: 1, AccrualAddr: srv.URL},
					storage: m,
					logger:  zap.NewNop().Sugar(),
				}
			}(),
			args:    args{ctx: context.Background()},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AccrualService{
				client:    tt.fields.client,
				cfg:       tt.fields.cfg,
				storage:   tt.fields.storage,
				logger:    tt.fields.logger,
				sendAfter: tt.fields.sendAfter,
				wg:        tt.fields.wg,
				errCh:     tt.fields.errCh,
			}
			if err := s.processOrders(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("AccrualService.processOrders() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAccrualService_getAccrual(t *testing.T) {
	type fields struct {
		client    *resty.Client
		cfg       config.AccrualConfig
		storage   Storage
		logger    *zap.SugaredLogger
		sendAfter atomic.Uint32
		wg        sync.WaitGroup
		errCh     chan error
	}
	type args struct {
		ctx      context.Context
		orderNum string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{

		{
			name: "successful_update",
			fields: func() fields {
				h := http.NewServeMux()
				h.HandleFunc("/api/orders/9", func(w http.ResponseWriter, r *http.Request) {
					_ = json.NewEncoder(w).Encode(map[string]any{"order": "9", "status": "PROCESSED", "accrual": 7})
				})
				s := httptest.NewServer(h)
				t.Cleanup(s.Close)
				m := mocks.NewStorage(t)
				m.EXPECT().UpdateOrder(mock.Anything, mock.MatchedBy(func(o *models.Order) bool {
					return o.Number == "9" && o.Status == models.StatusProcessed && o.Accrual == 7
				})).Return(nil)
				return fields{client: resty.New().SetBaseURL(s.URL), cfg: config.AccrualConfig{Timeout: 1}, storage: m, logger: zap.NewNop().Sugar()}
			}(),
			args:    args{ctx: context.Background(), orderNum: "9"},
			wantErr: false,
		},
		{
			name: "too_many_requests",
			fields: func() fields {
				handler := http.NewServeMux()
				handler.HandleFunc("/api/orders/123", func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(http.StatusTooManyRequests)
				})
				srv := httptest.NewServer(handler)
				t.Cleanup(srv.Close)
				return fields{client: resty.New().SetBaseURL(srv.URL), cfg: config.AccrualConfig{Timeout: 1}, storage: mocks.NewStorage(t), logger: zap.NewNop().Sugar()}
			}(),
			args:    args{ctx: context.Background(), orderNum: "123"},
			wantErr: true,
		},
		{
			name: "204_error",
			fields: func() fields {
				h := http.NewServeMux()
				h.HandleFunc("/api/orders/1", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
				s := httptest.NewServer(h)
				t.Cleanup(s.Close)
				return fields{client: resty.New().SetBaseURL(s.URL), cfg: config.AccrualConfig{Timeout: 1}, storage: mocks.NewStorage(t), logger: zap.NewNop().Sugar()}
			}(),
			args:    args{ctx: context.Background(), orderNum: "1"},
			wantErr: true,
		},
		{
			name: "500_error",
			fields: func() fields {
				h := http.NewServeMux()
				h.HandleFunc("/api/orders/2", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
				s := httptest.NewServer(h)
				t.Cleanup(s.Close)
				return fields{client: resty.New().SetBaseURL(s.URL), cfg: config.AccrualConfig{Timeout: 1}, storage: mocks.NewStorage(t), logger: zap.NewNop().Sugar()}
			}(),
			args:    args{ctx: context.Background(), orderNum: "2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AccrualService{
				client:    tt.fields.client,
				cfg:       tt.fields.cfg,
				storage:   tt.fields.storage,
				logger:    tt.fields.logger,
				sendAfter: tt.fields.sendAfter,
				wg:        tt.fields.wg,
				errCh:     tt.fields.errCh,
			}
			if err := s.getAccrual(tt.args.ctx, tt.args.orderNum); (err != nil) != tt.wantErr {
				t.Errorf("AccrualService.getAccrual() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

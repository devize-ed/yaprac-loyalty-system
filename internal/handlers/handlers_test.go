package handlers

import (
	"loyaltySys/internal/auth"
	"loyaltySys/internal/handlers/mocks"
	"loyaltySys/internal/models"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"loyaltySys/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

func testEnv(t *testing.T) (*httptest.Server, *mocks.Storage, *chi.Mux, *Handler) {
	t.Helper()
	logger := zap.NewNop().Sugar()

	t.Setenv("AUTH_SECRET", "test-secret")
	auth.InitJWTFromEnv(logger)

	st := mocks.NewStorage(t)
	h := NewHandler(st, logger)
	r := chi.NewRouter()
	srv := httptest.NewServer(r)

	return srv, st, r, h
}

// Injects a JWT token with the user_id claim into the request context.
func injectUserID(r *http.Request, id int64) *http.Request {
	token := jwtauth.New("HS256", []byte("test-secret"), nil)
	claims := map[string]interface{}{"user_id": strconv.FormatInt(id, 10)} // строка!
	_, signed, _ := token.Encode(claims)
	parsedToken, _ := token.Decode(signed)
	ctx := jwtauth.NewContext(r.Context(), parsedToken, nil)
	return r.WithContext(ctx)
}

func TestHandler_CreateUser(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	testUserID := int64(1)
	testUser := &models.User{Login: "test1", Password: "test1"}

	r.Post("/api/user/register", h.CreateUser())

	var tests = []struct {
		name         string
		requestBody  *models.User
		EXPECT       *mock.Call
		wantError    bool
		expectedCode int
	}{
		{
			name:         "add_user",
			requestBody:  testUser,
			EXPECT:       st.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(testUserID, nil).Once(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "user_exists",
			requestBody:  testUser,
			EXPECT:       st.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(int64(-1), db.ErrUserAlreadyExists).Once(),
			expectedCode: http.StatusConflict,
		},
		{
			name:         "invalid_user",
			requestBody:  &models.User{Login: "", Password: ""},
			EXPECT:       nil,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().SetBody(tt.requestBody).Post(srv.URL + "/api/user/register")
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, resp.StatusCode())
			if tt.expectedCode == http.StatusOK {
				authz := resp.Header().Get("Authorization")
				assert.NotEmpty(t, authz)
				assert.Contains(t, authz, "Bearer ")
			}
		})
	}

}

func TestHandler_LoginUser(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	testUser := &models.User{Login: "test1", Password: "test1"}
	hashed, err := bcrypt.GenerateFromPassword([]byte(testUser.Password), bcrypt.DefaultCost)
	assert.NoError(t, err)
	registeredUser := &models.User{ID: 1, Login: testUser.Login, Password: string(hashed)}

	r.Post("/api/user/login", h.LoginUser())

	var tests = []struct {
		name         string
		requestBody  *models.User
		EXPECT       *mock.Call
		wantError    bool
		expectedCode int
	}{
		{
			name:         "login_user",
			requestBody:  testUser,
			EXPECT:       st.EXPECT().GetUser(mock.Anything, mock.Anything).Return(registeredUser, nil).Once(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "user_not_found",
			requestBody:  testUser,
			EXPECT:       st.EXPECT().GetUser(mock.Anything, mock.Anything).Return(nil, db.ErrUserNotFound).Once(),
			expectedCode: http.StatusUnauthorized,
		},
		{
			name:         "invalid_request",
			requestBody:  &models.User{Login: "", Password: ""},
			EXPECT:       nil,
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().SetBody(tt.requestBody).Post(srv.URL + "/api/user/login")
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedCode, resp.StatusCode())
			if tt.expectedCode == http.StatusOK {
				authz := resp.Header().Get("Authorization")
				assert.NotEmpty(t, authz)
				assert.Contains(t, authz, "Bearer ")
			}
		})
	}

}

func TestHandler_CreateOrder(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	userID := int64(1)
	token, err := auth.GenerateToken(userID)
	assert.NoError(t, err)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(auth.TokenAuth))
		r.Use(jwtauth.Authenticator(auth.TokenAuth))
		r.Post("/api/user/orders", h.CreateOrder())
	})

	var tests = []struct {
		name         string
		order        string
		token        string
		EXPECT       *mock.Call
		expectedCode int
	}{
		{
			name:         "valid_order",
			order:        "12345678903",
			token:        token,
			EXPECT:       st.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(nil).Once(),
			expectedCode: http.StatusAccepted,
		},
		{
			name:         "order_already_added",
			order:        "12345678903",
			token:        token,
			EXPECT:       st.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(db.ErrOrderAlreadyAdded).Once(),
			expectedCode: http.StatusConflict,
		},
		{
			name:         "order_already_added_by_this_user",
			order:        "12345678903",
			token:        token,
			EXPECT:       st.EXPECT().CreateOrder(mock.Anything, mock.Anything).Return(db.ErrOrderAlreadyExists).Once(),
			expectedCode: http.StatusOK,
		},
		{
			name:         "invalid_order_number",
			order:        "1234567890123",
			token:        token,
			EXPECT:       nil,
			expectedCode: http.StatusUnprocessableEntity,
		},
		{
			name:         "invalid_request",
			order:        "",
			token:        token,
			EXPECT:       nil,
			expectedCode: http.StatusUnprocessableEntity,
		},
		{
			name:         "user_not_authenticated",
			order:        "12345678903",
			token:        "wrong_token",
			EXPECT:       nil,
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().
				SetHeader("Authorization", "Bearer "+tt.token).
				SetHeader("Content-Type", "text/plain").
				SetBody(tt.order).
				Post(srv.URL + "/api/user/orders")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode())
		})
	}
}

func TestHandler_GetOrders(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	userID := int64(1)
	token, err := auth.GenerateToken(userID)
	assert.NoError(t, err)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(auth.TokenAuth))
		r.Use(jwtauth.Authenticator(auth.TokenAuth))
		r.Get("/api/user/orders", h.GetOrders())
	})

	uploadedAt, err := time.Parse("2006-01-02T15:04:05-07:00", "2020-12-10T15:15:45+03:00")
	assert.NoError(t, err)

	orders := []*models.Order{
		{
			UserID:     1,
			Number:     "9278923470",
			Status:     models.StatusProcessed,
			UploadedAt: uploadedAt,
		},
		{
			UserID:     1,
			Number:     "12345678903",
			Status:     models.StatusProcessing,
			UploadedAt: uploadedAt,
		},
		{
			UserID:     1,
			Number:     "346436439",
			Status:     models.StatusInvalid,
			UploadedAt: uploadedAt,
		},
	}

	var tests = []struct {
		name         string
		token        string
		EXPECT       *mock.Call
		expectedCode int
		expectedBody string
	}{
		{
			name:         "successful_request",
			token:        token,
			EXPECT:       st.EXPECT().GetOrders(mock.Anything, mock.Anything).Return(orders, nil).Once(),
			expectedCode: http.StatusOK,
			expectedBody: `[{"number":"9278923470","status":"PROCESSED","uploaded_at":"2020-12-10T15:15:45+03:00"},{"number":"12345678903","status":"PROCESSING","uploaded_at":"2020-12-10T15:15:45+03:00"},{"number":"346436439","status":"INVALID","uploaded_at":"2020-12-10T15:15:45+03:00"}]`,
		},
		{
			name:         "no_orders",
			token:        token,
			EXPECT:       st.EXPECT().GetOrders(mock.Anything, mock.Anything).Return([]*models.Order{}, nil).Once(),
			expectedCode: http.StatusNoContent,
			expectedBody: "",
		},
		{
			name:         "user_not_authenticated",
			token:        "wrong_token",
			EXPECT:       nil,
			expectedCode: http.StatusUnauthorized,
			expectedBody: "token is unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().
				SetHeader("Authorization", "Bearer "+tt.token).
				Get(srv.URL + "/api/user/orders")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode())
			assert.Equal(t, tt.expectedBody, resp.String())
		})
	}
}

func TestHandler_GetBalance(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	userID := int64(1)
	token, err := auth.GenerateToken(userID)
	assert.NoError(t, err)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(auth.TokenAuth))
		r.Use(jwtauth.Authenticator(auth.TokenAuth))
		r.Get("/api/user/balance", h.GetBalance())
	})

	var tests = []struct {
		name         string
		token        string
		EXPECT       *mock.Call
		expectedCode int
		expectedBody string
	}{
		{
			name:  "successful_request",
			token: token,
			EXPECT: st.EXPECT().GetBalance(mock.Anything, mock.Anything).Return(&models.Balance{
				Current:   500.5,
				Withdrawn: 42.0,
			}, nil).Once(),
			expectedCode: http.StatusOK,
			expectedBody: `{"current":500.5,"withdrawn":42}`,
		},
		{
			name:         "user_not_authenticated",
			token:        "wrong_token",
			EXPECT:       nil,
			expectedCode: http.StatusUnauthorized,
			expectedBody: "token is unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().
				SetHeader("Authorization", "Bearer "+tt.token).
				Get(srv.URL + "/api/user/balance")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode())
			assert.Equal(t, tt.expectedBody, resp.String())
		})
	}
}

func TestHandler_Withdraw(t *testing.T) {

	srv, st, r, h := testEnv(t)
	defer srv.Close()

	userID := int64(1)
	token, err := auth.GenerateToken(userID)
	assert.NoError(t, err)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(auth.TokenAuth))
		r.Use(jwtauth.Authenticator(auth.TokenAuth))
		r.Post("/api/user/balance/withdraw", h.Withdraw())
	})

	var tests = []struct {
		name         string
		withdraw     *models.Withdrawal
		token        string
		EXPECT       *mock.Call
		expectedCode int
	}{
		{
			name: "successful_withdraw",
			withdraw: &models.Withdrawal{
				Order: "9278923470",
				Sum:   10.0,
			},
			token:        token,
			EXPECT:       st.EXPECT().Withdraw(mock.Anything, mock.Anything).Return(nil).Once(),
			expectedCode: http.StatusOK,
		},
		{
			name: "incuficient_balance",
			withdraw: &models.Withdrawal{
				Order: "12345678903",
				Sum:   10.0,
			},
			token:        token,
			EXPECT:       st.EXPECT().Withdraw(mock.Anything, mock.Anything).Return(db.ErrInsufficientBalance).Once(),
			expectedCode: http.StatusPaymentRequired,
		},
		{
			name: "invalid_order_number",
			withdraw: &models.Withdrawal{
				Order: "1234567890123",
				Sum:   10.0,
			},
			token:        token,
			EXPECT:       nil,
			expectedCode: http.StatusUnprocessableEntity,
		},
		{
			name: "invalid_request",
			withdraw: &models.Withdrawal{
				Order: "",
				Sum:   10.0,
			},
			token:        token,
			EXPECT:       nil,
			expectedCode: http.StatusUnprocessableEntity,
		},
		{
			name: "user_not_authenticated",
			withdraw: &models.Withdrawal{
				Order: "12345678903",
				Sum:   10.0,
			},
			token:        "wrong_token",
			EXPECT:       nil,
			expectedCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().
				SetHeader("Authorization", "Bearer "+tt.token).
				SetHeader("Content-Type", "application/json").
				SetBody(tt.withdraw).
				Post(srv.URL + "/api/user/balance/withdraw")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode())
		})
	}
}

func TestHandler_GetWithdrawals(t *testing.T) {
	srv, st, r, h := testEnv(t)
	defer srv.Close()

	userID := int64(1)
	token, err := auth.GenerateToken(userID)
	assert.NoError(t, err)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(auth.TokenAuth))
		r.Use(jwtauth.Authenticator(auth.TokenAuth))
		r.Get("/api/user/withdrawals", h.GetWithdrawals())
	})

	uploadedAt, err := time.Parse("2006-01-02T15:04:05-07:00", "2020-12-10T15:15:45+03:00")
	assert.NoError(t, err)

	withdrawals := []*models.Withdrawal{
		{
			UserID:      1,
			Order:       "9278923470",
			Sum:         10.0,
			ProcessedAt: uploadedAt,
		},
		{
			UserID:      1,
			Order:       "12345678903",
			Sum:         15.0,
			ProcessedAt: uploadedAt,
		},
		{
			UserID:      1,
			Order:       "346436439",
			Sum:         20.0,
			ProcessedAt: uploadedAt,
		},
	}

	var tests = []struct {
		name         string
		token        string
		EXPECT       *mock.Call
		expectedCode int
		expectedBody string
	}{
		{
			name:         "successful_request",
			token:        token,
			EXPECT:       st.EXPECT().GetWithdrawals(mock.Anything, mock.Anything).Return(withdrawals, nil).Once(),
			expectedCode: http.StatusOK,
			expectedBody: `[{"order":"9278923470","sum":10,"processed_at":"2020-12-10T15:15:45+03:00"},{"order":"12345678903","sum":15,"processed_at":"2020-12-10T15:15:45+03:00"},{"order":"346436439","sum":20,"processed_at":"2020-12-10T15:15:45+03:00"}]`,
		},
		{
			name:         "no_withdrawals",
			token:        token,
			EXPECT:       st.EXPECT().GetWithdrawals(mock.Anything, mock.Anything).Return([]*models.Withdrawal{}, nil).Once(),
			expectedCode: http.StatusNoContent,
			expectedBody: "",
		},
		{
			name:         "user_not_authenticated",
			token:        "wrong_token",
			EXPECT:       nil,
			expectedCode: http.StatusUnauthorized,
			expectedBody: "token is unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := resty.New().R().
				SetHeader("Authorization", "Bearer "+tt.token).
				Get(srv.URL + "/api/user/withdrawals")
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.StatusCode())
			assert.Equal(t, tt.expectedBody, resp.String())
		})
	}
}

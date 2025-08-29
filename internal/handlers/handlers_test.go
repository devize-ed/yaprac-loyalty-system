package handlers

import (
	"loyaltySys/internal/auth"
	"loyaltySys/internal/handlers/mocks"
	"loyaltySys/internal/models"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"loyaltySys/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func testEnv(t *testing.T) (*httptest.Server, *mocks.Storage, *chi.Mux, *Handler) {
	t.Helper()

	t.Setenv("AUTH_SECRET", "test-secret")
	if err := auth.InitJWTFromEnv(); err != nil {
		t.Fatalf("InitJWTFromEnv failed: %v", err)
	}

	logger := zap.NewNop().Sugar()
	st := mocks.NewStorage(t)
	h := NewHandler(st, logger)
	r := chi.NewRouter()
	srv := httptest.NewServer(r)

	return srv, st, r, h
}

// Injects a JWT token with the user_id claim into the request context.
func withUserID(r *http.Request, id int64) *http.Request {
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

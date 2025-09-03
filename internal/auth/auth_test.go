package auth

import (
	"context"
	"loyaltySys/internal/models"
	"sync"
	"testing"

	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGenerateToken(t *testing.T) {
	TokenAuth = nil
	tokenOnce = sync.Once{}

	t.Setenv("AUTH_SECRET", "sign-secret")
	InitJWTFromEnv(zap.NewNop().Sugar())

	const uid int64 = 123
	tokenStr, err := GenerateToken(uid)
	assert.NoError(t, err, "failed to generate token")
	assert.NotEmpty(t, tokenStr, "token is empty")

	tok, err := TokenAuth.Decode(tokenStr)
	assert.NoError(t, err, "failed to decode token")

	v, ok := tok.Get("user_id")
	assert.True(t, ok, "decoded token has no user_id claim")
	assert.Equal(t, "123", v, "user_id claim = %#v (type %T), want \"123\"", v, v)

	assert.True(t, ok, "decoded token has no exp claim")
}

func TestGetUserIDFromCtx(t *testing.T) {
	auth := jwtauth.New("HS256", []byte("any"), nil)

	type tc struct {
		name    string
		claims  map[string]any
		want    int64
		wantErr bool
	}

	tests := []tc{
		{
			name:    "ok",
			claims:  map[string]any{"user_id": "77"},
			want:    77,
			wantErr: false,
		},
		{
			name:    "no_user_id",
			claims:  map[string]any{"role": "user"},
			want:    0,
			wantErr: true,
		},
		{
			name:    "non_numeric_user_id",
			claims:  map[string]any{"user_id": "abc"},
			want:    0,
			wantErr: true,
		},
		{
			name:    "no_token",
			claims:  nil,
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var ctx context.Context
			if tt.claims != nil {
				tok, _, err := auth.Encode(tt.claims)
				assert.NoError(t, err, "failed to encode token")
				ctx = jwtauth.NewContext(context.Background(), tok, nil)
			} else {
				ctx = context.Background()
			}

			got, err := GetUserIDFromCtx(ctx)
			assert.Equalf(t, tt.wantErr, err != nil, "GetUserIDFromCtx() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equalf(t, tt.want, got, "GetUserIDFromCtx() = %v, want %v", got, tt.want)
		})
	}
}

func TestValidateUser(t *testing.T) {
	tests := []struct {
		name string
		u    models.User
		ok   bool
	}{
		{"ok", models.User{Login: "alice", Password: "pwd"}, true},
		{"empty login", models.User{Login: "", Password: "pwd"}, false},
		{"empty password", models.User{Login: "bob", Password: ""}, false},
		{"both empty", models.User{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := ValidateUser(tc.u)
			assert.Equal(t, tc.ok, ok, "ValidateUser() ok = %v, want %v", ok, tc.ok)
			assert.Equal(t, tc.ok, err == nil, "ValidateUser() err = %v, want nil", err)
		})
	}
}

func TestValidateOrderNumber(t *testing.T) {
	tests := []struct {
		number string
		ok     bool
	}{
		{"4242424242424242", true},
		{"4012888888881881", true},
		{"79927398713", true},
		{"", false},
		{"1234567890123", false},
	}
	for _, tc := range tests {
		t.Run(tc.number, func(t *testing.T) {
			ok, err := ValidateOrderNumber(tc.number)
			assert.Equal(t, tc.ok, ok, "ValidateOrderNumber() ok = %v, want %v", ok, tc.ok)
			assert.Equal(t, tc.ok, err == nil, "ValidateOrderNumber() err = %v, want nil", err)
		})
	}
}

func Test_checkLuhn(t *testing.T) {
	tests := []struct {
		number string
		ok     bool
	}{
		{"4242424242424242", true},
		{"1234567890123", false},
	}
	for _, tc := range tests {
		t.Run(tc.number, func(t *testing.T) {
			if tc.ok != checkLuhn(tc.number) {
				t.Fatalf("checkLuhn(%s) = %v, want %v", tc.number, checkLuhn(tc.number), tc.ok)
			}
			assert.Equal(t, tc.ok, checkLuhn(tc.number), "checkLuhn(%s) = %v, want %v", tc.number, checkLuhn(tc.number), tc.ok)
		})
	}
}

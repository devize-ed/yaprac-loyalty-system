package auth

import (
	"context"
	"errors"
	"fmt"
	"loyaltySys/internal/models"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.uber.org/zap"
)

var (
	tokenOnce sync.Once          // tokenOnce is a once.Do for the token auth.
	TokenAuth *jwtauth.JWTAuth   // TokenAuth is the JWT authentication middleware.
	tokenSkew = 30 * time.Second // tokenSkew is the acceptable skew for the token.
)

var (
	errClaimNotFound       = errors.New("user_id not found in claims")     // errClaimNotFound is the error returned when the user ID is not found in the claims.
	errCredRequired        = errors.New("login and password are required") // errCredRequired is the error returned when the login and password are required.
	errOrderNumberRequired = errors.New("order number is required")        // errOrderNumberRequired is the error returned when the order number is required.
	errInvalidOrderNumber  = errors.New("invalid order number")            // errInvalidOrderNumber is the error returned when the order number is invalid.
)

// InitJWTFromEnv initializes the JWT authentication middleware from the environment variables.
func InitJWTFromEnv(logger *zap.SugaredLogger) {
	tokenOnce.Do(func() {
		secret := os.Getenv("AUTH_SECRET")
		if secret == "" {
			logger.Warn("AUTH_SECRET is not set, setting test secret")
			secret = "test-secret"
		}
		TokenAuth = jwtauth.New("HS256", []byte(secret), nil, jwt.WithAcceptableSkew(tokenSkew))
	})
}

// GetUserIDFromCtx extracts the user ID from the JWT token in the context.
func GetUserIDFromCtx(ctx context.Context) (int64, error) {
	// Get the JWT token from the context
	_, claims, _ := jwtauth.FromContext(ctx)
	// Check if the user ID is in the claims
	userID, ok := claims["user_id"].(string)
	if !ok {
		return 0, errClaimNotFound
	}
	return strconv.ParseInt(userID, 10, 64)
}

// validateUser validates the user.
func ValidateUser(user models.User) (bool, error) {
	if user.Login == "" || user.Password == "" {
		return false, errCredRequired
	}
	return true, nil
}

// validateOrderNumber validates the order number.
func ValidateOrderNumber(orderNumber string) (bool, error) {
	if orderNumber == "" {
		return false, errOrderNumberRequired
	}
	if !checkLuhn(orderNumber) {
		return false, errInvalidOrderNumber
	}
	return true, nil
}

// checkLuhn is a helper function to check if the order number is valid.
func checkLuhn(purportedCC string) bool {
	sum := 0
	nDigits := len(purportedCC)
	parity := nDigits % 2
	for i := 0; i < nDigits; i++ {
		digit := int(purportedCC[i] - '0')
		if i%2 == parity {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
	}
	return sum%10 == 0
}

// generateToken generates a new JWT token for the user.
func GenerateToken(userID int64) (string, error) {
	claims := map[string]interface{}{
		"user_id":   strconv.FormatInt(userID, 10),
		"issued_at": time.Now().Unix(),
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	_, token, err := TokenAuth.Encode(claims)
	if err != nil {
		return "", fmt.Errorf("failed to encode token: %w", err)
	}
	return token, nil
}

package auth

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var TokenAuth *jwtauth.JWTAuth

// init initializes the JWT authentication TokenAuth.
func init() {
	// Get the secret from the environment variables
	secret := os.Getenv("AUTH_SECRET")
	// Create a new JWT authentication middleware
	TokenAuth = jwtauth.New("HS256", []byte(secret), nil, jwt.WithAcceptableSkew(30*time.Second))
}

// GetUserIDFromCtx extracts the user ID from the JWT token in the context.
func GetUserIDFromCtx(ctx context.Context) (int64, error) {
	// Get the JWT token from the context
	_, claims, _ := jwtauth.FromContext(ctx)
	// Check if the user ID is in the claims
	userID, ok := claims["user_id"].(int64)
	if !ok {
		return 0, errors.New("user_id not found in claims")
	}
	return userID, nil
}

// validateUser validates the user.
func ValidateUser(user models.User) (bool, error) {
	if user.Login == "" || user.Password == "" {
		return false, errors.New("login and password are required")
	}
	return true, nil
}

// validateOrderNumber validates the order number.
func ValidateOrderNumber(orderNumber string) (bool, error) {
	if orderNumber == "" {
		return false, errors.New("order number is required")
	}
	if !checkLuhn(orderNumber) {
		return false, errors.New("invalid order number")
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

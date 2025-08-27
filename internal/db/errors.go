package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrOrderAlreadyExists  = errors.New("order already exists")
	ErrOrderAlreadyAdded   = errors.New("order already added by another user")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrUserNotFound        = errors.New("user not found")
	ErrOrderNotFound       = errors.New("order not found")
)

// isErrorDuplicate checks for specific PostgreSQL error codes that indicate duplicate errors.
func isErrorDuplicate(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.UniqueViolation:
			return true
		}
	}
	return false
}

// isUserOrder checks if the order belongs to the user.
func (db *DB) isUserOrder(ctx context.Context, orderNumber string, userID int64) error {
	db.logger.Debugf("Checking if order %s is already added by user %d", orderNumber, userID)
	// Get the user ID of the order
	var existingUserID int64
	err := db.pool.QueryRow(ctx, "SELECT user_id FROM orders WHERE order_number = $1", orderNumber).Scan(&existingUserID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrOrderNotFound
		}
		return fmt.Errorf("failed to get order owner: %w", err)
	}

	// Check if the order belongs to the user
	db.logger.Debugf("Order %s belongs to user %d", orderNumber, existingUserID)
	if existingUserID == userID {
		return ErrOrderAlreadyExists
	}

	return ErrOrderAlreadyAdded
}

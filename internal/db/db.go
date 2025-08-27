package db

import (
	"context"
	"errors"
	"fmt"
	"loyaltySys/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DB struct for the database.
type DB struct {
	pool   *pgxpool.Pool
	logger *zap.SugaredLogger
}

// NewDB provides the new data base connection with the provided configuration.
func NewDB(ctx context.Context, dsn string, logger *zap.SugaredLogger) (*DB, error) {
	logger.Debugf("Connecting to database with DSN: %s", dsn)

	// Initialize a new connection pool with the provided DSN
	pool, err := initPool(ctx, dsn, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise a connection pool: %w", err)
	}

	logger.Debug("Database connection established successfully")
	return &DB{
		pool:   pool,
		logger: logger,
	}, nil
}

// initPool initializes a new connection pool.
func initPool(ctx context.Context, dsn string, logger *zap.SugaredLogger) (*pgxpool.Pool, error) {
	// Parse the DSN and create a new connection pool with tracing enabled
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the DSN: %w", err)
	}

	// Set the connection pool configuration
	poolCfg.ConnConfig.Tracer = &queryTracer{logger: logger}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a connection pool: %w", err)
	}

	// Ping the database to ensure the connection is established
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping the database: %w", err)
	}
	return pool, nil
}

// Close closes the database connection pool.
func (db *DB) Close() error {
	db.pool.Close()
	return nil
}

// CreateUser creates a new user and returns the user ID created by the database.
func (db *DB) CreateUser(ctx context.Context, user *models.User) (userID int64, err error) {
	db.logger.Debugf("Creating user %s", user.Login)
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return -1, fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()
	// Add a new user to the database if the user already exists, return an error
	if err := tx.QueryRow(ctx, "INSERT INTO users (login, password) VALUES ($1, $2) RETURNING id", user.Login, user.Password).Scan(&userID); err != nil {
		if isErrorDuplicate(err) {
			return -1, ErrUserAlreadyExists
		}
		return -1, fmt.Errorf("failed to create a user: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return -1, fmt.Errorf("failed to commit a transaction: %w", err)
	}
	return userID, nil
}

// GetUser gets the user by login and returns the hash of the password.
func (db *DB) GetUser(ctx context.Context, login string) (*models.User, error) {
	db.logger.Debugf("Getting user by login: %s", login)

	u := &models.User{}
	err := db.pool.QueryRow(ctx,
		`SELECT id, password FROM users WHERE login=$1`, login,
	).Scan(&u.ID, &u.Password)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select user: %w", err)
	}
	u.Login = login
	return u, nil
}

// CreateOrder creates a new order and returns an error if the order already exists.
func (db *DB) CreateOrder(ctx context.Context, order *models.Order) error {
	db.logger.Debugf("Creating order %s", order.Number)
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	// Try to insert the new order
	if _, err := tx.Exec(ctx, "INSERT INTO orders (order_number, user_id) VALUES ($1, $2)", order.Number, order.UserID); err != nil {
		// If duplicate, check which user owns the order
		if isErrorDuplicate(err) {
			return db.isUserOrder(ctx, order.Number, order.UserID)
		}
		return fmt.Errorf("failed to insert an order: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit a transaction: %w", err)
	}
	return nil
}

// GetOrders gets the orders for the user and returns them.
func (db *DB) GetOrders(ctx context.Context, userID int64) ([]*models.Order, error) {
	db.logger.Debugf("Getting orders for user %d", userID)
	// Get the orders for the user
	rows, err := db.pool.Query(ctx, "SELECT order_number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	defer rows.Close()

	orders := []*models.Order{}
	for rows.Next() {
		order := &models.Order{}
		var accrual *float64
		err := rows.Scan(&order.Number, &order.Status, &accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		if accrual != nil {
			order.Accrual = *accrual
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// GetBalance gets the balance for the user and returns it.
func (db *DB) GetBalance(ctx context.Context, userID int64) (*models.Balance, error) {
	db.logger.Debugf("Getting balance for user %d", userID)

	// Get the balance for the user
	balance := &models.Balance{}
	var accrual *float64
	var withdrawn *float64
	// Get the withdrawn sum
	err := db.pool.QueryRow(ctx, "SELECT COALESCE(SUM(summ), 0) FROM withdrawals WHERE user_id = $1", userID).Scan(&withdrawn)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Get the accrual sum
	err = db.pool.QueryRow(ctx, "SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED'", userID).Scan(&accrual)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	if withdrawn != nil {
		balance.Withdrawn = *withdrawn
	}
	if accrual != nil {
		balance.Current = *accrual - balance.Withdrawn
	}

	return balance, nil
}

// Withdraw requests a withdrawal from the user's balance and returns an error if the balance is less than the withdrawal sum.
func (db *DB) Withdraw(ctx context.Context, withdrawal *models.Withdrawal) error {
	db.logger.Debugf("Withdrawing %f for order %s", withdrawal.Sum, withdrawal.Order)
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	// Check if the balance is enough
	balance, err := db.GetBalance(ctx, withdrawal.UserID)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	if balance.Current < withdrawal.Sum {
		db.logger.Debugf("insufficient balance: %f < %f", balance.Current, withdrawal.Sum)
		return ErrInsufficientBalance
	}

	// Insert the new withdrawal
	if _, err := tx.Exec(ctx, "INSERT INTO withdrawals (order_number, user_id, summ) VALUES ($1, $2, $3)", withdrawal.Order, withdrawal.UserID, withdrawal.Sum); err != nil {
		if isErrorDuplicate(err) {
			return ErrOrderAlreadyExists
		}
		return fmt.Errorf("failed to create a withdrawal: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit a transaction: %w", err)
	}
	return nil
}

// GetWithdrawals gets the withdrawals for the user and returns them.
func (db *DB) GetWithdrawals(ctx context.Context, userID int64) ([]*models.Withdrawal, error) {
	db.logger.Debugf("Getting withdrawals for user %d", userID)

	// Get the withdrawals for the user
	rows, err := db.pool.Query(ctx, "SELECT order_number, summ, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get withdrawals: %w", err)
	}
	defer rows.Close()

	withdrawals := []*models.Withdrawal{}

	for rows.Next() {
		withdrawal := &models.Withdrawal{}
		err := rows.Scan(&withdrawal.Order, &withdrawal.Sum, &withdrawal.ProcessedAt)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	return withdrawals, nil
}

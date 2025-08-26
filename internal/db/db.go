package db

import (
	"context"
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
		return nil, fmt.Errorf("failed to ping the DB: %w", err)
	}
	return pool, nil
}

func (db *DB) Ping(ctx context.Context) error {
	db.logger.Debug("Pinging the database")
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping the database: %w", err)
	}
	db.logger.Debug("Database is connected")
	return nil
}

// Close closes the database connection pool.
func (db *DB) Close() error {
	db.pool.Close()
	return nil
}

// CreateUser creates a new user and returns the user ID created by the database.
func (db *DB) CreateUser(ctx context.Context, user *models.User) (int64, error) {
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
	var userID int64
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
func (db *DB) GetUser(ctx context.Context, login string) (hashPassword string, err error) {
	db.logger.Debugf("Getting user by login: %s", login)
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	err = tx.QueryRow(ctx, "SELECT password FROM users WHERE login = $1", login).Scan(&hashPassword)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("failed to get user by login: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("failed to commit a transaction: %w", err)
	}

	return hashPassword, nil
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
	if _, err := tx.Exec(ctx, "INSERT INTO orders (order, user_id) VALUES ($1, $2)", order.Number, order.UserID); err != nil {
		// If duplicate, check which user owns the order
		if isErrorDuplicate(err) {
			return db.isUserOrder(ctx, order.Number, order.UserID)
		}
		return fmt.Errorf("failed to create an order: %w", err)
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
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	// Get the orders for the user
	rows, err := tx.Query(ctx, "SELECT order, status, accrual, uploaded_at FROM orders WHERE user_id = $1", userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}
	defer rows.Close()

	orders := []*models.Order{}
	for rows.Next() {
		order := &models.Order{}
		err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit a transaction: %w", err)
	}

	return orders, nil
}

// GetBalance gets the balance for the user and returns it.
func (db *DB) GetBalance(ctx context.Context, userID int64) (*models.Balance, error) {
	db.logger.Debugf("Getting balance for user %d", userID)
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	// Get the balance for the user
	balance := &models.Balance{}
	err = tx.QueryRow(ctx, "SELECT SUM(accrual), SUM(withdrawn) FROM orders WHERE user_id = $1", userID).Scan(&balance.Current, &balance.Withdrawn)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit a transaction: %w", err)
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
		return ErrInsufficientBalance
	}

	// Try to insert the new withdrawal
	if _, err := tx.Exec(ctx, "INSERT INTO withdrawals (order, user_id, sum) VALUES ($1, $2, $3)", withdrawal.Order, withdrawal.UserID, withdrawal.Sum); err != nil {
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
	// Begin a new transaction
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin a transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && err != pgx.ErrTxClosed {
			db.logger.Errorf("failed to rollback a transaction: %w", err)
		}
	}()

	// Get the withdrawals for the user
	rows, err := tx.Query(ctx, "SELECT order, sum, processed_at FROM withdrawals WHERE user_id = $1", userID)
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

	// Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit a transaction: %w", err)
	}

	return withdrawals, nil
}

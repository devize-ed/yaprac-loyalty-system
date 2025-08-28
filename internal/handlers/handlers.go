package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"loyaltySys/internal/auth"
	"loyaltySys/internal/db"
	"loyaltySys/internal/models"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type Storage interface {
	CreateUser(ctx context.Context, user *models.User) (int64, error)
	GetUser(ctx context.Context, login string) (*models.User, error)
	CreateOrder(ctx context.Context, order *models.Order) error
	GetOrders(ctx context.Context, userID int64) ([]*models.Order, error)
	GetBalance(ctx context.Context, userID int64) (*models.Balance, error)
	GetWithdrawals(ctx context.Context, userID int64) ([]*models.Withdrawal, error)
	Withdraw(ctx context.Context, withdrawal *models.Withdrawal) error
}

func NewStorage(ctx context.Context, dsn string, logger *zap.SugaredLogger) Storage {
	db, err := db.NewDB(ctx, dsn, logger)
	if err != nil {
		logger.Fatal("failed to create storage", err)
		return nil
	}
	return db
}

type Handler struct {
	storage Storage
	logger  *zap.SugaredLogger
}

func NewHandler(s Storage, logger *zap.SugaredLogger) *Handler {
	return &Handler{
		storage: s,
		logger:  logger,
	}
}

// CreateUser registers a new user in the system and saves it to the database.
// It authenticates the user and generates a token for them.
func (h *Handler) CreateUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Creating user request")

		// Decode the request body into a User struct
		h.logger.Debug("Decoding user")
		user := models.User{}
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.logger.Error("failed to decode user", err)
			http.Error(w, "Failed to decode user", http.StatusBadRequest)
			return
		}
		// Validate the user
		if ok, err := auth.ValidateUser(user); !ok {
			h.logger.Error("invalid user", err)
			http.Error(w, "Invalid user", http.StatusBadRequest)
			return
		}
		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			h.logger.Error("failed to hash password", err)
			http.Error(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		user.Password = string(hashedPassword)

		// Create the user in the database
		userID, err := h.storage.CreateUser(r.Context(), &user)
		if err != nil {
			if errors.Is(err, db.ErrUserAlreadyExists) {
				h.logger.Error(err)
				http.Error(w, "User already exists", http.StatusConflict)
				return
			}
			h.logger.Error("failed to create user: ", err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		// Generate a token for the user
		token, err := auth.GenerateToken(userID)
		if err != nil {
			h.logger.Error("failed to generate token: ", err)
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}
		// Set the token in the response header
		w.Header().Set("Authorization", "Bearer "+token)
		w.WriteHeader(http.StatusOK)
	}
}

// LoginUser authenticates a user and generates a token for them.
func (h *Handler) LoginUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Login user request")

		// Decode the request body into a User struct
		h.logger.Debug("Decoding user")
		user := models.User{}
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.logger.Error("failed to decode user: ", err)
			http.Error(w, "Failed to decode user", http.StatusBadRequest)
			return
		}
		// Validate the user
		if ok, err := auth.ValidateUser(user); !ok {
			h.logger.Error("invalid user: ", err)
			http.Error(w, "Invalid user", http.StatusBadRequest)
			return
		}
		// Search the user in the database and compare the password
		h.logger.Debug("Searching user in the database")
		registeredUser, err := h.storage.GetUser(r.Context(), user.Login)
		if err != nil {
			h.logger.Error("failed to get user: ", err)
			http.Error(w, "Failed to get user", http.StatusInternalServerError)
			return
		}
		// Compare the password
		h.logger.Debug("Comparing password")
		if err := bcrypt.CompareHashAndPassword([]byte(registeredUser.Password), []byte(user.Password)); err != nil {
			h.logger.Error("invalid password: ", err)
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
		// Generate a token for the user
		h.logger.Debug("Generating token for user: ", registeredUser.ID)
		token, err := auth.GenerateToken(registeredUser.ID)
		if err != nil {
			h.logger.Error("failed to generate token: ", err)
			http.Error(w, "Failed to generate token", http.StatusInternalServerError)
			return
		}
		// Set the token in the response header
		w.Header().Set("Authorization", "Bearer "+token)
		w.WriteHeader(http.StatusOK)
	}
}

// CreateOrder creates a new order for a user.
func (h *Handler) CreateOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Creating order request")

		// Check if the order number is valid
		orderNumber, err := io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error("failed to read order number: ", err)
			http.Error(w, "Failed to read order number", http.StatusInternalServerError)
			return
		}
		// Check if the order number is valid
		h.logger.Debug("Order number: ", string(orderNumber))
		if ok, err := auth.ValidateOrderNumber(string(orderNumber)); !ok {
			h.logger.Error("invalid order number: ", err)
			http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
			return
		}
		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID: ", err)
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID: ", userID)
		// Create the order in the database
		err = h.storage.CreateOrder(r.Context(), newOrder(string(orderNumber), userID))
		if err != nil {
			// Check if the order already added by another user - return 409
			if errors.Is(err, db.ErrOrderAlreadyAdded) {
				h.logger.Error("order already added by another user: ", err)
				http.Error(w, "Order already added by another user", http.StatusConflict)
				return
				// Check if the order already added by this user - return 200
			} else if errors.Is(err, db.ErrOrderAlreadyExists) {
				h.logger.Error("order already added by this user: ", err)
				http.Error(w, "Order already added by this user", http.StatusOK)
				return
			}
			// Return 500
			h.logger.Error("failed to create order: ", err)
			http.Error(w, "Failed to create order", http.StatusInternalServerError)
			return
		}

		// Return 202 if the order is accepted for processing
		h.logger.Debug("Order accepted for processing")
		w.WriteHeader(http.StatusAccepted)
	}
}

// GetOrders returns all orders for a user.
func (h *Handler) GetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Getting orders request")

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID: ", err)
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID: ", userID)
		// Get the orders from the database
		orders, err := h.storage.GetOrders(r.Context(), userID)
		if err != nil {
			h.logger.Error("failed to get orders: ", err)
			http.Error(w, "Failed to get orders", http.StatusInternalServerError)
			return
			// Return 204 if no orders found for user - no content
		} else if len(orders) == 0 {
			h.logger.Debug("No orders found for user: ", userID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.logger.Debug("Orders found for user: ", userID)
		// Return the orders
		json.NewEncoder(w).Encode(orders)
		w.WriteHeader(http.StatusOK)
	}
}

// GetBalance returns the balance for a user.
func (h *Handler) GetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Getting balance request")

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID: ", err)
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID: ", userID)
		// Get the balance from the database
		balance, err := h.storage.GetBalance(r.Context(), userID)
		if err != nil {
			h.logger.Error("failed to get balance: ", err)
			http.Error(w, "Failed to get balance", http.StatusInternalServerError)
			return
		}
		h.logger.Debug("Balance: ", balance)
		// Return the balance
		json.NewEncoder(w).Encode(balance)
		w.WriteHeader(http.StatusOK)
	}
}

// WithdrawBalance withdraws bonus points of user from balance.
func (h *Handler) Withdraw() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Withdrawing balance request")

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID: ", err)
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID: ", userID)
		// Decode the request body into a Withdrawal struct
		h.logger.Debug("Decoding withdrawal")
		withdrawal := models.Withdrawal{}
		err = json.NewDecoder(r.Body).Decode(&withdrawal)
		if err != nil {
			h.logger.Error("failed to decode withdrawal: ", err)
			http.Error(w, "Failed to decode withdrawal", http.StatusInternalServerError)
			return
		}
		// Check if the withdrawal is valid
		if ok, err := auth.ValidateOrderNumber(withdrawal.Order); !ok {
			h.logger.Error("invalid order number: ", err)
			http.Error(w, "Invalid order number", http.StatusUnprocessableEntity)
			return
		}
		withdrawal.UserID = userID
		// Withdraw the balance
		err = h.storage.Withdraw(r.Context(), &withdrawal)
		if err != nil {
			if errors.Is(err, db.ErrInsufficientBalance) {
				h.logger.Error("insufficient balance: ", err)
				http.Error(w, "Insufficient balance", http.StatusPaymentRequired)
				return
			}
			if errors.Is(err, db.ErrOrderAlreadyExists) {
				h.logger.Error("withdrawal order number already exists: ", err)
				http.Error(w, "Withdrawal order number already exists", http.StatusConflict)
				return
			}
			h.logger.Error("failed to withdraw balance: ", err)
			http.Error(w, "Failed to withdraw balance", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// GetWithdrawals returns all withdrawals for a user.
func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Getting withdrawals request")

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID: ", err)
			http.Error(w, "Failed to get user ID", http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID: ", userID)
		// Get the withdrawals from the database
		withdrawals, err := h.storage.GetWithdrawals(r.Context(), userID)
		if err != nil {
			h.logger.Error("failed to get withdrawals: ", err)
			http.Error(w, "Failed to get withdrawals", http.StatusInternalServerError)
			return
		}
		h.logger.Debug("Withdrawals: ", withdrawals)
		// Return the withdrawals
		json.NewEncoder(w).Encode(withdrawals)
		w.WriteHeader(http.StatusOK)
	}
}

func newOrder(orderNumber string, userID int64) *models.Order {
	return &models.Order{
		UserID:     userID,
		Number:     orderNumber,
		Status:     "NEW",
		Accrual:    0,
		UploadedAt: time.Now(),
	}
}

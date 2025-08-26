package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"loyaltySys/internal/auth"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type storage interface {
	CreateUser(user models.User) (int, error)
	GetUser(user models.User) (models.User, error)
	CreateOrder(order *models.Order) error
	GetOrders(userID int64) ([]models.Order, error)
	GetBalance(userID int64) ([]models.Balance, error)
	GetWithdrawals(userID int64) ([]models.Withdrawal, error)
	WithdrawBalance(withdrawal *models.Withdrawal) error
}

type Handler struct {
	storage storage
	logger  *zap.SugaredLogger
}

func NewHandler(s storage, logger *zap.SugaredLogger) *Handler {
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
		h.logger.Debug("request body", r.Body)

		// Decode the request body into a User struct
		h.logger.Debug("Decoding user")
		user := models.User{}
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.logger.Error("failed to decode user", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Validate the user
		if ok, err := auth.ValidateUser(user); !ok {
			h.logger.Error("invalid user", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Hash the password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			h.logger.Error("failed to hash password", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		user.Password = string(hashedPassword)

		// Create the user in the database
		userID, err := h.storage.CreateUser(user)
		if err != nil {
			if errors.Is(err, errors.New("already exists")) {
				h.logger.Error("User already exists", err)
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			h.logger.Error("failed to create user", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Generate a token for the user
		claims := map[string]interface{}{
			"user_id": userID,
			"exp":     time.Now().Add(time.Hour).Unix(),
		}
		h.logger.Debug("Generating token for user", userID)
		_, token, err := auth.TokenAuth.Encode(claims)
		if err != nil {
			h.logger.Error("failed to encode token", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		h.logger.Debug("request body", r.Body)

		// Decode the request body into a User struct
		h.logger.Debug("Decoding user")
		user := models.User{}
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			h.logger.Error("failed to decode user", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Validate the user
		if ok, err := auth.ValidateUser(user); !ok {
			h.logger.Error("invalid user", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Search the user in the database and compare the password
		h.logger.Debug("Searching user in the database")
		registeredUser, err := h.storage.GetUser(user)
		if err != nil {
			h.logger.Error("failed to get user", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Compare the password
		h.logger.Debug("Comparing password")
		if err := bcrypt.CompareHashAndPassword([]byte(registeredUser.Password), []byte(user.Password)); err != nil {
			h.logger.Error("invalid password", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		// Generate a token for the user
		h.logger.Debug("Generating token for user", registeredUser.ID)
		claims := map[string]interface{}{
			"user_id": registeredUser.ID,
			"exp":     time.Now().Add(time.Hour).Unix(),
		}
		_, token, err := auth.TokenAuth.Encode(claims)
		if err != nil {
			h.logger.Error("failed to encode token", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		h.logger.Debug("request body", r.Body)

		// Check if the order number is valid
		orderNumber, err := io.ReadAll(r.Body)
		if err != nil {
			h.logger.Error("failed to read order number", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Check if the order number is valid
		h.logger.Debug("Order number", string(orderNumber))
		if ok, err := auth.ValidateOrderNumber(string(orderNumber)); !ok {
			h.logger.Error("invalid order number", err)
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
			return
		}
		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID", userID)
		// Create the order in the database
		err = h.storage.CreateOrder(NewOrder(string(orderNumber), userID))
		if err != nil {
			// Check if the order already added by another user - return 409
			if errors.Is(err, errors.New("already added by another user")) {
				h.logger.Error("order already added by another user", err)
				http.Error(w, err.Error(), http.StatusConflict)
				return
				// Check if the order already added by this user - return 200
			} else if errors.Is(err, errors.New("already added by this user")) {
				h.logger.Error("order already added by this user", err)
				http.Error(w, err.Error(), http.StatusOK)
				return
			}
			// Return 500 if the order is not created
			h.logger.Error("failed to create order", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		h.logger.Debug("request body", r.Body)

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID", userID)
		// Get the orders from the database
		orders, err := h.storage.GetOrders(userID)
		if err != nil {
			h.logger.Error("failed to get orders", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
			// Return 204 if no orders found for user - no content
		} else if len(orders) == 0 {
			h.logger.Debug("No orders found for user", userID)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.logger.Debug("Orders found for user", userID)
		// Return the orders
		json.NewEncoder(w).Encode(orders)
		w.WriteHeader(http.StatusOK)
	}
}

// GetBalance returns the balance for a user.
func (h *Handler) GetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Getting balance request")
		h.logger.Debug("request body", r.Body)

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID", userID)
		// Get the balance from the database
		balance, err := h.storage.GetBalance(userID)
		if err != nil {
			h.logger.Error("failed to get balance", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.logger.Debug("Balance", balance)
		// Return the balance
		json.NewEncoder(w).Encode(balance)
		w.WriteHeader(http.StatusOK)
	}
}

// WithdrawBalance withdraws bonus points of user from balance.
func (h *Handler) WithdrawBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debug("Withdrawing balance request")
		h.logger.Debug("request body", r.Body)

		// Get the user ID from the context
		userID, err := auth.GetUserIDFromCtx(r.Context())
		if err != nil {
			h.logger.Error("failed to get user ID", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		h.logger.Debug("User ID", userID)
		// Decode the request body into a Withdrawal struct
		h.logger.Debug("Decoding withdrawal")
		withdrawal := models.Withdrawal{}
		err = json.NewDecoder(r.Body).Decode(&withdrawal)
		if err != nil {
			h.logger.Error("failed to decode withdrawal", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Check if the withdrawal is valid
		if ok, err := auth.ValidateOrderNumber(withdrawal.Order); !ok {
			h.logger.Error("invalid order number", err)
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		// Withdraw the balance
		err = h.storage.WithdrawBalance(withdrawal)
		if err != nil {
			h.logger.Error("failed to withdraw balance", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// GetWithdrawals returns all withdrawals for a user.
func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func NewOrder(orderNumber string, userID int64) *models.Order {
	return &models.Order{
		UserID:     userID,
		Order:      orderNumber,
		Status:     "NEW",
		Accrual:    0,
		UploadedAt: time.Now(),
	}
}

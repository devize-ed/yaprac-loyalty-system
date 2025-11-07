package models

import "time"

// OrderStatus is a type that represents the status of an order
type OrderStatus string

// OrderStatus constants
const (
	StatusNew        OrderStatus = "NEW"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusInvalid    OrderStatus = "INVALID"
	StatusProcessed  OrderStatus = "PROCESSED"
)

type User struct {
	ID        int64     `json:"-"`
	Login     string    `json:"login"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"-"`
}

type Order struct {
	Number     string      `json:"number"`
	UserID     int64       `json:"-"`
	Status     OrderStatus `json:"status"`
	Accrual    float64     `json:"accrual,omitempty"`
	UploadedAt time.Time   `json:"uploaded_at,omitempty"`
}

// NewOrder creates a new order
func NewOrder(orderNumber string, userID int64) *Order {
	return &Order{
		UserID:     userID,
		Number:     orderNumber,
		Status:     "NEW",
		Accrual:    0,
		UploadedAt: time.Now(),
	}
}

type Withdrawal struct {
	Order       string    `json:"order"`
	UserID      int64     `json:"-"`
	Sum         float64   `json:"sum,omitempty"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
}

type Balance struct {
	Current   float64 `json:"current,omitempty"`
	Withdrawn float64 `json:"withdrawn,omitempty"`
}

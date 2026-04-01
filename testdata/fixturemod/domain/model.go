package domain

import "errors"

// Order is the core domain entity.
type Order struct {
	ID    int
	Total float64
}

// ErrNotFound is returned when an order cannot be located.
var ErrNotFound = errors.New("order not found")

// NewOrder creates a validated Order.
func NewOrder(id int, total float64) (Order, error) {
	if id <= 0 {
		return Order{}, errors.New("id must be positive")
	}
	if total < 0 {
		return Order{}, errors.New("total must be non-negative")
	}
	return Order{ID: id, Total: total}, nil
}

package application

import (
	"fmt"

	"example.com/fixturemod/domain"
)

// OrderService handles order-related use cases.
type OrderService struct{}

// PlaceOrder validates input and creates a new order.
func (s *OrderService) PlaceOrder(id int, total float64) (domain.Order, error) {
	order, err := domain.NewOrder(id, total)
	if err != nil {
		return domain.Order{}, fmt.Errorf("placing order: %w", err)
	}
	return order, nil
}

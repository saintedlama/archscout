package infrastructure

import (
	"fmt"

	"example.com/fixturemod/domain"
)

// OrderRepository is an in-memory store for orders.
type OrderRepository struct {
	store map[int]domain.Order
}

// NewOrderRepository constructs an empty repository.
func NewOrderRepository() *OrderRepository {
	return &OrderRepository{store: make(map[int]domain.Order)}
}

// Save stores an order.
func (r *OrderRepository) Save(order domain.Order) {
	r.store[order.ID] = order
}

// Find retrieves an order by ID.
func (r *OrderRepository) Find(id int) (domain.Order, error) {
	order, ok := r.store[id]
	if !ok {
		return domain.Order{}, fmt.Errorf("%w: id %d", domain.ErrNotFound, id)
	}
	return order, nil
}

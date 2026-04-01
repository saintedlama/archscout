package main

import (
	"fmt"

	"example.com/fixturemod/application"
	"example.com/fixturemod/infrastructure"
)

func main() {
	repo := infrastructure.NewOrderRepository()
	svc := &application.OrderService{}

	order, err := svc.PlaceOrder(1, 99.99)
	if err != nil {
		panic(err)
	}

	repo.Save(order)
	fmt.Println("order placed:", order.ID)
}

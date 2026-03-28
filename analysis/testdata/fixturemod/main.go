package main

import "fmt"

type Widget struct {
	Name string
}

var GlobalCounter = 1

func RootErr() error {
	if GlobalCounter > 0 {
		switch GlobalCounter {
		case 1:
			GlobalCounter++
		default:
			GlobalCounter = 0
		}
	}

	return fmt.Errorf("root error")
}

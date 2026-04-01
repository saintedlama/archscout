package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/saintedlama/archscout"
)

type stepTiming struct {
	Message string
	Delta   time.Duration
	Total   time.Duration
}

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	start := time.Now()
	last := start
	var steps []stepTiming

	_, err := archscout.LoadWorkspace(
		context.Background(),
		dir,
		archscout.WithReporter(func(msg string) {
			now := time.Now()
			steps = append(steps, stepTiming{
				Message: msg,
				Delta:   now.Sub(last),
				Total:   now.Sub(start),
			})
			last = now
		}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LoadWorkspace failed: %v\n", err)
		os.Exit(1)
	}

	total := time.Since(start)

	fmt.Printf("Workspace build performance report (%s)\n", dir)
	fmt.Println("------------------------------------------------------------")
	for _, step := range steps {
		fmt.Printf("+%8s  %8s  %s\n", round(step.Delta), round(step.Total), step.Message)
	}
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Total workspace build time: %s\n", round(total))
}

func round(d time.Duration) time.Duration {
	return d.Round(time.Millisecond)
}

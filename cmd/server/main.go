// Package main is the entry point for the weather service application.
// It initializes and starts the service with graceful shutdown support.
package main

import (
	"context"
	"log"

	"github.com/sean-rowe/weather-service/internal/app"
)

// Main initializes and runs the weather service application.
// It creates a new application instance, starts it with context,
// and handles graceful shutdown on termination signals.
func main() {
	ctx := context.Background()

	application, err := app.New()

	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := application.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	defer application.Stop()

	application.WaitForShutdown()
}

package main

import (
	"log"

	"github.com/70548887/sup-platform/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

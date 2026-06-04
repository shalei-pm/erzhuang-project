package main

import (
	"log"
	"net/http"
	"os"

	"github.com/shalei-pm/erzhuang-project/internal/app"
)

func main() {
	addr := getenv("ADDR", "127.0.0.1:18080")

	server := &http.Server{
		Addr:    addr,
		Handler: app.NewHandler(),
	}

	log.Printf("erzhuang-project listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

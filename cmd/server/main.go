package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/app"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	addr := getenv("ADDR", "127.0.0.1:18080")
	handler := app.NewHandler()

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		db, err := openDatabase(databaseURL)
		if err != nil {
			log.Fatalf("database setup failed: %v", err)
		}
		defer db.Close()

		handler = app.NewHandlerWithStore(app.NewPostgresStore(db))
		log.Print("database store enabled: postgres")
	} else {
		log.Print("database store disabled: using memory store")
	}

	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	log.Printf("erzhuang-project listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func openDatabase(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := app.EnsurePostgresSchema(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

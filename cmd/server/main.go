package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/app"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/channelai"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"

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
		assetStore, err := assets.NewStoreFromEnv()
		if err != nil {
			log.Fatalf("asset store setup failed: %v", err)
		}

		storeSpaceRepo := storespace.NewPostgresStore(db)
		storeSpaceService := storespace.NewService(storeSpaceRepo)
		storeSpaceService.UseSnapshotStore(storespace.NewAssetSnapshotStore(assetStore))
		var channelRecognizer storespace.ChannelRecognizer
		if recognizer, enabled, err := channelai.NewRecognizerFromEnv(); err != nil {
			log.Printf("channel ai recognizer disabled: %v", err)
		} else if enabled {
			channelRecognizer = storespace.NewChannelAIAdapter(recognizer)
			log.Print("channel ai recognizer enabled")
		}
		if ezvizAccounts, enabled, err := storespace.EzvizAccountsFromEnv(); err != nil {
			log.Fatalf("ezviz scanner setup failed: %v", err)
		} else if enabled {
			if err := storeSpaceService.SyncEzvizAccountNames(context.Background(), storespace.EzvizAccountNames(ezvizAccounts)); err != nil {
				log.Fatalf("ezviz account sync failed: %v", err)
			}
			scanner := storespace.NewEzvizScannerFromAccounts(ezvizAccounts)
			storeSpaceService = storespace.NewServiceWithScannerAndRecognizer(storeSpaceRepo, scanner, channelRecognizer)
			storeSpaceService.UseSnapshotStore(storespace.NewAssetSnapshotStore(assetStore))
			log.Printf("ezviz scanner enabled, synced %d account(s)", len(ezvizAccounts))
		}
		handler = app.NewHandlerWithServices(app.NewPostgresStore(db), designplan.NewServiceWithAssetStore(designplan.NewPostgresStore(db), assetStore), storeSpaceService)
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

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelPing()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}

	schemaCtx, cancelSchema := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelSchema()

	if err := app.EnsurePostgresSchema(schemaCtx, db); err != nil {
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

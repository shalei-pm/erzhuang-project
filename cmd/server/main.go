package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/app"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/channelai"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
	"github.com/shalei-pm/erzhuang-project/internal/h5monitor"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	addr := getenv("ADDR", "127.0.0.1:18080")
	handler := app.NewHandler()

	if config, err := databaseConfigFromEnv(); err != nil {
		log.Fatalf("database config failed: %v", err)
	} else if config.DSN != "" {
		db, err := openDatabase(config)
		if err != nil {
			log.Fatalf("database setup failed: %v", err)
		}
		defer db.Close()
		assetStore, err := assets.NewStoreFromEnv()
		if err != nil {
			log.Fatalf("asset store setup failed: %v", err)
		}

		var appStore app.Store
		var storeSpaceRepo storespace.Repository
		var h5RepositoryFactory func([]ezviz.Account) h5monitor.StoreRepository
		var designPlanService *designplan.Service
		if config.Driver == "mysql" {
			mysqlAppStore := app.NewMySQLStore(db)
			mysqlStoreSpaceRepo := storespace.NewMySQLStore(db)
			appStore = mysqlAppStore
			storeSpaceRepo = mysqlStoreSpaceRepo
			h5RepositoryFactory = func(accounts []ezviz.Account) h5monitor.StoreRepository {
				return storespace.NewMySQLH5MonitorRepository(mysqlStoreSpaceRepo, accounts)
			}
			designPlanService = designplan.NewServiceWithAssetStore(designplan.NewMemoryStore(), assetStore)
		} else {
			postgresAppStore := app.NewPostgresStore(db)
			postgresStoreSpaceRepo := storespace.NewPostgresStore(db)
			appStore = postgresAppStore
			storeSpaceRepo = postgresStoreSpaceRepo
			h5RepositoryFactory = func(accounts []ezviz.Account) h5monitor.StoreRepository {
				return storespace.NewH5MonitorRepository(postgresStoreSpaceRepo, accounts)
			}
			designPlanService = designplan.NewServiceWithAssetStoreAndAIProvider(designplan.NewPostgresStore(db), assetStore, postgresAppStore)
		}
		storeSpaceService := storespace.NewService(storeSpaceRepo)
		storeSpaceService.UseSnapshotStore(storespace.NewAssetSnapshotStore(assetStore))
		var h5MonitorService *h5monitor.Service
		var channelRecognizer storespace.ChannelRecognizer
		if _, enabled, err := channelai.NewRecognizerFromEnv(); err != nil {
			log.Printf("channel ai recognizer disabled: %v", err)
		} else if enabled {
			channelRecognizer = storespace.NewChannelAIAdapter(channelai.NewDynamicRecognizer(appStore))
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
			h5MonitorService = h5monitor.NewService(h5RepositoryFactory(ezvizAccounts), ezviz.NewClient(ezviz.ClientOptions{}))
			log.Printf("ezviz scanner enabled, synced %d account(s)", len(ezvizAccounts))
		}
		handler = app.NewHandlerWithServicesAndH5Monitor(appStore, designPlanService, storeSpaceService, h5MonitorService)
		log.Printf("database store enabled: %s", config.Driver)
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

type databaseConfig struct {
	Driver string
	DSN    string
}

func databaseConfigFromEnv() (databaseConfig, error) {
	driver := strings.ToLower(envValue("APP_DB_DRIVER", "K8S_SECRET_APP_DB_DRIVER"))
	if driver == "" {
		if envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN") != "" {
			driver = "mysql"
		} else if strings.TrimSpace(os.Getenv("DATABASE_URL")) != "" {
			driver = "postgres"
		}
	}

	switch driver {
	case "":
		return databaseConfig{}, nil
	case "postgres", "postgresql":
		dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
		if dsn == "" {
			return databaseConfig{}, errors.New("DATABASE_URL is required when APP_DB_DRIVER=postgres")
		}
		return databaseConfig{Driver: "postgres", DSN: dsn}, nil
	case "mysql":
		dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
		if dsn == "" {
			return databaseConfig{}, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required when APP_DB_DRIVER=mysql")
		}
		return databaseConfig{Driver: "mysql", DSN: mysqlDSNWithParseTime(dsn)}, nil
	default:
		return databaseConfig{}, errors.New("APP_DB_DRIVER must be postgres or mysql")
	}
}

func mysqlDSNWithParseTime(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "parseTime=true"
}

func openDatabase(config databaseConfig) (*sql.DB, error) {
	sqlDriver := "pgx"
	if config.Driver == "mysql" {
		sqlDriver = "mysql"
	}
	db, err := sql.Open(sqlDriver, config.DSN)
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

	if config.Driver == "mysql" {
		return db, nil
	}

	schemaCtx, cancelSchema := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancelSchema()

	if err := app.EnsurePostgresSchema(schemaCtx, db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func envValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

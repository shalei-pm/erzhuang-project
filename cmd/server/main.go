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
	"github.com/shalei-pm/erzhuang-project/internal/nvrlab"
	"github.com/shalei-pm/erzhuang-project/internal/nvrmonitor"
	"github.com/shalei-pm/erzhuang-project/internal/resourceview"
	"github.com/shalei-pm/erzhuang-project/internal/storespace"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	addr := getenv("ADDR", "127.0.0.1:18080")
	handler := app.NewHandler()
	var resourceViewService *resourceview.Service
	var nvrLabService *nvrlab.Service
	var nvrMonitorService *nvrmonitor.Service
	monitorPlaybackMode := app.MonitorPlaybackModeFromEnv()

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
		mysqlAppStore := app.NewMySQLStore(db)
		mysqlStoreSpaceRepo := storespace.NewMySQLStore(db)
		resourceViewRepository := resourceview.NewMySQLRepository(db)
		resourceViewService = resourceview.NewService(resourceViewRepository)
		if authorization := nvrLabAuthorizationFromEnv(); authorization != "" {
			nvrLabService = nvrlab.NewService(resourceViewRepository, nvrlab.NewHTTPAuthorizationClient(nil, authorization))
			nvrMonitorService = nvrmonitor.NewService(resourceViewRepository, nvrmonitor.NewHTTPAuthorizationClient(nil, authorization))
			log.Print("nvr stream authorization enabled")
		} else if monitorPlaybackMode == app.MonitorPlaybackModeNVR {
			monitorPlaybackMode = app.MonitorPlaybackModeLegacy
			log.Print("nvr monitor requested without authorization secret; using legacy monitor")
		}
		appStore = mysqlAppStore
		storeSpaceRepo = mysqlStoreSpaceRepo
		h5RepositoryFactory = func(accounts []ezviz.Account) h5monitor.StoreRepository {
			return storespace.NewMySQLH5MonitorRepository(mysqlStoreSpaceRepo, accounts)
		}
		designPlanService = designplan.NewServiceWithAssetStore(designplan.NewMemoryStore(), assetStore)
		storeSpaceService := storespace.NewService(storeSpaceRepo)
		storeSpaceService.UseSnapshotStore(storespace.NewAssetSnapshotStore(assetStore))
		var h5MonitorService *h5monitor.Service
		channelRecognizer := newDynamicChannelRecognizer(appStore)
		log.Print("channel ai recognizer enabled with runtime provider settings")
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
		handler = app.NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVR(appStore, designPlanService, storeSpaceService, h5MonitorService, resourceViewService, nvrLabService, nvrMonitorService, monitorPlaybackMode)
		log.Printf("database store and resource view enabled: %s", config.Driver)
	} else {
		handler = app.NewHandlerWithServicesAndH5MonitorAndResourceViewAndNVR(app.NewMemoryStore(), designplan.NewService(designplan.NewMemoryStore()), storespace.NewService(storespace.NewMemoryStore()), nil, resourceViewService, nvrLabService, nvrMonitorService, monitorPlaybackMode)
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
		}
	}

	switch driver {
	case "":
		return databaseConfig{}, nil
	case "mysql":
		dsn := envValue("MYSQL_DSN", "K8S_SECRET_MYSQL_DSN")
		if dsn == "" {
			return databaseConfig{}, errors.New("MYSQL_DSN or K8S_SECRET_MYSQL_DSN is required when APP_DB_DRIVER=mysql")
		}
		return databaseConfig{Driver: "mysql", DSN: mysqlDSNWithParseTime(dsn)}, nil
	default:
		return databaseConfig{}, errors.New("APP_DB_DRIVER must be mysql")
	}
}

func mysqlDSNWithParseTime(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || strings.Contains(dsn, "parseTime=") {
		return dsn
	}
	separator := "?"
	if queryStart := strings.LastIndex(dsn, "?"); queryStart > strings.LastIndex(dsn, "/") {
		separator = "&"
	}
	return dsn + separator + "parseTime=true"
}

func openDatabase(config databaseConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", config.DSN)
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

func nvrLabAuthorizationFromEnv() string {
	return envValue("K8S_SECRET_NVR_STREAM_AUTHORIZATION", "NVR_STREAM_AUTHORIZATION")
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func newDynamicChannelRecognizer(providerReader channelai.ProviderReader) storespace.ChannelRecognizer {
	return storespace.NewChannelAIAdapter(channelai.NewDynamicRecognizer(providerReader))
}

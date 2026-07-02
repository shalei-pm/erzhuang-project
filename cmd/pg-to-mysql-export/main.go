package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/mysqlmigration"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	var (
		sourceDSNEnv  string
		outDir        string
		externalOrgID string
		includeUsers  bool
		batchID       string
		timeout       time.Duration
	)
	flag.StringVar(&sourceDSNEnv, "source-dsn-env", "DATABASE_URL", "environment variable containing the PostgreSQL source DSN")
	flag.StringVar(&outDir, "out-dir", "migration-out", "directory for generated SQL and report files")
	flag.StringVar(&externalOrgID, "external-org-id", "", "optional comma-separated external_org_id scope for a canary/sample export")
	flag.BoolVar(&includeUsers, "include-users", true, "export tb_users and user role mapping")
	flag.StringVar(&batchID, "batch-id", "", "optional human-readable migration batch id for SQL comments")
	flag.DurationVar(&timeout, "timeout", 2*time.Minute, "export timeout")
	flag.Parse()

	dsn := strings.TrimSpace(os.Getenv(sourceDSNEnv))
	if dsn == "" {
		log.Fatalf("%s is required", sourceDSNEnv)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}

	export, err := mysqlmigration.ExportFromPostgres(ctx, db, mysqlmigration.Options{
		ExternalOrgIDs: strings.Split(externalOrgID, ","),
		IncludeUsers:   includeUsers,
		BatchID:        batchID,
	})
	if err != nil {
		log.Fatalf("export: %v", err)
	}

	files := map[string]string{
		"01-import.sql":         export.ImportSQL,
		"02-auto-increment.sql": export.AutoIncrementSQL,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(content), 0o644); err != nil {
			log.Fatalf("write %s: %v", name, err)
		}
	}
	reportPath := filepath.Join(outDir, "report.json")
	reportFile, err := os.Create(reportPath)
	if err != nil {
		log.Fatalf("create report: %v", err)
	}
	if err := mysqlmigration.WriteReportJSON(reportFile, export.Report); err != nil {
		_ = reportFile.Close()
		log.Fatalf("write report: %v", err)
	}
	if err := reportFile.Close(); err != nil {
		log.Fatalf("close report: %v", err)
	}

	fmt.Printf("generated %s\n", filepath.Join(outDir, "01-import.sql"))
	fmt.Printf("generated %s\n", filepath.Join(outDir, "02-auto-increment.sql"))
	fmt.Printf("generated %s\n", reportPath)
}

package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assetmigration"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

func main() {
	manifestPath := flag.String("manifest", "", "CSV manifest exported from db/oss_asset_inventory_sql_tb.sql")
	apply := flag.Bool("apply", false, "actually copy objects from source store to target OSS store")
	externalOrgID := flag.String("external-org-id", "", "optional store external_org_id filter, for example 10030")
	maxRows := flag.Int("max-rows", 0, "optional maximum number of copy-eligible rows to process")
	resultSQLPath := flag.String("result-sql", "", "optional SQL file to write migration status updates for copied rows")
	ossBucket := flag.String("oss-bucket", "", "OSS bucket name to use in generated result SQL; defaults to TARGET_OSS_BUCKET")
	batchID := flag.String("batch-id", "", "optional migration batch id for generated result SQL")
	timeout := flag.Duration("timeout", 5*time.Minute, "migration command timeout")
	flag.Parse()

	if strings.TrimSpace(*manifestPath) == "" {
		log.Fatal("missing --manifest")
	}
	file, err := os.Open(*manifestPath)
	if err != nil {
		log.Fatalf("open manifest: %v", err)
	}
	defer file.Close()
	rows, err := assetmigration.ReadManifest(file)
	if err != nil {
		log.Fatalf("read manifest: %v", err)
	}

	source, err := sourceStoreFromEnv()
	if err != nil {
		log.Fatalf("create source store: %v", err)
	}
	target, err := targetStoreFromEnv()
	if err != nil {
		log.Fatalf("create target store: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	summary, results := assetmigration.CopyManifest(ctx, source, target, rows, assetmigration.Options{
		Apply:         *apply,
		ExternalOrgID: *externalOrgID,
		MaxRows:       *maxRows,
	})
	if err := writeResults(os.Stdout, results); err != nil {
		log.Fatalf("write results: %v", err)
	}
	fmt.Fprintf(os.Stderr, "asset migration summary: total=%d would_copy=%d copied=%d skipped=%d errors=%d\n", summary.Total, summary.WouldCopy, summary.Copied, summary.Skipped, summary.Errors)
	if summary.Errors > 0 {
		os.Exit(1)
	}
	if strings.TrimSpace(*resultSQLPath) != "" {
		bucket := strings.TrimSpace(*ossBucket)
		if bucket == "" {
			bucket = strings.TrimSpace(os.Getenv("TARGET_OSS_BUCKET"))
		}
		if bucket == "" {
			log.Fatal("--result-sql requires --oss-bucket or TARGET_OSS_BUCKET")
		}
		if err := writeResultSQLFile(*resultSQLPath, results, bucket, *batchID); err != nil {
			log.Fatalf("write result sql: %v", err)
		}
		fmt.Fprintf(os.Stderr, "wrote result SQL to %s\n", *resultSQLPath)
	}
}

func sourceStoreFromEnv() (assets.Store, error) {
	return storeFromPrefixedEnv("SOURCE", "local")
}

func targetStoreFromEnv() (assets.Store, error) {
	return storeFromPrefixedEnv("TARGET", "oss")
}

func storeFromPrefixedEnv(prefix string, defaultMode string) (assets.Store, error) {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv(prefix + "_ASSET_STORE")))
	if mode == "" {
		mode = defaultMode
	}
	switch mode {
	case "local":
		root := strings.TrimSpace(os.Getenv(prefix + "_UPLOAD_DIR"))
		if root == "" {
			root = strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
		}
		if root == "" {
			root = "uploads"
		}
		return assets.NewLocalStore(root), nil
	case "supabase":
		baseURL := strings.TrimSpace(os.Getenv(prefix + "_SUPABASE_URL"))
		serviceKey := strings.TrimSpace(os.Getenv(prefix + "_SUPABASE_SERVICE_ROLE_KEY"))
		bucket := strings.TrimSpace(os.Getenv(prefix + "_SUPABASE_STORAGE_BUCKET"))
		if bucket == "" {
			bucket = "design-plan-assets"
		}
		if baseURL == "" || serviceKey == "" || bucket == "" {
			return nil, fmt.Errorf("%s_ASSET_STORE=supabase requires %s_SUPABASE_URL, %s_SUPABASE_SERVICE_ROLE_KEY, and %s_SUPABASE_STORAGE_BUCKET", prefix, prefix, prefix, prefix)
		}
		return assets.NewSupabaseStorageStore(assets.SupabaseStorageConfig{
			BaseURL:    baseURL,
			ServiceKey: serviceKey,
			Bucket:     bucket,
		}), nil
	case "oss":
		bucket := strings.TrimSpace(os.Getenv(prefix + "_OSS_BUCKET"))
		endpoint := strings.TrimSpace(os.Getenv(prefix + "_OSS_ENDPOINT"))
		accessKeyID := strings.TrimSpace(os.Getenv(prefix + "_OSS_ACCESS_KEY_ID"))
		accessKeySecret := strings.TrimSpace(os.Getenv(prefix + "_OSS_ACCESS_KEY_SECRET"))
		if bucket == "" || endpoint == "" || accessKeyID == "" || accessKeySecret == "" {
			return nil, fmt.Errorf("%s_ASSET_STORE=oss requires %s_OSS_BUCKET, %s_OSS_ENDPOINT, %s_OSS_ACCESS_KEY_ID, and %s_OSS_ACCESS_KEY_SECRET", prefix, prefix, prefix, prefix, prefix)
		}
		return assets.NewOSSStore(assets.OSSConfig{
			Bucket:          bucket,
			Endpoint:        endpoint,
			AccessKeyID:     accessKeyID,
			AccessKeySecret: accessKeySecret,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported %s_ASSET_STORE %q", prefix, mode)
	}
}

func writeResults(writer io.Writer, results []assetmigration.RowResult) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()
	if err := csvWriter.Write([]string{"action", "external_org_id", "logical_key", "target_oss_key", "bytes", "content_type", "error"}); err != nil {
		return err
	}
	for _, result := range results {
		if err := csvWriter.Write([]string{
			result.Action,
			result.Row.ExternalOrgID,
			result.Row.LogicalKey,
			result.Row.TargetOSSKey,
			fmt.Sprintf("%d", result.Bytes),
			result.ContentType,
			result.Error,
		}); err != nil {
			return err
		}
	}
	return csvWriter.Error()
}

func writeResultSQLFile(path string, results []assetmigration.RowResult, bucket string, batchID string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return assetmigration.WriteResultSQL(file, results, assetmigration.SQLUpdateOptions{
		Bucket:  bucket,
		BatchID: batchID,
	})
}

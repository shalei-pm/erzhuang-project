package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/nvrlab"
	"github.com/shalei-pm/erzhuang-project/internal/nvrsnapshot"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	flags := flag.NewFlagSet("nvr-snapshot-backfill", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	tenantID := flags.Int64("tenant-id", 0, "")
	cameraID := flags.Int64("camera-id", 0, "")
	allTenants := flags.Bool("all-tenants", false, "")
	force := flags.Bool("force", false, "")
	timeout := flags.Duration("timeout-per-camera", 20*time.Second, "")
	interval := flags.Duration("request-interval", 2*time.Second, "")
	if flags.Parse(args) != nil {
		return 2
	}
	validSelection := (*allTenants && *tenantID == 0 && *cameraID == 0) || (!*allTenants && *tenantID > 0 && *cameraID >= 0)
	if flags.NArg() != 0 || !validSelection || *timeout <= 0 || *timeout > 20*time.Second || *interval < 2*time.Second {
		return 2
	}
	dsn := envValue("K8S_SECRET_MYSQL_DSN", "MYSQL_DSN")
	authorization := envValue("K8S_SECRET_NVR_STREAM_AUTHORIZATION", "NVR_STREAM_AUTHORIZATION")
	if dsn == "" || authorization == "" {
		fmt.Fprintln(os.Stdout, "error_code=configuration_failed")
		return 1
	}
	objects, err := assets.NewStoreFromEnv()
	if err != nil {
		fmt.Fprintln(os.Stdout, "error_code=configuration_failed")
		return 1
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stdout, "error_code=database_connect_failed")
		return 1
	}
	defer db.Close()
	// A full, deliberately throttled run may exceed two hours. Each camera
	// remains individually bounded; SIGTERM stops the Job cleanly between work.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	authorizer := spikeAuthorizer{client: nvrlab.NewHTTPAuthorizationClient(&http.Client{Timeout: 20 * time.Second}, authorization)}
	capture := nvrsnapshot.NewWebSocketJPEGCapture(nvrsnapshot.NhooyrWebSocketDialer{}, nvrsnapshot.ExecCommandFactory{})
	repo := nvrsnapshot.NewMySQLRepository(db)
	selection := nvrsnapshot.Selection{TenantID: *tenantID, CameraID: *cameraID, AllTenants: *allTenants}
	summary, err := nvrsnapshot.NewBackfillService(repo, nvrsnapshot.NewCaptureService(authorizer, capture), objects).Run(ctx, nvrsnapshot.BackfillOptions{Selection: selection, Timeout: *timeout, RequestInterval: *interval, Force: *force})
	if err != nil {
		fmt.Fprintf(os.Stdout, "selected=%d skipped=%d succeeded=%d failed=%d error_code=backfill_failed\n", summary.Selected, summary.Skipped, summary.Succeeded, summary.Failed)
		return 1
	}
	fmt.Fprintf(os.Stdout, "selected=%d skipped=%d succeeded=%d failed=%d\n", summary.Selected, summary.Skipped, summary.Succeeded, summary.Failed)
	return 0
}

type spikeAuthorizer struct {
	client interface {
		CreateStreamURL(context.Context, int64, nvrlab.StreamSessionRequest) (string, error)
	}
}

func (a spikeAuthorizer) AuthorizeStream(ctx context.Context, cameraID int64, request nvrsnapshot.StreamRequest) (string, nvrsnapshot.ErrorCode) {
	if a.client == nil || request.Mode != nvrsnapshot.StreamModeLive {
		return "", nvrsnapshot.ErrorAuthorizationFailed
	}
	url, err := a.client.CreateStreamURL(ctx, cameraID, nvrlab.StreamSessionRequest{Mode: nvrlab.ModeLive})
	if err != nil || strings.TrimSpace(url) == "" {
		return "", nvrsnapshot.ErrorAuthorizationFailed
	}
	return url, ""
}
func envValue(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

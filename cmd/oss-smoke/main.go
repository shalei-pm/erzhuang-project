package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
	"github.com/shalei-pm/erzhuang-project/internal/osssmoke"
)

func main() {
	apply := flag.Bool("apply", false, "actually write, read, and delete one smoke object")
	key := flag.String("key", "", "optional smoke object key; defaults to smoke-tests/<timestamp>-<random>.txt")
	timeout := flag.Duration("timeout", 30*time.Second, "smoke check timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	store, err := assets.NewStoreFromEnv()
	if err != nil {
		log.Fatalf("create asset store: %v", err)
	}
	result, err := osssmoke.Run(ctx, store, osssmoke.Options{Apply: *apply, Key: *key})
	if err != nil {
		log.Fatalf("oss smoke failed: %v", err)
	}
	if result.DryRun {
		fmt.Fprintf(os.Stdout, "OSS smoke dry-run ok; key=%s bytes=%d content_type=%s\n", result.Key, result.Bytes, result.ContentType)
		fmt.Fprintln(os.Stdout, "Re-run with --apply to write, read, and delete this smoke object.")
		return
	}
	fmt.Fprintf(os.Stdout, "OSS smoke apply ok; key=%s bytes=%d content_type=%s\n", result.Key, result.Bytes, result.ContentType)
}

package osssmoke

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

type Options struct {
	Apply bool
	Key   string
	Now   func() time.Time
}

type Result struct {
	DryRun      bool
	Key         string
	ContentType string
	Bytes       int
}

func Run(ctx context.Context, store assets.Store, options Options) (*Result, error) {
	if store == nil {
		return nil, errors.New("asset store is required")
	}
	key := strings.TrimSpace(options.Key)
	if key == "" {
		generated, err := defaultKey(options.Now)
		if err != nil {
			return nil, err
		}
		key = generated
	}
	payload := []byte("erzhuang oss smoke " + time.Now().UTC().Format(time.RFC3339Nano))
	result := &Result{DryRun: !options.Apply, Key: key, ContentType: "text/plain; charset=utf-8", Bytes: len(payload)}
	if !options.Apply {
		return result, nil
	}
	if err := store.Save(ctx, key, bytes.NewReader(payload), result.ContentType); err != nil {
		return nil, fmt.Errorf("save smoke asset: %w", err)
	}
	reader, contentType, err := store.Open(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("open smoke asset: %w", err)
	}
	body, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read smoke asset: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close smoke asset: %w", closeErr)
	}
	if string(body) != string(payload) {
		return nil, errors.New("smoke asset content mismatch")
	}
	result.ContentType = contentType
	if err := deleteSmokeKey(ctx, store, key); err != nil {
		return nil, fmt.Errorf("delete smoke asset: %w", err)
	}
	return result, nil
}

type directDeleteStore interface {
	Delete(ctx context.Context, key string) error
}

func deleteSmokeKey(ctx context.Context, store assets.Store, key string) error {
	if directStore, ok := store.(directDeleteStore); ok {
		return directStore.Delete(ctx, key)
	}
	return store.DeletePrefix(ctx, key)
}

func defaultKey(now func() time.Time) (string, error) {
	if now == nil {
		now = time.Now
	}
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "smoke-tests/" + now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random[:]) + ".txt", nil
}

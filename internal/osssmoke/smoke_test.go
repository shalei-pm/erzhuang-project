package osssmoke

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/assets"
)

func TestRunDryRunDoesNotTouchStore(t *testing.T) {
	store := &fakeStore{}

	result, err := Run(context.Background(), store, Options{Key: "smoke-tests/test.txt"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.DryRun || result.Key != "smoke-tests/test.txt" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if store.calls != "" {
		t.Fatalf("dry-run touched store: %s", store.calls)
	}
}

func TestRunApplyWritesReadsAndDeletesExactKey(t *testing.T) {
	store := &fakeStore{}

	result, err := Run(context.Background(), store, Options{Apply: true, Key: "smoke-tests/test.txt"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.DryRun || result.Key != "smoke-tests/test.txt" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if store.calls != "save:smoke-tests/test.txt;open:smoke-tests/test.txt;delete:smoke-tests/test.txt;" {
		t.Fatalf("unexpected calls: %s", store.calls)
	}
}

type fakeStore struct {
	calls       string
	body        string
	contentType string
}

func (s *fakeStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.calls += "save:" + key + ";"
	s.body = string(data)
	s.contentType = contentType
	return nil
}

func (s *fakeStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if s.body == "" {
		return nil, "", assets.ErrNotFound
	}
	s.calls += "open:" + key + ";"
	return io.NopCloser(strings.NewReader(s.body)), s.contentType, nil
}

func (s *fakeStore) DeletePrefix(ctx context.Context, prefix string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.calls += "delete:" + prefix + ";"
	return nil
}

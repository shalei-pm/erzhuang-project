package assetmigration

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestReadManifestRequiresCoreColumns(t *testing.T) {
	_, err := ReadManifest(strings.NewReader("logical_key,target_oss_key\nuploads/a.pdf,uploads/a.pdf\n"))
	if err == nil || !strings.Contains(err.Error(), "suggested_migration_status") {
		t.Fatalf("expected missing status column error, got %v", err)
	}
}

func TestCopyManifestDryRunFiltersSampleAndDedupes(t *testing.T) {
	rows, err := ReadManifest(strings.NewReader(strings.Join([]string{
		"external_org_id,logical_key,target_oss_key,suggested_migration_status,skip_reason,logical_key_rank",
		"10030,uploads/a.pdf,uploads/a.pdf,pending,,1",
		"10030,uploads/a.pdf,uploads/a.pdf,pending,,2",
		"10047,uploads/b.pdf,uploads/b.pdf,pending,,1",
		"10030,,uploads/c.pdf,skipped,remote_http_url,1",
	}, "\n")))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	summary, results := CopyManifest(context.Background(), nil, nil, rows, Options{ExternalOrgID: "10030"})
	if summary.Total != 4 || summary.WouldCopy != 1 || summary.Skipped != 3 || summary.Copied != 0 || summary.Errors != 0 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	if results[0].Action != "would_copy" || results[1].Error != "duplicate_logical_key" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestCopyManifestApplyCopiesSourceToTarget(t *testing.T) {
	rows := []ManifestRow{{
		LogicalKey:               "uploads/a.pdf",
		TargetOSSKey:             "uploads/a.pdf",
		ExpectedContentType:      "application/pdf",
		SuggestedMigrationStatus: "pending",
		LogicalKeyRank:           1,
	}}
	source := &memoryStore{objects: map[string]string{"uploads/a.pdf": "pdf-body"}, contentTypes: map[string]string{"uploads/a.pdf": "application/pdf"}}
	target := &memoryStore{objects: map[string]string{}, contentTypes: map[string]string{}}

	summary, results := CopyManifest(context.Background(), source, target, rows, Options{Apply: true})
	if summary.Copied != 1 || summary.Errors != 0 {
		t.Fatalf("unexpected summary: %#v results=%#v", summary, results)
	}
	if target.objects["uploads/a.pdf"] != "pdf-body" || target.contentTypes["uploads/a.pdf"] != "application/pdf" {
		t.Fatalf("target not copied: %#v %#v", target.objects, target.contentTypes)
	}
}

type memoryStore struct {
	objects      map[string]string
	contentTypes map[string]string
}

func (s *memoryStore) Save(ctx context.Context, key string, body io.Reader, contentType string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.objects[key] = string(data)
	s.contentTypes[key] = contentType
	return nil
}

func (s *memoryStore) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	return io.NopCloser(strings.NewReader(s.objects[key])), s.contentTypes[key], nil
}

func (s *memoryStore) DeletePrefix(ctx context.Context, prefix string) error {
	return nil
}

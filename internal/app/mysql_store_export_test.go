package app

import (
	"context"
	"strings"
	"testing"

	"github.com/shalei-pm/erzhuang-project/internal/mysqlmigration"
)

func TestMySQLStoreSupportsPostgresMigrationExport(t *testing.T) {
	var _ interface {
		ExportMySQLMigration(context.Context, mysqlmigration.Options) (*mysqlmigration.Export, error)
	} = (*MySQLStore)(nil)
}

func TestMySQLStoreExportRequiresPostgresDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := NewMySQLStore(nil).ExportMySQLMigration(context.Background(), mysqlmigration.Options{})
	if err == nil {
		t.Fatal("ExportMySQLMigration returned nil error, want missing DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("ExportMySQLMigration error = %q, want DATABASE_URL message", err.Error())
	}
}

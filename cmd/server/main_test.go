package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDatabaseConfigFromEnvSelectsMySQL(t *testing.T) {
	t.Setenv("APP_DB_DRIVER", "mysql")
	t.Setenv("MYSQL_DSN", "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang")

	config, err := databaseConfigFromEnv()
	if err != nil {
		t.Fatalf("database config: %v", err)
	}

	if config.Driver != "mysql" {
		t.Fatalf("driver = %q, want mysql", config.Driver)
	}
	if config.DSN != "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang?parseTime=true" {
		t.Fatalf("dsn = %q, want MYSQL_DSN with parseTime", config.DSN)
	}
}

func TestDatabaseConfigFromEnvDefaultsToMySQLWhenBothDSNsExist(t *testing.T) {
	t.Setenv("DATABASE_URL", "legacy-database-url")
	t.Setenv("MYSQL_DSN", "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang")

	config, err := databaseConfigFromEnv()
	if err != nil {
		t.Fatalf("database config: %v", err)
	}

	if config.Driver != "mysql" {
		t.Fatalf("driver = %q, want mysql", config.Driver)
	}
	if config.DSN != "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang?parseTime=true" {
		t.Fatalf("dsn = %q, want MYSQL_DSN with parseTime", config.DSN)
	}
}

func TestDatabaseConfigFromEnvReadsMySQLFromK8SSecret(t *testing.T) {
	t.Setenv("K8S_SECRET_MYSQL_DSN", "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang")

	config, err := databaseConfigFromEnv()
	if err != nil {
		t.Fatalf("database config: %v", err)
	}

	if config.Driver != "mysql" {
		t.Fatalf("driver = %q, want mysql", config.Driver)
	}
	if config.DSN != "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang?parseTime=true" {
		t.Fatalf("dsn = %q, want K8S_SECRET_MYSQL_DSN with parseTime", config.DSN)
	}
}

func TestDatabaseConfigFromEnvIgnoresDatabaseURLWithoutMySQLDSN(t *testing.T) {
	t.Setenv("DATABASE_URL", "legacy-database-url")

	config, err := databaseConfigFromEnv()
	if err != nil {
		t.Fatalf("database config: %v", err)
	}

	if config.Driver != "" || config.DSN != "" {
		t.Fatalf("config = %#v, want empty config when only DATABASE_URL is set", config)
	}
}

func TestDatabaseConfigFromEnvRejectsLegacyDriver(t *testing.T) {
	t.Setenv("APP_DB_DRIVER", "legacy")
	t.Setenv("DATABASE_URL", "legacy-database-url")
	t.Setenv("MYSQL_DSN", "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang")

	if _, err := databaseConfigFromEnv(); err == nil {
		t.Fatal("database config returned nil error for legacy APP_DB_DRIVER, want unsupported driver error")
	}
}

func TestMySQLDSNWithParseTimePreservesExistingQuery(t *testing.T) {
	got := mysqlDSNWithParseTime("u:p@tcp(mysql:3306)/erzhuang?charset=utf8mb4")
	want := "u:p@tcp(mysql:3306)/erzhuang?charset=utf8mb4&parseTime=true"
	if got != want {
		t.Fatalf("dsn = %q, want %q", got, want)
	}
}

func TestMySQLDSNWithParseTimeAllowsQuestionMarkInPassword(t *testing.T) {
	got := mysqlDSNWithParseTime("u:p?with@mark@tcp(mysql:3306)/erzhuang")
	want := "u:p?with@mark@tcp(mysql:3306)/erzhuang?parseTime=true"
	if got != want {
		t.Fatalf("dsn = %q, want %q", got, want)
	}
}

func TestResourceViewUsesPrimaryMySQLOnly(t *testing.T) {
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, banned := range []string{
		"BUSINESS_MYSQL_DSN",
		"K8S_SECRET_BUSINESS_MYSQL_DSN",
		"businessDatabaseConfigFromEnv",
	} {
		if strings.Contains(source, banned) {
			t.Fatalf("main.go still contains %q", banned)
		}
	}
	if !strings.Contains(source, "resourceview.NewMySQLRepository(db)") {
		t.Fatal("main.go does not build resource view from the primary mysql database")
	}
}

func TestNewDynamicChannelRecognizerAlwaysReturnsRuntimeRecognizer(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("VISION_API_KEY", "")
	t.Setenv("CHANNEL_AI_PROVIDER", "")

	recognizer := newDynamicChannelRecognizer(fakeProviderReader{provider: "minimax"})
	if recognizer == nil {
		t.Fatal("recognizer is nil, want dynamic recognizer")
	}
}

type fakeProviderReader struct {
	provider string
}

func (r fakeProviderReader) GetAIProvider(ctx context.Context) (string, error) {
	return r.provider, nil
}

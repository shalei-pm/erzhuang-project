package main

import "testing"

func TestDatabaseConfigFromEnvSelectsMySQL(t *testing.T) {
	t.Setenv("APP_DB_DRIVER", "mysql")
	t.Setenv("DATABASE_URL", "postgres://example")
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
	t.Setenv("DATABASE_URL", "postgres://example")
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
	t.Setenv("DATABASE_URL", "postgres://example")
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

func TestDatabaseConfigFromEnvExplicitPostgresKeepsPostgres(t *testing.T) {
	t.Setenv("APP_DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("MYSQL_DSN", "mysql-user:mysql-pass@tcp(mysql:3306)/erzhuang")

	config, err := databaseConfigFromEnv()
	if err != nil {
		t.Fatalf("database config: %v", err)
	}

	if config.Driver != "postgres" {
		t.Fatalf("driver = %q, want postgres", config.Driver)
	}
	if config.DSN != "postgres://example" {
		t.Fatalf("dsn = %q, want DATABASE_URL", config.DSN)
	}
}

func TestMySQLDSNWithParseTimePreservesExistingQuery(t *testing.T) {
	got := mysqlDSNWithParseTime("u:p@tcp(mysql:3306)/erzhuang?charset=utf8mb4")
	want := "u:p@tcp(mysql:3306)/erzhuang?charset=utf8mb4&parseTime=true"
	if got != want {
		t.Fatalf("dsn = %q, want %q", got, want)
	}
}

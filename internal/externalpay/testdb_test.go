package externalpay

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/mylxsw/aidea-server/migrate/data"
	"github.com/mylxsw/eloquent/migrate"
)

// Each functional test gets its own schema so workers cannot claim another test's events.
func externalPayTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("AIDEA_EXTERNALPAY_TEST_DSN")
	if dsn == "" {
		t.Skip("set AIDEA_EXTERNALPAY_TEST_DSN to run payment integration tests against a disposable MariaDB server")
	}
	conf, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	conf.DBName = ""
	admin, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	name, err := randomID("aidea_pay_test_")
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE `%s` CHARACTER SET utf8mb4", name)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	conf.DBName = name
	db, err := sql.Open("mysql", conf.FormatDSN())
	if err != nil {
		admin.ExecContext(ctx, fmt.Sprintf("DROP DATABASE `%s`", name))
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		if _, err := admin.ExecContext(context.Background(), fmt.Sprintf("DROP DATABASE `%s`", name)); err != nil {
			t.Errorf("could not remove test database %s: %v", name, err)
		}
		admin.Close()
	})
	m := migrate.NewManager(db).Init(ctx)
	data.Migrate20261018DDL(m)
	if err := m.Run(ctx); err != nil {
		t.Fatal(err)
	}
	return db
}

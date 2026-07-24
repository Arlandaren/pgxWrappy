package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDSN returns the DSN for a throwaway test database, or skips the test
// if TEST_DATABASE_DSN isn't set (e.g. when running unit tests locally
// without a Postgres instance available).
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}
	return dsn
}

func newTestWrapper(t *testing.T) *Wrapper {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDSN(t))
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewWrapper(pool)
}

// createTable creates table with the given DDL body and registers a cleanup
// that drops it, so each test starts from a clean slate.
func createTable(t *testing.T, w *Wrapper, table, ddl string) {
	t.Helper()
	ctx := context.Background()

	if _, err := w.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
		t.Fatalf("failed to drop table %s: %v", table, err)
	}
	if _, err := w.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (%s)", table, ddl)); err != nil {
		t.Fatalf("failed to create table %s: %v", table, err)
	}
	t.Cleanup(func() {
		_, _ = w.Exec(context.Background(), fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
	})
}

type person struct {
	ID        int       `db:"id"`
	Name      string    `db:"name"`
	CreatedAt time.Time `db:"created_at"`
}

type address struct {
	Street string `db:"street"`
	City   string `db:"city"`
}

type personWithAddress struct {
	ID      int     `db:"id"`
	Name    string  `db:"name"`
	Address address `db:"-"`
}

// Regression test for the bug where any struct-kind field (including
// time.Time) was flattened as a nested struct instead of being scanned as
// a single column, causing "no matching struct field found for column
// created_at" for the extremely common created_at/updated_at pattern.
func TestWrapper_Get_WithTimeField(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_wrapper_get_time", "id serial primary key, name text not null, created_at timestamptz not null")

	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := w.Exec(ctx, "INSERT INTO test_wrapper_get_time (name, created_at) VALUES ($1, $2)", "alice", fixedTime); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var got person
	err := w.Get(ctx, &got, "SELECT id, name, created_at FROM test_wrapper_get_time WHERE name = $1", "alice")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Name != "alice" || !got.CreatedAt.Equal(fixedTime) {
		t.Fatalf("unexpected row: %+v", got)
	}
}

func TestWrapper_Get_NoRows(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_wrapper_get_norows", "id serial primary key, name text not null, created_at timestamptz not null")

	var got person
	err := w.Get(ctx, &got, "SELECT id, name, created_at FROM test_wrapper_get_norows WHERE id = $1", 999)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected pgx.ErrNoRows, got: %v", err)
	}
}

func TestWrapper_Select_MultipleRows(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_wrapper_select", "id serial primary key, name text not null, created_at timestamptz not null")

	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, name := range []string{"alice", "bob", "carol"} {
		if _, err := w.Exec(ctx, "INSERT INTO test_wrapper_select (name, created_at) VALUES ($1, $2)", name, fixedTime); err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	var got []person
	if err := w.Select(ctx, &got, "SELECT id, name, created_at FROM test_wrapper_select ORDER BY id"); err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d: %+v", len(got), got)
	}
	if got[0].Name != "alice" || got[1].Name != "bob" || got[2].Name != "carol" {
		t.Fatalf("unexpected rows: %+v", got)
	}

	var gotPtrs []*person
	if err := w.Select(ctx, &gotPtrs, "SELECT id, name, created_at FROM test_wrapper_select ORDER BY id"); err != nil {
		t.Fatalf("Select into []*person failed: %v", err)
	}
	if len(gotPtrs) != 3 || gotPtrs[0].Name != "alice" {
		t.Fatalf("unexpected pointer rows: %+v", gotPtrs)
	}
}

func TestWrapper_Select_NestedStruct(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_wrapper_nested", "id serial primary key, name text not null, street text not null, city text not null")

	if _, err := w.Exec(ctx, "INSERT INTO test_wrapper_nested (name, street, city) VALUES ($1, $2, $3)", "alice", "Main St", "Springfield"); err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var got personWithAddress
	err := w.Get(ctx, &got, "SELECT id, name, street, city FROM test_wrapper_nested WHERE name = $1", "alice")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Address.Street != "Main St" || got.Address.City != "Springfield" {
		t.Fatalf("unexpected nested struct: %+v", got)
	}
}

func TestWrapper_Transaction_CommitPersists(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_tx_commit", "id serial primary key, name text not null")

	tx, err := w.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO test_tx_commit (name) VALUES ($1)", "alice"); err != nil {
		t.Fatalf("tx insert failed: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	var count int
	row := w.QueryRow(ctx, "SELECT count(*) FROM test_tx_commit")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after commit, got %d", count)
	}
}

func TestWrapper_Transaction_RollbackDiscards(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_tx_rollback", "id serial primary key, name text not null")

	tx, err := w.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO test_tx_rollback (name) VALUES ($1)", "alice"); err != nil {
		t.Fatalf("tx insert failed: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	var count int
	row := w.QueryRow(ctx, "SELECT count(*) FROM test_tx_rollback")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", count)
	}
}

func TestTxWrapper_GetSelect(t *testing.T) {
	w := newTestWrapper(t)
	ctx := context.Background()
	createTable(t, w, "test_tx_getselect", "id serial primary key, name text not null, created_at timestamptz not null")

	fixedTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tx, err := w.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "INSERT INTO test_tx_getselect (name, created_at) VALUES ($1, $2)", "alice", fixedTime); err != nil {
		t.Fatalf("tx insert failed: %v", err)
	}

	var got person
	if err := tx.Get(ctx, &got, "SELECT id, name, created_at FROM test_tx_getselect WHERE name = $1", "alice"); err != nil {
		t.Fatalf("tx Get failed: %v", err)
	}
	if got.Name != "alice" || !got.CreatedAt.Equal(fixedTime) {
		t.Fatalf("unexpected row: %+v", got)
	}

	var gotAll []person
	if err := tx.Select(ctx, &gotAll, "SELECT id, name, created_at FROM test_tx_getselect"); err != nil {
		t.Fatalf("tx Select failed: %v", err)
	}
	if len(gotAll) != 1 {
		t.Fatalf("expected 1 row, got %d", len(gotAll))
	}
}

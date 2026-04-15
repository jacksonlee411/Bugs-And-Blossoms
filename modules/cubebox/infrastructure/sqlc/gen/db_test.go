package cubeboxsqlc

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type testDB struct{}

func (testDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (testDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) { return nil, nil }
func (testDB) QueryRow(context.Context, string, ...interface{}) pgx.Row        { return nil }

type testTx struct{ testDB }

func (testTx) Begin(context.Context) (pgx.Tx, error) { return testTx{}, nil }
func (testTx) Commit(context.Context) error          { return nil }
func (testTx) Rollback(context.Context) error        { return nil }
func (testTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (testTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (testTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (testTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (testTx) Conn() *pgx.Conn { return nil }

func TestQueriesNewAndWithTx(t *testing.T) {
	t.Parallel()

	db := testDB{}
	queries := New(db)
	if queries == nil || queries.db != db {
		t.Fatalf("unexpected queries: %#v", queries)
	}

	tx := testTx{}
	withTx := queries.WithTx(tx)
	if withTx == nil || withTx.db != tx {
		t.Fatalf("unexpected withTx: %#v", withTx)
	}
}

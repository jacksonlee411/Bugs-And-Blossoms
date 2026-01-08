package superadmin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read error") }

type stubPool struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (p stubPool) Begin(ctx context.Context) (pgx.Tx, error) { return p.beginFn(ctx) }
func (p stubPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.queryFn(ctx, sql, args...)
}

type stubTx struct {
	execErrAt int
	execErr   error
	execN     int

	queryRowFn func(sql string, args ...any) pgx.Row
	commitErr  error
}

func (t *stubTx) Begin(context.Context) (pgx.Tx, error) { return t, nil }
func (t *stubTx) Commit(context.Context) error          { return t.commitErr }
func (t *stubTx) Rollback(context.Context) error        { return nil }
func (t *stubTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}
func (t *stubTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return stubBatchResults{} }
func (t *stubTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (t *stubTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}
func (t *stubTx) Conn() *pgx.Conn { return nil }

func (t *stubTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	t.execN++
	if t.execErr != nil && t.execErrAt == t.execN {
		return pgconn.CommandTag{}, t.execErr
	}
	return pgconn.CommandTag{}, nil
}

func (t *stubTx) Query(context.Context, string, ...any) (pgx.Rows, error) { return &stubRows{}, nil }

func (t *stubTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(sql, args...)
	}
	return stubRow{err: errors.New("unexpected QueryRow")}
}

type stubRow struct {
	vals []any
	err  error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.vals[i].(string)
		case *bool:
			*d = r.vals[i].(bool)
		case *time.Time:
			*d = r.vals[i].(time.Time)
		case **time.Time:
			if r.vals[i] == nil {
				*d = nil
				continue
			}
			if v, ok := r.vals[i].(*time.Time); ok {
				*d = v
				continue
			}
			if v, ok := r.vals[i].(time.Time); ok {
				tmp := v
				*d = &tmp
				continue
			}
			return errors.New("unsupported dest")
		default:
			return errors.New("unsupported dest")
		}
	}
	return nil
}

type stubRows struct {
	vals      [][]any
	idx       int
	scanErrAt int
	scanN     int
	err       error
}

func (r *stubRows) Close()                        {}
func (r *stubRows) Err() error                    { return r.err }
func (r *stubRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *stubRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *stubRows) Next() bool {
	if r.idx >= len(r.vals) {
		return false
	}
	r.idx++
	return true
}
func (r *stubRows) Scan(dest ...any) error {
	r.scanN++
	if r.scanErrAt > 0 && r.scanN == r.scanErrAt {
		return errors.New("scan error")
	}
	row := r.vals[r.idx-1]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = row[i].(string)
		case *bool:
			*d = row[i].(bool)
		default:
			return errors.New("unsupported dest")
		}
	}
	return nil
}
func (r *stubRows) Values() ([]any, error) { return nil, nil }
func (r *stubRows) RawValues() [][]byte    { return nil }
func (r *stubRows) Conn() *pgx.Conn        { return nil }

type stubBatchResults struct{}

func (stubBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (stubBatchResults) Query() (pgx.Rows, error)         { return &stubRows{}, nil }
func (stubBatchResults) QueryRow() pgx.Row                { return stubRow{vals: []any{"ok"}} }
func (stubBatchResults) Close() error                     { return nil }

type stubIdentityProvider struct {
	ident authenticatedIdentity
	err   error
}

func (s stubIdentityProvider) AuthenticatePassword(context.Context, string, string) (authenticatedIdentity, error) {
	return s.ident, s.err
}

type authedHandler struct {
	h          http.Handler
	principals *memoryPrincipalStore
	sessions   *memorySessionStore
	principal  superadminPrincipal
	saSid      string
}

func newAuthedHandler(t *testing.T, pool pgBeginner) authedHandler {
	t.Helper()
	t.Setenv("AUTHZ_MODE", "enforce")

	principals := newMemoryPrincipalStore()
	sessions := newMemorySessionStore()
	p, err := principals.UpsertFromKratos(context.Background(), "admin@example.invalid", "kid-1")
	if err != nil {
		t.Fatal(err)
	}
	saSid, err := sessions.Create(context.Background(), p.ID, time.Now().Add(time.Hour), "ip", "ua")
	if err != nil {
		t.Fatal(err)
	}

	h, err := NewHandlerWithOptions(HandlerOptions{
		Pool:             pool,
		IdentityProvider: stubIdentityProvider{},
		Principals:       principals,
		Sessions:         sessions,
	})
	if err != nil {
		t.Fatal(err)
	}

	return authedHandler{
		h:          h,
		principals: principals,
		sessions:   sessions,
		principal:  p,
		saSid:      saSid,
	}
}

func (a authedHandler) newRequest(method string, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.AddCookie(&http.Cookie{Name: saSidCookieName, Value: a.saSid})
	return req
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read") }
func (errReadCloser) Close() error             { return nil }

type personRows struct {
	nextN   int
	scanErr error
	err     error
}

func (r *personRows) Close()                        {}
func (r *personRows) Err() error                    { return r.err }
func (r *personRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *personRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *personRows) Next() bool {
	if r.nextN > 0 {
		return false
	}
	r.nextN++
	return true
}
func (r *personRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if len(dest) >= 3 {
		*(dest[0].(*string)) = "p1"
		*(dest[1].(*string)) = "1"
		*(dest[2].(*string)) = "Person"
	}
	if len(dest) >= 5 {
		*(dest[3].(*string)) = "active"
		*(dest[4].(*time.Time)) = time.Unix(123, 0).UTC()
	}
	return nil
}
func (r *personRows) Values() ([]any, error) { return nil, nil }
func (r *personRows) RawValues() [][]byte    { return nil }
func (r *personRows) Conn() *pgx.Conn        { return nil }

type personQueryTx struct {
	*stubTx
	rows pgx.Rows
}

func (t *personQueryTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.queryErr != nil {
		return nil, t.queryErr
	}
	if t.rows != nil {
		return t.rows, nil
	}
	return &fakeRows{}, nil
}

func TestNormalizePernr(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		if _, err := normalizePernr(""); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := normalizePernr("A"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("too long", func(t *testing.T) {
		if _, err := normalizePernr("123456789"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("trim leading zeros", func(t *testing.T) {
		got, err := normalizePernr("00012")
		if err != nil {
			t.Fatal(err)
		}
		if got != "12" {
			t.Fatalf("expected 12, got %q", got)
		}
	})
	t.Run("zero", func(t *testing.T) {
		got, err := normalizePernr("00000000")
		if err != nil {
			t.Fatal(err)
		}
		if got != "0" {
			t.Fatalf("expected 0, got %q", got)
		}
	})
}

func TestPersonPGStore_ListPersons(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{scanErr: errors.New("scan")}}
			return tx, nil
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{err: errors.New("rows")}}
			return tx, nil
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}, rows: &personRows{}}
			return tx, nil
		}))
		_, err := store.ListPersons(context.Background(), "t1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{}}
			return tx, nil
		}))
		ps, err := store.ListPersons(context.Background(), "t1")
		if err != nil {
			t.Fatal(err)
		}
		if len(ps) != 1 {
			t.Fatalf("expected 1 person, got %d", len(ps))
		}
	})
}

func TestPersonPGStore_CreatePerson(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid pernr", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "BAD", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing display_name", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "1", " ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("insert error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"p1", time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		_, err := store.CreatePerson(context.Background(), "t1", "1", "A")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"p1", time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		p, err := store.CreatePerson(context.Background(), "t1", "0001", "A")
		if err != nil {
			t.Fatal(err)
		}
		if p.Pernr != "1" {
			t.Fatalf("expected pernr=1, got %q", p.Pernr)
		}
	})
}

func TestPersonPGStore_FindPersonByPernr(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid pernr", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{}, nil
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "BAD")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no rows", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: pgx.ErrNoRows}, nil
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "1")
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected pgx.ErrNoRows, got %v", err)
		}
	})

	t.Run("row error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{rowErr: errors.New("row")}, nil
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{commitErr: errors.New("commit")}
			tx.row = &stubRow{vals: []any{"p1", "1", "Person", "active", time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		_, err := store.FindPersonByPernr(context.Background(), "t1", "1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &stubTx{}
			tx.row = &stubRow{vals: []any{"p1", "1", "Person", "active", time.Unix(1, 0).UTC()}}
			return tx, nil
		}))
		p, err := store.FindPersonByPernr(context.Background(), "t1", "0001")
		if err != nil {
			t.Fatal(err)
		}
		if p.UUID != "p1" {
			t.Fatalf("expected p1, got %q", p.UUID)
		}
	})
}

type personErrStore struct {
	PersonStore
	listErr   error
	createErr error
}

func (s personErrStore) ListPersons(ctx context.Context, tenantID string) ([]Person, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.PersonStore.ListPersons(ctx, tenantID)
}

func (s personErrStore) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	if s.createErr != nil {
		return Person{}, s.createErr
	}
	return s.PersonStore.CreatePerson(ctx, tenantID, pernr, displayName)
}

func TestHandlePersonsAPI_Branches(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons", nil)
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/person/api/persons", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get ok", func(t *testing.T) {
		store := newPersonMemoryStore()
		if _, err := store.CreatePerson(context.Background(), "t1", "0001", "A"); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodGet, "/person/api/persons", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, store)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out["tenant_id"] != "t1" {
			t.Fatalf("tenant_id=%v", out["tenant_id"])
		}
	})

	t.Run("get store error", func(t *testing.T) {
		store := personErrStore{PersonStore: newPersonMemoryStore(), listErr: errors.New("boom")}
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/person/api/persons", strings.NewReader("{"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post create error", func(t *testing.T) {
		store := personErrStore{PersonStore: newPersonMemoryStore(), createErr: errors.New("pernr already exists")}
		req := httptest.NewRequest(http.MethodPost, "/person/api/persons", strings.NewReader(`{"pernr":"1","display_name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("post ok", func(t *testing.T) {
		store := newPersonMemoryStore()
		req := httptest.NewRequest(http.MethodPost, "/person/api/persons", strings.NewReader(`{"pernr":"0001","display_name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersonsAPI(rec, req, store)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out["pernr"] != "1" {
			t.Fatalf("pernr=%v", out["pernr"])
		}
	})
}

func TestPersonPGStore_ListPersonOptions(t *testing.T) {
	t.Run("begin error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set tenant error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{queryErr: errors.New("query")}, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{scanErr: errors.New("scan")}}
			return tx, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{err: errors.New("rows")}}
			return tx, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{commitErr: errors.New("commit")}, rows: &personRows{}}
			return tx, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "1", 100)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("ok (pernr prefix branch)", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{}}
			return tx, nil
		}))
		items, err := store.ListPersonOptions(context.Background(), "t1", "0001", 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(items))
		}
	})

	t.Run("ok (q not pernr branch)", func(t *testing.T) {
		store := newPersonPGStore(beginnerFunc(func(context.Context) (pgx.Tx, error) {
			tx := &personQueryTx{stubTx: &stubTx{}, rows: &personRows{}}
			return tx, nil
		}))
		_, err := store.ListPersonOptions(context.Background(), "t1", "Name", 100)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestPersonMemoryStore(t *testing.T) {
	store := newPersonMemoryStore().(*personMemoryStore)

	t.Run("create invalid pernr", func(t *testing.T) {
		if _, err := store.CreatePerson(context.Background(), "t1", "BAD", "A"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create missing display name", func(t *testing.T) {
		if _, err := store.CreatePerson(context.Background(), "t1", "1", " "); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create ok", func(t *testing.T) {
		if _, err := store.CreatePerson(context.Background(), "t1", "0001", "A"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		if _, err := store.CreatePerson(context.Background(), "t1", "1", "B"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("find ok", func(t *testing.T) {
		if _, err := store.FindPersonByPernr(context.Background(), "t1", "1"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("find no rows", func(t *testing.T) {
		if _, err := store.FindPersonByPernr(context.Background(), "t1", "2"); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected pgx.ErrNoRows, got %v", err)
		}
	})

	t.Run("options cap limit", func(t *testing.T) {
		items, err := store.ListPersonOptions(context.Background(), "t1", "A", 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1, got %d", len(items))
		}
	})

	t.Run("options empty q", func(t *testing.T) {
		items, err := store.ListPersonOptions(context.Background(), "t1", "", -1)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1, got %d", len(items))
		}
	})

	t.Run("options pernr prefix branch", func(t *testing.T) {
		items, err := store.ListPersonOptions(context.Background(), "t1", "0001", 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 {
			t.Fatalf("expected 1, got %d", len(items))
		}
	})
}

type personStoreStub struct {
	listFn    func(ctx context.Context, tenantID string) ([]Person, error)
	createFn  func(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error)
	findFn    func(ctx context.Context, tenantID string, pernr string) (Person, error)
	optionsFn func(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error)
}

func (s personStoreStub) ListPersons(ctx context.Context, tenantID string) ([]Person, error) {
	if s.listFn == nil {
		return nil, errors.New("not implemented")
	}
	return s.listFn(ctx, tenantID)
}
func (s personStoreStub) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	if s.createFn == nil {
		return Person{}, errors.New("not implemented")
	}
	return s.createFn(ctx, tenantID, pernr, displayName)
}
func (s personStoreStub) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error) {
	if s.findFn == nil {
		return Person{}, errors.New("not implemented")
	}
	return s.findFn(ctx, tenantID, pernr)
}
func (s personStoreStub) ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error) {
	if s.optionsFn == nil {
		return nil, errors.New("not implemented")
	}
	return s.optionsFn(ctx, tenantID, q, limit)
}

func TestPersonHandlers(t *testing.T) {
	t.Run("handlePersons tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/persons?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersons missing as_of redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/persons", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Location"), "as_of=") {
			t.Fatalf("location=%q", rec.Header().Get("Location"))
		}
	})

	t.Run("handlePersons invalid as_of returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/persons?as_of=BAD", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersons get list error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/persons?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, personStoreStub{
			listFn: func(context.Context, string) ([]Person, error) { return nil, errors.New("list") },
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "list") {
			t.Fatalf("unexpected body: %q", rec.Body.String())
		}
	})

	t.Run("handlePersons post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/person/persons?as_of=2026-01-01", nil)
		req.Body = errReadCloser{}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersons post create error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/person/persons?as_of=2026-01-01", strings.NewReader("pernr=1&display_name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, personStoreStub{
			listFn: func(context.Context, string) ([]Person, error) { return nil, nil },
			createFn: func(context.Context, string, string, string) (Person, error) {
				return Person{}, errors.New("create")
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersons post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/person/persons?as_of=2026-01-01", strings.NewReader("pernr=1&display_name=A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersons method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/person/persons?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1", Domain: "localhost", Name: "T"}))
		rec := httptest.NewRecorder()
		handlePersons(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("renderPersons with error", func(t *testing.T) {
		html := renderPersons(nil, Tenant{Name: "T"}, "2026-01-01", "err")
		if !strings.Contains(html, "err") {
			t.Fatal("expected error message")
		}
	})
	t.Run("renderPersons without error", func(t *testing.T) {
		_ = renderPersons([]Person{{UUID: "p1", Pernr: "1", DisplayName: "A", Status: "active"}}, Tenant{Name: "T"}, "2026-01-01", "")
	})

	t.Run("handlePersonOptionsAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:options", nil)
		rec := httptest.NewRecorder()
		handlePersonOptionsAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonOptionsAPI internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:options?limit=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonOptionsAPI(rec, req, personStoreStub{
			optionsFn: func(context.Context, string, string, int) ([]PersonOption, error) { return nil, errors.New("oops") },
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonOptionsAPI ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:options?q=A&limit=5", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonOptionsAPI(rec, req, personStoreStub{
			optionsFn: func(context.Context, string, string, int) ([]PersonOption, error) {
				return []PersonOption{{UUID: "p1", Pernr: "1", DisplayName: "A"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var out struct {
			Items []struct {
				PersonUUID string `json:"person_uuid"`
			} `json:"items"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		if len(out.Items) != 1 || out.Items[0].PersonUUID != "p1" {
			t.Fatalf("unexpected response: %+v", out)
		}
	})

	t.Run("handlePersonByPernrAPI tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=1", nil)
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonByPernrAPI missing pernr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonByPernrAPI invalid pernr", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=BAD", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, newPersonMemoryStore())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonByPernrAPI not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, personStoreStub{
			findFn: func(context.Context, string, string) (Person, error) { return Person{}, pgx.ErrNoRows },
		})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonByPernrAPI internal error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, personStoreStub{
			findFn: func(context.Context, string, string) (Person, error) { return Person{}, errors.New("db") },
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("handlePersonByPernrAPI ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/person/api/persons:by-pernr?pernr=0001", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePersonByPernrAPI(rec, req, personStoreStub{
			findFn: func(context.Context, string, string) (Person, error) {
				return Person{UUID: "p1", Pernr: "1", DisplayName: "A", Status: "active"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body, _ := io.ReadAll(rec.Body)
		if !strings.Contains(string(body), `"person_uuid":"p1"`) {
			t.Fatalf("unexpected body: %s", string(body))
		}
	})
}

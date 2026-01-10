package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakePersonStore struct {
	listPersonsErr error
	persons        []Person
}

func (s fakePersonStore) ListPersons(context.Context, string) ([]Person, error) {
	if s.listPersonsErr != nil {
		return nil, s.listPersonsErr
	}
	return append([]Person(nil), s.persons...), nil
}
func (fakePersonStore) CreatePerson(context.Context, string, string, string) (Person, error) {
	return Person{}, errors.New("not implemented")
}
func (fakePersonStore) FindPersonByPernr(context.Context, string, string) (Person, error) {
	return Person{}, errors.New("not implemented")
}
func (fakePersonStore) ListPersonOptions(context.Context, string, string, int) ([]PersonOption, error) {
	return nil, errors.New("not implemented")
}
func (fakePersonStore) ListExternalIdentityLinks(context.Context, string, int) ([]ExternalIdentityLink, error) {
	return nil, errors.New("not implemented")
}
func (fakePersonStore) LinkExternalIdentity(context.Context, string, string, string, string) error {
	return errors.New("not implemented")
}
func (fakePersonStore) DisableExternalIdentity(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (fakePersonStore) EnableExternalIdentity(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (fakePersonStore) IgnoreExternalIdentity(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (fakePersonStore) UnignoreExternalIdentity(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (fakePersonStore) UnlinkExternalIdentity(context.Context, string, string, string) error {
	return errors.New("not implemented")
}

type fakeTimePunchStore struct {
	listFn   func(ctx context.Context, tenantID string, personUUID string, fromUTC time.Time, toUTC time.Time, limit int) ([]TimePunch, error)
	submitFn func(ctx context.Context, tenantID string, initiatorID string, p submitTimePunchParams) (TimePunch, error)
	importFn func(ctx context.Context, tenantID string, initiatorID string, events []submitTimePunchParams) error
}

func (s fakeTimePunchStore) ListTimePunchesForPerson(ctx context.Context, tenantID string, personUUID string, fromUTC time.Time, toUTC time.Time, limit int) ([]TimePunch, error) {
	return s.listFn(ctx, tenantID, personUUID, fromUTC, toUTC, limit)
}
func (s fakeTimePunchStore) SubmitTimePunch(ctx context.Context, tenantID string, initiatorID string, p submitTimePunchParams) (TimePunch, error) {
	return s.submitFn(ctx, tenantID, initiatorID, p)
}
func (s fakeTimePunchStore) ImportTimePunches(ctx context.Context, tenantID string, initiatorID string, events []submitTimePunchParams) error {
	return s.importFn(ctx, tenantID, initiatorID, events)
}

func ctxWithTenant(ctx context.Context) context.Context {
	return withTenant(ctx, Tenant{ID: "00000000-0000-0000-0000-000000000001", Name: "t1"})
}

func ctxWithTenantAndPrincipal(ctx context.Context) context.Context {
	ctx = ctxWithTenant(ctx)
	return withPrincipal(ctx, Principal{ID: "00000000-0000-0000-0000-000000000010", TenantID: "00000000-0000-0000-0000-000000000001", RoleSlug: "tenant-admin", Status: "active"})
}

type errBodyReadCloser struct{}

func (errBodyReadCloser) Read([]byte) (int, error) { return 0, errors.New("read error") }
func (errBodyReadCloser) Close() error             { return nil }

func TestHandleAttendancePunches(t *testing.T) {
	people := []Person{{UUID: "pu1", Pernr: "1", DisplayName: "Alice"}}

	storeOK := fakeTimePunchStore{
		listFn: func(context.Context, string, string, time.Time, time.Time, int) ([]TimePunch, error) {
			return []TimePunch{{EventID: "e1", PersonUUID: "pu1", PunchTime: time.Unix(1, 0).UTC(), PunchType: "IN", SourceProvider: "MANUAL", TransactionTime: time.Unix(2, 0).UTC()}}, nil
		},
		submitFn: func(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
			return TimePunch{EventID: "e2", PersonUUID: "pu1", PunchTime: time.Unix(3, 0).UTC(), PunchType: "OUT", SourceProvider: "MANUAL", TransactionTime: time.Unix(4, 0).UTC()}, nil
		},
		importFn: func(context.Context, string, string, []submitTimePunchParams) error { return nil },
	}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of missing redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "as_of=") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("from_date invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01&from_date=BAD", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "from_date") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("to_date invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01&to_date=BAD", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "to_date") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("to_date before from_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-02&from_date=2026-01-02&to_date=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "to_date") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("persons error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{listPersonsErr: errors.New("boom")})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("list error", func(t *testing.T) {
		storeErr := storeOK
		storeErr.listFn = func(context.Context, string, string, time.Time, time.Time, int) ([]TimePunch, error) {
			return nil, errors.New("list fail")
		}
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01&person_uuid=pu1", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeErr, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "list fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("get ok (with results)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-punches?as_of=2026-01-01&person_uuid=pu1", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Attendance / Punches") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01&person_uuid=pu1", errBodyReadCloser{})
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "bad form") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post principal missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post manual person missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual&punch_at=2026-01-01T09:00&punch_type=IN"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "person_uuid is required") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post manual punch_at missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual&person_uuid=pu1&punch_type=IN"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "punch_at is required") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post manual punch_at invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual&person_uuid=pu1&punch_at=BAD&punch_type=IN"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "punch_at") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post manual punch_type invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual&person_uuid=pu1&punch_at=2026-01-01T09:00&punch_type=BAD"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "punch_type must be") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post manual submit error", func(t *testing.T) {
		storeErr := storeOK
		storeErr.submitFn = func(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
			return TimePunch{}, errors.New("submit fail")
		}
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=manual&person_uuid=pu1&punch_at=2026-01-01T09:00&punch_type=IN"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeErr, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "submit fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post manual ok redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01&from_date=2026-01-01&to_date=2026-01-02", strings.NewReader("op=manual&person_uuid=pu1&punch_at=2026-01-01T09:00&punch_type=IN&note=x"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "/org/attendance-punches") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("post import csv too large", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=import&csv="+strings.Repeat("a", 512*1024+1)))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import csv empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=import&csv=%0A%0A"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "csv is required") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post import too many lines", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 2001; i++ {
			b.WriteString("pu1,2026-01-01T09:00,IN\n")
		}
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", b.String())
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import invalid columns", func(t *testing.T) {
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", "pu1,2026-01-01T09:00\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import missing person_uuid", func(t *testing.T) {
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", ",2026-01-01T09:00,IN\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import invalid punch_at", func(t *testing.T) {
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", "pu1,BAD,IN\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import invalid punch_type", func(t *testing.T) {
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", "pu1,2026-01-01T09:00,BAD\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post import store error", func(t *testing.T) {
		storeErr := storeOK
		storeErr.importFn = func(context.Context, string, string, []submitTimePunchParams) error {
			return errors.New("import fail")
		}
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", "pu1,2026-01-01T09:00,IN\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeErr, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "import fail") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post import ok redirects", func(t *testing.T) {
		form := url.Values{}
		form.Set("op", "import")
		form.Set("csv", "pu1,2026-01-01T09:00,IN\n")
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01&from_date=2026-01-01&to_date=2026-01-02", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post unsupported op", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-punches?as_of=2026-01-01", strings.NewReader("op=nope"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/attendance-punches?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunches(rec, req, storeOK, fakePersonStore{persons: people})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandleAttendancePunchesAPI(t *testing.T) {
	storeOK := fakeTimePunchStore{
		listFn: func(context.Context, string, string, time.Time, time.Time, int) ([]TimePunch, error) {
			return []TimePunch{{EventID: "e1"}}, nil
		},
		submitFn: func(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
			return TimePunch{EventID: "e2"}, nil
		},
		importFn: func(context.Context, string, string, []submitTimePunchParams) error { return nil },
	}

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches", nil)
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("principal missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get missing person_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid from", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&from=BAD", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid to", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&to=BAD", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get invalid limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&limit=BAD", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list error", func(t *testing.T) {
		storeErr := storeOK
		storeErr.listFn = func(context.Context, string, string, time.Time, time.Time, int) ([]TimePunch, error) {
			return nil, errors.New("list fail")
		}
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&limit=2000", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeErr)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&limit=0", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Fatalf("content-type=%q", ct)
		}
	})

	t.Run("get ok (from/to)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/attendance-punches?person_uuid=pu1&from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader("{"))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post missing person_uuid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"punch_time":"2026-01-01T00:00:00Z","punch_type":"IN"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid punch_time", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"BAD","punch_type":"IN"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid punch_type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"2026-01-01T00:00:00Z","punch_type":"BAD"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post invalid source_provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"2026-01-01T00:00:00Z","punch_type":"IN","source_provider":"BAD"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post idempotency reused", func(t *testing.T) {
		storeErr := storeOK
		storeErr.submitFn = func(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
			return TimePunch{}, errors.New("STAFFING_IDEMPOTENCY_REUSED: boom")
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"2026-01-01T00:00:00Z","punch_type":"IN"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeErr)
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post submit error", func(t *testing.T) {
		storeErr := storeOK
		storeErr.submitFn = func(context.Context, string, string, submitTimePunchParams) (TimePunch, error) {
			return TimePunch{}, errors.New("submit fail")
		}
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"2026-01-01T00:00:00Z","punch_type":"IN"}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeErr)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/attendance-punches", strings.NewReader(`{"person_uuid":"pu1","punch_time":"2026-01-01T00:00:00Z","punch_type":"IN","payload":{}}`))
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/api/attendance-punches", nil)
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendancePunchesAPI(rec, req, storeOK)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestAttendanceHelpers(t *testing.T) {
	if _, err := parseDateInLocation("", shanghaiLocation()); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parseDateInLocation("BAD", shanghaiLocation()); err == nil {
		t.Fatal("expected error")
	}
	if _, err := parseDateInLocation("2026-01-01", shanghaiLocation()); err != nil {
		t.Fatalf("err=%v", err)
	}

	lines := splitNonEmptyLines(" \r\n\nx\r\ny\n ")
	if len(lines) != 2 || lines[0] != "x" || lines[1] != "y" {
		t.Fatalf("lines=%v", lines)
	}

	_ = renderAttendancePunches(nil, nil, Tenant{ID: "t1", Name: "t1"}, "2026-01-01", "", "2026-01-01", "2026-01-01", "err")
	_ = renderAttendancePunches(nil, nil, Tenant{ID: "t1", Name: "t1"}, "2026-01-01", "p1", "2026-01-01", "2026-01-01", "")
	_ = renderAttendancePunches([]TimePunch{{EventID: "e1", PunchTime: time.Unix(1, 0).UTC(), TransactionTime: time.Unix(2, 0).UTC()}}, nil, Tenant{ID: "t1", Name: "t1"}, "2026-01-01", "p1", "2026-01-01", "2026-01-01", "")
}

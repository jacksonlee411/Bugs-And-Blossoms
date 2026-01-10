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

func TestHandleAttendanceIntegrations(t *testing.T) {
	store := newPersonMemoryStore()

	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("as_of missing redirects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusFound {
			t.Fatalf("status=%d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); !strings.Contains(loc, "as_of=") {
			t.Fatalf("location=%q", loc)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("get list persons error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, personStoreStub{
			listFn: func(context.Context, string) ([]Person, error) { return nil, errors.New("boom") },
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "boom") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("get links error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, personStoreStub{
			listFn: func(context.Context, string) ([]Person, error) {
				return []Person{{UUID: "pu1", Pernr: "1", DisplayName: "Alice"}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "not implemented") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post principal missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader("op=link"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post bad form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req.Body = errReadCloser{}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("post unsupported op", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader("op=nope"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "unsupported op") {
			t.Fatalf("body=%q", rec.Body.String())
		}
	})

	t.Run("post op error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader("op=disable&provider=WECOM&external_user_id=missing"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctxWithTenantAndPrincipal(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("link flow", func(t *testing.T) {
		ctx := ctxWithTenantAndPrincipal(context.Background())
		p, err := store.CreatePerson(ctx, "00000000-0000-0000-0000-000000000001", "1", "Alice")
		if err != nil {
			t.Fatalf("create person: %v", err)
		}

		form := url.Values{}
		form.Set("op", "link")
		form.Set("provider", "WECOM")
		form.Set("external_user_id", "wecom_u1")
		form.Set("person_uuid", p.UUID)
		req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, store)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}

		req2 := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req2 = req2.WithContext(ctxWithTenant(req2.Context()))
		rec2 := httptest.NewRecorder()
		handleAttendanceIntegrations(rec2, req2, store)
		if rec2.Code != http.StatusOK {
			t.Fatalf("status=%d", rec2.Code)
		}
		if !strings.Contains(rec2.Body.String(), "wecom_u1") {
			t.Fatalf("body=%q", rec2.Body.String())
		}
	})

	t.Run("post all ops", func(t *testing.T) {
		s := newPersonMemoryStore().(*personMemoryStore)
		ctx := ctxWithTenantAndPrincipal(context.Background())
		p, err := s.CreatePerson(ctx, "00000000-0000-0000-0000-000000000001", "1", "Alice")
		if err != nil {
			t.Fatalf("create person: %v", err)
		}

		post := func(body string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/org/attendance-integrations?as_of=2026-01-01", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()
			handleAttendanceIntegrations(rec, req, s)
			return rec
		}

		if rec := post("op=link&provider=WECOM&external_user_id=ops_u1&person_uuid=" + url.QueryEscape(p.UUID)); rec.Code != http.StatusSeeOther {
			t.Fatalf("link status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec := post("op=disable&provider=WECOM&external_user_id=ops_u1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("disable status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec := post("op=enable&provider=WECOM&external_user_id=ops_u1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("enable status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec := post("op=unlink&provider=WECOM&external_user_id=ops_u1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("unlink status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec := post("op=ignore&provider=WECOM&external_user_id=ops_u1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("ignore status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec := post("op=unignore&provider=WECOM&external_user_id=ops_u1"); rec.Code != http.StatusSeeOther {
			t.Fatalf("unignore status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("render statuses", func(t *testing.T) {
		s := newPersonMemoryStore().(*personMemoryStore)
		ctx := ctxWithTenantAndPrincipal(context.Background())
		p, err := s.CreatePerson(ctx, "00000000-0000-0000-0000-000000000001", "1", "Alice")
		if err != nil {
			t.Fatal(err)
		}

		now := time.Now().UTC()
		s.links[s.externalIdentityKey("00000000-0000-0000-0000-000000000001", "WECOM", "pending_u")] = ExternalIdentityLink{
			TenantID:       "00000000-0000-0000-0000-000000000001",
			Provider:       "WECOM",
			ExternalUserID: "pending_u",
			Status:         "pending",
			LastSeenAt:     now.Add(-1 * time.Minute),
			SeenCount:      1,
		}
		s.links[s.externalIdentityKey("00000000-0000-0000-0000-000000000001", "WECOM", "active_u")] = ExternalIdentityLink{
			TenantID:       "00000000-0000-0000-0000-000000000001",
			Provider:       "WECOM",
			ExternalUserID: "active_u",
			Status:         "active",
			PersonUUID:     &p.UUID,
			LastSeenAt:     now.Add(-2 * time.Minute),
			SeenCount:      2,
		}
		s.links[s.externalIdentityKey("00000000-0000-0000-0000-000000000001", "WECOM", "disabled_u")] = ExternalIdentityLink{
			TenantID:       "00000000-0000-0000-0000-000000000001",
			Provider:       "WECOM",
			ExternalUserID: "disabled_u",
			Status:         "disabled",
			PersonUUID:     &p.UUID,
			LastSeenAt:     now.Add(-3 * time.Minute),
			SeenCount:      3,
		}
		s.links[s.externalIdentityKey("00000000-0000-0000-0000-000000000001", "WECOM", "ignored_u")] = ExternalIdentityLink{
			TenantID:       "00000000-0000-0000-0000-000000000001",
			Provider:       "WECOM",
			ExternalUserID: "ignored_u",
			Status:         "ignored",
			LastSeenAt:     now.Add(-4 * time.Minute),
			SeenCount:      4,
		}

		req := httptest.NewRequest(http.MethodGet, "/org/attendance-integrations?as_of=2026-01-01", nil)
		req = req.WithContext(ctxWithTenant(req.Context()))
		rec := httptest.NewRecorder()
		handleAttendanceIntegrations(rec, req, s)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{"pending_u", "active_u", "disabled_u", "ignored_u", "Ignore", "Disable", "Enable", "Unignore"} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in body=%q", want, body)
			}
		}
	})

	t.Run("render label branches", func(t *testing.T) {
		asOf := "2026-01-01"
		tenant := Tenant{Name: "T"}

		p1 := Person{UUID: "p1", Pernr: "1", DisplayName: ""}
		p2 := Person{UUID: "p2", Pernr: "", DisplayName: "NoPernr"}
		p3 := Person{UUID: "p3", Pernr: "3", DisplayName: "Bob"}
		persons := []Person{p1, p2, p3}

		personUUID := "p3"
		missingUUID := "missing-person"
		links := []ExternalIdentityLink{
			{Provider: "WECOM", ExternalUserID: "u_pending", Status: "pending", LastSeenAt: time.Unix(1, 0).UTC(), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "u_active_known", Status: "active", PersonUUID: &personUUID, LastSeenAt: time.Unix(2, 0).UTC(), SeenCount: 2},
			{Provider: "WECOM", ExternalUserID: "u_active_unknown", Status: "active", PersonUUID: &missingUUID, LastSeenAt: time.Unix(3, 0).UTC(), SeenCount: 3},
		}

		body := renderAttendanceIntegrations(tenant, asOf, persons, links, "err")
		for _, want := range []string{"u_pending", "u_active_known", "u_active_unknown", "err", "NoPernr"} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in body=%q", want, body)
			}
		}
	})

	t.Run("render sort closures coverage", func(t *testing.T) {
		asOf := "2026-01-01"
		tenant := Tenant{Name: "T"}

		personUUID := "p1"
		persons := []Person{{UUID: personUUID, Pernr: "1", DisplayName: "Alice"}}

		now := time.Now().UTC()
		links := []ExternalIdentityLink{
			{Provider: "WECOM", ExternalUserID: "pending_old", Status: "pending", LastSeenAt: now.Add(-2 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "pending_new", Status: "pending", LastSeenAt: now.Add(-1 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "active_old", Status: "active", PersonUUID: &personUUID, LastSeenAt: now.Add(-4 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "active_new", Status: "active", PersonUUID: &personUUID, LastSeenAt: now.Add(-3 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "disabled_old", Status: "disabled", PersonUUID: &personUUID, LastSeenAt: now.Add(-6 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "disabled_new", Status: "disabled", PersonUUID: &personUUID, LastSeenAt: now.Add(-5 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "ignored_old", Status: "ignored", LastSeenAt: now.Add(-8 * time.Minute), SeenCount: 1},
			{Provider: "WECOM", ExternalUserID: "ignored_new", Status: "ignored", LastSeenAt: now.Add(-7 * time.Minute), SeenCount: 1},
		}

		body := renderAttendanceIntegrations(tenant, asOf, persons, links, "")
		for _, want := range []string{"pending_old", "pending_new", "active_old", "active_new", "disabled_old", "disabled_new", "ignored_old", "ignored_new"} {
			if !strings.Contains(body, want) {
				t.Fatalf("missing %q in body=%q", want, body)
			}
		}
	})
}

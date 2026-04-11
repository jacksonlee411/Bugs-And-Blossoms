package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestSetIDContextResolverResolvePolicyContext(t *testing.T) {
	orgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	resolver := newSetIDContextResolver(
		setIDExplainOrgResolverStub{
			byCode: map[string]string{
				"BU-001": orgNodeKey,
			},
		},
		scopeAPIStore{
			resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
				return " a0001 ", nil
			},
		},
	)

	got, err := resolver.ResolvePolicyContext(context.Background(), setIDPolicyContextInput{
		TenantID:            "t1",
		CapabilityKey:       " Staffing.Assignment_Create.Field_Policy ",
		FieldKey:            " Field_X ",
		AsOf:                "2026-01-01",
		BusinessUnitOrgCode: "bu-001",
	})
	if err != nil {
		t.Fatalf("resolve policy context: %v", err)
	}
	if got.CapabilityKey != "staffing.assignment_create.field_policy" {
		t.Fatalf("capability_key=%q", got.CapabilityKey)
	}
	if got.FieldKey != "field_x" {
		t.Fatalf("field_key=%q", got.FieldKey)
	}
	if got.BusinessUnitOrgCode != "BU-001" {
		t.Fatalf("business_unit_org_code=%q", got.BusinessUnitOrgCode)
	}
	if got.BusinessUnitNodeKey != orgNodeKey {
		t.Fatalf("business_unit_node_key=%q", got.BusinessUnitNodeKey)
	}
	if got.ResolvedSetID != "A0001" {
		t.Fatalf("resolved_setid=%q", got.ResolvedSetID)
	}
	if got.SetIDSource != "custom" {
		t.Fatalf("setid_source=%q", got.SetIDSource)
	}
}

func TestSetIDContextResolverResolveOrgContextErrors(t *testing.T) {
	t.Run("invalid org code wins before missing dependencies", func(t *testing.T) {
		resolver := newSetIDContextResolver(nil, nil)
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "bad\x7f", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeBusinessUnitInvalid {
			t.Fatalf("code=%q", resolveErr.Code)
		}
		if !errors.Is(resolveErr.Cause, orgunitpkg.ErrOrgCodeInvalid) {
			t.Fatalf("cause=%v", resolveErr.Cause)
		}
	})

	t.Run("not found wins before missing setid resolver", func(t *testing.T) {
		resolver := newSetIDContextResolver(setIDExplainOrgResolverStub{byCode: map[string]string{}}, nil)
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "BU-404", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeBusinessUnitInvalid {
			t.Fatalf("code=%q", resolveErr.Code)
		}
		if !errors.Is(resolveErr.Cause, orgunitpkg.ErrOrgCodeNotFound) {
			t.Fatalf("cause=%v", resolveErr.Cause)
		}
	})

	t.Run("setid resolver missing after org resolution", func(t *testing.T) {
		resolver := newSetIDContextResolver(
			setIDExplainOrgResolverStub{byCode: map[string]string{"BU-001": mustOrgNodeKeyForTest(t, 10000001)}},
			nil,
		)
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "BU-001", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeSetIDResolverMissing {
			t.Fatalf("code=%q", resolveErr.Code)
		}
	})

	t.Run("org resolver missing after valid org normalization", func(t *testing.T) {
		resolver := newSetIDContextResolver(nil, scopeAPIStore{})
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "BU-001", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeOrgResolverMissing {
			t.Fatalf("code=%q", resolveErr.Code)
		}
	})

	t.Run("invalid org node key from resolver", func(t *testing.T) {
		resolver := newSetIDContextResolver(
			setIDExplainOrgResolverStub{byCode: map[string]string{"BU-001": "bad-node-key"}},
			scopeAPIStore{},
		)
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "BU-001", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeBusinessUnitInvalid {
			t.Fatalf("code=%q", resolveErr.Code)
		}
	})

	t.Run("binding missing on empty setid", func(t *testing.T) {
		resolver := newSetIDContextResolver(
			setIDExplainOrgResolverStub{byCode: map[string]string{"BU-001": mustOrgNodeKeyForTest(t, 10000001)}},
			scopeAPIStore{
				resolveSetIDFn: func(context.Context, string, string, string) (string, error) {
					return "   ", nil
				},
			},
		)
		_, err := resolver.ResolveOrgContext(context.Background(), "t1", "BU-001", "2026-01-01", "org_code")
		resolveErr, ok := asSetIDContextResolveError(err)
		if !ok {
			t.Fatalf("expected context resolve error, got=%v", err)
		}
		if resolveErr.Code != setIDContextCodeSetIDBindingMissing {
			t.Fatalf("code=%q", resolveErr.Code)
		}
	})
}

func TestClassifyResolvedSetIDSource(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: true},
		{name: "deflt", input: "deflt", want: orgUnitFieldOptionSetIDSourceDeflt},
		{name: "share", input: "share", want: "share_preview"},
		{name: "custom", input: "A0001", want: "custom"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := classifyResolvedSetIDSource(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("classify: %v", err)
			}
			if got != tc.want {
				t.Fatalf("source=%q", got)
			}
		})
	}
}

func TestSetIDContextResolveErrorHelpers(t *testing.T) {
	var nilErr *setIDContextResolveError
	if nilErr.Error() != "" {
		t.Fatalf("nil error=%q", nilErr.Error())
	}
	if nilErr.Unwrap() != nil {
		t.Fatalf("nil unwrap=%v", nilErr.Unwrap())
	}

	cause := errors.New("boom")
	err := &setIDContextResolveError{
		Code:  setIDContextCodeSetIDBindingMissing,
		Field: "org_code",
		Cause: cause,
	}
	if err.Error() != "org_code:setid_binding_missing" {
		t.Fatalf("error=%q", err.Error())
	}
	if err.Unwrap() != cause {
		t.Fatalf("unwrap=%v", err.Unwrap())
	}
	fieldOnlyErr := &setIDContextResolveError{Field: "org_code"}
	if fieldOnlyErr.Error() != "org_code" {
		t.Fatalf("field-only error=%q", fieldOnlyErr.Error())
	}
	codeOnlyErr := &setIDContextResolveError{Code: setIDContextCodeSetIDResolverMissing}
	if codeOnlyErr.Error() != setIDContextCodeSetIDResolverMissing {
		t.Fatalf("code-only error=%q", codeOnlyErr.Error())
	}
	got, ok := asSetIDContextResolveError(err)
	if !ok || got != err {
		t.Fatalf("resolveErr=%v ok=%v", got, ok)
	}
	if _, ok := asSetIDContextResolveError(errors.New("plain")); ok {
		t.Fatal("expected plain error not to match")
	}
}

func TestResolveSetIDExplainOrgCodeHelper(t *testing.T) {
	orgNodeKey := mustOrgNodeKeyForTest(t, 10000001)
	orgCode, gotOrgNodeKey, err := resolveSetIDExplainOrgCode(
		context.Background(),
		"t1",
		"bu-001",
		setIDExplainOrgResolverStub{
			byCode: map[string]string{"BU-001": orgNodeKey},
		},
	)
	if err != nil {
		t.Fatalf("resolve explain org code: %v", err)
	}
	if orgCode != "BU-001" {
		t.Fatalf("org_code=%q", orgCode)
	}
	if gotOrgNodeKey != orgNodeKey {
		t.Fatalf("org_node_key=%q", gotOrgNodeKey)
	}
}

func TestResolveSetIDOrgCodeRef_ResolverMissing(t *testing.T) {
	_, err := resolveSetIDOrgCodeRef(context.Background(), "t1", "BU-001", nil)
	if err == nil || !strings.Contains(err.Error(), "resolver missing") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveSetIDExplainOrgCodeHelperError(t *testing.T) {
	_, _, err := resolveSetIDExplainOrgCode(context.Background(), "t1", "BU-404", setIDExplainOrgResolverStub{byCode: map[string]string{}})
	if !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestWriteSetIDExplainOrgCodeError_Default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
	rec := httptest.NewRecorder()
	writeSetIDExplainOrgCodeError(rec, req, "org_code", errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"boom"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestWriteSetIDExplainOrgCodeError_FieldErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid", err: orgunitpkg.ErrOrgCodeInvalid, wantStatus: http.StatusBadRequest, wantCode: "org_code_invalid"},
		{name: "not found", err: orgunitpkg.ErrOrgCodeNotFound, wantStatus: http.StatusNotFound, wantCode: "org_code_not_found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/org/api/setid-explain", nil)
			rec := httptest.NewRecorder()
			writeSetIDExplainOrgCodeError(rec, req, "org_code", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("unexpected body: %s", rec.Body.String())
			}
		})
	}
}

func TestWriteOrgUnitSetIDContextError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "org resolver missing",
			err:        &setIDContextResolveError{Code: setIDContextCodeOrgResolverMissing, Field: "org_code"},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "orgunit_store_missing",
		},
		{
			name:       "setid resolver missing",
			err:        &setIDContextResolveError{Code: setIDContextCodeSetIDResolverMissing, Field: "org_code"},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "setid_resolver_missing",
		},
		{
			name:       "binding missing",
			err:        &setIDContextResolveError{Code: setIDContextCodeSetIDBindingMissing, Field: "org_code", Cause: errors.New("SETID_NOT_FOUND")},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "SETID_NOT_FOUND",
		},
		{
			name:       "binding missing without cause",
			err:        &setIDContextResolveError{Code: setIDContextCodeSetIDBindingMissing, Field: "org_code"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "setid_missing",
		},
		{
			name:       "source invalid",
			err:        &setIDContextResolveError{Code: setIDContextCodeSetIDSourceInvalid, Field: "org_code"},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   setIDContextCodeSetIDSourceInvalid,
		},
		{
			name:       "fallback org resolution error",
			err:        &setIDContextResolveError{Code: setIDContextCodeBusinessUnitInvalid, Field: "org_code", Cause: errors.New("boom")},
			wantStatus: http.StatusInternalServerError,
			wantCode:   "boom",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options", nil)
			rec := httptest.NewRecorder()
			writeOrgUnitSetIDContextError(rec, req, "org_code", tc.err)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.wantCode) {
				t.Fatalf("unexpected body: %s", rec.Body.String())
			}
		})
	}
}

func TestWriteOrgUnitSetIDContextError_Fallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options", nil)
	rec := httptest.NewRecorder()
	writeOrgUnitSetIDContextError(rec, req, "org_code", errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"boom"`) {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandleSetIDExplainAPI_DependencyErrors(t *testing.T) {
	makeReq := func() *http.Request {
		req := httptest.NewRequest(
			http.MethodGet,
			"/org/api/setid-explain?capability_key=staffing.assignment_create.field_policy&field_key=field_x&business_unit_org_code=BU-001&as_of=2026-01-01",
			nil,
		)
		return req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	}

	t.Run("org resolver missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleSetIDExplainAPI(rec, makeReq(), scopeAPIStore{}, nil)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "orgunit_resolver_missing") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("setid resolver missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleSetIDExplainAPI(
			rec,
			makeReq(),
			nil,
			setIDExplainOrgResolverStub{byCode: map[string]string{"BU-001": mustOrgNodeKeyForTest(t, 10000001)}},
		)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "setid_resolver_missing") {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleInternalRulesEvaluateAPI_OrgResolverMissing(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/rules/evaluate",
		bytes.NewBufferString(`{"capability_key":"staffing.assignment_create.field_policy","field_key":"field_x","business_unit_org_code":"BU-001","as_of":"2026-01-01"}`),
	)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	req = req.WithContext(withPrincipal(req.Context(), Principal{RoleSlug: "tenant-admin"}))
	rec := httptest.NewRecorder()
	handleInternalRulesEvaluateAPI(rec, req, scopeAPIStore{}, nil)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "orgunit_resolver_missing") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

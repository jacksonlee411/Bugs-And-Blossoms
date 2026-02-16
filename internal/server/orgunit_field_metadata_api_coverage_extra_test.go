package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

type dictListErrResolver struct{}

func (dictListErrResolver) ResolveValueLabel(context.Context, string, string, string, string) (string, bool, error) {
	return "", false, nil
}

func (dictListErrResolver) ListOptions(context.Context, string, string, string, string, int) ([]dictpkg.Option, error) {
	return nil, errors.New("boom")
}

func TestHandleOrgUnitFieldOptionsAPI_DictListError(t *testing.T) {
	if err := dictpkg.RegisterResolver(dictListErrResolver{}); err != nil {
		t.Fatalf("register err=%v", err)
	}
	t.Cleanup(func() {
		// Restore a working resolver for other tests in this package.
		_ = dictpkg.RegisterResolver(newDictMemoryStore())
	})

	store := orgUnitStoreWithEnabledFieldConfig{
		OrgUnitStore: newOrgUnitMemoryStore(),
		cfg: orgUnitTenantFieldConfig{
			FieldKey:         "org_type",
			ValueType:        "text",
			DataSourceType:   "DICT",
			DataSourceConfig: json.RawMessage(`{"dict_code":"org_type"}`),
		},
		ok: true,
	}

	req := httptest.NewRequest(http.MethodGet, "/org/api/org-units/fields:options?as_of=2026-01-01&field_key=org_type", nil)
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handleOrgUnitFieldOptionsAPI(rec, req, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

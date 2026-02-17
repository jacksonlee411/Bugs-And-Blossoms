package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDictPGStore_ExtraCoverage(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateDict and DisableDict wrappers", func(t *testing.T) {
		createTx := &stubTx{}
		createTx.row = &stubRow{vals: []any{int64(1), false}}
		createTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"expense_type","name":"Expense Type","status":"active","enabled_on":"2026-01-01"}`)}}
		storeCreate := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return createTx, nil })}
		item, wasRetry, err := storeCreate.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01", RequestCode: "r1", Initiator: "u1"})
		if err != nil || wasRetry || item.DictCode != "expense_type" {
			t.Fatalf("item=%+v retry=%v err=%v", item, wasRetry, err)
		}

		disableTx := &stubTx{}
		disableTx.row = &stubRow{vals: []any{int64(2), true}}
		disableTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"expense_type","name":"Expense Type","status":"inactive","enabled_on":"2026-01-01","disabled_on":"2026-01-02"}`)}}
		storeDisable := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return disableTx, nil })}
		disabled, disableRetry, err := storeDisable.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-02", RequestCode: "r2", Initiator: "u1"})
		if err != nil || !disableRetry || disabled.Status != "inactive" || disabled.DisabledOn == nil {
			t.Fatalf("disabled=%+v retry=%v err=%v", disabled, disableRetry, err)
		}
	})

	t.Run("submitDictEvent error branches", func(t *testing.T) {
		storeBegin := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		if _, _, err := storeBegin.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected begin error")
		}

		storeExec := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec")}, nil
		})}
		if _, _, err := storeExec.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected exec error")
		}

		store := &dictPGStore{pool: &fakeBeginner{}}
		if _, _, err := store.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"x": func() {}}, "r1", "u1"); err == nil {
			t.Fatal("expected marshal error")
		}

		queryErrTx := &stubTx{rowErr: errors.New("row")}
		storeQueryErr := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return queryErrTx, nil })}
		if _, _, err := storeQueryErr.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected query error")
		}

		badSnapshotTx := &stubTx{}
		badSnapshotTx.row = &stubRow{vals: []any{int64(1), false}}
		badSnapshotTx.row2 = &stubRow{vals: []any{[]byte(`{`)}}
		storeBadSnapshot := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return badSnapshotTx, nil })}
		if _, _, err := storeBadSnapshot.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected bad snapshot error")
		}

		commitErrTx := &stubTx{commitErr: errors.New("commit")}
		commitErrTx.row = &stubRow{vals: []any{int64(1), false}}
		commitErrTx.row2 = &stubRow{vals: []any{[]byte(`{"dict_code":"x","name":"X","status":"active","enabled_on":"2026-01-01"}`)}}
		storeCommitErr := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return commitErrTx, nil })}
		if _, _, err := storeCommitErr.submitDictEvent(ctx, "t1", "x", dictRegistryEventCreated, "2026-01-01", map[string]any{"name": "X"}, "r1", "u1"); err == nil {
			t.Fatal("expected commit error")
		}
	})

	t.Run("resolve source helpers", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"t1"}}
		if source, err := resolveDictSourceTenantTx(ctx, tx, "t1", "org_type"); err != nil || source != "t1" {
			t.Fatalf("source=%q err=%v", source, err)
		}

		txNoRows := &stubTx{}
		txNoRows.row = &stubRow{err: pgx.ErrNoRows}
		if _, err := resolveDictSourceTenantTx(ctx, txNoRows, "t1", "missing"); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}

		txErr := &stubTx{}
		txErr.row = &stubRow{err: errors.New("boom")}
		if _, err := resolveDictSourceTenantTx(ctx, txErr, "t1", "org_type"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("resolve label helper branches", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{"部门"}}
		if label, ok, err := resolveValueLabelByTenant(ctx, tx, "t1", "2026-01-01", "org_type", "10"); err != nil || !ok || label != "部门" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}

		txNoRows := &stubTx{}
		txNoRows.row = &stubRow{err: pgx.ErrNoRows}
		if _, ok, err := resolveValueLabelByTenant(ctx, txNoRows, "t1", "2026-01-01", "org_type", "10"); err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}

		txErr := &stubTx{}
		txErr.row = &stubRow{err: errors.New("boom")}
		if _, _, err := resolveValueLabelByTenant(ctx, txErr, "t1", "2026-01-01", "org_type", "10"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("assertTenantDictActiveAsOfTx second query error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{false}}
		tx.row2Err = errors.New("boom")
		if err := assertTenantDictActiveAsOfTx(ctx, tx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent query branch error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		tx.row2Err = errors.New("boom")
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"label": "部门"}, "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submitValueEvent payload marshal error", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{vals: []any{true}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"x": func() {}}, "r1", "u1"); err == nil {
			t.Fatal("expected marshal error")
		}
	})

	t.Run("submitValueEvent active check query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, _, err := store.submitValueEvent(ctx, "t1", "org_type", "10", dictEventCreated, "2026-01-01", map[string]any{"label": "部门"}, "r1", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("getDictFromEventTx query error", func(t *testing.T) {
		tx := &stubTx{rowErr: errors.New("boom")}
		if _, err := getDictFromEventTx(ctx, tx, "t1", 1); err == nil {
			t.Fatal("expected query error")
		}
	})

	t.Run("ListDictValueAudit source not found", func(t *testing.T) {
		tx := &stubTx{}
		tx.row = &stubRow{err: pgx.ErrNoRows}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListDictValueAudit(ctx, "t1", "missing", "10", 10); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestDictMemoryStore_ExtraCoverage(t *testing.T) {
	ctx := context.Background()
	store := newDictMemoryStore().(*dictMemoryStore)

	t.Run("ListDicts coverage branches (tenant/global merge + inactive skip + sort)", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"

		// tenant-only active dict
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		// tenant-only inactive (future) dict to hit the "continue" branch
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "future_dict", Name: "Future Dict", EnabledOn: "2099-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		// global-only dict to hit the global loop append path
		store.dicts[globalTenantID]["global_only"] = DictItem{DictCode: "global_only", Name: "Global Only", Status: "active", EnabledOn: "1970-01-01"}

		items, err := store.ListDicts(ctx, tenantID, "2026-01-01")
		if err != nil {
			t.Fatalf("ListDicts err=%v", err)
		}
		// At least 2 items to exercise sort comparator.
		if len(items) < 2 {
			t.Fatalf("expected >=2 items; got=%d", len(items))
		}
	})

	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "", Name: "X", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "x", Name: "", EnabledOn: "2026-01-01"}); !errors.Is(err, errDictNameRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "x", Name: "X", EnabledOn: ""}); !errors.Is(err, errDictEffectiveDayRequired) {
		t.Fatalf("err=%v", err)
	}

	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeRequired) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "unknown", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictNotFound) {
		t.Fatalf("err=%v", err)
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "org_type", DisabledOn: ""}); !errors.Is(err, errDictDisabledOnRequired) {
		t.Fatalf("err=%v", err)
	}

	created, _, err := store.CreateDict(ctx, "t1", DictCreateRequest{DictCode: "expense_type", Name: "Expense Type", EnabledOn: "2026-01-01"})
	if err != nil || created.Status != "active" {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	if got := dictActiveAsOf(created, "2025-01-01"); got {
		t.Fatal("expected inactive before enabled_on")
	}
	if _, _, err := store.DisableDict(ctx, "t1", DictDisableRequest{DictCode: "expense_type", DisabledOn: "2026-01-01"}); !errors.Is(err, errDictCodeConflict) {
		t.Fatalf("err=%v", err)
	}

	if _, err := store.ListDictValueAudit(ctx, "t1", "missing", "10", 10); !errors.Is(err, errDictNotFound) {
		t.Fatalf("err=%v", err)
	}

	if _, ok := store.resolveSourceTenant("t1", "unknown"); ok {
		t.Fatal("expected miss")
	}

	t.Run("DisableDict item not found branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "missing_dict", DisabledOn: "2026-01-02"}); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("DisableDict already disabled conflict branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		// Create + disable once, then disable again with a later day to hit the DisabledOn conflict check.
		if _, _, err := store.CreateDict(ctx, tenantID, DictCreateRequest{DictCode: "tmp_dict", Name: "Tmp Dict", EnabledOn: "2026-01-01"}); err != nil {
			t.Fatalf("CreateDict err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "tmp_dict", DisabledOn: "2026-01-03"}); err != nil {
			t.Fatalf("DisableDict err=%v", err)
		}
		if _, _, err := store.DisableDict(ctx, tenantID, DictDisableRequest{DictCode: "tmp_dict", DisabledOn: "2026-01-04"}); !errors.Is(err, errDictCodeConflict) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListDictValues status default + dict mismatch + status filter + same-code sort branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		now := time.Unix(0, 0).UTC()

		// Add an unrelated dict_code value to hit the "item.DictCode != dictCode" continue branch.
		store.values[tenantID] = append(store.values[tenantID], DictValueItem{DictCode: "other", Code: "x", Label: "X", EnabledOn: "1970-01-01", UpdatedAt: now})

		// Add two segments with the same code so the sort's "same code" branch is exercised.
		store.values[tenantID] = append(store.values[tenantID],
			DictValueItem{DictCode: dictCodeOrgType, Code: "30", Label: "中心", EnabledOn: "1970-01-01", UpdatedAt: now},
			DictValueItem{DictCode: dictCodeOrgType, Code: "30", Label: "中心(旧)", EnabledOn: "1960-01-01", UpdatedAt: now},
		)

		// Add an inactive value for status filtering.
		store.values[tenantID] = append(store.values[tenantID], DictValueItem{DictCode: dictCodeOrgType, Code: "40", Label: "未来", EnabledOn: "2099-01-01", UpdatedAt: now})

		// status empty => default to "all"
		if _, err := store.ListDictValues(ctx, tenantID, dictCodeOrgType, "2026-01-01", "", 10, ""); err != nil {
			t.Fatalf("ListDictValues err=%v", err)
		}
		// status filter branch
		if _, err := store.ListDictValues(ctx, tenantID, dictCodeOrgType, "2026-01-01", "", 10, "active"); err != nil {
			t.Fatalf("ListDictValues err=%v", err)
		}
	})

	t.Run("ResolveValueLabel dict not found branch", func(t *testing.T) {
		label, ok, err := store.ResolveValueLabel(ctx, "t1", "2026-01-01", "missing", "10")
		if err != nil || ok || label != "" {
			t.Fatalf("label=%q ok=%v err=%v", label, ok, err)
		}
	})

	t.Run("ListOptions propagate ListDictValues error", func(t *testing.T) {
		if _, err := store.ListOptions(ctx, "t1", "2026-01-01", "missing", "", 10); !errors.Is(err, errDictNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("ListDictValueAudit empty code branch", func(t *testing.T) {
		tenantID := "00000000-0000-0000-0000-000000000001"
		events, err := store.ListDictValueAudit(ctx, tenantID, dictCodeOrgType, "", 10)
		if err != nil || events == nil || len(events) != 0 {
			t.Fatalf("events=%v err=%v", events, err)
		}
	})

	inactive := DictItem{DictCode: "x", Name: "X", EnabledOn: "2026-01-01", DisabledOn: ptr("2026-01-02")}
	if dictActiveAsOf(inactive, "2026-01-02") {
		t.Fatal("expected inactive")
	}
	if dictStatusAsOf(inactive, "2026-01-02") != "inactive" {
		t.Fatal("expected inactive status")
	}
	value := DictValueItem{DictCode: "x", Code: "1", EnabledOn: "2026-01-01", DisabledOn: ptr("2026-01-02")}
	if valueStatusAsOf(value, "2026-01-02") != "inactive" {
		t.Fatal("expected inactive value status")
	}
	if dictDisplayName(dictCodeOrgType) != "Org Type" {
		t.Fatal("expected org type display")
	}
	if dictDisplayName(" expense_type ") != " expense_type " {
		t.Fatal("expected default display name passthrough")
	}
}

func TestDictAPI_ExtraCoverage(t *testing.T) {
	t.Run("create dict created status", func(t *testing.T) {
		req := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(`{"dict_code":"expense_type","name":"Expense Type","enabled_on":"2026-01-01","request_code":"r1"}`), true)
		rec := httptest.NewRecorder()
		handleDictsAPI(rec, req, dictStoreStub{createDictFn: func(context.Context, string, DictCreateRequest) (DictItem, bool, error) {
			return DictItem{DictCode: "expense_type", Name: "Expense Type", Status: "active", EnabledOn: "2026-01-01"}, false, nil
		}})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid dict code branches", func(t *testing.T) {
		cases := []struct {
			target string
			h      func(http.ResponseWriter, *http.Request, DictStore)
		}{
			{target: "/iam/api/dicts:disable", h: handleDictsDisableAPI},
			{target: "/iam/api/dicts/values:disable", h: handleDictValuesDisableAPI},
			{target: "/iam/api/dicts/values:correct", h: handleDictValuesCorrectAPI},
		}
		for _, tc := range cases {
			req := dictAPIRequest(http.MethodPost, tc.target, []byte(`{"dict_code":"bad-code","code":"10","label":"X","disabled_on":"2026-01-01","correction_day":"2026-01-01","request_code":"r1"}`), true)
			rec := httptest.NewRecorder()
			tc.h(rec, req, dictStoreStub{})
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("target=%s status=%d", tc.target, rec.Code)
			}
		}

		reqCreateInvalid := dictAPIRequest(http.MethodPost, "/iam/api/dicts", []byte(`{"dict_code":"bad-code","name":"X","enabled_on":"2026-01-01","request_code":"r1"}`), true)
		recCreateInvalid := httptest.NewRecorder()
		handleDictsAPI(recCreateInvalid, reqCreateInvalid, dictStoreStub{})
		if recCreateInvalid.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recCreateInvalid.Code)
		}

		reqAudit := dictAPIRequest(http.MethodGet, "/iam/api/dicts/values/audit?dict_code=bad-code&code=10", nil, true)
		recAudit := httptest.NewRecorder()
		handleDictValuesAuditAPI(recAudit, reqAudit, dictStoreStub{})
		if recAudit.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", recAudit.Code)
		}
	})

	t.Run("dictErrorCode extra mappings", func(t *testing.T) {
		cases := map[error]string{
			errDictCodeInvalid:           "dict_code_invalid",
			errDictNameRequired:          "dict_name_required",
			errDictCodeConflict:          "dict_code_conflict",
			errDictDisabled:              "dict_disabled",
			errDictDisabledOnRequired:    "dict_disabled_on_required",
			errDictDisabledOnInvalidDate: "dict_disabled_on_required",
			errDictValueDictDisabled:     "dict_value_dict_disabled",
		}
		for in, want := range cases {
			if got := dictErrorCode(in); got != want {
				t.Fatalf("want=%q got=%q", want, got)
			}
		}
		if got := dictErrorCode(errDictEffectiveDayRequired); got != "invalid_as_of" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_DISABLED_ON_REQUIRED")); got != "dict_disabled_on_required" {
			t.Fatalf("got=%q", got)
		}
		if got := dictErrorCode(errors.New("DICT_ENABLED_ON_REQUIRED")); got != "dict_enabled_on_required" {
			t.Fatalf("got=%q", got)
		}
	})
}

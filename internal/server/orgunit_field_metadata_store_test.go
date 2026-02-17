package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type metadataScanRow struct {
	vals []any
	err  error
}

func (r metadataScanRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	for i := range dest {
		var v any
		if i < len(r.vals) {
			v = r.vals[i]
		}
		if err := assignScanValue(dest[i], v); err != nil {
			return err
		}
	}
	return nil
}

type metadataScanRows struct {
	records [][]any
	idx     int
	scanErr error
	err     error
}

func (r *metadataScanRows) Close()                        {}
func (r *metadataScanRows) Err() error                    { return r.err }
func (r *metadataScanRows) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }
func (r *metadataScanRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}
func (r *metadataScanRows) Next() bool {
	if r.idx >= len(r.records) {
		return false
	}
	r.idx++
	return true
}
func (r *metadataScanRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	rec := r.records[r.idx-1]
	for i := range dest {
		var v any
		if i < len(rec) {
			v = rec[i]
		}
		if err := assignScanValue(dest[i], v); err != nil {
			return err
		}
	}
	return nil
}
func (r *metadataScanRows) Values() ([]any, error) { return nil, nil }
func (r *metadataScanRows) RawValues() [][]byte    { return nil }
func (r *metadataScanRows) Conn() *pgx.Conn        { return nil }

func assignScanValue(dest any, value any) error {
	// pgx scans NULL into pointer targets; we mimic minimal behavior needed by tests.
	switch d := dest.(type) {
	case *string:
		if value == nil {
			*d = ""
			return nil
		}
		*d = value.(string)
		return nil
	case *int:
		if value == nil {
			*d = 0
			return nil
		}
		*d = value.(int)
		return nil
	case *int64:
		if value == nil {
			*d = 0
			return nil
		}
		*d = value.(int64)
		return nil
	case *bool:
		if value == nil {
			*d = false
			return nil
		}
		*d = value.(bool)
		return nil
	case *time.Time:
		if value == nil {
			*d = time.Time{}
			return nil
		}
		*d = value.(time.Time)
		return nil
	case *[]byte:
		if value == nil {
			*d = nil
			return nil
		}
		*d = append([]byte(nil), value.([]byte)...)
		return nil
	case **string:
		if value == nil {
			*d = nil
			return nil
		}
		switch v := value.(type) {
		case string:
			tmp := v
			*d = &tmp
		case *string:
			*d = v
		default:
			return errors.New("unsupported scan type for **string")
		}
		return nil
	case **int:
		if value == nil {
			*d = nil
			return nil
		}
		switch v := value.(type) {
		case int:
			tmp := v
			*d = &tmp
		case *int:
			*d = v
		default:
			return errors.New("unsupported scan type for **int")
		}
		return nil
	default:
		return errors.New("unsupported scan dest type")
	}
}

func TestOrgUnitFieldMetadataStore_PureHelpers(t *testing.T) {
	t.Run("cloneRawJSON", func(t *testing.T) {
		if got := string(cloneRawJSON(nil)); got != "{}" {
			t.Fatalf("json=%q", got)
		}
		if got := string(cloneRawJSON([]byte{})); got != "{}" {
			t.Fatalf("json=%q", got)
		}
		if got := string(cloneRawJSON([]byte(`{"a":1}`))); got != `{"a":1}` {
			t.Fatalf("json=%q", got)
		}
	})

	t.Run("cloneOptionalString", func(t *testing.T) {
		if got := cloneOptionalString(nil); got != nil {
			t.Fatalf("expected nil")
		}
		in := "x"
		got := cloneOptionalString(&in)
		if got == nil || *got != "x" {
			t.Fatalf("got=%v", got)
		}
	})

	t.Run("decodeStringMap", func(t *testing.T) {
		m, err := decodeStringMap(nil)
		if err != nil || len(m) != 0 {
			t.Fatalf("m=%v err=%v", m, err)
		}
		if _, err := decodeStringMap([]byte("{")); err == nil {
			t.Fatalf("expected error")
		}
		m, err = decodeStringMap([]byte(`{"a":1,"b":"x"}`))
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if _, ok := m["a"]; ok {
			t.Fatalf("expected non-string to be ignored")
		}
		if m["b"] != "x" {
			t.Fatalf("b=%q", m["b"])
		}
	})

	t.Run("decodeExtLabelsFromPayload", func(t *testing.T) {
		m, err := decodeExtLabelsFromPayload(nil)
		if err != nil || len(m) != 0 {
			t.Fatalf("m=%v err=%v", m, err)
		}
		if _, err := decodeExtLabelsFromPayload([]byte("{")); err == nil {
			t.Fatalf("expected error")
		}
		m, err = decodeExtLabelsFromPayload([]byte(`{"x":1}`))
		if err != nil || len(m) != 0 {
			t.Fatalf("m=%v err=%v", m, err)
		}
		m, err = decodeExtLabelsFromPayload([]byte(`{"ext_labels_snapshot":"bad"}`))
		if err != nil || len(m) != 0 {
			t.Fatalf("m=%v err=%v", m, err)
		}
		m, err = decodeExtLabelsFromPayload([]byte(`{"ext_labels_snapshot":{"a":"A","b":1}}`))
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if m["a"] != "A" {
			t.Fatalf("a=%q", m["a"])
		}
		if _, ok := m["b"]; ok {
			t.Fatalf("expected non-string label to be ignored")
		}
	})

	t.Run("parseOrgUnitExtQueryValue", func(t *testing.T) {
		if got, err := parseOrgUnitExtQueryValue("text", "  hi "); err != nil || got.(string) != "hi" {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if _, err := parseOrgUnitExtQueryValue("int", "bad"); err == nil {
			t.Fatalf("expected int error")
		}
		if got, err := parseOrgUnitExtQueryValue("int", "42"); err != nil || got.(int) != 42 {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if _, err := parseOrgUnitExtQueryValue("uuid", "bad"); err == nil {
			t.Fatalf("expected uuid error")
		}
		u := "00000000-0000-0000-0000-000000000001"
		if got, err := parseOrgUnitExtQueryValue("uuid", u); err != nil || got.(string) != u {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if got, err := parseOrgUnitExtQueryValue("bool", "1"); err != nil || got.(bool) != true {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if got, err := parseOrgUnitExtQueryValue("bool", "FALSE"); err != nil || got.(bool) != false {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if _, err := parseOrgUnitExtQueryValue("bool", "nope"); err == nil {
			t.Fatalf("expected bool error")
		}
		if got, err := parseOrgUnitExtQueryValue("date", "2026-01-01"); err != nil || got.(string) != "2026-01-01" {
			t.Fatalf("got=%v err=%v", got, err)
		}
		if _, err := parseOrgUnitExtQueryValue("date", "bad"); err == nil {
			t.Fatalf("expected date error")
		}
		if _, err := parseOrgUnitExtQueryValue("unknown", "x"); err == nil {
			t.Fatalf("expected unsupported error")
		}
	})

	t.Run("quoteSQLIdentifier", func(t *testing.T) {
		if got := quoteSQLIdentifier(`ext_str_01`); got != `"ext_str_01"` {
			t.Fatalf("got=%q", got)
		}
		if got := quoteSQLIdentifier(`a"b`); got != `"a""b"` {
			t.Fatalf("got=%q", got)
		}
	})

	t.Run("orgUnitFieldConfigEnabledAsOf", func(t *testing.T) {
		cfg := orgUnitTenantFieldConfig{EnabledOn: "2026-01-01"}
		if orgUnitFieldConfigEnabledAsOf(cfg, "") {
			t.Fatalf("expected false")
		}
		if orgUnitFieldConfigEnabledAsOf(orgUnitTenantFieldConfig{EnabledOn: ""}, "2026-01-01") {
			t.Fatalf("expected false")
		}
		if orgUnitFieldConfigEnabledAsOf(orgUnitTenantFieldConfig{EnabledOn: "2026-02-01"}, "2026-01-01") {
			t.Fatalf("expected false")
		}
		if !orgUnitFieldConfigEnabledAsOf(cfg, "2026-01-01") {
			t.Fatalf("expected true")
		}
		blank := ""
		cfg.DisabledOn = &blank
		if !orgUnitFieldConfigEnabledAsOf(cfg, "2026-01-02") {
			t.Fatalf("expected true")
		}
		disabledOn := "2026-02-01"
		cfg.DisabledOn = &disabledOn
		if !orgUnitFieldConfigEnabledAsOf(cfg, "2026-01-31") {
			t.Fatalf("expected true")
		}
		if orgUnitFieldConfigEnabledAsOf(cfg, "2026-02-01") {
			t.Fatalf("expected false")
		}
	})

	t.Run("denyReasonPriority and dedupDenyReasons", func(t *testing.T) {
		_ = denyReasonPriority("FORBIDDEN")
		_ = denyReasonPriority(orgUnitErrEventNotFound)
		_ = denyReasonPriority(orgUnitErrEventRescinded)
		_ = denyReasonPriority(orgUnitErrRootDeleteForbidden)
		_ = denyReasonPriority(orgUnitErrHasChildrenCannotDelete)
		_ = denyReasonPriority(orgUnitErrHasDependenciesCannotDelete)
		_ = denyReasonPriority(orgUnitErrStatusCorrectionUnsupported)
		_ = denyReasonPriority("OTHER")

		if got := dedupDenyReasons(nil); len(got) != 0 {
			t.Fatalf("len=%d", len(got))
		}
		in := []string{" ", "FORBIDDEN", orgUnitErrEventNotFound, orgUnitErrEventNotFound, orgUnitErrHasChildrenCannotDelete, "FORBIDDEN"}
		got := dedupDenyReasons(in)
		if stringsJoin(got) != stringsJoin([]string{"FORBIDDEN", orgUnitErrEventNotFound, orgUnitErrHasChildrenCannotDelete}) {
			t.Fatalf("got=%v", got)
		}
	})
}

func stringsJoin(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += "," + items[i]
	}
	return out
}

func TestScanOrgUnitTenantFieldConfig(t *testing.T) {
	now := time.Unix(123, 0).UTC()
	disabled := "2026-02-01"
	row := metadataScanRow{vals: []any{
		"org_type",
		"text",
		"DICT",
		[]byte(`{"dict_code":"org_type"}`),
		nil,
		"ext_str_01",
		"2026-01-01",
		disabled,
		now,
	}}
	got, err := scanOrgUnitTenantFieldConfig(row)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got.FieldKey != "org_type" || got.PhysicalCol != "ext_str_01" {
		t.Fatalf("got=%#v", got)
	}
	if got.DisabledOn == nil || *got.DisabledOn != disabled {
		t.Fatalf("disabled=%v", got.DisabledOn)
	}

	_, err = scanOrgUnitTenantFieldConfig(metadataScanRow{err: errors.New("scan")})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestTenantFieldConfigRequestExistsTx(t *testing.T) {
	ctx := context.Background()

	t.Run("no rows", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		ok, err := tenantFieldConfigRequestExistsTx(ctx, tx, "t1", "r1", "ENABLE")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("row error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		_, err := tenantFieldConfigRequestExistsTx(ctx, tx, "t1", "r1", "ENABLE")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("match", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"ENABLE"}}}
		ok, err := tenantFieldConfigRequestExistsTx(ctx, tx, "t1", "r1", "ENABLE")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{"DISABLE"}}}
		ok, err := tenantFieldConfigRequestExistsTx(ctx, tx, "t1", "r1", "ENABLE")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
}

func TestGetTenantFieldConfigByKeyTx(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("success disabled_on nil and raw config empty", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{vals: []any{
			"short_name",
			"text",
			"PLAIN",
			[]byte{},
			nil,
			"ext_str_01",
			"2026-01-01",
			nil,
			now,
		}}}
		got, err := getTenantFieldConfigByKeyTx(ctx, tx, "t1", "short_name")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if got.DisabledOn != nil {
			t.Fatalf("expected nil disabled_on")
		}
		if string(got.DataSourceConfig) != "{}" {
			t.Fatalf("config=%s", string(got.DataSourceConfig))
		}
	})

	t.Run("row error", func(t *testing.T) {
		tx := &stubTx{row: metadataScanRow{err: errors.New("boom")}}
		_, err := getTenantFieldConfigByKeyTx(ctx, tx, "t1", "short_name")
		if err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestOrgUnitPGStore_FieldConfigReadersAndWriters(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(123, 0).UTC()

	t.Run("ListTenantFieldConfigs begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs scan error", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{{"a"}}, scanErr: errors.New("scan")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs rows err", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{}, err: errors.New("rows")}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs commit error", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{}}, commitErr: errors.New("commit")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListTenantFieldConfigs(ctx, "t1"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListTenantFieldConfigs success", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{{
			"org_type",
			"text",
			"DICT",
			[]byte(`{"dict_code":"org_type"}`),
			nil,
			"ext_str_01",
			"2026-01-01",
			nil,
			now,
		}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, err := store.ListTenantFieldConfigs(ctx, "t1")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(items) != 1 || items[0].FieldKey != "org_type" {
			t.Fatalf("items=%v", items)
		}
	})

	t.Run("ListEnabledTenantFieldConfigsAsOf begin error", func(t *testing.T) {
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListEnabledTenantFieldConfigsAsOf exec error", func(t *testing.T) {
		tx := &stubTx{execErr: errors.New("exec")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListEnabledTenantFieldConfigsAsOf query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListEnabledTenantFieldConfigsAsOf commit error", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{}}, commitErr: errors.New("commit")}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ListEnabledTenantFieldConfigsAsOf success", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{{
			"org_type",
			"text",
			"DICT",
			[]byte(`{"dict_code":"org_type"}`),
			nil,
			"ext_str_01",
			"2026-01-01",
			nil,
			now,
		}}}}
		store := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		items, err := store.ListEnabledTenantFieldConfigsAsOf(ctx, "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(items) != 1 || items[0].FieldKey != "org_type" {
			t.Fatalf("items=%v", items)
		}
	})

	t.Run("listEnabledTenantFieldConfigsAsOfTx scan error", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{{"a"}}, scanErr: errors.New("scan")}}
		if _, err := listEnabledTenantFieldConfigsAsOfTx(ctx, tx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("listEnabledTenantFieldConfigsAsOfTx rows error", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{}, err: errors.New("rows")}}
		if _, err := listEnabledTenantFieldConfigsAsOfTx(ctx, tx, "t1", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("listEnabledTenantFieldConfigsAsOfTx success", func(t *testing.T) {
		tx := &stubTx{rows: &metadataScanRows{records: [][]any{{
			"org_type",
			"text",
			"DICT",
			[]byte(`{"dict_code":"org_type"}`),
			nil,
			"ext_str_01",
			"2026-01-01",
			nil,
			now,
		}}}}
		items, err := listEnabledTenantFieldConfigsAsOfTx(ctx, tx, "t1", "2026-01-01")
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if len(items) != 1 || items[0].FieldKey != "org_type" {
			t.Fatalf("items=%v", items)
		}
	})

	t.Run("GetEnabledTenantFieldConfigAsOf begin/exec/query/commit and success", func(t *testing.T) {
		storeBeginErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := storeBeginErr.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}

		txExecErr := &stubTx{execErr: errors.New("exec")}
		storeExecErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr, nil })}
		if _, _, err := storeExecErr.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}

		txNoRows := &stubTx{row: metadataScanRow{err: pgx.ErrNoRows}}
		storeNoRows := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txNoRows, nil })}
		_, ok, err := storeNoRows.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}

		txRowErr := &stubTx{row: metadataScanRow{err: errors.New("row")}}
		storeRowErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txRowErr, nil })}
		if _, _, err := storeRowErr.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}

		txCommitErr := &stubTx{
			row:       metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", nil, now}},
			commitErr: errors.New("commit"),
		}
		storeCommitErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
		if _, _, err := storeCommitErr.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01"); err == nil {
			t.Fatalf("expected error")
		}

		txOK := &stubTx{
			row: metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", nil, now}},
		}
		storeOK := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
		cfg, ok, err := storeOK.GetEnabledTenantFieldConfigAsOf(ctx, "t1", "org_type", "2026-01-01")
		if err != nil || !ok || cfg.FieldKey != "org_type" {
			t.Fatalf("cfg=%#v ok=%v err=%v", cfg, ok, err)
		}
	})

	t.Run("EnableTenantFieldConfig error branches and success", func(t *testing.T) {
		storeBeginErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := storeBeginErr.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txExecErr := &stubTx{execErr: errors.New("exec")}
		storeExecErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr, nil })}
		if _, _, err := storeExecErr.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txReqErr := &stubTx{row: metadataScanRow{err: errors.New("row")}}
		storeReqErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txReqErr, nil })}
		if _, _, err := storeReqErr.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txExec2Err := &stubTx{
			execErr:   errors.New("exec2"),
			execErrAt: 2,
			row:       metadataScanRow{err: pgx.ErrNoRows},
		}
		storeExec2Err := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExec2Err, nil })}
		if _, _, err := storeExec2Err.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txGetErr := &stubTx{
			row:  metadataScanRow{err: pgx.ErrNoRows},
			row2: metadataScanRow{err: errors.New("get")},
		}
		storeGetErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txGetErr, nil })}
		if _, _, err := storeGetErr.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txCommitErr := &stubTx{
			row:       metadataScanRow{vals: []any{"ENABLE"}},
			row2:      metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", nil, now}},
			commitErr: errors.New("commit"),
		}
		storeCommitErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
		if _, _, err := storeCommitErr.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txOK := &stubTx{
			row:  metadataScanRow{vals: []any{"ENABLE"}},
			row2: metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", nil, now}},
		}
		storeOK := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
		cfg, wasRetry, err := storeOK.EnableTenantFieldConfig(ctx, "t1", "org_type", "text", "DICT", json.RawMessage(`{}`), nil, "2026-01-01", "r1", uuid.NewString())
		if err != nil || !wasRetry || cfg.FieldKey != "org_type" {
			t.Fatalf("cfg=%#v retry=%v err=%v", cfg, wasRetry, err)
		}
	})

	t.Run("DisableTenantFieldConfig error branches and success", func(t *testing.T) {
		storeBeginErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, _, err := storeBeginErr.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txExecErr := &stubTx{execErr: errors.New("exec")}
		storeExecErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExecErr, nil })}
		if _, _, err := storeExecErr.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txReqErr := &stubTx{row: metadataScanRow{err: errors.New("row")}}
		storeReqErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txReqErr, nil })}
		if _, _, err := storeReqErr.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txExec2Err := &stubTx{
			execErr:   errors.New("exec2"),
			execErrAt: 2,
			row:       metadataScanRow{err: pgx.ErrNoRows},
		}
		storeExec2Err := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txExec2Err, nil })}
		if _, _, err := storeExec2Err.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txGetErr := &stubTx{
			row:  metadataScanRow{vals: []any{"DISABLE"}},
			row2: metadataScanRow{err: errors.New("get")},
		}
		storeGetErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txGetErr, nil })}
		if _, _, err := storeGetErr.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txCommitErr := &stubTx{
			row:       metadataScanRow{vals: []any{"DISABLE"}},
			row2:      metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", "2026-02-01", now}},
			commitErr: errors.New("commit"),
		}
		storeCommitErr := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txCommitErr, nil })}
		if _, _, err := storeCommitErr.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString()); err == nil {
			t.Fatalf("expected error")
		}

		txOK := &stubTx{
			row:  metadataScanRow{vals: []any{"DISABLE"}},
			row2: metadataScanRow{vals: []any{"org_type", "text", "DICT", []byte(`{}`), nil, "ext_str_01", "2026-01-01", "2026-02-01", now}},
		}
		storeOK := &orgUnitPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return txOK, nil })}
		cfg, wasRetry, err := storeOK.DisableTenantFieldConfig(ctx, "t1", "org_type", "2026-02-01", "r1", uuid.NewString())
		if err != nil || !wasRetry || cfg.DisabledOn == nil {
			t.Fatalf("cfg=%#v retry=%v err=%v", cfg, wasRetry, err)
		}
	})
}

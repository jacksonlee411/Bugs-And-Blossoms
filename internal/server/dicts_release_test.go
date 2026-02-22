package server

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDictBaselineReleaseHelpers(t *testing.T) {
	req := normalizeDictBaselineReleaseRequest(DictBaselineReleaseRequest{
		SourceTenantID: " ",
		TargetTenantID: " 00000000-0000-0000-0000-000000000001 ",
		AsOf:           " 2026-01-01 ",
		ReleaseID:      " rel-1 ",
		RequestID:      " req-1 ",
		Operator:       " op ",
		Initiator:      " init ",
	})
	if req.SourceTenantID != globalTenantID || req.TargetTenantID != "00000000-0000-0000-0000-000000000001" || req.AsOf != "2026-01-01" || req.ReleaseID != "rel-1" || req.RequestID != "req-1" || req.Operator != "op" || req.Initiator != "init" {
		t.Fatalf("normalized req=%+v", req)
	}

	valid := DictBaselineReleaseRequest{
		SourceTenantID: globalTenantID,
		TargetTenantID: "00000000-0000-0000-0000-000000000001",
		AsOf:           "2026-01-01",
		ReleaseID:      "rel-1",
		RequestID:      "req-1",
	}
	if err := validateDictBaselineReleaseRequest(valid, true); err != nil {
		t.Fatalf("validate err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{}, false); !errors.Is(err, errDictReleaseIDRequired) {
		t.Fatalf("err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{ReleaseID: "r"}, false); !errors.Is(err, errDictReleaseTargetRequired) {
		t.Fatalf("err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{ReleaseID: "r", TargetTenantID: "00000000-0000-0000-0000-000000000001"}, false); !errors.Is(err, errDictEffectiveDayRequired) {
		t.Fatalf("err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{ReleaseID: "r", TargetTenantID: "00000000-0000-0000-0000-000000000001", AsOf: "2026-01-01"}, true); !errors.Is(err, errDictRequestIDRequired) {
		t.Fatalf("err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{ReleaseID: "r", TargetTenantID: "00000000-0000-0000-0000-000000000001", AsOf: "2026-01-01", SourceTenantID: "bad", RequestID: "req"}, true); !errors.Is(err, errDictReleaseSourceInvalid) {
		t.Fatalf("err=%v", err)
	}
	if err := validateDictBaselineReleaseRequest(DictBaselineReleaseRequest{ReleaseID: "r", TargetTenantID: "bad", AsOf: "2026-01-01", SourceTenantID: globalTenantID, RequestID: "req"}, true); !errors.Is(err, errDictReleaseTargetRequired) {
		t.Fatalf("err=%v", err)
	}

	if got := dictBaselineReleaseRequestCode("req", "dict", 9); got != "req#dict#9" {
		t.Fatalf("request code=%q", got)
	}
	if got := dictBaselineReleaseTaskID("rel", "00000000-0000-0000-0000-000000000001", "2026-01-01"); got == "" {
		t.Fatal("task id empty")
	}
	conflicts := make([]DictBaselineReleaseConflict, 0)
	appendReleaseConflict(&conflicts, 1, DictBaselineReleaseConflict{Kind: "a"})
	appendReleaseConflict(&conflicts, 1, DictBaselineReleaseConflict{Kind: "b"})
	if len(conflicts) != 1 || conflicts[0].Kind != "a" {
		t.Fatalf("conflicts=%+v", conflicts)
	}
	key := joinDictValueKey("org_type", "10")
	dictCode, code := splitDictValueKey(key)
	if dictCode != "org_type" || code != "10" {
		t.Fatalf("split=%s %s", dictCode, code)
	}
	dictOnly, codeOnly := splitDictValueKey("org_type")
	if dictOnly != "org_type" || codeOnly != "" {
		t.Fatalf("split no sep=%s %s", dictOnly, codeOnly)
	}
}

func TestWithReleaseMetadata(t *testing.T) {
	req := DictBaselineReleaseRequest{
		SourceTenantID: globalTenantID,
		TargetTenantID: "00000000-0000-0000-0000-000000000001",
		AsOf:           "2026-01-01",
		ReleaseID:      "rel-1",
		Operator:       "u1",
	}

	got, err := withReleaseMetadata(nil, req, 1, "src-r1")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("unmarshal err=%v", err)
	}
	if _, ok := payload["release"]; !ok {
		t.Fatalf("payload=%v", payload)
	}

	got2, err := withReleaseMetadata([]byte(`{"label":"部门"}`), req, 2, "src-r2")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if err := json.Unmarshal(got2, &payload); err != nil {
		t.Fatalf("unmarshal err=%v", err)
	}
	if payload["label"] != "部门" {
		t.Fatalf("payload=%v", payload)
	}

	if _, err := withReleaseMetadata([]byte(`{`), req, 3, "src-r3"); !errors.Is(err, errDictReleasePayloadInvalid) {
		t.Fatalf("err=%v", err)
	}
}

func TestDictPGStorePreviewBaseline(t *testing.T) {
	ctx := context.Background()

	t.Run("begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		})}
		_, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("validate error", func(t *testing.T) {
		store := &dictPGStore{pool: &fakeBeginner{}}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{}); !errors.Is(err, errDictReleaseIDRequired) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("load source exec error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) {
			return &stubTx{execErr: errors.New("exec"), execErrAt: 1}, nil
		})}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source dict query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query"), queryErrAt: 1}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source dict scan error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{"org_type"}}, scanErr: errors.New("scan")}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source dict rows err", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source value query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &recordRows{records: [][]any{}},
			queryErr:   errors.New("query"),
			queryErrAt: 2,
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source value scan error", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{{"org_type"}}, scanErr: errors.New("scan")},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source value rows err", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{}, err: errors.New("rows")},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("target dict query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &recordRows{records: [][]any{}},
			rows2:      &recordRows{records: [][]any{}},
			queryErr:   errors.New("query"),
			queryErrAt: 3,
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &recordRows{records: [][]any{}},
			rows2:     &recordRows{records: [][]any{}},
			rows3:     &recordRows{records: [][]any{}},
			rows4:     &recordRows{records: [][]any{}},
			commitErr: errors.New("commit"),
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success with conflicts", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{"org_type", "Org Type"}}},
			rows2: &recordRows{records: [][]any{{"org_type", "10", "部门"}}},
			rows3: &recordRows{records: [][]any{{"org_type", "OrgTypeTarget"}}},
			rows4: &recordRows{records: [][]any{}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		preview, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
			MaxConflicts:   1,
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if preview.SourceDictCount != 1 || preview.SourceValueCount != 1 || preview.TargetDictCount != 1 || preview.TargetValueCount != 0 {
			t.Fatalf("preview=%+v", preview)
		}
		if preview.DictNameMismatchCount != 1 || preview.MissingValueCount != 1 || len(preview.Conflicts) != 1 {
			t.Fatalf("preview=%+v", preview)
		}
	})

	t.Run("success no conflicts with default limit", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{"org_type", "Org Type"}}},
			rows2: &recordRows{records: [][]any{{"org_type", "10", "部门"}}},
			rows3: &recordRows{records: [][]any{{"org_type", "Org Type"}}},
			rows4: &recordRows{records: [][]any{{"org_type", "10", "部门"}}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		preview, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if preview.MissingDictCount != 0 || preview.DictNameMismatchCount != 0 || preview.MissingValueCount != 0 || preview.ValueLabelMismatchCount != 0 || len(preview.Conflicts) != 0 {
			t.Fatalf("preview=%+v", preview)
		}
	})

	t.Run("success includes missing dict and value label mismatch", func(t *testing.T) {
		tx := &stubTx{
			rows: &recordRows{records: [][]any{
				{"org_type", "Org Type"},
				{"cost_center", "Cost Center"},
			}},
			rows2: &recordRows{records: [][]any{
				{"org_type", "10", "部门"},
			}},
			rows3: &recordRows{records: [][]any{
				{"org_type", "Org Type"},
			}},
			rows4: &recordRows{records: [][]any{
				{"org_type", "10", "单位"},
			}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		preview, err := store.PreviewBaseline(ctx, DictBaselineReleaseRequest{
			SourceTenantID: globalTenantID,
			TargetTenantID: "00000000-0000-0000-0000-000000000001",
			AsOf:           "2026-01-01",
			ReleaseID:      "rel-1",
		})
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if preview.MissingDictCount != 1 || preview.ValueLabelMismatchCount != 1 {
			t.Fatalf("preview=%+v", preview)
		}
	})
}

func TestDictPGStorePublishBaseline(t *testing.T) {
	ctx := context.Background()
	req := DictBaselineReleaseRequest{
		SourceTenantID: globalTenantID,
		TargetTenantID: "00000000-0000-0000-0000-000000000001",
		AsOf:           "2026-01-01",
		ReleaseID:      "rel-1",
		RequestID:      "req-1",
		Operator:       "00000000-0000-0000-0000-000000000001",
		Initiator:      "00000000-0000-0000-0000-000000000001",
	}

	t.Run("validate error", func(t *testing.T) {
		store := &dictPGStore{pool: &fakeBeginner{}}
		if _, err := store.PublishBaseline(ctx, DictBaselineReleaseRequest{}); !errors.Is(err, errDictReleaseIDRequired) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("begin error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("load source exec error", func(t *testing.T) {
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return &stubTx{execErr: errors.New("exec"), execErrAt: 1}, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("dict query error", func(t *testing.T) {
		tx := &stubTx{queryErr: errors.New("query"), queryErrAt: 1}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("dict scan error", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{{int64(1)}}, scanErr: errors.New("scan")}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("dict rows err", func(t *testing.T) {
		tx := &stubTx{rows: &recordRows{records: [][]any{}, err: errors.New("rows")}}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("value query error", func(t *testing.T) {
		tx := &stubTx{
			rows:       &recordRows{records: [][]any{}},
			queryErr:   errors.New("query"),
			queryErrAt: 2,
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("value scan error", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{{int64(1)}}, scanErr: errors.New("scan")},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("value rows err", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{}, err: errors.New("rows")},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("baseline not ready", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{}},
			rows2: &recordRows{records: [][]any{}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); !errors.Is(err, errDictBaselineNotReady) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("set target tenant error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2:     &recordRows{records: [][]any{}},
			execErr:   errors.New("exec"),
			execErrAt: 2,
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("release payload invalid", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{`)}}},
			rows2: &recordRows{records: [][]any{}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); !errors.Is(err, errDictReleasePayloadInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("release payload invalid on value event", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2: &recordRows{records: [][]any{{int64(2), "org_type", "10", "DICT_VALUE_CREATED", "1970-01-01", "src-r2", []byte(`{`)}}},
			row:   &stubRow{vals: []any{int64(11), false}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); !errors.Is(err, errDictReleasePayloadInvalid) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("submit dict event error", func(t *testing.T) {
		tx := &stubTx{
			rows:   &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2:  &recordRows{records: [][]any{}},
			rowErr: errors.New("row"),
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("submit value event error", func(t *testing.T) {
		tx := &stubTx{
			rows:    &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2:   &recordRows{records: [][]any{{int64(2), "org_type", "10", "DICT_VALUE_CREATED", "1970-01-01", "src-r2", []byte(`{"label":"部门"}`)}}},
			row:     &stubRow{vals: []any{int64(11), false}},
			row2Err: errors.New("row2"),
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &stubTx{
			rows:      &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2:     &recordRows{records: [][]any{}},
			row:       &stubRow{vals: []any{int64(11), false}},
			commitErr: errors.New("commit"),
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		if _, err := store.PublishBaseline(ctx, req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2: &recordRows{records: [][]any{{int64(2), "org_type", "10", "DICT_VALUE_CREATED", "1970-01-01", "src-r2", []byte(`{"label":"部门"}`)}}},
			row:   &stubRow{vals: []any{int64(11), false}},
			row2:  &stubRow{vals: []any{int64(12), true}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		result, err := store.PublishBaseline(ctx, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.Status != "succeeded" || result.DictEventsApplied != 1 || result.DictEventsRetried != 0 || result.ValueEventsApplied != 0 || result.ValueEventsRetried != 1 {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("success dict retry and value applied", func(t *testing.T) {
		tx := &stubTx{
			rows:  &recordRows{records: [][]any{{int64(1), "org_type", "DICT_CREATED", "1970-01-01", "src-r1", []byte(`{"name":"Org Type"}`)}}},
			rows2: &recordRows{records: [][]any{{int64(2), "org_type", "10", "DICT_VALUE_CREATED", "1970-01-01", "src-r2", []byte(`{"label":"部门"}`)}}},
			row:   &stubRow{vals: []any{int64(11), true}},
			row2:  &stubRow{vals: []any{int64(12), false}},
		}
		store := &dictPGStore{pool: beginnerFunc(func(context.Context) (pgx.Tx, error) { return tx, nil })}
		result, err := store.PublishBaseline(ctx, req)
		if err != nil {
			t.Fatalf("err=%v", err)
		}
		if result.DictEventsRetried != 1 || result.ValueEventsApplied != 1 {
			t.Fatalf("result=%+v", result)
		}
	})
}

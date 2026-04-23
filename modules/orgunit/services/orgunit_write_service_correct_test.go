package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

func TestCorrectMapsParentForMove(t *testing.T) {
	var captured map[string]any
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, orgCode string) (int, error) {
			switch orgCode {
			case "ROOT":
				return 10000001, nil
			case "PARENT":
				return 20000002, nil
			default:
				return 0, errors.New("unexpected org code")
			}
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, patch json.RawMessage, _ string, _ string) (string, error) {
			if err := json.Unmarshal(patch, &captured); err != nil {
				return "", err
			}
			return "corr", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "root",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			ParentOrgCode: new("parent"),
		},
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if captured["new_parent_org_node_key"] != mustEncodeTestOrgNodeKey(20000002) {
		t.Fatalf("expected new_parent_org_node_key mapped, got %#v", captured)
	}
	if _, ok := captured["parent_org_node_key"]; ok {
		t.Fatalf("unexpected parent_org_node_key in patch: %#v", captured)
	}
}

func TestCorrectRejectsNameOnMove(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			Name: new("Rename"),
		},
	})
	if err == nil || !httperr.IsBadRequest(err) {
		t.Fatalf("expected bad request, got %v", err)
	}
}

func TestCorrectManagerPernrNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "evt-1", nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
		Patch: OrgUnitCorrectionPatch{
			ManagerPernr: new("1001"),
		},
	})
	if err != nil {
		t.Fatalf("expected manager pernr to normalize without person lookup, got %v", err)
	}
}

func TestCorrectRequiresPatch(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
	}

	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req1",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchRequired {
		t.Fatalf("expected patch required, got %v", err)
	}
}

func TestCorrectInvalidDate(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-13-01",
		RequestID:           "req",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
		t.Fatalf("expected effective date invalid, got %v", err)
	}
}

func TestCorrectInvalidOrgCode(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             " \t ",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
		t.Fatalf("expected org code invalid, got %v", err)
	}
}

func TestCorrectRequestIDRequired(t *testing.T) {
	svc := NewOrgUnitWriteService(orgUnitWriteStoreStub{})
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
	})
	if err == nil || !httperr.IsBadRequest(err) || err.Error() != "request_id is required" {
		t.Fatalf("expected request_id required, got %v", err)
	}
}

func TestCorrectOrgCodeNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, orgunitpkg.ErrOrgCodeNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || err.Error() != errOrgCodeNotFound {
		t.Fatalf("expected org code not found, got %v", err)
	}
}

func TestCorrectResolveError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 0, errors.New("resolve")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
	})
	if err == nil || err.Error() != "resolve" {
		t.Fatalf("expected resolve error, got %v", err)
	}
}

func TestCorrectEventNotFound(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{}, ports.ErrOrgEventNotFound
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: new("Name"),
		},
	})
	if err == nil || err.Error() != errOrgEventNotFound {
		t.Fatalf("expected org event not found, got %v", err)
	}
}

func TestCorrectEventError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{}, errors.New("find")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: new("Name"),
		},
	})
	if err == nil || err.Error() != "find" {
		t.Fatalf("expected find error, got %v", err)
	}
}

func TestCorrectMarshalError(t *testing.T) {
	withMarshalJSON(t, func(any) ([]byte, error) {
		return nil, errors.New("marshal")
	})
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: new("Name"),
		},
	})
	if err == nil || err.Error() != "marshal" {
		t.Fatalf("expected marshal error, got %v", err)
	}
}

func TestCorrectSubmitError(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "", errors.New("submit")
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			Name: new("Name"),
		},
	})
	if err == nil || err.Error() != "submit" {
		t.Fatalf("expected submit error, got %v", err)
	}
}

func TestCorrectPolicyResolveError(t *testing.T) {
	orig := resolveOrgUnitMutationPolicyInWrite
	resolveOrgUnitMutationPolicyInWrite = func(OrgUnitMutationPolicyKey, OrgUnitMutationPolicyFacts) (OrgUnitMutationPolicyDecision, error) {
		return OrgUnitMutationPolicyDecision{}, errors.New("boom")
	}
	t.Cleanup(func() { resolveOrgUnitMutationPolicyInWrite = orig })

	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) { return 10000001, nil },
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate, EffectiveDate: "2026-01-01"}, nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	_, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			EffectiveDate: new("2026-01-01"),
		},
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected boom, got %v", err)
	}
}

func TestCorrectUsesPatchedEffectiveDate(t *testing.T) {
	store := orgUnitWriteStoreStub{
		resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
			return 10000001, nil
		},
		findEventByEffectiveFn: func(_ context.Context, _ string, _ int, _ string) (types.OrgUnitEvent, error) {
			return types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, nil
		},
		submitCorrectionFn: func(_ context.Context, _ string, _ int, _ string, _ json.RawMessage, _ string, _ string) (string, error) {
			return "corr", nil
		},
	}
	svc := NewOrgUnitWriteService(store)
	res, err := svc.Correct(context.Background(), "t1", CorrectOrgUnitRequest{
		OrgCode:             "ROOT",
		TargetEffectiveDate: "2026-01-01",
		RequestID:           "req",
		Patch: OrgUnitCorrectionPatch{
			EffectiveDate: new("2026-02-01"),
		},
	})
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if res.EffectiveDate != "2026-02-01" {
		t.Fatalf("expected patched effective date, got %v", res.EffectiveDate)
	}
}

func TestBuildCorrectionPatch(t *testing.T) {
	ctx := context.Background()
	emptyCfgs := []types.TenantFieldConfig{}

	t.Run("effective_date_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			EffectiveDate: new("2026-13-01"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errEffectiveDateInvalid {
			t.Fatalf("expected effective date invalid, got %v", err)
		}
	})

	t.Run("name_empty", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: new(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != "name is required" {
			t.Fatalf("expected name required, got %v", err)
		}
	})

	t.Run("name_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			Name: new("Name"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["name"] != "Name" || fields["name"] != "Name" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("name_rename", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Name: new("Name"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["new_name"] != "Name" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
	})

	t.Run("name_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, OrgUnitCorrectionPatch{
			Name: new("Name"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_empty_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new(" "),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_org_node_key"] != "" || fields["parent_org_code"] != "" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("parent_empty_move", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventMove}, OrgUnitCorrectionPatch{
			ParentOrgCode: new(" "),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("parent_invalid", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("A\n1"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errOrgCodeInvalid {
			t.Fatalf("expected org code invalid, got %v", err)
		}
	})

	t.Run("parent_not_found", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, orgunitpkg.ErrOrgCodeNotFound
			},
		})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err == nil || err.Error() != errParentNotFoundAsOf {
			t.Fatalf("expected parent not found, got %v", err)
		}
	})

	t.Run("parent_resolve_error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 0, errors.New("resolve")
			},
		})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err == nil || err.Error() != "resolve" {
			t.Fatalf("expected resolve error, got %v", err)
		}
	})

	t.Run("parent_success_create", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{
			resolveOrgIDFn: func(_ context.Context, _ string, _ string) (int, error) {
				return 20000002, nil
			},
		})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ParentOrgCode: new("PARENT"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["parent_org_node_key"] != mustEncodeTestOrgNodeKey(20000002) || fields["parent_org_code"] != "PARENT" {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("is_business_unit_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			IsBusinessUnit: new(true),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("is_business_unit_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventSetBusinessUnit}, OrgUnitCorrectionPatch{
			IsBusinessUnit: new(true),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["is_business_unit"] != true || fields["is_business_unit"] != true {
			t.Fatalf("unexpected patch map: %#v %#v", patchMap, fields)
		}
	})

	t.Run("manager_not_allowed", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			ManagerPernr: new("1001"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected patch field not allowed, got %v", err)
		}
	})

	t.Run("manager_resolve_error", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: new("bad"),
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errManagerPernrInvalid {
			t.Fatalf("expected invalid pernr, got %v", err)
		}
	})

	t.Run("manager_success", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventCreate}, OrgUnitCorrectionPatch{
			ManagerPernr: new("1001"),
		}, emptyCfgs)
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if patchMap["manager_pernr"] != "1001" {
			t.Fatalf("unexpected patch map: %#v", patchMap)
		}
		if fields["manager_pernr"] != "1001" {
			t.Fatalf("unexpected fields: %#v", fields)
		}
	})

	t.Run("ext blank field key rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{" ": "x"},
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("ext missing config rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"org_type": "10"},
		}, emptyCfgs)
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("ext config blank key is skipped", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"org_type": "10"},
		}, []types.TenantFieldConfig{{FieldKey: " "}})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})

	t.Run("ext custom plain field is accepted", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		patchMap, fields, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"x_cost_center": "CC-001"},
		}, []types.TenantFieldConfig{{FieldKey: "x_cost_center", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)}})
		if err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		gotExt, ok := patchMap["ext"].(map[string]any)
		if !ok || gotExt["x_cost_center"] != "CC-001" {
			t.Fatalf("patchMap ext=%v", patchMap["ext"])
		}
		gotFields, ok := fields["ext"].(map[string]any)
		if !ok || gotFields["x_cost_center"] != "CC-001" {
			t.Fatalf("fields ext=%v", fields["ext"])
		}
	})

	t.Run("ext config exists but invalid field_key rejects", func(t *testing.T) {
		svc := newWriteService(orgUnitWriteStoreStub{})
		_, _, _, err := svc.buildCorrectionPatch(ctx, "t1", types.OrgUnitEvent{EventType: types.OrgUnitEventRename}, OrgUnitCorrectionPatch{
			Ext: map[string]any{"unknown_field": "x"},
		}, []types.TenantFieldConfig{{FieldKey: "unknown_field", ValueType: "text", DataSourceType: "PLAIN", DataSourceConfig: json.RawMessage(`{}`)}})
		if err == nil || !httperr.IsBadRequest(err) || err.Error() != errPatchFieldNotAllowed {
			t.Fatalf("expected PATCH_FIELD_NOT_ALLOWED, got %v", err)
		}
	})
}

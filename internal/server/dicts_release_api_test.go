package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type dictReleaseStoreStub struct {
	previewFn func(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error)
	publishFn func(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleaseResult, error)
}

func (s dictReleaseStoreStub) PreviewBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
	if s.previewFn != nil {
		return s.previewFn(ctx, req)
	}
	return DictBaselineReleasePreview{}, nil
}

func (s dictReleaseStoreStub) PublishBaseline(ctx context.Context, req DictBaselineReleaseRequest) (DictBaselineReleaseResult, error) {
	if s.publishFn != nil {
		return s.publishFn(ctx, req)
	}
	return DictBaselineReleaseResult{}, nil
}

func TestHandleDictReleaseAPI_Coverage(t *testing.T) {
	makeReq := func(method string, target string, body string, withTenantCtx bool) *http.Request {
		req := httptest.NewRequest(method, target, stringsReader(body))
		if withTenantCtx {
			req = req.WithContext(withTenant(req.Context(), Tenant{
				ID:     "00000000-0000-0000-0000-000000000001",
				Domain: "localhost",
				Name:   "T1",
			}))
			req = req.WithContext(withPrincipal(req.Context(), Principal{
				ID:       "00000000-0000-0000-0000-000000000111",
				TenantID: "00000000-0000-0000-0000-000000000001",
				RoleSlug: "tenant-admin",
				Status:   "active",
			}))
		}
		return req
	}

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodGet, "/iam/api/dicts:release", "", true), dictReleaseStoreStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{}`, true), nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{}`, false), dictReleaseStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{`, true), dictReleaseStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("preview error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{"release_id":"r1","request_id":"req1","as_of":"2026-01-01"}`, true), dictReleaseStoreStub{
			previewFn: func(context.Context, DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				return DictBaselineReleasePreview{}, errDictReleaseIDRequired
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("preview conflict", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{"release_id":"r1","request_id":"req1","as_of":"2026-01-01","max_conflicts":1}`, true), dictReleaseStoreStub{
			previewFn: func(_ context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				if req.ReleaseID != "r1" || req.RequestID != "req1" || req.AsOf != "2026-01-01" || req.TargetTenantID == "" || req.Operator == "" || req.Initiator == "" || req.MaxConflicts != 1 {
					t.Fatalf("req=%+v", req)
				}
				return DictBaselineReleasePreview{
					ReleaseID:         "r1",
					SourceTenantID:    globalTenantID,
					TargetTenantID:    req.TargetTenantID,
					AsOf:              req.AsOf,
					MissingDictCount:  1,
					Conflicts:         []DictBaselineReleaseConflict{{Kind: "dict_missing", DictCode: "org_type"}},
					SourceDictCount:   1,
					TargetDictCount:   0,
					SourceValueCount:  1,
					TargetValueCount:  0,
					MissingValueCount: 1,
				}, nil
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("publish error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{"release_id":"r1","request_id":"req1","as_of":"2026-01-01"}`, true), dictReleaseStoreStub{
			previewFn: func(context.Context, DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				return DictBaselineReleasePreview{}, nil
			},
			publishFn: func(context.Context, DictBaselineReleaseRequest) (DictBaselineReleaseResult, error) {
				return DictBaselineReleaseResult{}, errDictBaselineNotReady
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("publish success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleaseAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release", `{"source_tenant_id":"00000000-0000-0000-0000-000000000000","release_id":"r1","request_id":"req1","as_of":"2026-01-01"}`, true), dictReleaseStoreStub{
			previewFn: func(context.Context, DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				return DictBaselineReleasePreview{}, nil
			},
			publishFn: func(_ context.Context, req DictBaselineReleaseRequest) (DictBaselineReleaseResult, error) {
				return DictBaselineReleaseResult{
					TaskID:         "task-1",
					ReleaseID:      req.ReleaseID,
					RequestID:      req.RequestID,
					SourceTenantID: req.SourceTenantID,
					TargetTenantID: req.TargetTenantID,
					AsOf:           req.AsOf,
					Status:         "succeeded",
				}, nil
			},
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body DictBaselineReleaseResult
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("err=%v", err)
		}
		if body.ReleaseID != "r1" || body.RequestID != "req1" || body.Status != "succeeded" {
			t.Fatalf("body=%+v", body)
		}
	})
}

func TestHandleDictReleasePreviewAPI_Coverage(t *testing.T) {
	makeReq := func(method string, target string, body string, withTenantCtx bool) *http.Request {
		req := httptest.NewRequest(method, target, stringsReader(body))
		if withTenantCtx {
			req = req.WithContext(withTenant(req.Context(), Tenant{
				ID:     "00000000-0000-0000-0000-000000000001",
				Domain: "localhost",
				Name:   "T1",
			}))
			req = req.WithContext(withPrincipal(req.Context(), Principal{
				ID:       "00000000-0000-0000-0000-000000000111",
				TenantID: "00000000-0000-0000-0000-000000000001",
				RoleSlug: "tenant-admin",
				Status:   "active",
			}))
		}
		return req
	}

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodGet, "/iam/api/dicts:release:preview", "", true), dictReleaseStoreStub{})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("store missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release:preview", `{}`, true), nil)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("tenant missing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release:preview", `{}`, false), dictReleaseStoreStub{})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("bad json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release:preview", `{`, true), dictReleaseStoreStub{})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("preview error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release:preview", `{"release_id":"r1","as_of":"2026-01-01"}`, true), dictReleaseStoreStub{
			previewFn: func(context.Context, DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				return DictBaselineReleasePreview{}, errDictReleaseSourceInvalid
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleDictReleasePreviewAPI(rec, makeReq(http.MethodPost, "/iam/api/dicts:release:preview", `{"source_tenant_id":"00000000-0000-0000-0000-000000000000","release_id":"r1","as_of":"2026-01-01","max_conflicts":3}`, true), dictReleaseStoreStub{
			previewFn: func(_ context.Context, req DictBaselineReleaseRequest) (DictBaselineReleasePreview, error) {
				if req.ReleaseID != "r1" || req.AsOf != "2026-01-01" || req.SourceTenantID != globalTenantID || req.MaxConflicts != 3 {
					t.Fatalf("req=%+v", req)
				}
				return DictBaselineReleasePreview{ReleaseID: "r1", AsOf: req.AsOf, SourceTenantID: req.SourceTenantID, TargetTenantID: req.TargetTenantID}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestWriteDictReleaseAPIError_Coverage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/iam/api/dicts:release", nil)
	rec := httptest.NewRecorder()
	writeDictReleaseAPIError(rec, req, errDictReleaseIDRequired, "x")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	writeDictReleaseAPIError(rec, req, errDictReleasePayloadInvalid, "x")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d", rec.Code)
	}

	rec = httptest.NewRecorder()
	writeDictReleaseAPIError(rec, req, errors.New("unknown"), "x")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", rec.Code)
	}
}

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type staffingErrReadCloser struct{}

func (staffingErrReadCloser) Read([]byte) (int, error) { return 0, errors.New("read") }
func (staffingErrReadCloser) Close() error             { return nil }

func TestFindDeprecatedField_Coverage(t *testing.T) {
	if got := findDeprecatedField(nil, "a"); got != "" {
		t.Fatalf("got=%q", got)
	}
	if got := findDeprecatedField(mustParseValues(t, "a=1&b=2"), "x", "b"); got != "b" {
		t.Fatalf("got=%q", got)
	}
}

func mustParseValues(t *testing.T, raw string) map[string][]string {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/?"+raw, nil)
	return r.URL.Query()
}

func TestHandlePositionsAPI_Coverage(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("invalid as_of", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=bad", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("deprecated query param rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01&org_unit_id=1", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{listFn: func(context.Context, string, string) ([]Position, error) { return nil, nil }})
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET list error stable => 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, &pgconn.PgError{Message: "STAFFING_LIST_FAILED"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET list error bad request => 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET list error invalid input => 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, &pgconn.PgError{Code: "22P02"}
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET list error default => 500", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return nil, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET parse orgunit id fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{PositionUUID: "pos1", OrgUnitID: "bad"}}, nil
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET resolver missing when org ids present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, nil, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{PositionUUID: "pos1", OrgUnitID: "10000001"}}, nil
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET resolve org codes error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{
			resolveOrgCodesFn: func(context.Context, string, []int) (map[int]string, error) {
				return nil, errors.New("resolve")
			},
		}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{PositionUUID: "pos1", OrgUnitID: "10000001"}}, nil
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET resolve org codes missing mapping", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{
			resolveOrgCodesFn: func(context.Context, string, []int) (map[int]string, error) {
				return map[int]string{}, nil
			},
		}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{PositionUUID: "pos1", OrgUnitID: "10000001"}}, nil
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET ok (no org ids path)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{{PositionUUID: "pos1", OrgUnitID: ""}}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("GET ok (dedup org ids)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/org/api/positions?as_of=2026-01-01", nil)
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			listFn: func(context.Context, string, string) ([]Position, error) {
				return []Position{
					{PositionUUID: "pos1", OrgUnitID: "10000001"},
					{PositionUUID: "pos2", OrgUnitID: "10000001"},
					{PositionUUID: "pos3", OrgUnitID: ""},
				}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST body read error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", nil)
		req.Body = staffingErrReadCloser{}
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST bad json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader("{bad"))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST deprecated keys rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"org_unit_id":"1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST deprecated position_id rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"position_id":"1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST decode struct error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":123}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST invalid effective_date", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"bad","org_code":"ORG-1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST invalid org_code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"bad\u007f"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST resolver missing when org_code present", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, nil, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST resolve org_id errors (invalid/not found/internal)", func(t *testing.T) {
		cases := []struct {
			name   string
			err    error
			status int
		}{
			{name: "invalid", err: orgunitpkg.ErrOrgCodeInvalid, status: http.StatusBadRequest},
			{name: "notfound", err: orgunitpkg.ErrOrgCodeNotFound, status: http.StatusNotFound},
			{name: "internal", err: errors.New("boom"), status: http.StatusInternalServerError},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1"}`))
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
				rec := httptest.NewRecorder()
				handlePositionsAPI(rec, req, staffingOrgStoreStub{
					resolveOrgIDFn: func(context.Context, string, string) (int, error) { return 0, tc.err },
				}, positionStoreStub{
					createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
						return Position{}, nil
					},
					updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
						return Position{}, nil
					},
				})
				if rec.Code != tc.status {
					t.Fatalf("status=%d", rec.Code)
				}
			})
		}
	})

	t.Run("POST org_code missing and position_uuid missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST create conflict", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1","name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Message: "STAFFING_IDEMPOTENCY_REUSED"}
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusConflict {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST create stable error => 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1","name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Message: "STAFFING_CREATE_FAILED"}
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST create invalid input => 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1","name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Code: "22P02", Message: "invalid input syntax for type uuid"}
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST update bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"position_uuid":"pos1","name":"A","effective_date":"2026-01-01"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, newBadRequestError("bad")
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST update stable error => 422", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"position_uuid":"pos1","name":"A","effective_date":"2026-01-01"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, &pgconn.PgError{Message: "STAFFING_UPDATE_FAILED"}
			},
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST update internal error => 500", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"position_uuid":"pos1","name":"A","effective_date":"2026-01-01"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, errors.New("boom")
			},
		})
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("POST update ok, then org_code resolve errors and ok", func(t *testing.T) {
		type orgResolveCase struct {
			name   string
			p      Position
			org    OrgUnitCodeResolver
			status int
		}
		cases := []orgResolveCase{
			{name: "resolver missing", p: Position{PositionUUID: "pos1", OrgUnitID: "10000001", EffectiveAt: "2026-01-01"}, org: nil, status: http.StatusInternalServerError},
			{name: "orgunit id invalid", p: Position{PositionUUID: "pos1", OrgUnitID: "bad", EffectiveAt: "2026-01-01"}, org: staffingOrgStoreStub{}, status: http.StatusInternalServerError},
			{name: "resolve org_code failed", p: Position{PositionUUID: "pos1", OrgUnitID: "10000001", EffectiveAt: "2026-01-01"}, org: staffingOrgStoreStub{
				resolveOrgCodeFn: func(context.Context, string, int) (string, error) { return "", errors.New("boom") },
			}, status: http.StatusInternalServerError},
			{name: "ok", p: Position{PositionUUID: "pos1", OrgUnitID: "10000001", EffectiveAt: "2026-01-01"}, org: staffingOrgStoreStub{}, status: http.StatusOK},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				body := bytes.NewBufferString(`{"position_uuid":"pos1","name":"A","effective_date":"2026-01-01"}`)
				req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", body)
				req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
				rec := httptest.NewRecorder()
				handlePositionsAPI(rec, req, tc.org, positionStoreStub{
					createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
						return Position{}, nil
					},
					updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
						return tc.p, nil
					},
				})
				if rec.Code != tc.status {
					t.Fatalf("status=%d", rec.Code)
				}
			})
		}
	})

	t.Run("POST create ok and body returns json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"effective_date":"2026-01-01","org_code":"ORG-1","job_profile_uuid":"jp1","name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{PositionUUID: "pos1", OrgUnitID: "10000001", EffectiveAt: "2026-01-01"}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		var got staffingPositionAPIResponse
		if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("POST update ok with empty OrgUnitID does not resolve org code", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", strings.NewReader(`{"position_uuid":"pos1","effective_date":"2026-01-01","name":"A"}`))
		req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
		rec := httptest.NewRecorder()
		handlePositionsAPI(rec, req, nil, positionStoreStub{
			createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
				return Position{}, nil
			},
			updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
				return Position{PositionUUID: "pos1", OrgUnitID: "", EffectiveAt: "2026-01-01"}, nil
			},
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestHandlePositionsAPI_Post_ReadAllBranchUsesRequestBody(t *testing.T) {
	// Ensure the io.ReadAll(r.Body) path is exercised with a non-nil body.
	req := httptest.NewRequest(http.MethodPost, "/org/api/positions?as_of=2026-01-01", io.NopCloser(bytes.NewReader([]byte(`{"position_uuid":"pos1","effective_date":"2026-01-01","name":"A"}`))))
	req = req.WithContext(withTenant(req.Context(), Tenant{ID: "t1"}))
	rec := httptest.NewRecorder()
	handlePositionsAPI(rec, req, staffingOrgStoreStub{}, positionStoreStub{
		createFn: func(context.Context, string, string, string, string, string, string) (Position, error) {
			return Position{}, nil
		},
		updateFn: func(context.Context, string, string, string, string, string, string, string, string, string) (Position, error) {
			return Position{PositionUUID: "pos1", OrgUnitID: "", EffectiveAt: "2026-01-01"}, nil
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	personmodule "github.com/jacksonlee411/Bugs-And-Blossoms/modules/person"
	personservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/person/services"
)

type Person = personservices.Person

type PersonOption = personservices.PersonOption

type PersonStore interface {
	ListPersons(ctx context.Context, tenantID string) ([]Person, error)
	CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error)
	FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error)
	ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]PersonOption, error)
}

func newPersonPGStore(pool pgBeginner) PersonStore {
	return personmodule.NewPGStore(pool)
}

func normalizePernr(raw string) (string, error) {
	return personservices.NormalizePernr(raw)
}

func newPersonMemoryStore() PersonStore {
	return personmodule.NewMemoryStore()
}

func handlePersonOptionsAPI(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	items, err := store.ListPersonOptions(r.Context(), tenant.ID, q, limit)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "PERSON_INTERNAL", "person internal")
		return
	}

	type item struct {
		PersonUUID  string `json:"person_uuid"`
		Pernr       string `json:"pernr"`
		DisplayName string `json:"display_name"`
	}
	type resp struct {
		Items []item `json:"items"`
	}

	out := resp{Items: make([]item, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, item{
			PersonUUID:  it.UUID,
			Pernr:       it.Pernr,
			DisplayName: it.DisplayName,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(out)
}

func handlePersonsAPI(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	switch r.Method {
	case http.MethodGet:
		items, err := store.ListPersons(r.Context(), tenant.ID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "PERSON_INTERNAL", "person internal")
			return
		}
		type item struct {
			PersonUUID  string `json:"person_uuid"`
			Pernr       string `json:"pernr"`
			DisplayName string `json:"display_name"`
			Status      string `json:"status"`
			CreatedAt   string `json:"created_at"`
		}
		out := make([]item, 0, len(items))
		for _, it := range items {
			out = append(out, item{
				PersonUUID:  it.UUID,
				Pernr:       it.Pernr,
				DisplayName: it.DisplayName,
				Status:      it.Status,
				CreatedAt:   it.CreatedAt.UTC().Format(time.RFC3339Nano),
			})
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tenant_id": tenant.ID,
			"persons":   out,
		})
		return

	case http.MethodPost:
		var req struct {
			Pernr       string `json:"pernr"`
			DisplayName string `json:"display_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "bad_json", "bad json")
			return
		}
		pernr := strings.TrimSpace(req.Pernr)
		displayName := strings.TrimSpace(req.DisplayName)
		p, err := store.CreatePerson(r.Context(), tenant.ID, pernr, displayName)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "PERSON_CREATE_FAILED", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"person_uuid":  p.UUID,
			"pernr":        p.Pernr,
			"display_name": p.DisplayName,
			"status":       p.Status,
		})
		return

	default:
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func handlePersonByPernrAPI(w http.ResponseWriter, r *http.Request, store PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	raw := strings.TrimSpace(r.URL.Query().Get("pernr"))
	if raw == "" {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "PERSON_PERNR_INVALID", "pernr invalid")
		return
	}
	if _, err := normalizePernr(raw); err != nil {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "PERSON_PERNR_INVALID", "pernr invalid")
		return
	}

	p, err := store.FindPersonByPernr(r.Context(), tenant.ID, raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, "PERSON_NOT_FOUND", "person not found")
			return
		}
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "PERSON_INTERNAL", "person internal")
		return
	}

	type resp struct {
		PersonUUID  string `json:"person_uuid"`
		Pernr       string `json:"pernr"`
		DisplayName string `json:"display_name"`
		Status      string `json:"status"`
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp{
		PersonUUID:  p.UUID,
		Pernr:       p.Pernr,
		DisplayName: p.DisplayName,
		Status:      p.Status,
	})
}

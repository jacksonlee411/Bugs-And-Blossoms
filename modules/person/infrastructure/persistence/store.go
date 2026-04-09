package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	personservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/person/services"
)

type pgBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type PGStore struct {
	pool pgBeginner
}

type MemoryStore struct {
	byTenant map[string][]personservices.Person
}

func NewPGStore(pool pgBeginner) *PGStore {
	return &PGStore{pool: pool}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byTenant: make(map[string][]personservices.Person),
	}
}

func (s *PGStore) ListPersons(ctx context.Context, tenantID string) ([]personservices.Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT person_uuid::text, pernr, display_name, status, created_at
FROM person.persons
WHERE tenant_uuid = $1::uuid
ORDER BY created_at DESC, person_uuid DESC
LIMIT 200
`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []personservices.Person
	for rows.Next() {
		var p personservices.Person
		if err := rows.Scan(&p.UUID, &p.Pernr, &p.DisplayName, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PGStore) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (personservices.Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return personservices.Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return personservices.Person{}, err
	}

	prepared, err := personservices.PrepareCreatePerson(pernr, displayName)
	if err != nil {
		return personservices.Person{}, err
	}

	var p personservices.Person
	p.Pernr = prepared.Pernr
	p.DisplayName = prepared.DisplayName
	p.Status = "active"
	if err := tx.QueryRow(ctx, `
	INSERT INTO person.persons (tenant_uuid, pernr, display_name, status)
	VALUES ($1::uuid, $2::text, $3::text, 'active')
	RETURNING person_uuid::text, created_at
	`, tenantID, prepared.Pernr, prepared.DisplayName).Scan(&p.UUID, &p.CreatedAt); err != nil {
		return personservices.Person{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return personservices.Person{}, err
	}
	return p, nil
}

func (s *PGStore) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (personservices.Person, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return personservices.Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return personservices.Person{}, err
	}

	canonical, err := personservices.PrepareFindPersonByPernr(pernr)
	if err != nil {
		return personservices.Person{}, err
	}

	var p personservices.Person
	if err := tx.QueryRow(ctx, `
SELECT person_uuid::text, pernr, display_name, status, created_at
FROM person.persons
WHERE tenant_uuid = $1::uuid AND pernr = $2::text
`, tenantID, canonical).Scan(&p.UUID, &p.Pernr, &p.DisplayName, &p.Status, &p.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return personservices.Person{}, pgx.ErrNoRows
		}
		return personservices.Person{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return personservices.Person{}, err
	}
	return p, nil
}

func (s *PGStore) ListPersonOptions(ctx context.Context, tenantID string, q string, limit int) ([]personservices.PersonOption, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	q = strings.TrimSpace(q)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	pernrPrefix := ""
	if q != "" {
		if canonical, err := personservices.NormalizePernr(q); err == nil {
			pernrPrefix = canonical
		}
	}

	rows, err := tx.Query(ctx, `
SELECT person_uuid::text, pernr, display_name
FROM person.persons
WHERE tenant_uuid = $1::uuid
  AND (
    $2::text = '' OR pernr LIKE ($2::text || '%')
    OR display_name ILIKE ('%' || $3::text || '%')
  )
ORDER BY display_name ASC, pernr ASC
LIMIT $4::int
`, tenantID, pernrPrefix, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []personservices.PersonOption
	for rows.Next() {
		var p personservices.PersonOption
		if err := rows.Scan(&p.UUID, &p.Pernr, &p.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *MemoryStore) ListPersons(_ context.Context, tenantID string) ([]personservices.Person, error) {
	return append([]personservices.Person(nil), s.byTenant[tenantID]...), nil
}

func (s *MemoryStore) CreatePerson(_ context.Context, tenantID string, pernr string, displayName string) (personservices.Person, error) {
	prepared, err := personservices.PrepareCreatePerson(pernr, displayName)
	if err != nil {
		return personservices.Person{}, err
	}
	for _, p := range s.byTenant[tenantID] {
		if p.Pernr == prepared.Pernr {
			return personservices.Person{}, errors.New("pernr already exists")
		}
	}
	p := personservices.Person{
		UUID:        "person-" + prepared.Pernr,
		Pernr:       prepared.Pernr,
		DisplayName: prepared.DisplayName,
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	s.byTenant[tenantID] = append(s.byTenant[tenantID], p)
	return p, nil
}

func (s *MemoryStore) FindPersonByPernr(_ context.Context, tenantID string, pernr string) (personservices.Person, error) {
	canonical, err := personservices.PrepareFindPersonByPernr(pernr)
	if err != nil {
		return personservices.Person{}, err
	}
	for _, p := range s.byTenant[tenantID] {
		if p.Pernr == canonical {
			return p, nil
		}
	}
	return personservices.Person{}, pgx.ErrNoRows
}

func (s *MemoryStore) ListPersonOptions(_ context.Context, tenantID string, q string, limit int) ([]personservices.PersonOption, error) {
	q = strings.TrimSpace(q)
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var out []personservices.PersonOption
	for _, p := range s.byTenant[tenantID] {
		if q == "" || strings.Contains(strings.ToLower(p.DisplayName), strings.ToLower(q)) || strings.HasPrefix(p.Pernr, strings.TrimLeft(q, "0")) {
			out = append(out, personservices.PersonOption{UUID: p.UUID, Pernr: p.Pernr, DisplayName: p.DisplayName})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

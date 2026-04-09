package persistence

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	setidresolver "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/setid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

type SetIDPGBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type SetIDPGStore struct {
	Pool SetIDPGBeginner
}

func NewSetIDPGStore(pool SetIDPGBeginner) *SetIDPGStore {
	return &SetIDPGStore{Pool: pool}
}

func (s *SetIDPGStore) withTx(ctx context.Context, tenantID string, fn func(tx pgx.Tx) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SetIDPGStore) EnsureBootstrap(ctx context.Context, tenantID string, initiatorID string) error {
	if err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, initiatorID)
		return err
	}); err != nil {
		return err
	}
	return s.EnsureGlobalShareSetID(ctx, initiatorID)
}

func (s *SetIDPGStore) ListSetIDs(ctx context.Context, tenantID string) ([]ports.SetID, error) {
	var out []ports.SetID
	if err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
	SELECT setid, name, status
	FROM orgunit.setids
	WHERE tenant_uuid = $1::uuid
ORDER BY setid ASC
`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r ports.SetID
			if err := rows.Scan(&r.SetID, &r.Name, &r.Status); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	globalSetids, err := s.ListGlobalSetIDs(ctx)
	if err != nil {
		return nil, err
	}
	out = append(out, globalSetids...)
	sort.Slice(out, func(i, j int) bool { return out[i].SetID < out[j].SetID })
	return out, nil
}

func (s *SetIDPGStore) CreateSetID(ctx context.Context, tenantID string, setID string, name string, effectiveDate string, requestID string, initiatorID string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		eventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
SELECT orgunit.submit_setid_event(
  $1::uuid,
  $2::uuid,
  'CREATE',
  $3::text,
  jsonb_build_object('name', $4::text, 'effective_date', $5::text),
  $6::text,
  $7::uuid
);
`, eventID, tenantID, setID, name, effectiveDate, requestID, initiatorID)
		return err
	})
}

func (s *SetIDPGStore) ListSetIDBindings(ctx context.Context, tenantID string, asOfDate string) ([]ports.SetIDBindingRow, error) {
	var out []ports.SetIDBindingRow
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT
  org_id::text,
  setid,
  lower(validity)::text,
  COALESCE(upper(validity)::text, '')
FROM orgunit.setid_binding_versions
WHERE tenant_uuid = $1::uuid
  AND validity @> $2::date
ORDER BY org_id::text ASC
`, tenantID, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r ports.SetIDBindingRow
			if err := rows.Scan(&r.OrgUnitID, &r.SetID, &r.ValidFrom, &r.ValidTo); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *SetIDPGStore) BindSetID(ctx context.Context, tenantID string, orgUnitID string, effectiveDate string, setID string, requestID string, initiatorID string) error {
	return s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, initiatorID); err != nil {
			return err
		}
		if _, err := parseOrgID8SetID(orgUnitID); err != nil {
			return err
		}
		eventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
SELECT orgunit.submit_setid_binding_event(
  $1::uuid,
  $2::uuid,
  $3::int,
  $4::date,
  $5::text,
  $6::text,
  $7::uuid
);
`, eventID, tenantID, orgUnitID, effectiveDate, setID, requestID, initiatorID)
		return err
	})
}

func (s *SetIDPGStore) ResolveSetID(ctx context.Context, tenantID string, orgUnitID string, asOfDate string) (string, error) {
	var out string
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		resolved, err := setidresolver.Resolve(ctx, tx, tenantID, orgUnitID, asOfDate)
		if err != nil {
			return err
		}
		out = strings.ToUpper(strings.TrimSpace(resolved))
		return nil
	})
	return out, err
}

func (s *SetIDPGStore) EnsureGlobalShareSetID(ctx context.Context, initiatorID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var globalTenantID string
	if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&globalTenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, globalTenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_actor_scope', 'saas', true);`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_share_read', 'on', true);`); err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_global_setid_event(
  $1::uuid,
  $2::uuid,
  'BOOTSTRAP',
  'SHARE',
  jsonb_build_object('name', 'Shared'),
  'bootstrap:share',
  $3::uuid
);
`, eventID, globalTenantID, initiatorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SetIDPGStore) ListGlobalSetIDs(ctx context.Context) ([]ports.SetID, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var globalTenantID string
	if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&globalTenantID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, globalTenantID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_share_read', 'on', true);`); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
SELECT setid, name, status
FROM orgunit.global_setids
WHERE tenant_uuid = $1::uuid
ORDER BY setid ASC
`, globalTenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.SetID
	for rows.Next() {
		var r ports.SetID
		if err := rows.Scan(&r.SetID, &r.Name, &r.Status); err != nil {
			return nil, err
		}
		r.IsShared = true
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *SetIDPGStore) CreateGlobalSetID(ctx context.Context, name string, requestID string, initiatorID string, actorScope string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var globalTenantID string
	if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&globalTenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, globalTenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_actor_scope', $1, true);`, actorScope); err != nil {
		return err
	}

	eventID, err := uuidv7.NewString()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_global_setid_event(
  $1::uuid,
  $2::uuid,
  'CREATE',
  'SHARE',
  jsonb_build_object('name', $3::text),
  $4::text,
  $5::uuid
);
`, eventID, globalTenantID, name, requestID, initiatorID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SetIDPGStore) ListScopeCodes(ctx context.Context, tenantID string) ([]ports.ScopeCode, error) {
	var out []ports.ScopeCode
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT scope_code, owner_module, share_mode, is_stable
FROM orgunit.scope_code_registry()
ORDER BY scope_code ASC
`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var r ports.ScopeCode
			if err := rows.Scan(&r.ScopeCode, &r.OwnerModule, &r.ShareMode, &r.IsStable); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *SetIDPGStore) CreateScopePackage(ctx context.Context, tenantID string, scopeCode string, packageCode string, ownerSetID string, name string, effectiveDate string, requestID string, initiatorID string) (ports.ScopePackage, error) {
	var out ports.ScopePackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT orgunit.ensure_setid_bootstrap($1::uuid, $2::uuid);`, tenantID, initiatorID); err != nil {
			return err
		}
		var packageID string
		if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&packageID); err != nil {
			return err
		}
		eventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_scope_package_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  jsonb_build_object('package_code', $6::text, 'owner_setid', $7::text, 'name', $8::text),
  $9::text,
  $10::uuid
);
`, eventID, tenantID, scopeCode, packageID, effectiveDate, packageCode, ownerSetID, name, requestID, initiatorID); err != nil {
			return err
		}
		subEventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		subRequestID := requestID
		if subRequestID != "" {
			subRequestID += ":owner-sub"
		}
		if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_scope_subscription_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::text,
  $5::uuid,
  $6::uuid,
  'SUBSCRIBE',
  $7::date,
  $8::text,
  $9::uuid
);
`, subEventID, tenantID, ownerSetID, scopeCode, packageID, tenantID, effectiveDate, subRequestID, initiatorID); err != nil {
			return err
		}
		pkg, err := FetchScopePackageByID(ctx, tx, tenantID, packageID, false)
		if errors.Is(err, pgx.ErrNoRows) {
			var existingID string
			if err := tx.QueryRow(ctx, `
SELECT package_id::text
FROM orgunit.setid_scope_package_events
WHERE tenant_uuid = $1::uuid AND request_id = $2::text
ORDER BY id DESC
LIMIT 1
`, tenantID, requestID).Scan(&existingID); err != nil {
				return err
			}
			pkg, err = FetchScopePackageByID(ctx, tx, tenantID, existingID, false)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		out = pkg
		return nil
	})
	return out, err
}

func (s *SetIDPGStore) DisableScopePackage(ctx context.Context, tenantID string, packageID string, effectiveDate string, requestID string, initiatorID string) (ports.ScopePackage, error) {
	var out ports.ScopePackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		eventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_scope_package_event(
  $1::uuid,
  $2::uuid,
  (SELECT scope_code FROM orgunit.setid_scope_packages WHERE tenant_uuid = $2::uuid AND package_id = $3::uuid),
  $3::uuid,
  'DISABLE',
  $4::date,
  '{}'::jsonb,
  $5::text,
  $6::uuid
);
`, eventID, tenantID, packageID, effectiveDate, requestID, initiatorID); err != nil {
			return err
		}
		pkg, err := FetchScopePackageByID(ctx, tx, tenantID, packageID, false)
		if err != nil {
			return err
		}
		out = pkg
		return nil
	})
	return out, err
}

func (s *SetIDPGStore) ListScopePackages(ctx context.Context, tenantID string, scopeCode string) ([]ports.ScopePackage, error) {
	var out []ports.ScopePackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT p.package_id::text,
       p.scope_code,
       p.package_code,
       p.owner_setid,
       p.name,
       p.status,
       COALESCE(lower(v.validity)::text, '') AS effective_date,
       COALESCE(p.updated_at::text, '') AS updated_at
FROM orgunit.setid_scope_packages p
LEFT JOIN LATERAL (
  SELECT validity
  FROM orgunit.setid_scope_package_versions v
  WHERE v.tenant_uuid = p.tenant_uuid
    AND v.package_id = p.package_id
  ORDER BY v.last_event_id DESC
  LIMIT 1
) v ON true
WHERE p.tenant_uuid = $1::uuid AND p.scope_code = $2::text
ORDER BY package_code ASC
`, tenantID, scopeCode)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r ports.ScopePackage
			if err := rows.Scan(&r.PackageID, &r.ScopeCode, &r.PackageCode, &r.OwnerSetID, &r.Name, &r.Status, &r.EffectiveDate, &r.UpdatedAt); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *SetIDPGStore) ListOwnedScopePackages(ctx context.Context, tenantID string, scopeCode string, asOfDate string) ([]ports.OwnedScopePackage, error) {
	var out []ports.OwnedScopePackage
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
SELECT p.package_id::text,
       p.scope_code,
       p.package_code,
       p.owner_setid,
       p.name,
       v.status,
       lower(v.validity)::text
FROM orgunit.setid_scope_packages p
JOIN orgunit.setid_scope_package_versions v
  ON v.tenant_uuid = p.tenant_uuid
 AND v.package_id = p.package_id
 AND v.validity @> $3::date
JOIN orgunit.setids s
  ON s.tenant_uuid = p.tenant_uuid
 AND s.setid = p.owner_setid
 AND s.status = 'active'
WHERE p.tenant_uuid = $1::uuid
  AND p.scope_code = $2::text
  AND v.status = 'active'
ORDER BY p.package_code ASC
`, tenantID, scopeCode, asOfDate)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r ports.OwnedScopePackage
			if err := rows.Scan(&r.PackageID, &r.ScopeCode, &r.PackageCode, &r.OwnerSetID, &r.Name, &r.Status, &r.EffectiveDate); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	return out, err
}

func (s *SetIDPGStore) CreateScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, packageID string, packageOwner string, effectiveDate string, requestID string, initiatorID string) (ports.ScopeSubscription, error) {
	var out ports.ScopeSubscription
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		eventID, err := uuidv7.NewString()
		if err != nil {
			return err
		}
		ownerTenantID := tenantID
		if strings.EqualFold(packageOwner, "global") {
			if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&ownerTenantID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_scope_subscription_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::text,
  $5::uuid,
  $6::uuid,
  'SUBSCRIBE',
  $7::date,
  $8::text,
  $9::uuid
);
`, eventID, tenantID, setID, scopeCode, packageID, ownerTenantID, effectiveDate, requestID, initiatorID); err != nil {
			return err
		}
		sub, err := FetchScopeSubscription(ctx, tx, tenantID, setID, scopeCode, effectiveDate)
		if err != nil {
			return err
		}
		out = sub
		return nil
	})
	return out, err
}

func (s *SetIDPGStore) GetScopeSubscription(ctx context.Context, tenantID string, setID string, scopeCode string, asOfDate string) (ports.ScopeSubscription, error) {
	var out ports.ScopeSubscription
	err := s.withTx(ctx, tenantID, func(tx pgx.Tx) error {
		sub, err := FetchScopeSubscription(ctx, tx, tenantID, setID, scopeCode, asOfDate)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("SCOPE_SUBSCRIPTION_MISSING")
			}
			return err
		}
		out = sub
		return nil
	})
	return out, err
}

func (s *SetIDPGStore) CreateGlobalScopePackage(ctx context.Context, scopeCode string, packageCode string, name string, effectiveDate string, requestID string, initiatorID string, actorScope string) (ports.ScopePackage, error) {
	var out ports.ScopePackage
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var globalTenantID string
	if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&globalTenantID); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, globalTenantID); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_share_read', 'on', true);`); err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_actor_scope', $1, true);`, strings.ToLower(actorScope)); err != nil {
		return out, err
	}
	var packageID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&packageID); err != nil {
		return out, err
	}
	eventID, err := uuidv7.NewString()
	if err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `
SELECT orgunit.submit_global_scope_package_event(
  $1::uuid,
  $2::uuid,
  $3::text,
  $4::uuid,
  'CREATE',
  $5::date,
  jsonb_build_object('package_code', $6::text, 'name', $7::text),
  $8::text,
  $9::uuid
);
`, eventID, globalTenantID, scopeCode, packageID, effectiveDate, packageCode, name, requestID, initiatorID); err != nil {
		return out, err
	}
	pkg, err := FetchScopePackageByID(ctx, tx, globalTenantID, packageID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingID string
		if err := tx.QueryRow(ctx, `
SELECT package_id::text
FROM orgunit.global_setid_scope_package_events
WHERE tenant_uuid = $1::uuid AND request_id = $2::text
ORDER BY id DESC
LIMIT 1
`, globalTenantID, requestID).Scan(&existingID); err != nil {
			return out, err
		}
		pkg, err = FetchScopePackageByID(ctx, tx, globalTenantID, existingID, true)
		if err != nil {
			return out, err
		}
	} else if err != nil {
		return out, err
	}
	out = pkg
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *SetIDPGStore) ListGlobalScopePackages(ctx context.Context, scopeCode string) ([]ports.ScopePackage, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	var globalTenantID string
	if err := tx.QueryRow(ctx, `SELECT orgunit.global_tenant_id()::text;`).Scan(&globalTenantID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, globalTenantID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('app.allow_share_read', 'on', true);`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `
SELECT package_id::text, scope_code, package_code, ''::text AS owner_setid, name, status
FROM orgunit.global_setid_scope_packages
WHERE tenant_uuid = $1::uuid AND scope_code = $2::text
ORDER BY package_code ASC
`, globalTenantID, scopeCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ports.ScopePackage
	for rows.Next() {
		var r ports.ScopePackage
		if err := rows.Scan(&r.PackageID, &r.ScopeCode, &r.PackageCode, &r.OwnerSetID, &r.Name, &r.Status); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func FetchScopePackageByID(ctx context.Context, tx pgx.Tx, tenantID string, packageID string, isGlobal bool) (ports.ScopePackage, error) {
	var out ports.ScopePackage
	var err error
	if isGlobal {
		err = tx.QueryRow(ctx, `
SELECT package_id::text, scope_code, package_code, ''::text AS owner_setid, name, status
FROM orgunit.global_setid_scope_packages
WHERE tenant_uuid = $1::uuid AND package_id = $2::uuid
`, tenantID, packageID).Scan(&out.PackageID, &out.ScopeCode, &out.PackageCode, &out.OwnerSetID, &out.Name, &out.Status)
	} else {
		err = tx.QueryRow(ctx, `
SELECT package_id::text, scope_code, package_code, owner_setid, name, status
FROM orgunit.setid_scope_packages
WHERE tenant_uuid = $1::uuid AND package_id = $2::uuid
`, tenantID, packageID).Scan(&out.PackageID, &out.ScopeCode, &out.PackageCode, &out.OwnerSetID, &out.Name, &out.Status)
	}
	return out, err
}

func FetchScopeSubscription(ctx context.Context, tx pgx.Tx, tenantID string, setID string, scopeCode string, asOfDate string) (ports.ScopeSubscription, error) {
	var out ports.ScopeSubscription
	var ownerTenantID string
	var endDate string
	if err := tx.QueryRow(ctx, `
SELECT setid, scope_code, package_id::text, package_owner_tenant_uuid::text,
  lower(validity)::text,
  COALESCE(upper(validity)::text, '')
FROM orgunit.setid_scope_subscriptions
WHERE tenant_uuid = $1::uuid
  AND setid = $2::text
  AND scope_code = $3::text
  AND validity @> $4::date
ORDER BY lower(validity) DESC
LIMIT 1
`, tenantID, setID, scopeCode, asOfDate).Scan(&out.SetID, &out.ScopeCode, &out.PackageID, &ownerTenantID, &out.EffectiveDate, &endDate); err != nil {
		return out, err
	}
	out.EndDate = endDate
	if ownerTenantID == tenantID {
		out.PackageOwner = "tenant"
	} else {
		out.PackageOwner = "global"
	}
	return out, nil
}

func parseOrgID8SetID(input string) (int, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, errors.New("org_id is required")
	}
	if len(trimmed) != 8 {
		return 0, errors.New("org_id must be 8 digits")
	}
	value := 0
	for _, r := range trimmed {
		if r < '0' || r > '9' {
			return 0, errors.New("org_id must be 8 digits")
		}
		value = value*10 + int(r-'0')
	}
	if value < 10000000 || value > 99999999 {
		return 0, errors.New("org_id must be 8 digits")
	}
	return value, nil
}

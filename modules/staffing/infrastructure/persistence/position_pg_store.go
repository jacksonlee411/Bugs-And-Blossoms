package persistence

import (
	"context"
	"strconv"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type PositionPGStore struct {
	pool pgBeginner
}

func NewPositionPGStore(pool pgBeginner) ports.PositionStore {
	return &PositionPGStore{pool: pool}
}

func (s *PositionPGStore) ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]types.Position, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, `
		SELECT
		  position_uuid::text,
		  org_node_key::text,
		  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
		  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
		  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
		  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
		  COALESCE(job_profile_code, '') AS job_profile_code,
		  COALESCE(name, '') AS name,
		  lifecycle_status,
		  capacity_fte::text AS capacity_fte,
		  effective_date::text AS effective_date
		FROM staffing.get_position_snapshot($1::uuid, $2::date)
		ORDER BY effective_date DESC, position_uuid::text ASC
	`, tenantID, asOfDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []types.Position
	for rows.Next() {
		var p types.Position
		if err := rows.Scan(
			&p.PositionUUID,
			&p.OrgNodeKey,
			&p.ReportsToPositionUUID,
			&p.JobCatalogSetID,
			&p.JobCatalogSetIDAsOf,
			&p.JobProfileUUID,
			&p.JobProfileCode,
			&p.Name,
			&p.LifecycleStatus,
			&p.CapacityFTE,
			&p.EffectiveAt,
		); err != nil {
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

func (s *PositionPGStore) CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string, jobProfileUUID string, capacityFTE string, name string) (types.Position, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.Position{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.Position{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return types.Position{}, httperr.NewBadRequest("effective_date is required")
	}
	orgNodeKey, err = normalizePositionOrgNodeKey(orgNodeKey)
	if err != nil {
		return types.Position{}, err
	}
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	if jobProfileUUID == "" {
		return types.Position{}, httperr.NewBadRequest("job_profile_uuid is required")
	}
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)

	var positionID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&positionID); err != nil {
		return types.Position{}, err
	}
	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return types.Position{}, err
	}

	payload := `{"org_node_key":` + strconv.Quote(orgNodeKey) +
		`,"job_profile_uuid":` + strconv.Quote(jobProfileUUID)
	if capacityFTE != "" {
		payload += `,"capacity_fte":` + strconv.Quote(capacityFTE)
	}
	if name != "" {
		payload += `,"name":` + strconv.Quote(name)
	}
	payload += `}`

	if _, err := tx.Exec(ctx, `
SELECT staffing.submit_position_event(
  $1::uuid,
  $2::uuid,
  $3::uuid,
  'CREATE',
  $4::date,
  $5::jsonb,
  $6::text,
  $7::uuid
)
`, eventID, tenantID, positionID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return types.Position{}, err
	}

	var out types.Position
	if err := tx.QueryRow(ctx, `
		SELECT
		  position_uuid::text,
				  org_node_key::text,
		  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
		  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
		  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
		  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
		  COALESCE(job_profile_code, '') AS job_profile_code,
		  COALESCE(name, '') AS name,
		  lifecycle_status,
		  capacity_fte::text AS capacity_fte,
		  effective_date::text AS effective_date
		FROM staffing.get_position_snapshot($1::uuid, $3::date)
		WHERE position_uuid = $2::uuid
		LIMIT 1
	`, tenantID, positionID, effectiveDate).Scan(
		&out.PositionUUID,
		&out.OrgNodeKey,
		&out.ReportsToPositionUUID,
		&out.JobCatalogSetID,
		&out.JobCatalogSetIDAsOf,
		&out.JobProfileUUID,
		&out.JobProfileCode,
		&out.Name,
		&out.LifecycleStatus,
		&out.CapacityFTE,
		&out.EffectiveAt,
	); err != nil {
		return types.Position{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.Position{}, err
	}

	return out, nil
}

func (s *PositionPGStore) UpdatePositionCurrent(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgNodeKey string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (types.Position, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return types.Position{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_tenant', $1, true);`, tenantID); err != nil {
		return types.Position{}, err
	}

	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return types.Position{}, httperr.NewBadRequest("effective_date is required")
	}
	positionUUID = strings.TrimSpace(positionUUID)
	if positionUUID == "" {
		return types.Position{}, httperr.NewBadRequest("position_uuid is required")
	}
	orgNodeKey = strings.TrimSpace(orgNodeKey)
	reportsToPositionUUID = strings.TrimSpace(reportsToPositionUUID)
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	capacityFTE = strings.TrimSpace(capacityFTE)
	name = strings.TrimSpace(name)
	lifecycleStatus = strings.TrimSpace(lifecycleStatus)

	if orgNodeKey != "" {
		orgNodeKey, err = normalizePositionOrgNodeKey(orgNodeKey)
		if err != nil {
			return types.Position{}, err
		}
	}

	payloadParts := make([]string, 0, 6)
	if orgNodeKey != "" {
		payloadParts = append(payloadParts, `"org_node_key":`+strconv.Quote(orgNodeKey))
	}
	if reportsToPositionUUID != "" {
		payloadParts = append(payloadParts, `"reports_to_position_uuid":`+strconv.Quote(reportsToPositionUUID))
	}
	if jobProfileUUID != "" {
		payloadParts = append(payloadParts, `"job_profile_uuid":`+strconv.Quote(jobProfileUUID))
	}
	if capacityFTE != "" {
		payloadParts = append(payloadParts, `"capacity_fte":`+strconv.Quote(capacityFTE))
	}
	if name != "" {
		payloadParts = append(payloadParts, `"name":`+strconv.Quote(name))
	}
	if lifecycleStatus != "" {
		payloadParts = append(payloadParts, `"lifecycle_status":`+strconv.Quote(lifecycleStatus))
	}
	if len(payloadParts) == 0 {
		return types.Position{}, httperr.NewBadRequest("at least one patch field is required")
	}
	payload := `{` + strings.Join(payloadParts, ",") + `}`

	var eventID string
	if err := tx.QueryRow(ctx, `SELECT gen_random_uuid()::text;`).Scan(&eventID); err != nil {
		return types.Position{}, err
	}

	if _, err := tx.Exec(ctx, `
	SELECT staffing.submit_position_event(
	  $1::uuid,
	  $2::uuid,
	  $3::uuid,
	  'UPDATE',
	  $4::date,
	  $5::jsonb,
	  $6::text,
	  $7::uuid
	)
	`, eventID, tenantID, positionUUID, effectiveDate, []byte(payload), eventID, tenantID); err != nil {
		return types.Position{}, err
	}

	var out types.Position
	if err := tx.QueryRow(ctx, `
			SELECT
			  position_uuid::text,
			  org_node_key::text,
			  COALESCE(reports_to_position_uuid::text, '') AS reports_to_position_uuid,
			  COALESCE(jobcatalog_setid, '') AS jobcatalog_setid,
			  COALESCE(jobcatalog_setid_as_of::text, '') AS jobcatalog_setid_as_of,
			  COALESCE(job_profile_uuid::text, '') AS job_profile_uuid,
			  COALESCE(job_profile_code, '') AS job_profile_code,
			  COALESCE(name, '') AS name,
			  lifecycle_status,
			  capacity_fte::text AS capacity_fte,
			  effective_date::text AS effective_date
			FROM staffing.get_position_snapshot($1::uuid, $3::date)
			WHERE position_uuid = $2::uuid
			LIMIT 1
		`, tenantID, positionUUID, effectiveDate).Scan(
		&out.PositionUUID,
		&out.OrgNodeKey,
		&out.ReportsToPositionUUID,
		&out.JobCatalogSetID,
		&out.JobCatalogSetIDAsOf,
		&out.JobProfileUUID,
		&out.JobProfileCode,
		&out.Name,
		&out.LifecycleStatus,
		&out.CapacityFTE,
		&out.EffectiveAt,
	); err != nil {
		return types.Position{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return types.Position{}, err
	}
	return out, nil
}

func normalizePositionOrgNodeKey(input string) (string, error) {
	orgNodeKey, err := orgunitpkg.NormalizeOrgNodeKey(strings.TrimSpace(input))
	if err == nil {
		return orgNodeKey, nil
	}
	if strings.TrimSpace(input) == "" {
		return "", httperr.NewBadRequest("org_node_key is required")
	}
	return "", httperr.NewBadRequest("org_node_key invalid")
}

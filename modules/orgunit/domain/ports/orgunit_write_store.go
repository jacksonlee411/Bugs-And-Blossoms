package ports

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
)

var (
	ErrOrgEventNotFound = errors.New("org_event_not_found")
	ErrPersonNotFound   = errors.New("person_not_found")
)

type OrgUnitWriteStore interface {
	SubmitEvent(ctx context.Context, tenantID string, eventUUID string, orgID *int, eventType string, effectiveDate string, payload json.RawMessage, requestCode string, initiatorUUID string) (int64, error)
	SubmitCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, patch json.RawMessage, requestID string, initiatorUUID string) (string, error)
	SubmitStatusCorrection(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, targetStatus string, requestID string, initiatorUUID string) (string, error)
	SubmitRescindEvent(ctx context.Context, tenantID string, orgID int, targetEffectiveDate string, reason string, requestID string, initiatorUUID string) (string, error)
	SubmitRescindOrg(ctx context.Context, tenantID string, orgID int, reason string, requestID string, initiatorUUID string) (int, error)
	FindEventByUUID(ctx context.Context, tenantID string, eventUUID string) (types.OrgUnitEvent, error)
	FindEventByEffectiveDate(ctx context.Context, tenantID string, orgID int, effectiveDate string) (types.OrgUnitEvent, error)
	ListEnabledTenantFieldConfigsAsOf(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, error)
	ResolveOrgID(ctx context.Context, tenantID string, orgCode string) (int, error)
	ResolveOrgCode(ctx context.Context, tenantID string, orgID int) (string, error)
	FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (types.Person, error)
}

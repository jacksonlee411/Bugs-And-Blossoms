package ports

import (
	"context"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
)

type PositionStore interface {
	ListPositionsCurrent(ctx context.Context, tenantID string, asOfDate string) ([]types.Position, error)
	CreatePositionCurrent(ctx context.Context, tenantID string, effectiveDate string, orgNodeKey string, jobProfileUUID string, capacityFTE string, name string) (types.Position, error)
	UpdatePositionCurrent(ctx context.Context, tenantID string, positionUUID string, effectiveDate string, orgNodeKey string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (types.Position, error)
}

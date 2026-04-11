package persistence

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

type PositionMemoryStore struct {
	positions map[string][]types.Position
}

func NewPositionMemoryStore() ports.PositionStore {
	return NewPositionMemoryStoreWithState(nil)
}

func NewPositionMemoryStoreWithState(positions map[string][]types.Position) *PositionMemoryStore {
	if positions == nil {
		positions = make(map[string][]types.Position)
	}
	return &PositionMemoryStore{
		positions: positions,
	}
}

func (s *PositionMemoryStore) ListPositionsCurrent(_ context.Context, tenantID string, _ string) ([]types.Position, error) {
	return append([]types.Position(nil), s.positions[tenantID]...), nil
}

func (s *PositionMemoryStore) CreatePositionCurrent(_ context.Context, tenantID string, effectiveDate string, orgNodeKey string, jobProfileUUID string, capacityFTE string, name string) (types.Position, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return types.Position{}, httperr.NewBadRequest("effective_date is required")
	}
	orgNodeKey = strings.TrimSpace(orgNodeKey)
	if orgNodeKey == "" {
		return types.Position{}, httperr.NewBadRequest("org_node_key is required")
	}
	jobProfileUUID = strings.TrimSpace(jobProfileUUID)
	if jobProfileUUID == "" {
		return types.Position{}, httperr.NewBadRequest("job_profile_uuid is required")
	}
	capacityFTE = strings.TrimSpace(capacityFTE)
	if capacityFTE == "" {
		capacityFTE = "1.0"
	}
	name = strings.TrimSpace(name)

	id := "pos-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	p := types.Position{
		PositionUUID:          id,
		OrgNodeKey:            orgNodeKey,
		ReportsToPositionUUID: "",
		JobCatalogSetID:       "",
		JobCatalogSetIDAsOf:   "",
		JobProfileUUID:        jobProfileUUID,
		JobProfileCode:        "",
		Name:                  name,
		LifecycleStatus:       "active",
		CapacityFTE:           capacityFTE,
		EffectiveAt:           effectiveDate,
	}
	s.positions[tenantID] = append(s.positions[tenantID], p)
	return p, nil
}

func (s *PositionMemoryStore) UpdatePositionCurrent(_ context.Context, tenantID string, positionUUID string, effectiveDate string, orgNodeKey string, reportsToPositionUUID string, jobProfileUUID string, capacityFTE string, name string, lifecycleStatus string) (types.Position, error) {
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
	if orgNodeKey == "" && reportsToPositionUUID == "" && jobProfileUUID == "" && capacityFTE == "" && name == "" && lifecycleStatus == "" {
		return types.Position{}, httperr.NewBadRequest("at least one patch field is required")
	}

	for i := range s.positions[tenantID] {
		if s.positions[tenantID][i].PositionUUID != positionUUID {
			continue
		}
		if orgNodeKey != "" {
			s.positions[tenantID][i].OrgNodeKey = orgNodeKey
		}
		if reportsToPositionUUID != "" {
			s.positions[tenantID][i].ReportsToPositionUUID = reportsToPositionUUID
		}
		if jobProfileUUID != "" {
			s.positions[tenantID][i].JobProfileUUID = jobProfileUUID
		}
		if capacityFTE != "" {
			s.positions[tenantID][i].CapacityFTE = capacityFTE
		}
		if name != "" {
			s.positions[tenantID][i].Name = name
		}
		if lifecycleStatus != "" {
			s.positions[tenantID][i].LifecycleStatus = lifecycleStatus
		}
		s.positions[tenantID][i].EffectiveAt = effectiveDate
		return s.positions[tenantID][i], nil
	}
	return types.Position{}, errors.New("position not found")
}

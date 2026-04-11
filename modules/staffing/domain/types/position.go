package types

type Position struct {
	PositionUUID          string
	OrgNodeKey            string
	ReportsToPositionUUID string
	JobCatalogSetID       string
	JobCatalogSetIDAsOf   string
	JobProfileUUID        string
	JobProfileCode        string
	Name                  string
	LifecycleStatus       string
	CapacityFTE           string
	EffectiveAt           string
}

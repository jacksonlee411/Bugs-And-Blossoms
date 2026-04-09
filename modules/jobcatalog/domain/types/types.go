package types

type JobFamilyGroup struct {
	JobFamilyGroupUUID string
	JobFamilyGroupCode string
	Name               string
	IsActive           bool
	EffectiveDay       string
}

type JobLevel struct {
	JobLevelUUID string
	JobLevelCode string
	Name         string
	IsActive     bool
	EffectiveDay string
}

type JobFamily struct {
	JobFamilyUUID      string
	JobFamilyCode      string
	JobFamilyGroupCode string
	Name               string
	IsActive           bool
	EffectiveDay       string
}

type JobProfile struct {
	JobProfileUUID    string
	JobProfileCode    string
	Name              string
	IsActive          bool
	EffectiveDay      string
	FamilyCodesCSV    string
	PrimaryFamilyCode string
}

type JobCatalogPackage struct {
	PackageUUID string
	PackageCode string
	OwnerSetID  string
}

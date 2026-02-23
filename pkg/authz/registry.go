package authz

const (
	RoleTenantAdmin  = "tenant-admin"
	RoleTenantViewer = "tenant-viewer"
	RoleAnonymous    = "anonymous"
	RoleSuperadmin   = "superadmin"
)

const (
	ActionRead  = "read"
	ActionAdmin = "admin"
	ActionDebug = "debug"
)

const DomainGlobal = "global"

const (
	ObjectIAMPing             = "iam.ping"
	ObjectIAMSession          = "iam.session"
	ObjectIAMDicts            = "iam.dicts"
	ObjectIAMDictRelease      = "iam.dict_release"
	ObjectOrgUnitOrgUnits     = "orgunit.orgunits"
	ObjectOrgUnitSetID        = "orgunit.setid"
	ObjectOrgSetIDCapability  = "org.setid_capability_config"
	ObjectOrgShareRead        = "org.share_read"
	ObjectJobCatalogCatalog   = "jobcatalog.catalog"
	ObjectPersonPersons       = "person.persons"
	ObjectStaffingPositions   = "staffing.positions"
	ObjectStaffingAssignments = "staffing.assignments"

	ObjectSuperadminTenants = "superadmin.tenants"
	ObjectSuperadminSession = "superadmin.session"
)

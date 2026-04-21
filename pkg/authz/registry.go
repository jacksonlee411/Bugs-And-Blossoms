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
	ObjectIAMPing         = "iam.ping"
	ObjectIAMSession      = "iam.session"
	ObjectIAMDicts        = "iam.dicts"
	ObjectIAMDictRelease  = "iam.dict_release"
	ObjectOrgUnitOrgUnits = "orgunit.orgunits"
	ObjectOrgShareRead    = "org.share_read"

	ObjectSuperadminTenants = "superadmin.tenants"
	ObjectSuperadminSession = "superadmin.session"
)

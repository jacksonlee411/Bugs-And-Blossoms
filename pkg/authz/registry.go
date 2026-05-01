package authz

const (
	RoleTenantAdmin  = "tenant-admin"
	RoleTenantViewer = "tenant-viewer"
	RoleAnonymous    = "anonymous"
	RoleSuperadmin   = "superadmin"
)

const (
	ActionRead       = "read"
	ActionAdmin      = "admin"
	ActionDebug      = "debug"
	ActionUpdate     = "update"
	ActionRotate     = "rotate"
	ActionSelect     = "select"
	ActionVerify     = "verify"
	ActionDeactivate = "deactivate"
	ActionUse        = "use"
)

const DomainGlobal = "global"

const (
	ObjectIAMPing                = "iam.ping"
	ObjectIAMSession             = "iam.session"
	ObjectIAMDicts               = "iam.dicts"
	ObjectIAMDictRelease         = "iam.dict_release"
	ObjectCubeBoxConversations   = "cubebox.conversations"
	ObjectCubeBoxModelProvider   = "cubebox.model_provider"
	ObjectCubeBoxModelCredential = "cubebox.model_credential"
	ObjectCubeBoxModelSelection  = "cubebox.model_selection"
	ObjectOrgUnitOrgUnits        = "orgunit.orgunits"
	ObjectOrgShareRead           = "org.share_read"

	ObjectSuperadminTenants = "superadmin.tenants"
	ObjectSuperadminSession = "superadmin.session"
)

func AuthzCapabilityKey(object string, action string) string {
	return object + ":" + action
}

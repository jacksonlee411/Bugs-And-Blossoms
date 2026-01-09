package authz

const (
	RoleTenantAdmin = "tenant-admin"
	RoleAnonymous   = "anonymous"
	RoleSuperadmin  = "superadmin"
)

const (
	ActionRead  = "read"
	ActionAdmin = "admin"
	ActionDebug = "debug"
)

const DomainGlobal = "global"

const (
	ObjectIAMPing                           = "iam.ping"
	ObjectIAMSession                        = "iam.session"
	ObjectOrgUnitOrgUnits                   = "orgunit.orgunits"
	ObjectOrgUnitSetID                      = "orgunit.setid"
	ObjectJobCatalogCatalog                 = "jobcatalog.catalog"
	ObjectPersonPersons                     = "person.persons"
	ObjectStaffingPositions                 = "staffing.positions"
	ObjectStaffingAttendancePunches         = "staffing.attendance-punches"
	ObjectStaffingAttendanceDailyResults    = "staffing.attendance-daily-results"
	ObjectStaffingAttendanceTimeBank        = "staffing.attendance-time-bank"
	ObjectStaffingAttendanceTimeProfile     = "staffing.attendance-time-profile"
	ObjectStaffingAttendanceHolidayCalendar = "staffing.attendance-holiday-calendar"
	ObjectStaffingAssignments               = "staffing.assignments"
	ObjectStaffingPayrollPeriods            = "staffing.payroll-periods"
	ObjectStaffingPayrollRuns               = "staffing.payroll-runs"
	ObjectStaffingPayrollRecalcRequests     = "staffing.payroll-recalc-requests"
	ObjectStaffingPayslips                  = "staffing.payslips"

	ObjectStaffingPayrollSocialInsurancePolicies = "staffing.payroll-social-insurance-policies"

	ObjectSuperadminTenants = "superadmin.tenants"
	ObjectSuperadminSession = "superadmin.session"
)

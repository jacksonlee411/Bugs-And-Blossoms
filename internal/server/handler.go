package server

import (
	"context"
	"embed"
	"errors"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

//go:embed assets/*
var embeddedAssets embed.FS

func NewHandler() (http.Handler, error) {
	return NewHandlerWithOptions(HandlerOptions{})
}

type HandlerOptions struct {
	TenancyResolver             TenancyResolver
	IdentityProvider            identityProvider
	OrgUnitStore                OrgUnitStore
	OrgUnitSnapshot             OrgUnitSnapshotStore
	SetIDStore                  SetIDGovernanceStore
	JobCatalogStore             JobCatalogStore
	PersonStore                 PersonStore
	PositionStore               PositionStore
	AssignmentStore             AssignmentStore
	PayrollStore                PayrollStore
	AttendanceStore             TimePunchStore
	AttendanceDailyResultsStore DailyAttendanceResultStore
	AttendanceTimeBankStore     TimeBankCycleStore
}

func NewHandlerWithOptions(opts HandlerOptions) (http.Handler, error) {
	allowlistPath := os.Getenv("ALLOWLIST_PATH")
	if allowlistPath == "" {
		p, err := defaultAllowlistPath()
		if err != nil {
			return nil, err
		}
		allowlistPath = p
	}

	a, err := routing.LoadAllowlist(allowlistPath)
	if err != nil {
		return nil, err
	}

	classifier, err := routing.NewClassifier(a, "server")
	if err != nil {
		return nil, err
	}

	orgStore := opts.OrgUnitStore
	orgSnapshotStore := opts.OrgUnitSnapshot
	setidStore := opts.SetIDStore
	jobcatalogStore := opts.JobCatalogStore
	personStore := opts.PersonStore
	positionStore := opts.PositionStore
	assignmentStore := opts.AssignmentStore
	payrollStore := opts.PayrollStore
	attendanceStore := opts.AttendanceStore
	attendanceDailyResultsStore := opts.AttendanceDailyResultsStore
	attendanceTimeBankStore := opts.AttendanceTimeBankStore
	tenancyResolver := opts.TenancyResolver
	identityProvider := opts.IdentityProvider

	var pgPool *pgxpool.Pool
	if orgStore == nil {
		dsn := dbDSNFromEnv()
		pool, err := pgxpool.New(context.Background(), dsn)
		if err != nil {
			return nil, err
		}
		pgPool = pool
		orgStore = newOrgUnitPGStore(pgPool)
	}

	if orgSnapshotStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			orgSnapshotStore = newOrgUnitSnapshotPGStore(pgStore.pool)
		}
	}

	if setidStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			setidStore = newSetIDPGStore(pgStore.pool)
		} else {
			setidStore = newSetIDMemoryStore()
		}
	}

	if jobcatalogStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			jobcatalogStore = newJobCatalogPGStore(pgStore.pool)
		} else {
			jobcatalogStore = newJobCatalogMemoryStore()
		}
	}

	if personStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			personStore = newPersonPGStore(pgStore.pool)
		} else {
			personStore = newPersonMemoryStore()
		}
	}

	if positionStore == nil || assignmentStore == nil || payrollStore == nil || attendanceStore == nil || attendanceDailyResultsStore == nil || attendanceTimeBankStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			s := newStaffingPGStore(pgStore.pool)
			if positionStore == nil {
				positionStore = s
			}
			if assignmentStore == nil {
				assignmentStore = s
			}
			if payrollStore == nil {
				payrollStore = s
			}
			if attendanceStore == nil {
				attendanceStore = s
			}
			if attendanceDailyResultsStore == nil {
				attendanceDailyResultsStore = s
			}
			if attendanceTimeBankStore == nil {
				attendanceTimeBankStore = s
			}
		} else {
			s := newStaffingMemoryStore()
			if positionStore == nil {
				positionStore = s
			}
			if assignmentStore == nil {
				assignmentStore = s
			}
			if attendanceStore == nil {
				attendanceStore = s
			}
			if attendanceDailyResultsStore == nil {
				attendanceDailyResultsStore = s
			}
			if attendanceTimeBankStore == nil {
				attendanceTimeBankStore = s
			}
		}
	}

	var attendanceConfigStore AttendanceConfigStore
	if s, ok := attendanceStore.(AttendanceConfigStore); ok {
		attendanceConfigStore = s
	} else if s, ok := attendanceDailyResultsStore.(AttendanceConfigStore); ok {
		attendanceConfigStore = s
	} else {
		return nil, errors.New("server: missing attendance config store (expected AttendanceStore to implement AttendanceConfigStore)")
	}

	router := routing.NewRouter(classifier)

	authorizer, err := loadAuthorizer()
	if err != nil {
		return nil, err
	}

	if tenancyResolver == nil {
		if pgPool == nil {
			return nil, errors.New("server: missing tenancy resolver (set HandlerOptions.TenancyResolver or use default PG stores)")
		}
		tenancyResolver = newTenancyDBResolver(pgPool)
	}

	principals := newPrincipalStore(pgPool)
	sessions := newSessionStore(pgPool)
	guarded := withTenantAndSession(tenancyResolver, principals, sessions, withAuthz(classifier, authorizer, router))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusFound)
	}))

	router.Handle(routing.RouteClassOps, http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))
	router.Handle(routing.RouteClassOps, http.MethodGet, "/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	}))

	router.Handle(routing.RouteClassAuthn, http.MethodGet, "/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeShell(w, r, renderLoginForm(""))
	}))
	router.Handle(routing.RouteClassAuthn, http.MethodPost, "/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ := currentTenant(r.Context())

		if err := r.ParseForm(); err != nil {
			writeShellWithStatus(w, r, http.StatusUnprocessableEntity, renderLoginForm("invalid form"))
			return
		}
		email := r.FormValue("email")
		password := r.FormValue("password")
		if email == "" || password == "" {
			writeShellWithStatus(w, r, http.StatusUnprocessableEntity, renderLoginForm("email and password required"))
			return
		}

		provider := identityProvider
		if provider == nil {
			p, err := newKratosIdentityProviderFromEnv()
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "identity_provider_error", "identity provider error")
				return
			}
			provider = p
		}

		ident, err := provider.AuthenticatePassword(r.Context(), tenant, email, password)
		if err != nil {
			if errors.Is(err, errInvalidCredentials) {
				writeShellWithStatus(w, r, http.StatusUnprocessableEntity, renderLoginForm("invalid credentials"))
				return
			}
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "identity_error", "identity error")
			return
		}

		p, err := principals.UpsertFromKratos(r.Context(), tenant.ID, ident.Email, "tenant-admin", ident.KratosIdentityID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_error", "principal error")
			return
		}

		expiresAt := time.Now().Add(sidTTLFromEnv())
		sid, err := sessions.Create(r.Context(), tenant.ID, p.ID, expiresAt, r.RemoteAddr, r.UserAgent())
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "session_error", "session error")
			return
		}
		setSIDCookie(w, sid)
		http.Redirect(w, r, "/app?as_of="+currentUTCDateString(), http.StatusFound)
	}))
	router.Handle(routing.RouteClassAuthn, http.MethodPost, "/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sid, ok := readSID(r); ok {
			_ = sessions.Revoke(r.Context(), sid)
		}
		clearSIDCookie(w)
		http.Redirect(w, r, "/login", http.StatusFound)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/lang/en", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLangCookie(w, "en")
		redirectBack(w, r)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/lang/zh", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setLangCookie(w, "zh")
		redirectBack(w, r)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/app", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAsOf(w, r); !ok {
			return
		}
		writeShell(w, r, "")
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/app/home", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asOf, ok := requireAsOf(w, r)
		if !ok {
			return
		}
		l := lang(r)
		writePage(w, r, `<h1>Home</h1>`+
			`<p>Pick a module:</p>`+
			`<ul>`+
			`<li><a href="/org/nodes?as_of=`+asOf+`" hx-get="/org/nodes?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_orgunit")+`</a></li>`+
			`<li><a href="/org/snapshot?as_of=`+asOf+`" hx-get="/org/snapshot?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_orgunit_snapshot")+`</a></li>`+
			`<li><a href="/org/setid?as_of=`+asOf+`" hx-get="/org/setid?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_setid")+`</a></li>`+
			`<li><a href="/org/job-catalog?as_of=`+asOf+`" hx-get="/org/job-catalog?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_jobcatalog")+`</a></li>`+
			`<li><a href="/org/positions?as_of=`+asOf+`" hx-get="/org/positions?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_staffing")+`</a></li>`+
			`<li><a href="/org/attendance-punches?as_of=`+asOf+`" hx-get="/org/attendance-punches?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_attendance")+`</a></li>`+
			`<li><a href="/org/attendance-daily-results?as_of=`+asOf+`" hx-get="/org/attendance-daily-results?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_attendance_daily_results")+`</a></li>`+
			`<li><a href="/org/attendance-time-bank?as_of=`+asOf+`" hx-get="/org/attendance-time-bank?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_attendance_time_bank")+`</a></li>`+
			`<li><a href="/org/attendance-time-profile?as_of=`+asOf+`" hx-get="/org/attendance-time-profile?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_attendance_time_profile")+`</a></li>`+
			`<li><a href="/org/attendance-holiday-calendar?as_of=`+asOf+`" hx-get="/org/attendance-holiday-calendar?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_attendance_holiday_calendar")+`</a></li>`+
			`<li><a href="/org/payroll-periods?as_of=`+asOf+`" hx-get="/org/payroll-periods?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_payroll")+`</a></li>`+
			`<li><a href="/person/persons?as_of=`+asOf+`" hx-get="/person/persons?as_of=`+asOf+`" hx-target="#content" hx-push-url="true">`+tr(l, "nav_person")+`</a></li>`+
			`</ul>`)
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/nav", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asOf, ok := requireAsOf(w, r)
		if !ok {
			return
		}
		writeContent(w, r, renderNav(r, asOf))
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/topbar", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asOf, ok := requireAsOf(w, r)
		if !ok {
			return
		}
		writeContent(w, r, renderTopbar(r, asOf))
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/ui/flash", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeContent(w, r, "")
	}))

	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/nodes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgNodes(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/nodes", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgNodes(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/snapshot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgSnapshot(w, r, orgSnapshotStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/snapshot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgSnapshot(w, r, orgSnapshotStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/setid", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetID(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/setid", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetID(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/job-catalog", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleJobCatalog(w, r, jobcatalogStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/job-catalog", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleJobCatalog(w, r, jobcatalogStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositions(w, r, orgStore, positionStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositions(w, r, orgStore, positionStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignments(w, r, positionStore, assignmentStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignments(w, r, positionStore, assignmentStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-punches", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendancePunches(w, r, attendanceStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/attendance-punches", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendancePunches(w, r, attendanceStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-daily-results", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceDailyResults(w, r, attendanceDailyResultsStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-time-bank", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceTimeBank(w, r, attendanceTimeBankStore, attendanceDailyResultsStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-time-profile", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceTimeProfile(w, r, attendanceConfigStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/attendance-time-profile", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceTimeProfile(w, r, attendanceConfigStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-holiday-calendar", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceHolidayCalendar(w, r, attendanceConfigStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/attendance-holiday-calendar", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceHolidayCalendar(w, r, attendanceConfigStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/attendance-daily-results/{person_uuid}/{work_date}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceDailyResultDetail(w, r, attendanceDailyResultsStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/attendance-daily-results/{person_uuid}/{work_date}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceDailyResultDetail(w, r, attendanceDailyResultsStore, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-periods", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollPeriods(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-periods", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollPeriods(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-runs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRuns(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-runs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRuns(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-runs/{run_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRunDetail(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-runs/{run_id}/calculate", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRunCalculate(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-runs/{run_id}/finalize", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRunFinalize(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-runs/{run_id}/payslips", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslips(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-runs/{run_id}/payslips/{payslip_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslipDetail(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslipNetGuaranteedIITItems(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-social-insurance-policies", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollSocialInsurancePolicies(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-social-insurance-policies", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollSocialInsurancePolicies(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-recalc-requests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequests(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/org/payroll-recalc-requests/{recalc_request_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequestDetail(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/org/payroll-recalc-requests/{recalc_request_id}/apply", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequestApply(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositionsAPI(w, r, positionStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositionsAPI(w, r, positionStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentsAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentsAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/attendance-punches", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendancePunchesAPI(w, r, attendanceStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/attendance-punches", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendancePunchesAPI(w, r, attendanceStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/attendance-punch-voids", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendancePunchVoidsAPI(w, r, attendanceDailyResultsStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/attendance-recalc", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceRecalcAPI(w, r, attendanceDailyResultsStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/attendance-daily-results", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAttendanceDailyResultsAPI(w, r, attendanceDailyResultsStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-periods", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollPeriodsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-periods", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollPeriodsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-runs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRunsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-runs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRunsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-runs/{run_id}/payslips/{payslip_id}/net-guaranteed-iit-items", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslipNetGuaranteedIITItemsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-balances", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollBalancesAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-iit-special-additional-deductions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollIITSADAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payslips", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslipsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payslips/{payslip_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayslipAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-social-insurance-policies", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollSocialInsurancePoliciesAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-social-insurance-policies", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollSocialInsurancePoliciesAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-recalc-requests", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequestsAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/payroll-recalc-requests/{recalc_request_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequestAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/payroll-recalc-requests/{recalc_request_id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePayrollRecalcRequestAPI(w, r, payrollStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodGet, "/person/persons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersons(w, r, personStore)
	}))
	router.Handle(routing.RouteClassUI, http.MethodPost, "/person/persons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersons(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/person/api/persons:options", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonOptionsAPI(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/person/api/persons:by-pernr", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonByPernrAPI(w, r, personStore)
	}))

	assetsSub, _ := fs.Sub(embeddedAssets, "assets")

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub))))
	mux.Handle("/", guarded)

	return mux, nil
}

func MustNewHandler() http.Handler {
	h, err := NewHandler()
	if err != nil {
		panic(errors.New("server: failed to build handler: " + err.Error()))
	}
	return h
}

func defaultAllowlistPath() (string, error) {
	path := "config/routing/allowlist.yaml"
	for range 8 {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		path = filepath.Join("..", path)
	}
	return "", errors.New("server: allowlist not found")
}

func redirectBack(w http.ResponseWriter, r *http.Request) {
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/app"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}

func isHX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "lang",
		Value:    lang,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func withTenantAndSession(tenants TenancyResolver, principals principalStore, sessions sessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/health" || path == "/healthz" || path == "/assets" || pathHasPrefixSegment(path, "/assets") {
			next.ServeHTTP(w, r)
			return
		}

		tenantDomain := effectiveHost(r)
		t, ok, err := tenants.ResolveTenant(r.Context(), tenantDomain)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_resolve_error", "tenant resolve error")
			return
		}
		if !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusNotFound, "tenant_not_found", "tenant not found")
			return
		}
		r = r.WithContext(withTenant(r.Context(), t))

		if path == "/login" || pathHasPrefixSegment(path, "/lang") {
			next.ServeHTTP(w, r)
			return
		}

		sid, ok := readSID(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		sess, ok, err := sessions.Lookup(r.Context(), sid)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "session_lookup_error", "session lookup error")
			return
		}
		if !ok || sess.TenantID != t.ID {
			clearSIDCookie(w)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		p, ok, err := principals.GetByID(r.Context(), t.ID, sess.PrincipalID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_lookup_error", "principal lookup error")
			return
		}
		if !ok || p.Status != "active" {
			clearSIDCookie(w)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		r = r.WithContext(withPrincipal(r.Context(), p))

		next.ServeHTTP(w, r)
	})
}

func pathHasPrefixSegment(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return len(path) > len(prefix) && path[:len(prefix)+1] == prefix+"/"
}

func renderNav(r *http.Request, asOf string) string {
	l := lang(r)
	return `<nav><ul>` +
		`<li><a href="/org/nodes?as_of=` + asOf + `" hx-get="/org/nodes?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit") + `</a></li>` +
		`<li><a href="/org/snapshot?as_of=` + asOf + `" hx-get="/org/snapshot?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_orgunit_snapshot") + `</a></li>` +
		`<li><a href="/org/setid?as_of=` + asOf + `" hx-get="/org/setid?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_setid") + `</a></li>` +
		`<li><a href="/org/job-catalog?as_of=` + asOf + `" hx-get="/org/job-catalog?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_jobcatalog") + `</a></li>` +
		`<li><a href="/org/positions?as_of=` + asOf + `" hx-get="/org/positions?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_staffing") + `</a></li>` +
		`<li><a href="/org/attendance-punches?as_of=` + asOf + `" hx-get="/org/attendance-punches?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_attendance") + `</a></li>` +
		`<li><a href="/org/attendance-daily-results?as_of=` + asOf + `" hx-get="/org/attendance-daily-results?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_attendance_daily_results") + `</a></li>` +
		`<li><a href="/org/attendance-time-bank?as_of=` + asOf + `" hx-get="/org/attendance-time-bank?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_attendance_time_bank") + `</a></li>` +
		`<li><a href="/org/attendance-time-profile?as_of=` + asOf + `" hx-get="/org/attendance-time-profile?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_attendance_time_profile") + `</a></li>` +
		`<li><a href="/org/attendance-holiday-calendar?as_of=` + asOf + `" hx-get="/org/attendance-holiday-calendar?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_attendance_holiday_calendar") + `</a></li>` +
		`<li><a href="/org/payroll-periods?as_of=` + asOf + `" hx-get="/org/payroll-periods?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_payroll") + `</a></li>` +
		`<li><a href="/person/persons?as_of=` + asOf + `" hx-get="/person/persons?as_of=` + asOf + `" hx-target="#content" hx-push-url="true">` + tr(l, "nav_person") + `</a></li>` +
		`</ul></nav>`
}

func renderTopbar(r *http.Request, asOf string) string {
	l := lang(r)
	currentURL := strings.TrimSpace(r.Header.Get("HX-Current-URL"))
	if currentURL == "" {
		currentURL = strings.TrimSpace(r.Header.Get("Referer"))
	}

	targetPath := "/app/home"
	var q url.Values
	if currentURL != "" {
		if u, err := url.Parse(currentURL); err == nil {
			if u.Path != "" {
				targetPath = u.Path
			}
			q = u.Query()
		}
	}
	if targetPath == "/" || targetPath == "/app" || targetPath == "/login" {
		targetPath = "/app/home"
	}

	var keys []string
	for k := range q {
		if k == "as_of" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(`<div>`)
	b.WriteString(`<a href="/lang/en">EN</a> | <a href="/lang/zh">中文</a>`)
	b.WriteString(`<form method="GET" action="` + html.EscapeString(targetPath) + `" hx-get="` + html.EscapeString(targetPath) + `" hx-target="#content" hx-push-url="true" style="display:inline">`)
	for _, k := range keys {
		for _, v := range q[k] {
			b.WriteString(`<input type="hidden" name="` + html.EscapeString(k) + `" value="` + html.EscapeString(v) + `" />`)
		}
	}
	b.WriteString(`<span style="margin-left:12px">` + tr(l, "as_of") + `</span>`)
	b.WriteString(`<input type="date" name="as_of" value="` + html.EscapeString(asOf) + `" />`)
	b.WriteString(`<button type="submit">Go</button>`)
	b.WriteString(`</form>`)
	b.WriteString(`</div>`)

	return b.String()
}

func lang(r *http.Request) string {
	c, err := r.Cookie("lang")
	if err != nil {
		return "en"
	}
	if c.Value == "zh" {
		return "zh"
	}
	return "en"
}

func tr(lang string, key string) string {
	if lang == "zh" {
		switch key {
		case "nav_orgunit":
			return "组织架构"
		case "nav_orgunit_snapshot":
			return "组织架构快照"
		case "nav_setid":
			return "SetID 管理"
		case "nav_jobcatalog":
			return "职位分类"
		case "nav_staffing":
			return "用工任职"
		case "nav_attendance":
			return "考勤 / 打卡"
		case "nav_attendance_daily_results":
			return "考勤 / 日结果"
		case "nav_attendance_time_bank":
			return "考勤 / 额度与银行"
		case "nav_attendance_time_profile":
			return "考勤 / TimeProfile"
		case "nav_attendance_holiday_calendar":
			return "考勤 / 假期日历"
		case "nav_payroll":
			return "薪酬"
		case "nav_person":
			return "人员"
		case "as_of":
			return "有效日期"
		}
	}

	switch key {
	case "nav_orgunit":
		return "OrgUnit"
	case "nav_orgunit_snapshot":
		return "OrgUnit Snapshot"
	case "nav_setid":
		return "SetID Governance"
	case "nav_jobcatalog":
		return "Job Catalog"
	case "nav_staffing":
		return "Staffing"
	case "nav_attendance":
		return "Attendance / Punches"
	case "nav_attendance_daily_results":
		return "Attendance / Daily Results"
	case "nav_attendance_time_bank":
		return "Attendance / Time Bank"
	case "nav_attendance_time_profile":
		return "Attendance / TimeProfile"
	case "nav_attendance_holiday_calendar":
		return "Attendance / Holiday Calendar"
	case "nav_payroll":
		return "Payroll"
	case "nav_person":
		return "Person"
	case "as_of":
		return "As-of"
	default:
		return ""
	}
}

func renderLoginForm(errMsg string) string {
	var b strings.Builder
	b.WriteString(`<h1>Login</h1>`)
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}
	b.WriteString(`<form method="POST" action="/login">`)
	b.WriteString(`<label>Email <input type="email" name="email" autocomplete="username" required></label><br>`)
	b.WriteString(`<label>Password <input type="password" name="password" autocomplete="current-password" required></label><br>`)
	b.WriteString(`<button type="submit">Login</button>`)
	b.WriteString(`</form>`)
	return b.String()
}

func writeShell(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	writeShellWithStatus(w, r, http.StatusOK, bodyHTML)
}

func writeShellWithStatus(w http.ResponseWriter, r *http.Request, status int, bodyHTML string) {
	writeShellWithStatusFromAssets(w, r, status, bodyHTML, embeddedAssets)
}

func writeShellWithStatusFromAssets(w http.ResponseWriter, r *http.Request, status int, bodyHTML string, assets fs.FS) {
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = currentUTCDateString()
	}

	out, err := renderAstroShellFromAssets(assets, r, asOf, bodyHTML)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "shell_error", "shell error")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(out))
}

func writeContent(w http.ResponseWriter, _ *http.Request, bodyHTML string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(bodyHTML))
}

func writeContentWithStatus(w http.ResponseWriter, _ *http.Request, status int, bodyHTML string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(bodyHTML))
}

func writePage(w http.ResponseWriter, r *http.Request, bodyHTML string) {
	if isHX(r) {
		writeContent(w, r, bodyHTML)
		return
	}
	writeShell(w, r, bodyHTML)
}

func writePageWithStatus(w http.ResponseWriter, r *http.Request, status int, bodyHTML string) {
	if isHX(r) {
		writeContentWithStatus(w, r, status, bodyHTML)
		return
	}
	writeShellWithStatus(w, r, status, bodyHTML)
}

const astroShellPath = "assets/astro/app.html"
const astroAsOfToken = "__BB_AS_OF__"

func renderAstroShellFromAssets(assets fs.FS, r *http.Request, asOf string, bodyHTML string) (string, error) {
	b, err := fs.ReadFile(assets, astroShellPath)
	if err != nil {
		return "", err
	}
	return renderAstroShellFromTemplate(string(b), r, asOf, bodyHTML)
}

func renderAstroShellFromTemplate(shell string, r *http.Request, asOf string, bodyHTML string) (string, error) {
	if !strings.Contains(shell, astroAsOfToken) {
		return "", errors.New("shell missing as_of token")
	}

	if bodyHTML != "" {
		var err error
		shell, err = replaceMainContent(shell, bodyHTML)
		if err != nil {
			return "", err
		}
	}

	if _, ok := currentPrincipal(r.Context()); !ok {
		shell = strings.ReplaceAll(shell, ` hx-trigger="load"`, "")
	}

	shell = strings.ReplaceAll(shell, astroAsOfToken, asOf)
	if strings.Contains(shell, astroAsOfToken) {
		return "", errors.New("shell still contains as_of token after injection")
	}

	return shell, nil
}

func replaceMainContent(shell string, bodyHTML string) (string, error) {
	idIdx := strings.Index(shell, `id="content"`)
	if idIdx < 0 {
		return "", errors.New("shell missing #content mount")
	}

	openStart := strings.LastIndex(shell[:idIdx], "<main")
	if openStart < 0 {
		return "", errors.New("shell missing <main> for #content")
	}

	openEndRel := strings.Index(shell[openStart:], ">")
	if openEndRel < 0 {
		return "", errors.New("shell has unterminated <main> tag")
	}
	openEnd := openStart + openEndRel

	closeRel := strings.Index(shell[openEnd+1:], "</main>")
	if closeRel < 0 {
		return "", errors.New("shell missing closing </main> for #content")
	}
	closeIdx := openEnd + 1 + closeRel

	return shell[:openEnd+1] + bodyHTML + shell[closeIdx:], nil
}

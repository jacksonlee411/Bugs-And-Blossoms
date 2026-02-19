package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitpersistence "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
)

//go:embed assets/*
var embeddedAssets embed.FS

func NewHandler() (http.Handler, error) {
	return NewHandlerWithOptions(HandlerOptions{})
}

type HandlerOptions struct {
	TenancyResolver     TenancyResolver
	IdentityProvider    identityProvider
	OrgUnitStore        OrgUnitStore
	OrgUnitWriteService orgunitservices.OrgUnitWriteService
	SetIDStore          SetIDGovernanceStore
	JobCatalogStore     JobCatalogStore
	PersonStore         PersonStore
	PositionStore       PositionStore
	AssignmentStore     AssignmentStore
	DictStore           DictStore
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
	orgUnitWriteService := opts.OrgUnitWriteService
	setidStore := opts.SetIDStore
	jobcatalogStore := opts.JobCatalogStore
	personStore := opts.PersonStore
	positionStore := opts.PositionStore
	assignmentStore := opts.AssignmentStore
	dictStore := opts.DictStore
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

	if orgUnitWriteService == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			orgUnitWriteService = orgunitservices.NewOrgUnitWriteService(orgunitpersistence.NewOrgUnitPGStore(pgStore.pool))
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

	if positionStore == nil || assignmentStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			s := newStaffingPGStore(pgStore.pool)
			if positionStore == nil {
				positionStore = s
			}
			if assignmentStore == nil {
				assignmentStore = s
			}
		} else {
			s := newStaffingMemoryStore()
			if positionStore == nil {
				positionStore = s
			}
			if assignmentStore == nil {
				assignmentStore = s
			}
		}
	}

	if dictStore == nil {
		if pgStore, ok := orgStore.(*orgUnitPGStore); ok {
			dictStore = newDictPGStore(pgStore.pool)
		} else {
			dictStore = newDictMemoryStore()
		}
	}
	if err := dictpkg.RegisterResolver(dictStore); err != nil {
		return nil, err
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

	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/sessions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, _ := currentTenant(r.Context())

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_json", "invalid json")
			return
		}
		email := strings.TrimSpace(req.Email)
		password := req.Password
		if email == "" || strings.TrimSpace(password) == "" {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_form", "email and password required")
			return
		}

		provider := identityProvider
		if provider == nil {
			p, err := newKratosIdentityProviderFromEnv()
			if err != nil {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "identity_provider_error", "identity provider error")
				return
			}
			provider = p
		}

		ident, err := provider.AuthenticatePassword(r.Context(), tenant, email, password)
		if err != nil {
			if errors.Is(err, errInvalidCredentials) {
				routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_credentials", "invalid credentials")
				return
			}
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "identity_error", "identity error")
			return
		}

		roleSlug := strings.TrimSpace(strings.ToLower(ident.RoleSlug))
		if roleSlug == "" {
			roleSlug = authz.RoleTenantAdmin
		}
		if roleSlug != authz.RoleTenantAdmin && roleSlug != authz.RoleTenantViewer {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusUnprocessableEntity, "invalid_identity_role", "invalid identity role")
			return
		}

		p, err := principals.UpsertFromKratos(r.Context(), tenant.ID, ident.Email, roleSlug, ident.KratosIdentityID)
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "principal_error", "principal error")
			return
		}

		expiresAt := time.Now().Add(sidTTLFromEnv())
		sid, err := sessions.Create(r.Context(), tenant.ID, p.ID, expiresAt, r.RemoteAddr, r.UserAgent())
		if err != nil {
			routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "session_error", "session error")
			return
		}
		setSIDCookie(w, sid)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNoContent)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/iam/api/dicts", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictsAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/dicts", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictsAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/dicts:disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictsDisableAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/iam/api/dicts/values", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictValuesAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/dicts/values", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictValuesAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/dicts/values:disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictValuesDisableAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/iam/api/dicts/values:correct", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictValuesCorrectAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/iam/api/dicts/values/audit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDictValuesAuditAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassAuthn, http.MethodPost, "/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sid, ok := readSID(r); ok {
			_ = sessions.Revoke(r.Context(), sid)
		}
		clearSIDCookie(w)
		http.Redirect(w, r, "/app/login", http.StatusFound)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositionsAPI(w, r, orgStore, positionStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/positions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositionsAPI(w, r, orgStore, positionStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/positions:options", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePositionsOptionsAPI(w, r, orgStore, jobcatalogStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentsAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/assignments", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentsAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/assignment-events:correct", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentEventsCorrectAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/assignment-events:rescind", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAssignmentEventsRescindAPI(w, r, assignmentStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/setids", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetIDsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/setids", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetIDsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/setid-bindings", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetIDBindingsAPI(w, r, setidStore, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/setid-bindings", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSetIDBindingsAPI(w, r, setidStore, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/scope-packages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScopePackagesAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/scope-packages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScopePackagesAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/owned-scope-packages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOwnedScopePackagesAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/scope-packages/{package_id}/disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScopePackageDisableAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/scope-subscriptions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScopeSubscriptionsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/scope-subscriptions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleScopeSubscriptionsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/global-setids", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGlobalSetIDsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/global-setids", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGlobalSetIDsAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/global-scope-packages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGlobalScopePackagesAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/global-scope-packages", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGlobalScopePackagesAPI(w, r, setidStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsAPI(w, r, orgStore, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsAPI(w, r, orgStore, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/field-definitions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldDefinitionsAPI(w, r)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/field-configs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldConfigsAPI(w, r, orgStore, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/field-configs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldConfigsAPI(w, r, orgStore, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/field-configs:enable-candidates", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldConfigsEnableCandidatesAPI(w, r, dictStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/field-configs:disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldConfigsDisableAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/field-policies", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldPoliciesAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/field-policies:disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldPoliciesDisableAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/field-policies:resolve-preview", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldPoliciesResolvePreviewAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/fields:options", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitFieldOptionsAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/mutation-capabilities", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitMutationCapabilitiesAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/append-capabilities", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitAppendCapabilitiesAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/write-capabilities", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitWriteCapabilitiesAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/details", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsDetailsAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/versions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsVersionsAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/audit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsAuditAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/org/api/org-units/search", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsSearchAPI(w, r, orgStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/rename", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsRenameAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/move", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsMoveAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/disable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsDisableAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/enable", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsEnableAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/write", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsWriteAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/corrections", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsCorrectionsAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/status-corrections", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsStatusCorrectionsAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/rescinds", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsRescindsAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/rescinds/org", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsRescindsOrgAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/org/api/org-units/set-business-unit", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleOrgUnitsBusinessUnitAPI(w, r, orgUnitWriteService)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/person/api/persons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonsAPI(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/person/api/persons", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonsAPI(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/person/api/persons:options", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonOptionsAPI(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/person/api/persons:by-pernr", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePersonByPernrAPI(w, r, personStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodGet, "/jobcatalog/api/catalog", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleJobCatalogAPI(w, r, setidStore, jobcatalogStore)
	}))
	router.Handle(routing.RouteClassInternalAPI, http.MethodPost, "/jobcatalog/api/catalog/actions", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleJobCatalogWriteAPI(w, r, setidStore, jobcatalogStore)
	}))

	assetsSub, _ := fs.Sub(embeddedAssets, "assets")

	entrypoint := http.NewServeMux()
	entrypoint.Handle("/app", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWebMUIIndex(w, r, embeddedAssets)
	}))
	entrypoint.Handle("/app/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWebMUIIndex(w, r, embeddedAssets)
	}))
	entrypoint.Handle("/", router)

	guarded := withTenantAndSession(classifier, tenancyResolver, principals, sessions, withAuthz(classifier, authorizer, entrypoint))

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSub))))
	mux.Handle("/", guarded)

	return mux, nil
}

const webMUIIndexPath = "assets/web/index.html"

func serveWebMUIIndex(w http.ResponseWriter, r *http.Request, assets fs.FS) {
	b, err := fs.ReadFile(assets, webMUIIndexPath)
	if err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "web_mui_index_missing", "web ui missing")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
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

func withTenantAndSession(classifier *routing.Classifier, tenants TenancyResolver, principals principalStore, sessions sessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		rc := routing.RouteClassUI
		if classifier != nil {
			rc = classifier.Classify(path)
		}

		if path == "/health" || path == "/healthz" || path == "/assets" || pathHasPrefixSegment(path, "/assets") {
			next.ServeHTTP(w, r)
			return
		}

		tenantDomain := effectiveHost(r)
		t, ok, err := tenants.ResolveTenant(r.Context(), tenantDomain)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "tenant_resolve_error", "tenant resolve error")
			return
		}
		if !ok {
			routing.WriteError(w, r, rc, http.StatusNotFound, "tenant_not_found", "tenant not found")
			return
		}
		r = r.WithContext(withTenant(r.Context(), t))

		// DEV-PLAN-103/103A: tenant app UI is MUI-only under /app/**.
		// For non-/app UI paths (e.g. old URLs like /login, /org/*), do not redirect-to-login alias;
		// let the router return 404 instead.
		if rc == routing.RouteClassUI && path != "/" && !pathHasPrefixSegment(path, "/app") {
			next.ServeHTTP(w, r)
			return
		}

		if path == "/app/login" || (path == "/iam/api/sessions" && r.Method == http.MethodPost) {
			next.ServeHTTP(w, r)
			return
		}

		sid, ok := readSID(r)
		if !ok {
			if rc == routing.RouteClassInternalAPI || rc == routing.RouteClassPublicAPI || rc == routing.RouteClassWebhook {
				routing.WriteError(w, r, rc, http.StatusUnauthorized, "unauthorized", "unauthorized")
				return
			}
			http.Redirect(w, r, "/app/login", http.StatusFound)
			return
		}

		sess, ok, err := sessions.Lookup(r.Context(), sid)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "session_lookup_error", "session lookup error")
			return
		}
		if !ok || sess.TenantID != t.ID {
			clearSIDCookie(w)
			if rc == routing.RouteClassInternalAPI || rc == routing.RouteClassPublicAPI || rc == routing.RouteClassWebhook {
				routing.WriteError(w, r, rc, http.StatusUnauthorized, "unauthorized", "unauthorized")
				return
			}
			http.Redirect(w, r, "/app/login", http.StatusFound)
			return
		}

		p, ok, err := principals.GetByID(r.Context(), t.ID, sess.PrincipalID)
		if err != nil {
			routing.WriteError(w, r, rc, http.StatusInternalServerError, "principal_lookup_error", "principal lookup error")
			return
		}
		if !ok || p.Status != "active" {
			clearSIDCookie(w)
			if rc == routing.RouteClassInternalAPI || rc == routing.RouteClassPublicAPI || rc == routing.RouteClassWebhook {
				routing.WriteError(w, r, rc, http.StatusUnauthorized, "unauthorized", "unauthorized")
				return
			}
			http.Redirect(w, r, "/app/login", http.StatusFound)
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

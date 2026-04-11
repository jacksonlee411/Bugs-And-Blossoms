package server

import (
	"context"
	"errors"
	"strings"

	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

const (
	setIDContextCodeBusinessUnitInvalid   = "business_unit_context_invalid"
	setIDContextCodeSetIDBindingMissing   = "setid_binding_missing"
	setIDContextCodeSetIDBindingAmbiguous = "setid_binding_ambiguous"
	setIDContextCodeSetIDSourceInvalid    = "setid_source_invalid"
	setIDContextCodeOrgResolverMissing    = "orgunit_resolver_missing"
	setIDContextCodeSetIDResolverMissing  = "setid_resolver_missing"
)

type setIDPolicyContextInput struct {
	TenantID            string
	CapabilityKey       string
	FieldKey            string
	AsOf                string
	BusinessUnitOrgCode string
}

type setIDPolicyContext struct {
	TenantID            string
	CapabilityKey       string
	FieldKey            string
	AsOf                string
	BusinessUnitOrgCode string
	BusinessUnitNodeKey string
	ResolvedSetID       string
	SetIDSource         string
}

type resolvedSetIDContext struct {
	OrgCode       string
	OrgNodeKey    string
	ResolvedSetID string
	SetIDSource   string
}

type setIDContextResolveError struct {
	Code  string
	Field string
	Cause error
}

func (e *setIDContextResolveError) Error() string {
	if e == nil {
		return ""
	}
	code := strings.TrimSpace(e.Code)
	field := strings.TrimSpace(e.Field)
	if field == "" {
		return code
	}
	if code == "" {
		return field
	}
	return field + ":" + code
}

func (e *setIDContextResolveError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type setIDContextResolver struct {
	orgResolver OrgUnitCodeResolver
	setIDStore  setIDContextSetIDResolver
}

type setIDContextSetIDResolver interface {
	ResolveSetID(ctx context.Context, tenantID string, orgNodeKey string, asOfDate string) (string, error)
}

func newSetIDContextResolver(orgResolver OrgUnitCodeResolver, setIDStore setIDContextSetIDResolver) setIDContextResolver {
	return setIDContextResolver{
		orgResolver: orgResolver,
		setIDStore:  setIDStore,
	}
}

func (r setIDContextResolver) ResolvePolicyContext(ctx context.Context, input setIDPolicyContextInput) (setIDPolicyContext, error) {
	businessUnitCtx, err := r.ResolveOrgContext(
		ctx,
		input.TenantID,
		input.BusinessUnitOrgCode,
		input.AsOf,
		"business_unit_org_code",
	)
	if err != nil {
		return setIDPolicyContext{}, err
	}
	return setIDPolicyContext{
		TenantID:            strings.TrimSpace(input.TenantID),
		CapabilityKey:       strings.ToLower(strings.TrimSpace(input.CapabilityKey)),
		FieldKey:            strings.ToLower(strings.TrimSpace(input.FieldKey)),
		AsOf:                strings.TrimSpace(input.AsOf),
		BusinessUnitOrgCode: businessUnitCtx.OrgCode,
		BusinessUnitNodeKey: businessUnitCtx.OrgNodeKey,
		ResolvedSetID:       businessUnitCtx.ResolvedSetID,
		SetIDSource:         businessUnitCtx.SetIDSource,
	}, nil
}

func (r setIDContextResolver) ResolveOrgContext(ctx context.Context, tenantID string, orgCode string, asOf string, field string) (resolvedSetIDContext, error) {
	normalizedOrgCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeBusinessUnitInvalid,
			Field: strings.TrimSpace(field),
			Cause: err,
		}
	}
	if r.orgResolver == nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeOrgResolverMissing,
			Field: strings.TrimSpace(field),
		}
	}
	orgNodeKey, err := r.orgResolver.ResolveOrgNodeKeyByCode(ctx, strings.TrimSpace(tenantID), normalizedOrgCode)
	if err != nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeBusinessUnitInvalid,
			Field: strings.TrimSpace(field),
			Cause: err,
		}
	}
	ref := setIDResolvedOrgRef{
		OrgCode:    normalizedOrgCode,
		OrgNodeKey: strings.TrimSpace(orgNodeKey),
	}
	if ref.OrgNodeKey, err = normalizeOrgNodeKeyInput(ref.OrgNodeKey); err != nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeBusinessUnitInvalid,
			Field: strings.TrimSpace(field),
			Cause: err,
		}
	}
	if r.setIDStore == nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeSetIDResolverMissing,
			Field: strings.TrimSpace(field),
		}
	}

	resolvedSetID, err := r.setIDStore.ResolveSetID(ctx, strings.TrimSpace(tenantID), ref.OrgNodeKey, strings.TrimSpace(asOf))
	if err != nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeSetIDBindingMissing,
			Field: strings.TrimSpace(field),
			Cause: err,
		}
	}
	resolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	if resolvedSetID == "" {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeSetIDBindingMissing,
			Field: strings.TrimSpace(field),
		}
	}

	setIDSource, err := classifyResolvedSetIDSource(resolvedSetID)
	if err != nil {
		return resolvedSetIDContext{}, &setIDContextResolveError{
			Code:  setIDContextCodeSetIDSourceInvalid,
			Field: strings.TrimSpace(field),
			Cause: err,
		}
	}

	return resolvedSetIDContext{
		OrgCode:       ref.OrgCode,
		OrgNodeKey:    ref.OrgNodeKey,
		ResolvedSetID: resolvedSetID,
		SetIDSource:   setIDSource,
	}, nil
}

func classifyResolvedSetIDSource(resolvedSetID string) (string, error) {
	resolvedSetID = strings.ToUpper(strings.TrimSpace(resolvedSetID))
	switch resolvedSetID {
	case "":
		return "", errors.New("resolved_setid empty")
	case orgUnitFieldOptionSetIDDeflt:
		return orgUnitFieldOptionSetIDSourceDeflt, nil
	case "SHARE":
		return "share_preview", nil
	default:
		return "custom", nil
	}
}

func asSetIDContextResolveError(err error) (*setIDContextResolveError, bool) {
	var target *setIDContextResolveError
	if !errors.As(err, &target) {
		return nil, false
	}
	return target, true
}

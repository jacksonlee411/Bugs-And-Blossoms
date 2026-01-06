package authz

import (
	"errors"
	"os"
	"strings"

	"github.com/casbin/casbin/v2"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
)

type Mode string

const (
	ModeEnforce  Mode = "enforce"
	ModeShadow   Mode = "shadow"
	ModeDisabled Mode = "disabled"
)

func ModeFromEnv() (Mode, error) {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("AUTHZ_MODE")))
	if raw == "" {
		return ModeEnforce, nil
	}
	switch Mode(raw) {
	case ModeEnforce, ModeShadow:
		return Mode(raw), nil
	case ModeDisabled:
		if os.Getenv("AUTHZ_UNSAFE_ALLOW_DISABLED") != "1" {
			return "", errors.New("authz: AUTHZ_MODE=disabled requires AUTHZ_UNSAFE_ALLOW_DISABLED=1")
		}
		return ModeDisabled, nil
	default:
		return "", errors.New("authz: invalid AUTHZ_MODE (expected enforce|shadow|disabled)")
	}
}

type Authorizer struct {
	enforcer *casbin.Enforcer
	mode     Mode
}

func NewAuthorizer(modelPath string, policyPath string, mode Mode) (*Authorizer, error) {
	adapter := fileadapter.NewAdapter(policyPath)
	enforcer, err := casbin.NewEnforcer(modelPath)
	if err != nil {
		return nil, err
	}
	enforcer.SetAdapter(adapter)
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}
	return &Authorizer{enforcer: enforcer, mode: mode}, nil
}

func SubjectFromRoleSlug(roleSlug string) string {
	roleSlug = strings.TrimSpace(strings.ToLower(roleSlug))
	if roleSlug == "" {
		roleSlug = "anonymous"
	}
	return "role:" + roleSlug
}

func DomainFromTenantID(tenantID string) string {
	return strings.ToLower(strings.TrimSpace(tenantID))
}

func (a *Authorizer) Authorize(subject string, domain string, object string, action string) (allowed bool, enforced bool, err error) {
	switch a.mode {
	case ModeDisabled:
		return true, false, nil
	case ModeShadow:
		ok, err := a.enforcer.Enforce(subject, domain, object, action)
		if err != nil {
			return false, false, err
		}
		return ok, false, nil
	case ModeEnforce:
		ok, err := a.enforcer.Enforce(subject, domain, object, action)
		if err != nil {
			return false, true, err
		}
		return ok, true, nil
	default:
		return false, false, errors.New("authz: unknown mode")
	}
}

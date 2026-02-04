package server

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/iam/infrastructure/kratos"
)

var errInvalidCredentials = errors.New("server: invalid credentials")

type authenticatedIdentity struct {
	KratosIdentityID string
	Email            string
	RoleSlug         string
}

type identityProvider interface {
	AuthenticatePassword(ctx context.Context, tenant Tenant, email string, password string) (authenticatedIdentity, error)
}

type kratosIdentityProvider struct {
	client *kratos.Client
}

func newKratosIdentityProviderFromEnv() (identityProvider, error) {
	publicURL := strings.TrimSpace(os.Getenv("KRATOS_PUBLIC_URL"))
	if publicURL == "" {
		publicURL = "http://127.0.0.1:4433"
	}
	c, err := kratos.New(publicURL)
	if err != nil {
		return nil, err
	}
	return &kratosIdentityProvider{client: c}, nil
}

func (p *kratosIdentityProvider) AuthenticatePassword(ctx context.Context, tenant Tenant, email string, password string) (authenticatedIdentity, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	identifier := tenant.ID + ":" + email

	ident, err := p.client.LoginPassword(ctx, identifier, password)
	if err != nil {
		var he *kratos.HTTPError
		if errors.As(err, &he) {
			switch he.StatusCode {
			case 400, 401, 403:
				return authenticatedIdentity{}, errInvalidCredentials
			}
		}
		return authenticatedIdentity{}, err
	}

	tenantTrait, ok := stringTrait(ident.Traits, "tenant_uuid")
	if !ok || tenantTrait != tenant.ID {
		return authenticatedIdentity{}, errors.New("server: kratos tenant mismatch")
	}
	emailTrait, ok := stringTrait(ident.Traits, "email")
	if !ok || strings.ToLower(strings.TrimSpace(emailTrait)) != email {
		return authenticatedIdentity{}, errors.New("server: kratos email mismatch")
	}
	if ident.ID == "" {
		return authenticatedIdentity{}, errors.New("server: kratos missing identity id")
	}

	roleSlug, _ := stringTrait(ident.Traits, "role_slug")
	roleSlug = strings.ToLower(strings.TrimSpace(roleSlug))

	return authenticatedIdentity{
		KratosIdentityID: ident.ID,
		Email:            email,
		RoleSlug:         roleSlug,
	}, nil
}

func stringTrait(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

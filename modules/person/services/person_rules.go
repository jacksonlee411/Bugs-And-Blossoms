package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

type Person struct {
	UUID        string
	Pernr       string
	DisplayName string
	Status      string
	CreatedAt   time.Time
}

type Store interface {
	CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error)
	FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error)
}

type Facade struct {
	store Store
}

type PreparedCreatePerson struct {
	Pernr       string
	DisplayName string
}

var pernrDigitsMax8Re = regexp.MustCompile(`^[0-9]{1,8}$`)

func NewFacade(store Store) Facade {
	return Facade{store: store}
}

func NormalizePernr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("pernr is required")
	}
	if !pernrDigitsMax8Re.MatchString(raw) {
		return "", errors.New("pernr must be 1-8 digits")
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		raw = "0"
	}
	return raw, nil
}

func PrepareCreatePerson(pernr string, displayName string) (PreparedCreatePerson, error) {
	canonical, err := NormalizePernr(pernr)
	if err != nil {
		return PreparedCreatePerson{}, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return PreparedCreatePerson{}, errors.New("display_name is required")
	}
	return PreparedCreatePerson{
		Pernr:       canonical,
		DisplayName: displayName,
	}, nil
}

func PrepareFindPersonByPernr(pernr string) (string, error) {
	return NormalizePernr(pernr)
}

func (f Facade) CreatePerson(ctx context.Context, tenantID string, pernr string, displayName string) (Person, error) {
	prepared, err := PrepareCreatePerson(pernr, displayName)
	if err != nil {
		return Person{}, err
	}
	return f.store.CreatePerson(ctx, tenantID, prepared.Pernr, prepared.DisplayName)
}

func (f Facade) FindPersonByPernr(ctx context.Context, tenantID string, pernr string) (Person, error) {
	canonical, err := PrepareFindPersonByPernr(pernr)
	if err != nil {
		return Person{}, err
	}
	return f.store.FindPersonByPernr(ctx, tenantID, canonical)
}

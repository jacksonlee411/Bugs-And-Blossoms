package superadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/jackc/pgx/v5"
)

var superadminPrincipalRandReader io.Reader = rand.Reader

type memoryPrincipalStore struct {
	mu    sync.Mutex
	byKey map[string]superadminPrincipal
	byID  map[string]superadminPrincipal
}

func newMemoryPrincipalStore() *memoryPrincipalStore {
	return &memoryPrincipalStore{
		byKey: map[string]superadminPrincipal{},
		byID:  map[string]superadminPrincipal{},
	}
}

func (s *memoryPrincipalStore) UpsertFromKratos(_ context.Context, email string, kratosIdentityID string) (superadminPrincipal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := email
	if p, ok := s.byKey[key]; ok {
		if p.Status != "active" {
			return superadminPrincipal{}, errors.New("superadmin: principal is not active")
		}
		if p.KratosIdentityID != "" && p.KratosIdentityID != kratosIdentityID {
			return superadminPrincipal{}, errors.New("superadmin: principal kratos identity mismatch")
		}
		if p.KratosIdentityID == "" {
			p.KratosIdentityID = kratosIdentityID
			s.byKey[key] = p
			s.byID[p.ID] = p
		}
		return p, nil
	}

	var idb [16]byte
	if _, err := superadminPrincipalRandReader.Read(idb[:]); err != nil {
		return superadminPrincipal{}, err
	}
	id := base64.RawURLEncoding.EncodeToString(idb[:])
	p := superadminPrincipal{
		ID:               id,
		Email:            email,
		Status:           "active",
		KratosIdentityID: kratosIdentityID,
	}
	s.byKey[key] = p
	s.byID[id] = p
	return p, nil
}

func (s *memoryPrincipalStore) GetByID(_ context.Context, principalID string) (superadminPrincipal, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.byID[principalID]
	if !ok {
		return superadminPrincipal{}, false, nil
	}
	return p, true, nil
}

type pgPrincipalStore struct {
	q queryExecer
}

func newPrincipalStoreFromDB(db queryExecer) principalStore {
	if db == nil {
		return newMemoryPrincipalStore()
	}
	return &pgPrincipalStore{q: db}
}

func (s *pgPrincipalStore) UpsertFromKratos(ctx context.Context, email string, kratosIdentityID string) (superadminPrincipal, error) {
	var p superadminPrincipal
	p.Email = email
	p.Status = "active"
	p.KratosIdentityID = kratosIdentityID

	err := s.q.QueryRow(ctx, `
INSERT INTO iam.superadmin_principals (email, status, kratos_identity_id)
VALUES ($1, 'active', $2::uuid)
ON CONFLICT (email) DO UPDATE SET
  kratos_identity_id = COALESCE(iam.superadmin_principals.kratos_identity_id, EXCLUDED.kratos_identity_id),
  updated_at = now()
RETURNING id::text, status, COALESCE(kratos_identity_id::text, '');
`, email, kratosIdentityID).Scan(&p.ID, &p.Status, &p.KratosIdentityID)
	if err != nil {
		return superadminPrincipal{}, err
	}
	if p.Status != "active" {
		return superadminPrincipal{}, errors.New("superadmin: principal is not active")
	}
	if p.KratosIdentityID == "" {
		return superadminPrincipal{}, errors.New("superadmin: principal missing kratos identity")
	}
	if p.KratosIdentityID != kratosIdentityID {
		return superadminPrincipal{}, errors.New("superadmin: principal kratos identity mismatch")
	}
	return p, nil
}

func (s *pgPrincipalStore) GetByID(ctx context.Context, principalID string) (superadminPrincipal, bool, error) {
	var p superadminPrincipal
	err := s.q.QueryRow(ctx, `
SELECT id::text, email, status, COALESCE(kratos_identity_id::text, '')
FROM iam.superadmin_principals
WHERE id = $1;
`, principalID).Scan(&p.ID, &p.Email, &p.Status, &p.KratosIdentityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return superadminPrincipal{}, false, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return superadminPrincipal{}, false, err
		}
		return superadminPrincipal{}, false, err
	}
	return p, true, nil
}

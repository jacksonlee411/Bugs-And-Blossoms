package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type beginnerStub struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}

func (b beginnerStub) Begin(ctx context.Context) (pgx.Tx, error) {
	return b.beginFn(ctx)
}

type txStub struct {
	pgx.Tx

	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (t *txStub) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.execFn != nil {
		return t.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (t *txStub) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.queryFn != nil {
		return t.queryFn(ctx, sql, args...)
	}
	return &rowsStub{}, nil
}

func (t *txStub) Commit(ctx context.Context) error {
	if t.commitFn != nil {
		return t.commitFn(ctx)
	}
	return nil
}

func (t *txStub) Rollback(ctx context.Context) error {
	if t.rollbackFn != nil {
		return t.rollbackFn(ctx)
	}
	return nil
}

type rowsStub struct {
	nextIdx int
	nextSeq []bool
	scanFn  func(dest ...any) error
	err     error
}

func (r *rowsStub) Next() bool {
	if r.nextSeq == nil {
		return false
	}
	if r.nextIdx >= len(r.nextSeq) {
		return false
	}
	v := r.nextSeq[r.nextIdx]
	r.nextIdx++
	return v
}

func (r *rowsStub) Scan(dest ...any) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return nil
}

func (r *rowsStub) Err() error { return r.err }

func (r *rowsStub) Close() {}

func (r *rowsStub) CommandTag() pgconn.CommandTag { return pgconn.CommandTag{} }

func (r *rowsStub) FieldDescriptions() []pgconn.FieldDescription { return nil }

func (r *rowsStub) Values() ([]any, error) { return nil, nil }

func (r *rowsStub) RawValues() [][]byte { return nil }

func (r *rowsStub) Conn() *pgx.Conn { return nil }

func TestNormalizeExternalIdentityProvider(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		if _, err := normalizeExternalIdentityProvider(""); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := normalizeExternalIdentityProvider("bad"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("wecom", func(t *testing.T) {
		got, err := normalizeExternalIdentityProvider("wecom")
		if err != nil {
			t.Fatal(err)
		}
		if got != "WECOM" {
			t.Fatalf("got=%q", got)
		}
	})
	t.Run("dingtalk", func(t *testing.T) {
		got, err := normalizeExternalIdentityProvider(" dingtalk ")
		if err != nil {
			t.Fatal(err)
		}
		if got != "DINGTALK" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestNormalizeExternalUserID(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		if _, err := normalizeExternalUserID(""); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("trim", func(t *testing.T) {
		got, err := normalizeExternalUserID("  u1 ")
		if err != nil {
			t.Fatal(err)
		}
		if got != "u1" {
			t.Fatalf("got=%q", got)
		}
	})
}

func TestPersonMemoryStore_ExternalIdentityOps(t *testing.T) {
	s := newPersonMemoryStore().(*personMemoryStore)
	tenantID := "t1"

	if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u-missing"); err == nil {
		t.Fatal("expected error")
	}

	if err := s.LinkExternalIdentity(context.Background(), tenantID, "BAD", "u1", "p1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "", "p1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", ""); err == nil {
		t.Fatal("expected error")
	}

	if err := s.LinkExternalIdentity(context.Background(), tenantID, "wecom", "u1", "p1"); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "p1"); err != nil {
		t.Fatal(err)
	}

	if got, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 0); err != nil {
		t.Fatal(err)
	} else if len(got) != 1 || got[0].Provider != "WECOM" || got[0].ExternalUserID != "u1" {
		t.Fatalf("got=%+v", got)
	}

	if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 99999); err != nil {
		t.Fatal(err)
	}

	if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := s.EnableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}

	if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := s.IgnoreExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := s.UnignoreExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}

	if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u-missing"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.IgnoreExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
		t.Fatal(err)
	}
	if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
		t.Fatal("expected error")
	}

	if err := s.DisableExternalIdentity(context.Background(), tenantID, "BAD", "u1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", ""); err == nil {
		t.Fatal("expected error")
	}
	if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "", "u1"); err == nil {
		t.Fatal("expected error")
	}
	if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", ""); err == nil {
		t.Fatal("expected error")
	}

	now := time.Unix(1, 0).UTC()
	s.links[s.externalIdentityKey(tenantID, "WECOM", "a")] = ExternalIdentityLink{TenantID: tenantID, Provider: "WECOM", ExternalUserID: "a", Status: "pending", LastSeenAt: now}
	s.links[s.externalIdentityKey(tenantID, "DINGTALK", "b")] = ExternalIdentityLink{TenantID: tenantID, Provider: "DINGTALK", ExternalUserID: "b", Status: "pending", LastSeenAt: now}
	s.links[s.externalIdentityKey(tenantID, "WECOM", "c")] = ExternalIdentityLink{TenantID: tenantID, Provider: "WECOM", ExternalUserID: "c", Status: "pending", LastSeenAt: now}
	if got, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 2); err != nil {
		t.Fatal(err)
	} else if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestPersonPGStore_ListExternalIdentityLinks_Coverage(t *testing.T) {
	tenantID := "00000000-0000-0000-0000-000000000001"

	t.Run("begin error", func(t *testing.T) {
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config exec error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, errors.New("exec")
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 99999); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("query error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return nil, errors.New("query")
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn: func(...any) error {
				return errors.New("scan")
			},
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rs, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows err", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn:  scanOneExternalIdentityLink("WECOM", "u1", "active", "person-1"),
			err:     errors.New("rows"),
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rs, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn:  scanOneExternalIdentityLink("WECOM", "u1", "active", "person-1"),
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rs, nil
			},
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn:  scanOneExternalIdentityLink("WECOM", "u1", "active", "person-1"),
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rs, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		links, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 1 || links[0].Provider != "WECOM" || links[0].ExternalUserID != "u1" || links[0].PersonUUID == nil {
			t.Fatalf("links=%+v", links)
		}
	})

	t.Run("success without person_uuid", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn:  scanOneExternalIdentityLink("WECOM", "u1", "pending", ""),
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(context.Context, string, ...any) (pgx.Rows, error) {
				return rs, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		links, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 1 || links[0].PersonUUID != nil {
			t.Fatalf("links=%+v", links)
		}
	})

	t.Run("limit capped to 2000", func(t *testing.T) {
		rs := &rowsStub{
			nextSeq: []bool{true, false},
			scanFn:  scanOneExternalIdentityLink("WECOM", "u1", "active", "person-1"),
		}
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
				if len(args) != 2 {
					t.Fatalf("args=%v", args)
				}
				limit, ok := args[1].(int)
				if !ok {
					t.Fatalf("unexpected limit type: %T", args[1])
				}
				if limit != 2000 {
					t.Fatalf("limit=%d", limit)
				}
				return rs, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if _, err := s.ListExternalIdentityLinks(context.Background(), tenantID, 99999); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPersonPGStore_LinkExternalIdentity_Coverage(t *testing.T) {
	tenantID := "00000000-0000-0000-0000-000000000001"

	t.Run("begin error", func(t *testing.T) {
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config exec error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
				return pgconn.CommandTag{}, errors.New("exec")
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("provider invalid", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "", "u1", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("external user id missing", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("person_uuid missing", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("insert exec error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "INSERT INTO person.external_identity_links") {
					return pgconn.CommandTag{}, errors.New("insert")
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &txStub{
			execFn:   func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "person-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.LinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1", "person-1"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPersonPGStore_UpdateExternalIdentityStatus_Coverage(t *testing.T) {
	tenantID := "00000000-0000-0000-0000-000000000001"

	t.Run("begin error", func(t *testing.T) {
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config exec error", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec")
		}}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("provider invalid", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("external user id invalid", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.updateExternalIdentityStatus(context.Background(), tenantID, "WECOM", "u1", "nope"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows affected 0", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 0"), nil
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update exec error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.CommandTag{}, errors.New("update")
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				}
				return pgconn.CommandTag{}, nil
			},
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success for all cases", func(t *testing.T) {
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) {
			tx := &txStub{execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				}
				return pgconn.CommandTag{}, nil
			}}
			return tx, nil
		}}}

		if err := s.DisableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
			t.Fatal(err)
		}
		if err := s.EnableExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
			t.Fatal(err)
		}
		if err := s.IgnoreExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
			t.Fatal(err)
		}
		if err := s.UnignoreExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
			t.Fatal(err)
		}
	})
}

func TestPersonPGStore_UnlinkExternalIdentity_Coverage(t *testing.T) {
	tenantID := "00000000-0000-0000-0000-000000000001"

	t.Run("begin error", func(t *testing.T) {
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config exec error", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec")
		}}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("provider invalid", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("external user id invalid", func(t *testing.T) {
		tx := &txStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", ""); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rows affected 0", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 0"), nil
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update exec error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.CommandTag{}, errors.New("update")
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				}
				return pgconn.CommandTag{}, nil
			},
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &txStub{
			execFn: func(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
				if strings.Contains(sql, "UPDATE person.external_identity_links") {
					return pgconn.NewCommandTag("UPDATE 1"), nil
				}
				return pgconn.CommandTag{}, nil
			},
		}
		s := &personPGStore{pool: beginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }}}
		if err := s.UnlinkExternalIdentity(context.Background(), tenantID, "WECOM", "u1"); err != nil {
			t.Fatal(err)
		}
	})
}

func scanOneExternalIdentityLink(provider string, externalUserID string, status string, personUUID string) func(dest ...any) error {
	return func(dest ...any) error {
		if len(dest) != 10 {
			return errors.New("unexpected scan dest")
		}
		*dest[0].(*string) = provider
		*dest[1].(*string) = externalUserID
		*dest[2].(*string) = status
		*dest[3].(*string) = personUUID
		*dest[4].(*time.Time) = time.Unix(1, 0).UTC()
		*dest[5].(*time.Time) = time.Unix(2, 0).UTC()
		*dest[6].(*int) = 1
		*dest[7].(*[]byte) = []byte(`{}`)
		*dest[8].(*time.Time) = time.Unix(3, 0).UTC()
		*dest[9].(*time.Time) = time.Unix(4, 0).UTC()
		return nil
	}
}

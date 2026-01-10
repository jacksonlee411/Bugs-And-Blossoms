package attendanceintegrations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type pgBeginnerStub struct {
	beginFn func(ctx context.Context) (pgx.Tx, error)
}

func (b pgBeginnerStub) Begin(ctx context.Context) (pgx.Tx, error) {
	return b.beginFn(ctx)
}

type rowStub struct {
	scanFn func(dest ...any) error
}

func (r rowStub) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type pgTxStub struct {
	pgx.Tx

	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	commitFn   func(ctx context.Context) error
	rollbackFn func(ctx context.Context) error
}

func (t *pgTxStub) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.execFn != nil {
		return t.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (t *pgTxStub) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return rowStub{scanFn: func(...any) error { return nil }}
}

func (t *pgTxStub) Commit(ctx context.Context) error {
	if t.commitFn != nil {
		return t.commitFn(ctx)
	}
	return nil
}

func (t *pgTxStub) Rollback(ctx context.Context) error {
	if t.rollbackFn != nil {
		return t.rollbackFn(ctx)
	}
	return nil
}

func TestNormalizeProvider(t *testing.T) {
	if got, err := normalizeProvider(Provider("dingtalk")); err != nil || got != ProviderDingTalk {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if got, err := normalizeProvider(Provider(" wecom ")); err != nil || got != ProviderWeCom {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := normalizeProvider(Provider("nope")); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeExternalUserID(t *testing.T) {
	if _, err := normalizeExternalUserID(""); err == nil {
		t.Fatal("expected error")
	}
	if got, err := normalizeExternalUserID(" u1 "); err != nil || got != "u1" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestNormalizeJSONObj(t *testing.T) {
	if got, err := normalizeJSONObj(nil); err != nil || string(got) != "{}" {
		t.Fatalf("got=%q err=%v", string(got), err)
	}
	if _, err := normalizeJSONObj([]byte("{")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := normalizeJSONObj([]byte(`[]`)); err == nil {
		t.Fatal("expected error")
	}
	if got, err := normalizeJSONObj([]byte(`{"a":1}`)); err != nil || string(got) != `{"a":1}` {
		t.Fatalf("got=%q err=%v", string(got), err)
	}
}

func TestPGStore_TouchExternalIdentityLink(t *testing.T) {
	t.Run("tenant missing", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).TouchExternalIdentityLink(context.Background(), "", ProviderDingTalk, "u1", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("provider invalid", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).TouchExternalIdentityLink(context.Background(), "t1", Provider("bad"), "u1", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("external user id invalid", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "", []byte(`{}`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("payload invalid json", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("payload not object", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`[]`))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("begin error", func(t *testing.T) {
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) {
			return nil, errors.New("begin")
		}})
		if _, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		tx := &pgTxStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec")
		}}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		if _, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("upsert scan error", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(...any) error { return errors.New("scan") }}
			},
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		if _, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "pending"
					*dest[1].(*string) = ""
					return nil
				}}
			},
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		if _, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success without person_uuid", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "pending"
					*dest[1].(*string) = ""
					return nil
				}}
			},
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		res, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if res.Status != IdentityStatusPending || res.PersonUUID != nil {
			t.Fatalf("res=%+v", res)
		}
	})

	t.Run("success with person_uuid", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(dest ...any) error {
					*dest[0].(*string) = "active"
					*dest[1].(*string) = "p1"
					return nil
				}}
			},
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		res, err := s.TouchExternalIdentityLink(context.Background(), "t1", ProviderDingTalk, "u1", []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if res.Status != IdentityStatus("active") || res.PersonUUID == nil || *res.PersonUUID != "p1" {
			t.Fatalf("res=%+v", res)
		}
	})
}

func TestPGStore_SubmitTimePunch(t *testing.T) {
	now := time.Now().UTC()

	t.Run("tenant missing", func(t *testing.T) {
		if _, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("person missing", func(t *testing.T) {
		if _, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{TenantID: "t1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("initiator missing", func(t *testing.T) {
		if _, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{TenantID: "t1", PersonUUID: "p1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("request id missing", func(t *testing.T) {
		if _, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{TenantID: "t1", PersonUUID: "p1", InitiatorID: "i1"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("provider invalid", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: Provider("bad"),
			PunchType:      "RAW",
			PunchTime:      now,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("punch type missing", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchTime:      now,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("payload invalid", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchType:      "RAW",
			PunchTime:      now,
			Payload:        []byte("{"),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("source raw invalid", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:         "t1",
			PersonUUID:       "p1",
			InitiatorID:      "i1",
			RequestID:        "r1",
			SourceProvider:   ProviderDingTalk,
			PunchType:        "RAW",
			PunchTime:        now,
			Payload:          []byte(`{}`),
			SourceRawPayload: []byte("{"),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("device info not object", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:         "t1",
			PersonUUID:       "p1",
			InitiatorID:      "i1",
			RequestID:        "r1",
			SourceProvider:   ProviderDingTalk,
			PunchType:        "RAW",
			PunchTime:        now,
			Payload:          []byte(`{}`),
			SourceRawPayload: []byte(`{}`),
			DeviceInfo:       []byte(`[]`),
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("punch time required", func(t *testing.T) {
		_, err := NewPGStore(pgBeginnerStub{}).SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchType:      "RAW",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("begin error", func(t *testing.T) {
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return nil, errors.New("begin") }})
		_, err := s.SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchType:      "RAW",
			PunchTime:      now,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("set_config error", func(t *testing.T) {
		tx := &pgTxStub{execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("exec")
		}}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		_, err := s.SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchType:      "RAW",
			PunchTime:      now,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("kernel scan error", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(...any) error { return errors.New("scan") }}
			},
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		_, err := s.SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderWeCom,
			PunchType:      "RAW",
			PunchTime:      now,
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 99
					return nil
				}}
			},
			commitFn: func(context.Context) error { return errors.New("commit") },
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		if _, err := s.SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderDingTalk,
			PunchType:      "RAW",
			PunchTime:      now,
		}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		tx := &pgTxStub{
			execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil },
			queryRowFn: func(context.Context, string, ...any) pgx.Row {
				return rowStub{scanFn: func(dest ...any) error {
					*dest[0].(*int64) = 100
					return nil
				}}
			},
		}
		s := NewPGStore(pgBeginnerStub{beginFn: func(context.Context) (pgx.Tx, error) { return tx, nil }})
		got, err := s.SubmitTimePunch(context.Background(), SubmitTimePunchParams{
			TenantID:       "t1",
			PersonUUID:     "p1",
			InitiatorID:    "i1",
			RequestID:      "r1",
			SourceProvider: ProviderWeCom,
			PunchType:      "raw",
			PunchTime:      now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != 100 {
			t.Fatalf("got=%d", got)
		}
	})
}

package attendanceintegrations

import (
	"context"
	"errors"
	"testing"
	"time"
)

type storeStub struct {
	touchFn  func(ctx context.Context, tenantID string, provider Provider, externalUserID string, lastSeenPayload []byte) (IdentityResolution, error)
	submitFn func(ctx context.Context, params SubmitTimePunchParams) (int64, error)
}

func (s storeStub) TouchExternalIdentityLink(ctx context.Context, tenantID string, provider Provider, externalUserID string, lastSeenPayload []byte) (IdentityResolution, error) {
	return s.touchFn(ctx, tenantID, provider, externalUserID, lastSeenPayload)
}

func (s storeStub) SubmitTimePunch(ctx context.Context, params SubmitTimePunchParams) (int64, error) {
	return s.submitFn(ctx, params)
}

func TestIngestExternalPunch(t *testing.T) {
	punch := ExternalPunch{
		Provider:         ProviderDingTalk,
		ExternalUserID:   "u1",
		PunchTime:        time.Unix(1, 0).UTC(),
		PunchType:        "RAW",
		RequestID:        "r1",
		LastSeenPayload:  []byte(`{}`),
		Payload:          []byte(`{}`),
		SourceRawPayload: []byte(`{}`),
		DeviceInfo:       []byte(`{}`),
	}

	t.Run("tenant missing", func(t *testing.T) {
		_, err := IngestExternalPunch(context.Background(), storeStub{}, "", "i1", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("initiator missing", func(t *testing.T) {
		_, err := IngestExternalPunch(context.Background(), storeStub{}, "t1", "", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("touch error", func(t *testing.T) {
		_, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{}, errors.New("boom")
			},
		}, "t1", "i1", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unknown identity status", func(t *testing.T) {
		_, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: "weird"}, nil
			},
		}, "t1", "i1", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("active but missing person_uuid", func(t *testing.T) {
		_, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusActive, PersonUUID: nil}, nil
			},
		}, "t1", "i1", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("active submit error", func(t *testing.T) {
		personUUID := "p1"
		_, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusActive, PersonUUID: &personUUID}, nil
			},
			submitFn: func(context.Context, SubmitTimePunchParams) (int64, error) {
				return 0, errors.New("submit")
			},
		}, "t1", "i1", punch)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("active ingested", func(t *testing.T) {
		personUUID := "p1"
		got, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusActive, PersonUUID: &personUUID}, nil
			},
			submitFn: func(_ context.Context, params SubmitTimePunchParams) (int64, error) {
				if params.TenantID != "t1" || params.InitiatorID != "i1" || params.PersonUUID != "p1" || params.SourceProvider != ProviderDingTalk {
					t.Fatalf("params=%+v", params)
				}
				return 123, nil
			},
		}, "t1", "i1", punch)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != IngestOutcomeIngested || got.EventDBID != 123 {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("pending unmapped", func(t *testing.T) {
		got, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusPending}, nil
			},
		}, "t1", "i1", punch)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != IngestOutcomeUnmapped {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("ignored", func(t *testing.T) {
		got, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusIgnored}, nil
			},
		}, "t1", "i1", punch)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != IngestOutcomeIgnored {
			t.Fatalf("got=%+v", got)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		personUUID := "p1"
		got, err := IngestExternalPunch(context.Background(), storeStub{
			touchFn: func(context.Context, string, Provider, string, []byte) (IdentityResolution, error) {
				return IdentityResolution{Status: IdentityStatusDisabled, PersonUUID: &personUUID}, nil
			},
		}, "t1", "i1", punch)
		if err != nil {
			t.Fatal(err)
		}
		if got.Outcome != IngestOutcomeDisabled {
			t.Fatalf("got=%+v", got)
		}
	})
}

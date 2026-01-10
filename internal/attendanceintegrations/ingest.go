package attendanceintegrations

import (
	"context"
	"errors"
	"strings"
)

type Store interface {
	TouchExternalIdentityLink(ctx context.Context, tenantID string, provider Provider, externalUserID string, lastSeenPayload []byte) (IdentityResolution, error)
	SubmitTimePunch(ctx context.Context, params SubmitTimePunchParams) (int64, error)
}

func IngestExternalPunch(ctx context.Context, store Store, tenantID string, initiatorID string, punch ExternalPunch) (IngestResult, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return IngestResult{}, errors.New("tenant_id is required")
	}
	initiatorID = strings.TrimSpace(initiatorID)
	if initiatorID == "" {
		return IngestResult{}, errors.New("initiator_id is required")
	}

	res, err := store.TouchExternalIdentityLink(ctx, tenantID, punch.Provider, punch.ExternalUserID, punch.LastSeenPayload)
	if err != nil {
		return IngestResult{}, err
	}

	out := IngestResult{
		IdentityStatus: res.Status,
		PersonUUID:     res.PersonUUID,
	}

	switch res.Status {
	case IdentityStatusActive:
		if res.PersonUUID == nil || strings.TrimSpace(*res.PersonUUID) == "" {
			return IngestResult{}, errors.New("active identity missing person_uuid")
		}

		eventDBID, err := store.SubmitTimePunch(ctx, SubmitTimePunchParams{
			TenantID:         tenantID,
			PersonUUID:       *res.PersonUUID,
			PunchTime:        punch.PunchTime,
			PunchType:        punch.PunchType,
			SourceProvider:   punch.Provider,
			Payload:          punch.Payload,
			SourceRawPayload: punch.SourceRawPayload,
			DeviceInfo:       punch.DeviceInfo,
			RequestID:        punch.RequestID,
			InitiatorID:      initiatorID,
		})
		if err != nil {
			return IngestResult{}, err
		}

		out.Outcome = IngestOutcomeIngested
		out.EventDBID = eventDBID
		return out, nil
	case IdentityStatusPending:
		out.Outcome = IngestOutcomeUnmapped
		return out, nil
	case IdentityStatusIgnored:
		out.Outcome = IngestOutcomeIgnored
		return out, nil
	case IdentityStatusDisabled:
		out.Outcome = IngestOutcomeDisabled
		return out, nil
	default:
		return IngestResult{}, errors.New("unknown identity status")
	}
}

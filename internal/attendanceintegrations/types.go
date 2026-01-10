package attendanceintegrations

import (
	"encoding/json"
	"time"
)

type Provider string

const (
	ProviderDingTalk Provider = "DINGTALK"
	ProviderWeCom    Provider = "WECOM"
)

type IdentityStatus string

const (
	IdentityStatusPending  IdentityStatus = "pending"
	IdentityStatusActive   IdentityStatus = "active"
	IdentityStatusDisabled IdentityStatus = "disabled"
	IdentityStatusIgnored  IdentityStatus = "ignored"
)

type IdentityResolution struct {
	Status     IdentityStatus
	PersonUUID *string
}

type ExternalPunch struct {
	Provider         Provider
	ExternalUserID   string
	PunchTime        time.Time
	PunchType        string
	RequestID        string
	Payload          json.RawMessage
	SourceRawPayload json.RawMessage
	DeviceInfo       json.RawMessage

	LastSeenPayload json.RawMessage
}

type IngestOutcome string

const (
	IngestOutcomeIngested IngestOutcome = "ingested"
	IngestOutcomeUnmapped IngestOutcome = "unmapped"
	IngestOutcomeIgnored  IngestOutcome = "ignored"
	IngestOutcomeDisabled IngestOutcome = "disabled"
)

type IngestResult struct {
	Outcome        IngestOutcome
	IdentityStatus IdentityStatus
	PersonUUID     *string
	EventDBID      int64
}

type SubmitTimePunchParams struct {
	TenantID         string
	PersonUUID       string
	PunchTime        time.Time
	PunchType        string
	SourceProvider   Provider
	Payload          json.RawMessage
	SourceRawPayload json.RawMessage
	DeviceInfo       json.RawMessage
	RequestID        string
	InitiatorID      string
}

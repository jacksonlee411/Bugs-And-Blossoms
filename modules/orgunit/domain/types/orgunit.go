package types

import (
	"encoding/json"
	"time"
)

type OrgUnitEventType string

const (
	OrgUnitEventCreate          OrgUnitEventType = "CREATE"
	OrgUnitEventUpdate          OrgUnitEventType = "UPDATE"
	OrgUnitEventMove            OrgUnitEventType = "MOVE"
	OrgUnitEventRename          OrgUnitEventType = "RENAME"
	OrgUnitEventDisable         OrgUnitEventType = "DISABLE"
	OrgUnitEventEnable          OrgUnitEventType = "ENABLE"
	OrgUnitEventSetBusinessUnit OrgUnitEventType = "SET_BUSINESS_UNIT"
)

type OrgUnitEvent struct {
	ID              int64
	EventUUID       string
	OrgNodeKey      string
	EventType       OrgUnitEventType
	EffectiveDate   string
	Payload         json.RawMessage
	TransactionTime time.Time
}

type OrgUnitResult struct {
	OrgCode       string
	EffectiveDate string
	Fields        map[string]any
}

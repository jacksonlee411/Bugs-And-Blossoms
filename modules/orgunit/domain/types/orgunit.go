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
	OrgID           int
	EventType       OrgUnitEventType
	EffectiveDate   string
	Payload         json.RawMessage
	TransactionTime time.Time
}

type Person struct {
	UUID        string
	Pernr       string
	DisplayName string
	Status      string
}

type OrgUnitResult struct {
	OrgID         string
	OrgCode       string
	EffectiveDate string
	Fields        map[string]any
}

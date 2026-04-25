package cubebox

import "strings"

const QueryEntityConfirmedEventType = "turn.query_entity.confirmed"

type QueryEntity struct {
	Domain        string `json:"domain"`
	Intent        string `json:"intent,omitempty"`
	EntityKey     string `json:"entity_key,omitempty"`
	AsOf          string `json:"as_of,omitempty"`
	SourceAPIKey  string `json:"source_api_key,omitempty"`
	TargetOrgCode string `json:"target_org_code,omitempty"`
	ParentOrgCode string `json:"parent_org_code,omitempty"`
}

type QueryContext struct {
	RecentConfirmedEntity *QueryEntity
}

func QueryContextFromEvents(events []CanonicalEvent) QueryContext {
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if strings.TrimSpace(event.Type) != QueryEntityConfirmedEventType {
			continue
		}
		if entity := DecodeQueryEntity(event.Payload); entity != nil {
			return QueryContext{RecentConfirmedEntity: entity}
		}
	}
	return QueryContext{}
}

func DecodeQueryEntity(payload map[string]any) *QueryEntity {
	if len(payload) == 0 {
		return nil
	}
	if raw, ok := payload["entity"].(map[string]any); ok {
		return NormalizeQueryEntity(QueryEntity{
			Domain:        stringValue(raw["domain"]),
			Intent:        stringValue(raw["intent"]),
			EntityKey:     stringValue(raw["entity_key"]),
			AsOf:          stringValue(raw["as_of"]),
			SourceAPIKey:  stringValue(raw["source_api_key"]),
			TargetOrgCode: stringValue(raw["target_org_code"]),
			ParentOrgCode: stringValue(raw["parent_org_code"]),
		})
	}
	return NormalizeQueryEntity(QueryEntity{
		Domain:        stringValue(payload["domain"]),
		Intent:        stringValue(payload["intent"]),
		EntityKey:     stringValue(payload["entity_key"]),
		AsOf:          stringValue(payload["as_of"]),
		SourceAPIKey:  stringValue(payload["source_api_key"]),
		TargetOrgCode: stringValue(payload["target_org_code"]),
		ParentOrgCode: stringValue(payload["parent_org_code"]),
	})
}

func NormalizeQueryEntity(entity QueryEntity) *QueryEntity {
	entity.Domain = strings.ToLower(strings.TrimSpace(entity.Domain))
	entity.Intent = strings.TrimSpace(entity.Intent)
	entity.EntityKey = strings.TrimSpace(entity.EntityKey)
	entity.AsOf = strings.TrimSpace(entity.AsOf)
	entity.SourceAPIKey = strings.TrimSpace(entity.SourceAPIKey)
	entity.TargetOrgCode = strings.TrimSpace(entity.TargetOrgCode)
	entity.ParentOrgCode = strings.TrimSpace(entity.ParentOrgCode)
	if entity.Domain == "" || entity.EntityKey == "" {
		return nil
	}
	return &entity
}

func (e QueryEntity) Payload() map[string]any {
	normalized := NormalizeQueryEntity(e)
	if normalized == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"domain":     normalized.Domain,
		"entity_key": normalized.EntityKey,
	}
	if intent := normalized.Intent; intent != "" {
		payload["intent"] = intent
	}
	if asOf := normalized.AsOf; asOf != "" {
		payload["as_of"] = asOf
	}
	if sourceAPIKey := normalized.SourceAPIKey; sourceAPIKey != "" {
		payload["source_api_key"] = sourceAPIKey
	}
	if targetOrgCode := normalized.TargetOrgCode; targetOrgCode != "" {
		payload["target_org_code"] = targetOrgCode
	}
	if parentOrgCode := normalized.ParentOrgCode; parentOrgCode != "" {
		payload["parent_org_code"] = parentOrgCode
	}
	return payload
}

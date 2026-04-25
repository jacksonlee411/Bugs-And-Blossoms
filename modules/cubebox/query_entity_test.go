package cubebox

import "testing"

func TestQueryContextFromEventsReturnsMostRecentConfirmedEntity(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: QueryEntityConfirmedEventType,
			Payload: map[string]any{"entity": map[string]any{
				"domain":         "orgunit",
				"intent":         "orgunit.details",
				"entity_key":     "100000",
				"as_of":          "2026-04-24",
				"source_api_key": "orgunit.details",
			}},
		},
		{
			Type: "turn.agent_message.delta",
			Payload: map[string]any{
				"delta": "ignored",
			},
		},
		{
			Type: QueryEntityConfirmedEventType,
			Payload: map[string]any{"entity": map[string]any{
				"domain":          "orgunit",
				"intent":          "orgunit.list",
				"entity_key":      "200000",
				"as_of":           "2026-04-25",
				"source_api_key":  "orgunit.list",
				"parent_org_code": "200000",
			}},
		},
	})

	if context.RecentConfirmedEntity == nil {
		t.Fatal("expected recent entity")
	}
	if context.RecentConfirmedEntity.EntityKey != "200000" || context.RecentConfirmedEntity.AsOf != "2026-04-25" {
		t.Fatalf("unexpected entity=%#v", context.RecentConfirmedEntity)
	}
}

func TestQueryContextFromEventsNormalizesConfirmedEntityDomain(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: QueryEntityConfirmedEventType,
			Payload: map[string]any{"entity": map[string]any{
				"domain":     " OrgUnit ",
				"entity_key": "100000",
			}},
		},
	})

	if context.RecentConfirmedEntity == nil {
		t.Fatal("expected recent entity")
	}
	if context.RecentConfirmedEntity.Domain != "orgunit" {
		t.Fatalf("expected lower-case domain, got %#v", context.RecentConfirmedEntity)
	}
}

func TestQueryContextFromEventsSkipsInvalidConfirmedEntity(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type:    QueryEntityConfirmedEventType,
			Payload: map[string]any{"entity": map[string]any{"domain": "orgunit"}},
		},
	})

	if context.RecentConfirmedEntity != nil {
		t.Fatalf("expected invalid entity skipped, got %#v", context.RecentConfirmedEntity)
	}
}

func TestQueryEntityPayloadUsesMinimalSchema(t *testing.T) {
	entity := QueryEntity{
		Domain:        " OrgUnit ",
		Intent:        " orgunit.details ",
		EntityKey:     " 100000 ",
		AsOf:          " 2026-04-25 ",
		SourceAPIKey:  " orgunit.details ",
		TargetOrgCode: " ",
		ParentOrgCode: " ROOT ",
	}

	payload := entity.Payload()
	if payload["domain"] != "orgunit" || payload["entity_key"] != "100000" || payload["as_of"] != "2026-04-25" {
		t.Fatalf("unexpected payload=%#v", payload)
	}
	if _, ok := payload["target_org_code"]; ok {
		t.Fatalf("did not expect empty target_org_code in payload=%#v", payload)
	}
	if payload["parent_org_code"] != "ROOT" {
		t.Fatalf("unexpected parent_org_code=%#v", payload)
	}
}

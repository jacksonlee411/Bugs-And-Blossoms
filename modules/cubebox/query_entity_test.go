package cubebox

import (
	"reflect"
	"testing"
)

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
	if len(context.RecentConfirmedEntities) != 2 {
		t.Fatalf("expected recent entities, got %#v", context.RecentConfirmedEntities)
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

func TestQueryContextFromEventsExtractsCandidatesAndClarification(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: "turn.user_message.accepted",
			Payload: map[string]any{
				"text": "查询飞虫公司的下级组织，只有名称",
			},
		},
		{
			Type: QueryCandidatesPresentedEventType,
			Payload: map[string]any{
				"candidates": []any{
					map[string]any{"domain": "orgunit", "entity_key": "200000", "name": "飞虫公司", "as_of": "2026-04-25"},
					map[string]any{"domain": "orgunit", "entity_key": "300000", "name": "鲜花公司", "as_of": "2026-04-25"},
				},
			},
		},
		{
			Type: QueryClarificationRequestedEventType,
			Payload: map[string]any{
				"intent":              "orgunit.list",
				"missing_params":      []any{"parent_org_code"},
				"clarifying_question": "请先确认你要查哪个组织。",
			},
		},
		{
			Type: QueryContextResolvedEventType,
			Payload: map[string]any{
				"entity": map[string]any{
					"domain":     "orgunit",
					"entity_key": "200000",
					"as_of":      "2026-04-25",
				},
			},
		},
	})

	if len(context.RecentCandidates) != 2 {
		t.Fatalf("expected recent candidates, got %#v", context.RecentCandidates)
	}
	if context.LastClarification == nil || context.LastClarification.ClarifyingQuestion == "" {
		t.Fatalf("expected clarification, got %#v", context.LastClarification)
	}
	if context.ResolvedEntity == nil || context.ResolvedEntity.EntityKey != "200000" {
		t.Fatalf("expected resolved entity, got %#v", context.ResolvedEntity)
	}
	if len(context.RecentDialogueTurns) == 0 {
		t.Fatalf("expected dialogue turns, got %#v", context.RecentDialogueTurns)
	}
}

func TestQueryContextFromEventsMergesAssistantDeltasWithoutDuplicatingClarification(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: "turn.user_message.accepted",
			Payload: map[string]any{
				"message_id": "msg_user_1",
				"text":       "查询飞虫公司的下级组织，只有名称",
			},
		},
		{
			Type: QueryClarificationRequestedEventType,
			Payload: map[string]any{
				"intent":              "orgunit.list",
				"missing_params":      []string{"parent_org_code"},
				"clarifying_question": "请先确认你要查哪个组织。",
			},
		},
		{
			Type: "turn.agent_message.delta",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
				"delta":      "请先确认",
			},
		},
		{
			Type: "turn.agent_message.delta",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
				"delta":      "你要查哪个组织。",
			},
		},
		{
			Type: "turn.agent_message.completed",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
			},
		},
	})

	if context.LastClarification == nil {
		t.Fatalf("expected clarification, got %#v", context.LastClarification)
	}
	if got, want := len(context.RecentDialogueTurns), 1; got != want {
		t.Fatalf("expected %d dialogue turn, got %#v", want, context.RecentDialogueTurns)
	}
	turn := context.RecentDialogueTurns[0]
	if turn.UserPrompt != "查询飞虫公司的下级组织，只有名称" {
		t.Fatalf("unexpected user prompt=%#v", turn)
	}
	if turn.AssistantReply != "请先确认你要查哪个组织。" {
		t.Fatalf("unexpected assistant reply=%#v", turn)
	}
}

func TestQueryContextFromEventsCapsCandidatesAt100(t *testing.T) {
	raw := make([]any, 0, 120)
	for i := 0; i < 120; i++ {
		raw = append(raw, map[string]any{
			"domain":     "orgunit",
			"entity_key": string(rune('a' + (i % 26))),
			"name":       "candidate",
		})
	}

	context := QueryContextFromEvents([]CanonicalEvent{{
		Type: QueryCandidatesPresentedEventType,
		Payload: map[string]any{
			"candidates": raw,
		},
	}})

	if len(context.RecentCandidates) != 100 {
		t.Fatalf("expected 100 candidates, got %d", len(context.RecentCandidates))
	}
}

func TestDecodeQueryClarificationAcceptsStringSlice(t *testing.T) {
	clarification := DecodeQueryClarification(map[string]any{
		"intent":              "orgunit.list",
		"missing_params":      []string{"parent_org_code", " as_of "},
		"clarifying_question": "请补充参数。",
	})

	if clarification == nil {
		t.Fatal("expected clarification")
	}
	if !reflect.DeepEqual(clarification.MissingParams, []string{"parent_org_code", "as_of"}) {
		t.Fatalf("unexpected missing params=%#v", clarification.MissingParams)
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

func TestQueryCandidatePayloadUsesMinimalSchema(t *testing.T) {
	candidate := QueryCandidate{
		Domain:    " OrgUnit ",
		EntityKey: " 100000 ",
		Name:      " 飞虫与鲜花 ",
		AsOf:      " 2026-04-25 ",
		Status:    " active ",
	}

	payload := candidate.Payload()
	if payload["domain"] != "orgunit" || payload["entity_key"] != "100000" {
		t.Fatalf("unexpected payload=%#v", payload)
	}
	if payload["name"] != "飞虫与鲜花" || payload["as_of"] != "2026-04-25" || payload["status"] != "active" {
		t.Fatalf("unexpected payload=%#v", payload)
	}
}

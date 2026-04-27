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
				"domain":              "orgunit",
				"intent":              "orgunit.details",
				"entity_key":          "100000",
				"as_of":               "2026-04-24",
				"source_executor_key": "orgunit.details",
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
				"domain":              "orgunit",
				"intent":              "orgunit.list",
				"entity_key":          "200000",
				"as_of":               "2026-04-25",
				"source_executor_key": "orgunit.list",
				"parent_org_code":     "200000",
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

func TestQueryContextFromEventsAcceptsLegacySourceAPIKeyDuringReplay(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{{
		Type: QueryEntityConfirmedEventType,
		Payload: map[string]any{"entity": map[string]any{
			"domain":         "orgunit",
			"entity_key":     "100000",
			"as_of":          "2026-04-24",
			"source_api_key": "orgunit.details",
		}},
	}})

	if context.RecentConfirmedEntity == nil {
		t.Fatal("expected recent entity")
	}
	if got := context.RecentConfirmedEntity.SourceExecutorKey; got != "orgunit.details" {
		t.Fatalf("expected legacy source_api_key normalized, got %#v", context.RecentConfirmedEntity)
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
				"group_id":             "candgrp_test_1",
				"candidate_source":     "execution_error",
				"candidate_count":      2,
				"cannot_silent_select": true,
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
				"candidate_group_id":  "candgrp_test_1",
			},
		},
	})

	if len(context.RecentCandidates) != 2 {
		t.Fatalf("expected recent candidates, got %#v", context.RecentCandidates)
	}
	if got, want := len(context.RecentCandidateGroups), 1; got != want {
		t.Fatalf("expected %d candidate group, got %#v", want, context.RecentCandidateGroups)
	}
	group := context.RecentCandidateGroups[0]
	if group.GroupID != "candgrp_test_1" || group.CandidateSource != "execution_error" || group.CandidateCount != 2 || !group.CannotSilentSelect {
		t.Fatalf("unexpected candidate group=%#v", group)
	}
	if context.LastClarification == nil || context.LastClarification.ClarifyingQuestion == "" {
		t.Fatalf("expected clarification, got %#v", context.LastClarification)
	}
	if context.LastClarification.CandidateGroupID != "candgrp_test_1" {
		t.Fatalf("expected candidate group id, got %#v", context.LastClarification)
	}
	if context.LastClarification.SourceTurnID != "" {
		t.Fatalf("expected empty source turn id when turn id absent, got %#v", context.LastClarification)
	}
	if context.LastClarification.ErrorCode != "" || context.LastClarification.CandidateCount != 0 || context.LastClarification.CannotSilentSelect {
		t.Fatalf("expected empty optional clarification fields when absent, got %#v", context.LastClarification)
	}
	if len(context.RecentDialogueTurns) == 0 {
		t.Fatalf("expected dialogue turns, got %#v", context.RecentDialogueTurns)
	}
	if context.ClarificationResume == nil {
		t.Fatalf("expected open clarification resume, got %#v", context)
	}
	if context.ClarificationResume.RawUserReply != "" || context.ClarificationResume.ReplyCandidate {
		t.Fatalf("expected resume without raw reply before next user turn, got %#v", context.ClarificationResume)
	}
	if context.ClarificationResume.CandidateSource != "execution_error" || context.ClarificationResume.CandidateCount != 2 || !context.ClarificationResume.CannotSilentSelect {
		t.Fatalf("expected candidate facts in resume, got %#v", context.ClarificationResume)
	}
	if got := len(context.ClarificationResume.Candidates); got != 2 {
		t.Fatalf("expected resume candidates, got %d in %#v", got, context.ClarificationResume)
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

func TestQueryCandidateGroupPayloadUsesFrozenSchema(t *testing.T) {
	payload := cubeboxQueryCandidateGroupForPayloadTest().Payload()
	if payload["group_id"] != "candgrp_test_1" || payload["candidate_source"] != "execution_error" || payload["candidate_count"] != 2 {
		t.Fatalf("unexpected payload=%#v", payload)
	}
	if payload["cannot_silent_select"] != true {
		t.Fatalf("expected cannot_silent_select, got %#v", payload)
	}
	rawCandidates, ok := payload["candidates"].([]any)
	if !ok || len(rawCandidates) != 2 {
		t.Fatalf("unexpected candidates payload=%#v", payload)
	}
}

func cubeboxQueryCandidateGroupForPayloadTest() QueryCandidateGroup {
	return QueryCandidateGroup{
		GroupID:            "candgrp_test_1",
		CandidateSource:    "execution_error",
		CandidateCount:     2,
		CannotSilentSelect: true,
		Candidates: []QueryCandidate{
			{Domain: "orgunit", EntityKey: "200000", Name: "飞虫公司", AsOf: "2026-04-25"},
			{Domain: "orgunit", EntityKey: "300000", Name: "鲜花公司", AsOf: "2026-04-25"},
		},
	}
}

func TestDecodeQueryClarificationAcceptsStringSlice(t *testing.T) {
	clarification := DecodeQueryClarification(map[string]any{
		"source_turn_id":       "turn_prev",
		"intent":               "orgunit.list",
		"missing_params":       []string{"parent_org_code", " as_of "},
		"clarifying_question":  "请补充参数。",
		"error_code":           "org_unit_search_ambiguous",
		"candidate_group_id":   "candgrp_test_1",
		"candidate_source":     "execution_error",
		"candidate_count":      float64(2),
		"cannot_silent_select": true,
		"known_params": map[string]any{
			"as_of_text": "2025年1月",
		},
	})

	if clarification == nil {
		t.Fatal("expected clarification")
	}
	if !reflect.DeepEqual(clarification.MissingParams, []string{"parent_org_code", "as_of"}) {
		t.Fatalf("unexpected missing params=%#v", clarification.MissingParams)
	}
	if clarification.SourceTurnID != "turn_prev" {
		t.Fatalf("unexpected source turn id=%#v", clarification)
	}
	if got, _ := clarification.KnownParams["as_of_text"].(string); got != "2025年1月" {
		t.Fatalf("unexpected known params=%#v", clarification.KnownParams)
	}
	if clarification.ErrorCode != "org_unit_search_ambiguous" || clarification.CandidateGroupID != "candgrp_test_1" || clarification.CandidateSource != "execution_error" || clarification.CandidateCount != 2 || !clarification.CannotSilentSelect {
		t.Fatalf("unexpected clarification=%#v", clarification)
	}
}

func TestQueryContextFromEventsBuildsOpenClarificationResume(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: "turn.user_message.accepted",
			Payload: map[string]any{
				"text": "查出顶级点的全部各级下级组织，时间节点是2025年1月",
			},
		},
		{
			Type: "turn.agent_message.delta",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
				"delta":      "请提供完整查询日期，例如 2025-01-01。",
			},
		},
		{
			Type: "turn.agent_message.completed",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
			},
		},
		{
			Type:   QueryClarificationRequestedEventType,
			TurnID: stringPtr("turn_prev"),
			Payload: map[string]any{
				"intent":              "orgunit.list",
				"missing_params":      []string{"as_of"},
				"clarifying_question": "请提供完整查询日期，例如 2025-01-01。",
			},
		},
	})

	if context.LastClarification == nil {
		t.Fatalf("expected last clarification, got %#v", context)
	}
	if context.ClarificationResume == nil {
		t.Fatalf("expected clarification resume, got %#v", context)
	}
	if context.ClarificationResume.ReplyCandidate {
		t.Fatalf("expected reply candidate false before raw reply injection, got %#v", context.ClarificationResume)
	}
	if context.ClarificationResume.SourceTurnID != "turn_prev" {
		t.Fatalf("unexpected source turn id=%#v", context.ClarificationResume)
	}
	if context.ClarificationResume.RawUserReply != "" {
		t.Fatalf("expected empty raw reply before injection, got %#v", context.ClarificationResume)
	}
}

func TestBuildQueryClarificationResumeCopiesRawReply(t *testing.T) {
	context := QueryContext{
		LastClarification: &QueryClarification{
			SourceTurnID:       "turn_prev",
			Intent:             "orgunit.list",
			MissingParams:      []string{"as_of"},
			ClarifyingQuestion: "请提供完整查询日期，例如 2025-01-01。",
			KnownParams: map[string]any{
				"as_of_text": "2025年1月",
			},
		},
		RecentDialogueTurns: []QueryDialogueTurn{
			{UserPrompt: "查出顶级点的全部各级下级组织，时间节点是2025年1月", AssistantReply: "请提供完整查询日期，例如 2025-01-01。"},
		},
	}
	resume := BuildQueryClarificationResume(context, "1日")
	if resume == nil {
		t.Fatal("expected clarification resume")
	}
	if !resume.ReplyCandidate || resume.RawUserReply != "1日" {
		t.Fatalf("unexpected resume=%#v", resume)
	}
	if got, _ := resume.KnownParams["as_of_text"].(string); got != "2025年1月" {
		t.Fatalf("unexpected known params=%#v", resume.KnownParams)
	}
}

func TestBuildQueryClarificationResumeIncludesMatchingCandidateGroup(t *testing.T) {
	context := QueryContext{
		LastClarification: &QueryClarification{
			SourceTurnID:       "turn_prev",
			ClarifyingQuestion: "请确认要继续查询哪一个。",
			CandidateGroupID:   "candgrp_finance",
			CandidateCount:     3,
			CannotSilentSelect: true,
		},
		RecentCandidateGroups: []QueryCandidateGroup{
			{
				GroupID:         "candgrp_other",
				CandidateSource: "results",
				CandidateCount:  1,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "100001", Name: "销售部"},
				},
			},
			{
				GroupID:         "candgrp_finance",
				CandidateSource: "execution_error",
				CandidateCount:  3,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "200001", Name: "财务部", AsOf: "2026-01-07"},
					{Domain: "orgunit", EntityKey: "200002", Name: "财务一组", AsOf: "2026-01-07"},
					{Domain: "orgunit", EntityKey: "200004", Name: "财务四组", AsOf: "2026-01-07"},
				},
			},
		},
	}

	resume := BuildQueryClarificationResume(context, "以上全部")
	if resume == nil {
		t.Fatal("expected clarification resume")
	}
	if !resume.ReplyCandidate || resume.RawUserReply != "以上全部" {
		t.Fatalf("unexpected reply facts=%#v", resume)
	}
	if resume.CandidateGroupID != "candgrp_finance" || resume.CandidateSource != "execution_error" || resume.CandidateCount != 3 || !resume.CannotSilentSelect {
		t.Fatalf("unexpected candidate facts=%#v", resume)
	}
	if got := len(resume.Candidates); got != 3 {
		t.Fatalf("expected 3 candidates, got %d in %#v", got, resume.Candidates)
	}
	if resume.Candidates[0].EntityKey != "200001" || resume.Candidates[2].Name != "财务四组" {
		t.Fatalf("unexpected candidates=%#v", resume.Candidates)
	}
}

func TestBuildQueryEvidenceWindowProjectsNeutralObservations(t *testing.T) {
	context := QueryContext{
		RecentConfirmedEntities: []QueryEntity{
			{
				Domain:            "orgunit",
				Intent:            "orgunit.details",
				EntityKey:         "100000",
				AsOf:              "2026-04-25",
				SourceExecutorKey: "orgunit.details",
				TargetOrgCode:     "100000",
				ParentOrgCode:     "ROOT",
			},
		},
		RecentCandidateGroups: []QueryCandidateGroup{
			{
				GroupID:            "candgrp_finance",
				CandidateSource:    "execution_error",
				CandidateCount:     3,
				CannotSilentSelect: true,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "200001", Name: "财务部", AsOf: "2026-01-07"},
					{Domain: "orgunit", EntityKey: "200002", Name: "财务一组", AsOf: "2026-01-07"},
					{Domain: "orgunit", EntityKey: "200004", Name: "财务四组", AsOf: "2026-01-07"},
				},
			},
		},
		LastClarification: &QueryClarification{
			SourceTurnID:       "turn_prev",
			ClarifyingQuestion: "找到了多个候选项，请确认要继续查询哪一个。",
			CandidateGroupID:   "candgrp_finance",
			CandidateCount:     3,
			CannotSilentSelect: true,
		},
	}
	context.ClarificationResume = BuildQueryClarificationResume(context, "以上全部")

	window := BuildQueryEvidenceWindow(context, "审计信息", QueryEvidenceWindowBudget{
		MaxEntityObservations: 5,
		MaxOptionGroups:       5,
		MaxOptionsPerGroup:    2,
		MaxDialogueTurns:      5,
	})

	if window.CurrentUserInput != "审计信息" {
		t.Fatalf("unexpected current user input=%#v", window)
	}
	if got, want := len(window.Observations), 2; got != want {
		t.Fatalf("expected %d observations, got %#v", want, window.Observations)
	}
	entity := window.Observations[0]
	if entity.Kind != "entity_fact" {
		t.Fatalf("expected entity fact observation, got %#v", entity)
	}
	item, ok := entity.ResultSummary["item"].(map[string]any)
	if !ok || item["entity_key"] != "100000" || item["as_of"] != "2026-04-25" {
		t.Fatalf("unexpected entity item=%#v", entity.ResultSummary)
	}
	for _, forbidden := range []string{"intent", "source_executor_key", "target_org_code", "parent_org_code"} {
		if _, exists := item[forbidden]; exists {
			t.Fatalf("evidence entity item leaked %q in %#v", forbidden, item)
		}
	}
	options := window.Observations[1]
	if options.Kind != "presented_options" || options.ResultSummary["group_id"] != "candgrp_finance" {
		t.Fatalf("unexpected presented options=%#v", options)
	}
	items, ok := options.ResultSummary["items"].([]map[string]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected options truncated to 2, got %#v", options.ResultSummary["items"])
	}
	if window.OpenClarification == nil || !window.OpenClarification.ReplyCandidate || window.OpenClarification.RawUserReply != "以上全部" {
		t.Fatalf("expected open clarification resume, got %#v", window.OpenClarification)
	}
	if got := len(window.OpenClarification.Options); got != 2 {
		t.Fatalf("expected open clarification options truncated to 2, got %d", got)
	}
}

func TestBuildQueryEvidenceWindowKeepsClarificationOptionsSeparateFromResultLists(t *testing.T) {
	context := QueryContext{
		RecentCandidateGroups: []QueryCandidateGroup{
			{
				GroupID:         "resultgrp_finance",
				CandidateSource: "results",
				CandidateCount:  2,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "200001", Name: "财务部", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "200002", Name: "财务一组", AsOf: "2026-04-27"},
				},
			},
			{
				GroupID:            "candgrp_ambiguous",
				CandidateSource:    "execution_error",
				CandidateCount:     2,
				CannotSilentSelect: true,
				Candidates: []QueryCandidate{
					{Domain: "orgunit", EntityKey: "300001", Name: "华东销售中心", AsOf: "2026-04-27"},
					{Domain: "orgunit", EntityKey: "300002", Name: "华东运营中心", AsOf: "2026-04-27"},
				},
			},
		},
	}

	window := BuildQueryEvidenceWindow(context, "他们的路径", QueryEvidenceWindowBudget{
		MaxEntityObservations: 5,
		MaxOptionGroups:       5,
		MaxOptionsPerGroup:    5,
		MaxDialogueTurns:      5,
	})

	if got, want := len(window.Observations), 2; got != want {
		t.Fatalf("expected %d observations, got %#v", want, window.Observations)
	}
	if window.Observations[0].Kind != "result_list" {
		t.Fatalf("expected first observation result_list, got %#v", window.Observations[0])
	}
	if window.Observations[1].Kind != "presented_options" {
		t.Fatalf("expected second observation presented_options, got %#v", window.Observations[1])
	}
}

func TestQueryContextFromEventsClearsOpenClarificationResumeAfterNextUserMessage(t *testing.T) {
	context := QueryContextFromEvents([]CanonicalEvent{
		{
			Type: "turn.user_message.accepted",
			Payload: map[string]any{
				"text": "查出顶级点的全部各级下级组织，时间节点是2025年1月",
			},
		},
		{
			Type: "turn.agent_message.delta",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
				"delta":      "请提供完整查询日期，例如 2025-01-01。",
			},
		},
		{
			Type: "turn.agent_message.completed",
			Payload: map[string]any{
				"message_id": "msg_agent_1",
			},
		},
		{
			Type:   QueryClarificationRequestedEventType,
			TurnID: stringPtr("turn_prev"),
			Payload: map[string]any{
				"intent":              "orgunit.list",
				"missing_params":      []string{"as_of"},
				"clarifying_question": "请提供完整查询日期，例如 2025-01-01。",
			},
		},
		{
			Type: "turn.user_message.accepted",
			Payload: map[string]any{
				"text": "1日",
			},
		},
	})

	if context.LastClarification == nil {
		t.Fatalf("expected last clarification retained for audit, got %#v", context)
	}
	if context.ClarificationResume != nil {
		t.Fatalf("expected open clarification resume cleared after next user message, got %#v", context.ClarificationResume)
	}
}

func TestQueryEntityPayloadUsesMinimalSchema(t *testing.T) {
	entity := QueryEntity{
		Domain:            " OrgUnit ",
		Intent:            " orgunit.details ",
		EntityKey:         " 100000 ",
		AsOf:              " 2026-04-25 ",
		SourceExecutorKey: " orgunit.details ",
		TargetOrgCode:     " ",
		ParentOrgCode:     " ROOT ",
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

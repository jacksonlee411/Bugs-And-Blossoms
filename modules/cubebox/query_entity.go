package cubebox

import (
	"strings"
)

const QueryEntityConfirmedEventType = "turn.query_entity.confirmed"
const QueryCandidatesPresentedEventType = "turn.query_candidates.presented"
const QueryClarificationRequestedEventType = "turn.query_clarification.requested"
const QueryContextResolvedEventType = "turn.query_context.resolved"

const queryContextMaxDialogueTurns = 5
const queryContextMaxEntities = 5
const queryContextMaxCandidateGroups = 5
const queryContextMaxCandidateItems = 100

type QueryEntity struct {
	Domain        string `json:"domain"`
	Intent        string `json:"intent,omitempty"`
	EntityKey     string `json:"entity_key,omitempty"`
	AsOf          string `json:"as_of,omitempty"`
	SourceAPIKey  string `json:"source_api_key,omitempty"`
	TargetOrgCode string `json:"target_org_code,omitempty"`
	ParentOrgCode string `json:"parent_org_code,omitempty"`
}

type QueryDialogueTurn struct {
	UserPrompt       string `json:"user_prompt,omitempty"`
	AssistantReply   string `json:"assistant_reply,omitempty"`
	ClarificationFor string `json:"clarification_for,omitempty"`
}

type QueryCandidate struct {
	Domain    string `json:"domain"`
	EntityKey string `json:"entity_key"`
	Name      string `json:"name,omitempty"`
	AsOf      string `json:"as_of,omitempty"`
	Status    string `json:"status,omitempty"`
}

type QueryCandidateGroup struct {
	GroupID            string           `json:"group_id,omitempty"`
	CandidateSource    string           `json:"candidate_source,omitempty"`
	CandidateCount     int              `json:"candidate_count,omitempty"`
	CannotSilentSelect bool             `json:"cannot_silent_select,omitempty"`
	Candidates         []QueryCandidate `json:"candidates,omitempty"`
}

type QueryClarification struct {
	Intent             string   `json:"intent,omitempty"`
	MissingParams      []string `json:"missing_params,omitempty"`
	ClarifyingQuestion string   `json:"clarifying_question,omitempty"`
	ErrorCode          string   `json:"error_code,omitempty"`
	CandidateGroupID   string   `json:"candidate_group_id,omitempty"`
	CandidateSource    string   `json:"candidate_source,omitempty"`
	CandidateCount     int      `json:"candidate_count,omitempty"`
	CannotSilentSelect bool     `json:"cannot_silent_select,omitempty"`
}

type QueryContext struct {
	RecentConfirmedEntity   *QueryEntity
	RecentConfirmedEntities []QueryEntity
	RecentDialogueTurns     []QueryDialogueTurn
	LastClarification       *QueryClarification
	RecentCandidateGroups   []QueryCandidateGroup
	RecentCandidates        []QueryCandidate
}

func QueryContextFromEvents(events []CanonicalEvent) QueryContext {
	context := QueryContext{}
	confirmed := make([]QueryEntity, 0, queryContextMaxEntities)
	dialogue := make([]QueryDialogueTurn, 0, queryContextMaxDialogueTurns)
	candidateGroups := make([]QueryCandidateGroup, 0, queryContextMaxCandidateGroups)
	assistantReplies := map[string]string{}
	currentDialogue := QueryDialogueTurn{}
	hasCurrentDialogue := false

	appendDialogue := func(turn QueryDialogueTurn) {
		trimmed := QueryDialogueTurn{
			UserPrompt:       strings.TrimSpace(turn.UserPrompt),
			AssistantReply:   strings.TrimSpace(turn.AssistantReply),
			ClarificationFor: strings.TrimSpace(turn.ClarificationFor),
		}
		if trimmed.UserPrompt == "" && trimmed.AssistantReply == "" && trimmed.ClarificationFor == "" {
			return
		}
		dialogue = append(dialogue, trimmed)
		if len(dialogue) > queryContextMaxDialogueTurns {
			dialogue = dialogue[len(dialogue)-queryContextMaxDialogueTurns:]
		}
	}
	flushCurrentDialogue := func() {
		if !hasCurrentDialogue {
			return
		}
		appendDialogue(currentDialogue)
		currentDialogue = QueryDialogueTurn{}
		hasCurrentDialogue = false
	}

	for _, event := range events {
		switch strings.TrimSpace(event.Type) {
		case QueryEntityConfirmedEventType:
			if entity := DecodeQueryEntity(event.Payload); entity != nil {
				entityCopy := *entity
				context.RecentConfirmedEntity = &entityCopy
				confirmed = append(confirmed, *entity)
				if len(confirmed) > queryContextMaxEntities {
					confirmed = confirmed[len(confirmed)-queryContextMaxEntities:]
				}
			}
		case QueryCandidatesPresentedEventType:
			group := DecodeQueryCandidateGroup(event.Payload)
			if group == nil || len(group.Candidates) == 0 {
				continue
			}
			candidateGroups = append(candidateGroups, *group)
			if len(candidateGroups) > queryContextMaxCandidateGroups {
				candidateGroups = candidateGroups[len(candidateGroups)-queryContextMaxCandidateGroups:]
			}
		case QueryClarificationRequestedEventType:
			if clarification := DecodeQueryClarification(event.Payload); clarification != nil {
				context.LastClarification = clarification
			}
		case "turn.agent_message.delta":
			reply := strings.TrimSpace(stringValue(event.Payload["delta"]))
			if reply == "" {
				continue
			}
			messageID := strings.TrimSpace(stringValue(event.Payload["message_id"]))
			if messageID == "" {
				messageID = "__legacy_assistant_message__"
			}
			assistantReplies[messageID] += reply
		case "turn.agent_message.completed":
			messageID := strings.TrimSpace(stringValue(event.Payload["message_id"]))
			if messageID == "" {
				messageID = "__legacy_assistant_message__"
			}
			reply := strings.TrimSpace(assistantReplies[messageID])
			delete(assistantReplies, messageID)
			if reply == "" {
				continue
			}
			if !hasCurrentDialogue {
				currentDialogue = QueryDialogueTurn{}
				hasCurrentDialogue = true
			}
			if currentDialogue.AssistantReply != "" {
				flushCurrentDialogue()
				currentDialogue = QueryDialogueTurn{}
				hasCurrentDialogue = true
			}
			currentDialogue.AssistantReply = reply
			flushCurrentDialogue()
		case "turn.user_message.accepted":
			prompt := strings.TrimSpace(stringValue(event.Payload["text"]))
			if prompt == "" {
				continue
			}
			flushCurrentDialogue()
			currentDialogue = QueryDialogueTurn{UserPrompt: prompt}
			hasCurrentDialogue = true
		}
	}
	flushCurrentDialogue()

	context.RecentConfirmedEntities = confirmed
	context.RecentDialogueTurns = dialogue
	context.RecentCandidateGroups = candidateGroups
	if len(candidateGroups) > 0 {
		context.RecentCandidates = append([]QueryCandidate(nil), candidateGroups[len(candidateGroups)-1].Candidates...)
	}
	return context
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

func NormalizeQueryCandidate(candidate QueryCandidate) *QueryCandidate {
	candidate.Domain = strings.ToLower(strings.TrimSpace(candidate.Domain))
	candidate.EntityKey = strings.TrimSpace(candidate.EntityKey)
	candidate.Name = strings.TrimSpace(candidate.Name)
	candidate.AsOf = strings.TrimSpace(candidate.AsOf)
	candidate.Status = strings.TrimSpace(candidate.Status)
	if candidate.Domain == "" || candidate.EntityKey == "" {
		return nil
	}
	return &candidate
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

func (c QueryCandidate) Payload() map[string]any {
	normalized := NormalizeQueryCandidate(c)
	if normalized == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"domain":     normalized.Domain,
		"entity_key": normalized.EntityKey,
	}
	if normalized.Name != "" {
		payload["name"] = normalized.Name
	}
	if normalized.AsOf != "" {
		payload["as_of"] = normalized.AsOf
	}
	if normalized.Status != "" {
		payload["status"] = normalized.Status
	}
	return payload
}

func (g QueryCandidateGroup) Payload() map[string]any {
	candidates := make([]any, 0, minInt(len(g.Candidates), queryContextMaxCandidateItems))
	for _, item := range g.Candidates {
		payload := item.Payload()
		if len(payload) == 0 {
			continue
		}
		candidates = append(candidates, payload)
		if len(candidates) >= queryContextMaxCandidateItems {
			break
		}
	}
	if len(candidates) == 0 {
		return map[string]any{}
	}
	payload := map[string]any{
		"candidates": candidates,
	}
	if groupID := strings.TrimSpace(g.GroupID); groupID != "" {
		payload["group_id"] = groupID
	}
	if candidateSource := strings.TrimSpace(g.CandidateSource); candidateSource != "" {
		payload["candidate_source"] = candidateSource
	}
	candidateCount := g.CandidateCount
	if candidateCount <= 0 {
		candidateCount = len(candidates)
	}
	if candidateCount > 0 {
		payload["candidate_count"] = candidateCount
	}
	if g.CannotSilentSelect {
		payload["cannot_silent_select"] = true
	}
	return payload
}

func DecodeQueryCandidates(payload map[string]any) []QueryCandidate {
	if len(payload) == 0 {
		return nil
	}
	rawItems, ok := payload["candidates"].([]any)
	if !ok {
		return nil
	}
	items := make([]QueryCandidate, 0, minInt(len(rawItems), queryContextMaxCandidateItems))
	for _, rawItem := range rawItems {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		candidate := NormalizeQueryCandidate(QueryCandidate{
			Domain:    stringValue(itemMap["domain"]),
			EntityKey: stringValue(itemMap["entity_key"]),
			Name:      stringValue(itemMap["name"]),
			AsOf:      stringValue(itemMap["as_of"]),
			Status:    stringValue(itemMap["status"]),
		})
		if candidate == nil {
			continue
		}
		items = append(items, *candidate)
		if len(items) >= queryContextMaxCandidateItems {
			break
		}
	}
	return items
}

func DecodeQueryCandidateGroup(payload map[string]any) *QueryCandidateGroup {
	candidates := DecodeQueryCandidates(payload)
	if len(candidates) == 0 {
		return nil
	}
	group := &QueryCandidateGroup{
		GroupID:            strings.TrimSpace(stringValue(payload["group_id"])),
		CandidateSource:    strings.TrimSpace(stringValue(payload["candidate_source"])),
		CandidateCount:     decodeQueryInt(payload["candidate_count"]),
		CannotSilentSelect: decodeQueryBool(payload["cannot_silent_select"]),
		Candidates:         candidates,
	}
	if group.CandidateCount <= 0 {
		group.CandidateCount = len(candidates)
	}
	return group
}

func DecodeQueryClarification(payload map[string]any) *QueryClarification {
	if len(payload) == 0 {
		return nil
	}
	out := &QueryClarification{
		Intent:             strings.TrimSpace(stringValue(payload["intent"])),
		ClarifyingQuestion: strings.TrimSpace(stringValue(payload["clarifying_question"])),
		ErrorCode:          strings.TrimSpace(stringValue(payload["error_code"])),
		CandidateGroupID:   strings.TrimSpace(stringValue(payload["candidate_group_id"])),
		CandidateSource:    strings.TrimSpace(stringValue(payload["candidate_source"])),
	}
	out.MissingParams = decodeQueryStringList(payload["missing_params"])
	out.CandidateCount = decodeQueryInt(payload["candidate_count"])
	out.CannotSilentSelect = decodeQueryBool(payload["cannot_silent_select"])
	if out.Intent == "" &&
		out.ClarifyingQuestion == "" &&
		out.ErrorCode == "" &&
		out.CandidateGroupID == "" &&
		out.CandidateSource == "" &&
		out.CandidateCount == 0 &&
		!out.CannotSilentSelect &&
		len(out.MissingParams) == 0 {
		return nil
	}
	return out
}

func decodeQueryStringList(raw any) []string {
	switch items := raw.(type) {
	case []string:
		out := make([]string, 0, len(items))
		for _, item := range items {
			value := strings.TrimSpace(item)
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		return out
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			value := strings.TrimSpace(stringValue(item))
			if value == "" {
				continue
			}
			out = append(out, value)
		}
		return out
	default:
		return nil
	}
}

func decodeQueryInt(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func decodeQueryBool(raw any) bool {
	value, ok := raw.(bool)
	return ok && value
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

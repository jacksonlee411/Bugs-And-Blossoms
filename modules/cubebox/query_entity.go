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

type QueryEvidenceWindowBudget struct {
	MaxEntityObservations int
	MaxOptionGroups       int
	MaxOptionsPerGroup    int
	MaxDialogueTurns      int
}

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
	SourceTurnID       string         `json:"source_turn_id,omitempty"`
	Intent             string         `json:"intent,omitempty"`
	MissingParams      []string       `json:"missing_params,omitempty"`
	ClarifyingQuestion string         `json:"clarifying_question,omitempty"`
	ErrorCode          string         `json:"error_code,omitempty"`
	CandidateGroupID   string         `json:"candidate_group_id,omitempty"`
	CandidateSource    string         `json:"candidate_source,omitempty"`
	CandidateCount     int            `json:"candidate_count,omitempty"`
	CannotSilentSelect bool           `json:"cannot_silent_select,omitempty"`
	KnownParams        map[string]any `json:"known_params,omitempty"`
}

type QueryClarificationResume struct {
	ReplyCandidate      bool                `json:"reply_candidate,omitempty"`
	SourceTurnID        string              `json:"source_turn_id,omitempty"`
	Intent              string              `json:"intent,omitempty"`
	MissingParams       []string            `json:"missing_params,omitempty"`
	ClarifyingQuestion  string              `json:"clarifying_question,omitempty"`
	KnownParams         map[string]any      `json:"known_params,omitempty"`
	CandidateGroupID    string              `json:"candidate_group_id,omitempty"`
	CandidateSource     string              `json:"candidate_source,omitempty"`
	CandidateCount      int                 `json:"candidate_count,omitempty"`
	CannotSilentSelect  bool                `json:"cannot_silent_select,omitempty"`
	Candidates          []QueryCandidate    `json:"candidates,omitempty"`
	RecentDialogueTurns []QueryDialogueTurn `json:"recent_dialogue_turns,omitempty"`
	RawUserReply        string              `json:"raw_user_reply,omitempty"`
}

type QueryContext struct {
	RecentConfirmedEntity   *QueryEntity
	RecentConfirmedEntities []QueryEntity
	RecentDialogueTurns     []QueryDialogueTurn
	LastClarification       *QueryClarification
	ClarificationResume     *QueryClarificationResume
	RecentCandidateGroups   []QueryCandidateGroup
	RecentCandidates        []QueryCandidate
}

type QueryEvidenceWindow struct {
	CurrentUserInput  string                      `json:"current_user_input,omitempty"`
	RecentTurns       []QueryDialogueTurn         `json:"recent_turns,omitempty"`
	Observations      []QueryEvidenceObservation  `json:"observations,omitempty"`
	OpenClarification *QueryEvidenceClarification `json:"open_clarification,omitempty"`
}

type QueryEvidenceObservation struct {
	Source        string         `json:"source,omitempty"`
	Kind          string         `json:"kind,omitempty"`
	ResultSummary map[string]any `json:"result_summary,omitempty"`
}

type QueryEvidenceClarification struct {
	ReplyCandidate             bool             `json:"reply_candidate,omitempty"`
	SourceTurnID               string           `json:"source_turn_id,omitempty"`
	Intent                     string           `json:"intent,omitempty"`
	MissingParams              []string         `json:"missing_params,omitempty"`
	ClarifyingQuestion         string           `json:"clarifying_question,omitempty"`
	KnownParams                map[string]any   `json:"known_params,omitempty"`
	OptionGroupID              string           `json:"option_group_id,omitempty"`
	OptionSource               string           `json:"option_source,omitempty"`
	OptionCount                int              `json:"option_count,omitempty"`
	RequiresExplicitUserChoice bool             `json:"requires_explicit_user_choice,omitempty"`
	Options                    []QueryCandidate `json:"options,omitempty"`
	RawUserReply               string           `json:"raw_user_reply,omitempty"`
}

func BuildQueryEvidenceWindow(context QueryContext, currentUserInput string, budget QueryEvidenceWindowBudget) QueryEvidenceWindow {
	window := QueryEvidenceWindow{
		CurrentUserInput: strings.TrimSpace(currentUserInput),
		RecentTurns:      projectQueryEvidenceDialogueTurns(context.RecentDialogueTurns, budget.MaxDialogueTurns),
	}
	for _, entity := range projectQueryEvidenceEntities(context.RecentConfirmedEntities, budget.MaxEntityObservations) {
		window.Observations = append(window.Observations, QueryEvidenceObservation{
			Source: "query_event",
			Kind:   "entity_fact",
			ResultSummary: map[string]any{
				"item": queryEvidenceEntityItem(entity),
			},
		})
	}
	for _, group := range projectQueryEvidenceOptionGroups(context.RecentCandidateGroups, budget.MaxOptionGroups, budget.MaxOptionsPerGroup) {
		summary := map[string]any{
			"items": queryEvidenceOptionItems(group.Candidates),
		}
		if groupID := strings.TrimSpace(group.GroupID); groupID != "" {
			summary["group_id"] = groupID
		}
		if source := strings.TrimSpace(group.CandidateSource); source != "" {
			summary["option_source"] = source
		}
		count := group.CandidateCount
		if count <= 0 {
			count = len(group.Candidates)
		}
		if count > 0 {
			summary["item_count"] = count
		}
		if group.CannotSilentSelect {
			summary["requires_explicit_user_choice"] = true
		}
		window.Observations = append(window.Observations, QueryEvidenceObservation{
			Source:        "query_event",
			Kind:          "presented_options",
			ResultSummary: summary,
		})
	}
	window.OpenClarification = projectQueryEvidenceClarification(context.ClarificationResume, budget.MaxOptionsPerGroup)
	return window
}

func QueryContextFromEvents(events []CanonicalEvent) QueryContext {
	context := QueryContext{}
	confirmed := make([]QueryEntity, 0, queryContextMaxEntities)
	dialogue := make([]QueryDialogueTurn, 0, queryContextMaxDialogueTurns)
	candidateGroups := make([]QueryCandidateGroup, 0, queryContextMaxCandidateGroups)
	assistantReplies := map[string]string{}
	currentDialogue := QueryDialogueTurn{}
	hasCurrentDialogue := false
	var lastClarificationEvent *CanonicalEvent
	clarificationStillOpen := false

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
				if clarification.SourceTurnID == "" && event.TurnID != nil {
					clarification.SourceTurnID = strings.TrimSpace(*event.TurnID)
				}
				context.LastClarification = clarification
				eventCopy := event
				lastClarificationEvent = &eventCopy
				clarificationStillOpen = true
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
			clarificationStillOpen = false
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
	if clarificationStillOpen && context.LastClarification != nil && lastClarificationEvent != nil {
		context.ClarificationResume = BuildQueryClarificationResume(context, "")
	}
	return context
}

func BuildQueryClarificationResume(context QueryContext, rawUserReply string) *QueryClarificationResume {
	clarification := copyQueryClarificationLocal(context.LastClarification)
	if clarification == nil {
		return nil
	}
	reply := strings.TrimSpace(rawUserReply)
	candidateGroup := matchingClarificationCandidateGroup(context.RecentCandidateGroups, clarification.CandidateGroupID)
	candidateSource := strings.TrimSpace(clarification.CandidateSource)
	if candidateSource == "" {
		candidateSource = strings.TrimSpace(candidateGroup.CandidateSource)
	}
	candidateCount := clarification.CandidateCount
	if candidateCount <= 0 {
		candidateCount = candidateGroup.CandidateCount
	}
	candidates := copyQueryCandidates(candidateGroup.Candidates)
	if candidateCount <= 0 {
		candidateCount = len(candidates)
	}
	resume := &QueryClarificationResume{
		ReplyCandidate:      reply != "",
		SourceTurnID:        strings.TrimSpace(clarification.SourceTurnID),
		Intent:              strings.TrimSpace(clarification.Intent),
		MissingParams:       append([]string(nil), clarification.MissingParams...),
		ClarifyingQuestion:  strings.TrimSpace(clarification.ClarifyingQuestion),
		KnownParams:         copyQueryKnownParams(clarification.KnownParams),
		CandidateGroupID:    strings.TrimSpace(clarification.CandidateGroupID),
		CandidateSource:     candidateSource,
		CandidateCount:      candidateCount,
		CannotSilentSelect:  clarification.CannotSilentSelect || candidateGroup.CannotSilentSelect,
		Candidates:          candidates,
		RecentDialogueTurns: append([]QueryDialogueTurn(nil), context.RecentDialogueTurns...),
		RawUserReply:        reply,
	}
	if resume.SourceTurnID == "" &&
		resume.Intent == "" &&
		len(resume.MissingParams) == 0 &&
		resume.ClarifyingQuestion == "" &&
		len(resume.KnownParams) == 0 &&
		resume.CandidateGroupID == "" &&
		resume.CandidateSource == "" &&
		resume.CandidateCount == 0 &&
		!resume.CannotSilentSelect &&
		len(resume.Candidates) == 0 &&
		len(resume.RecentDialogueTurns) == 0 &&
		resume.RawUserReply == "" {
		return nil
	}
	if len(resume.KnownParams) == 0 {
		resume.KnownParams = nil
	}
	return resume
}

func matchingClarificationCandidateGroup(groups []QueryCandidateGroup, groupID string) QueryCandidateGroup {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return QueryCandidateGroup{}
	}
	for i := len(groups) - 1; i >= 0; i-- {
		if strings.TrimSpace(groups[i].GroupID) == groupID {
			group := groups[i]
			group.Candidates = copyQueryCandidates(groups[i].Candidates)
			return group
		}
	}
	return QueryCandidateGroup{}
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

func projectQueryEvidenceDialogueTurns(turns []QueryDialogueTurn, maxTurns int) []QueryDialogueTurn {
	if len(turns) == 0 {
		return nil
	}
	selected := append([]QueryDialogueTurn(nil), turns...)
	if maxTurns > 0 && len(selected) > maxTurns {
		selected = selected[len(selected)-maxTurns:]
	}
	out := make([]QueryDialogueTurn, 0, len(selected))
	for _, turn := range selected {
		item := QueryDialogueTurn{
			UserPrompt:       strings.TrimSpace(turn.UserPrompt),
			AssistantReply:   strings.TrimSpace(turn.AssistantReply),
			ClarificationFor: strings.TrimSpace(turn.ClarificationFor),
		}
		if item.UserPrompt == "" && item.AssistantReply == "" && item.ClarificationFor == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func projectQueryEvidenceEntities(entities []QueryEntity, maxEntities int) []QueryEntity {
	if len(entities) == 0 {
		return nil
	}
	selected := append([]QueryEntity(nil), entities...)
	if maxEntities > 0 && len(selected) > maxEntities {
		selected = selected[len(selected)-maxEntities:]
	}
	out := make([]QueryEntity, 0, len(selected))
	for _, entity := range selected {
		normalized := NormalizeQueryEntity(entity)
		if normalized == nil {
			continue
		}
		out = append(out, *normalized)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func queryEvidenceEntityItem(entity QueryEntity) map[string]any {
	item := map[string]any{
		"domain":     entity.Domain,
		"entity_key": entity.EntityKey,
	}
	if entity.AsOf != "" {
		item["as_of"] = entity.AsOf
	}
	return item
}

func projectQueryEvidenceOptionGroups(groups []QueryCandidateGroup, maxGroups int, maxOptions int) []QueryCandidateGroup {
	if len(groups) == 0 {
		return nil
	}
	selected := append([]QueryCandidateGroup(nil), groups...)
	if maxGroups > 0 && len(selected) > maxGroups {
		selected = selected[len(selected)-maxGroups:]
	}
	out := make([]QueryCandidateGroup, 0, len(selected))
	for _, group := range selected {
		candidates := copyQueryCandidates(group.Candidates)
		if maxOptions > 0 && len(candidates) > maxOptions {
			candidates = candidates[:maxOptions]
		}
		if len(candidates) == 0 {
			continue
		}
		item := QueryCandidateGroup{
			GroupID:            strings.TrimSpace(group.GroupID),
			CandidateSource:    strings.TrimSpace(group.CandidateSource),
			CandidateCount:     group.CandidateCount,
			CannotSilentSelect: group.CannotSilentSelect,
			Candidates:         candidates,
		}
		if item.CandidateCount <= 0 {
			item.CandidateCount = len(candidates)
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func queryEvidenceOptionItems(candidates []QueryCandidate) []map[string]any {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		payload := candidate.Payload()
		if len(payload) == 0 {
			continue
		}
		out = append(out, payload)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func projectQueryEvidenceClarification(in *QueryClarificationResume, maxOptions int) *QueryEvidenceClarification {
	if in == nil {
		return nil
	}
	options := copyQueryCandidates(in.Candidates)
	if maxOptions > 0 && len(options) > maxOptions {
		options = options[:maxOptions]
	}
	out := &QueryEvidenceClarification{
		ReplyCandidate:             in.ReplyCandidate,
		SourceTurnID:               strings.TrimSpace(in.SourceTurnID),
		Intent:                     strings.TrimSpace(in.Intent),
		MissingParams:              append([]string(nil), in.MissingParams...),
		ClarifyingQuestion:         strings.TrimSpace(in.ClarifyingQuestion),
		KnownParams:                copyQueryKnownParams(in.KnownParams),
		OptionGroupID:              strings.TrimSpace(in.CandidateGroupID),
		OptionSource:               strings.TrimSpace(in.CandidateSource),
		OptionCount:                in.CandidateCount,
		RequiresExplicitUserChoice: in.CannotSilentSelect,
		Options:                    options,
		RawUserReply:               strings.TrimSpace(in.RawUserReply),
	}
	if out.OptionCount <= 0 {
		out.OptionCount = len(options)
	}
	if len(out.KnownParams) == 0 {
		out.KnownParams = nil
	}
	if out.SourceTurnID == "" &&
		out.Intent == "" &&
		len(out.MissingParams) == 0 &&
		out.ClarifyingQuestion == "" &&
		len(out.KnownParams) == 0 &&
		out.OptionGroupID == "" &&
		out.OptionSource == "" &&
		out.OptionCount == 0 &&
		!out.RequiresExplicitUserChoice &&
		len(out.Options) == 0 &&
		out.RawUserReply == "" {
		return nil
	}
	return out
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
		SourceTurnID:       strings.TrimSpace(stringValue(payload["source_turn_id"])),
		Intent:             strings.TrimSpace(stringValue(payload["intent"])),
		ClarifyingQuestion: strings.TrimSpace(stringValue(payload["clarifying_question"])),
		ErrorCode:          strings.TrimSpace(stringValue(payload["error_code"])),
		CandidateGroupID:   strings.TrimSpace(stringValue(payload["candidate_group_id"])),
		CandidateSource:    strings.TrimSpace(stringValue(payload["candidate_source"])),
	}
	out.MissingParams = decodeQueryStringList(payload["missing_params"])
	out.CandidateCount = decodeQueryInt(payload["candidate_count"])
	out.CannotSilentSelect = decodeQueryBool(payload["cannot_silent_select"])
	out.KnownParams = copyQueryKnownParams(decodeQueryObject(payload["known_params"]))
	if out.Intent == "" &&
		out.SourceTurnID == "" &&
		out.ClarifyingQuestion == "" &&
		out.ErrorCode == "" &&
		out.CandidateGroupID == "" &&
		out.CandidateSource == "" &&
		out.CandidateCount == 0 &&
		!out.CannotSilentSelect &&
		len(out.MissingParams) == 0 &&
		len(out.KnownParams) == 0 {
		return nil
	}
	if len(out.KnownParams) == 0 {
		out.KnownParams = nil
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

func decodeQueryObject(raw any) map[string]any {
	value, ok := raw.(map[string]any)
	if !ok || len(value) == 0 {
		return nil
	}
	return value
}

func copyQueryKnownParams(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	target := make(map[string]any, len(source))
	for key, value := range source {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		target[name] = value
	}
	if len(target) == 0 {
		return nil
	}
	return target
}

func copyQueryCandidates(source []QueryCandidate) []QueryCandidate {
	if len(source) == 0 {
		return nil
	}
	target := make([]QueryCandidate, 0, len(source))
	for _, item := range source {
		normalized := NormalizeQueryCandidate(item)
		if normalized == nil {
			continue
		}
		target = append(target, *normalized)
	}
	if len(target) == 0 {
		return nil
	}
	return target
}

func copyQueryClarificationLocal(in *QueryClarification) *QueryClarification {
	if in == nil {
		return nil
	}
	out := *in
	out.MissingParams = append([]string(nil), in.MissingParams...)
	out.KnownParams = copyQueryKnownParams(in.KnownParams)
	return &out
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

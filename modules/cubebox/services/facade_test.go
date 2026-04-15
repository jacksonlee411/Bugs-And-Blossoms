package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

type stubConversationReader struct {
	listRows        []cubeboxsqlc.IamCubeboxConversation
	getRow          cubeboxsqlc.IamCubeboxConversation
	getErr          error
	turnRows        []cubeboxsqlc.IamCubeboxTurn
	transitionRows  []cubeboxsqlc.IamCubeboxStateTransition
	syncFn          func(cubeboxdomain.Conversation) error
	blockingTasks   int64
	deleteRows      int64
	taskRow         cubeboxsqlc.IamCubeboxTask
	taskErr         error
	taskDispatchRow cubeboxsqlc.IamCubeboxTask
	taskDispatchErr error
	taskActorID     string
	submitTaskRow   cubeboxsqlc.IamCubeboxTask
	submitExisted   bool
	submitErr       error
	cancelTaskRow   cubeboxsqlc.IamCubeboxTask
	cancelAccepted  bool
	cancelErr       error
	dispatchRows    []cubeboxsqlc.IamCubeboxTaskDispatchOutbox
	updateTaskFn    func(cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error)
	insertEventFn   func(cubeboxdomain.TaskEventRecord) error
	updateOutboxFn  func(cubeboxdomain.TaskDispatchOutboxUpdate) error
}

func (s *stubConversationReader) ListConversations(context.Context, string, string, int32, time.Time, string) ([]cubeboxsqlc.IamCubeboxConversation, error) {
	return s.listRows, nil
}
func (s *stubConversationReader) GetConversation(context.Context, string, string) (cubeboxsqlc.IamCubeboxConversation, error) {
	return s.getRow, s.getErr
}
func (s *stubConversationReader) ListConversationTurns(context.Context, string, string) ([]cubeboxsqlc.IamCubeboxTurn, error) {
	return s.turnRows, nil
}
func (s *stubConversationReader) ListConversationStateTransitions(context.Context, string, string) ([]cubeboxsqlc.IamCubeboxStateTransition, error) {
	return s.transitionRows, nil
}
func (s *stubConversationReader) SyncConversationSnapshot(_ context.Context, _ string, conversation cubeboxdomain.Conversation) error {
	if s.syncFn != nil {
		return s.syncFn(conversation)
	}
	s.getRow = cubeboxsqlc.IamCubeboxConversation{
		ConversationID: conversation.ConversationID,
		ActorID:        conversation.ActorID,
		ActorRole:      conversation.ActorRole,
		State:          conversation.State,
		CurrentPhase:   conversation.CurrentPhase,
		CreatedAt:      pgtype.Timestamptz{Time: conversation.CreatedAt.UTC(), Valid: !conversation.CreatedAt.IsZero()},
		UpdatedAt:      pgtype.Timestamptz{Time: conversation.UpdatedAt.UTC(), Valid: !conversation.UpdatedAt.IsZero()},
	}
	s.turnRows = make([]cubeboxsqlc.IamCubeboxTurn, 0, len(conversation.Turns))
	for _, turn := range conversation.Turns {
		s.turnRows = append(s.turnRows, cubeboxsqlc.IamCubeboxTurn{
			TurnID:              turn.TurnID,
			UserInput:           turn.UserInput,
			State:               turn.State,
			Phase:               turn.Phase,
			RiskTier:            turn.RiskTier,
			RequestID:           turn.RequestID,
			TraceID:             turn.TraceID,
			PolicyVersion:       turn.PolicyVersion,
			CompositionVersion:  turn.CompositionVersion,
			MappingVersion:      turn.MappingVersion,
			IntentJson:          mustJSONBytesFromValue(turn.Intent),
			PlanJson:            mustJSONBytesFromValue(turn.Plan),
			CandidatesJson:      mustJSONBytesFromValue(turn.Candidates),
			RouteDecisionJson:   mustJSONBytesFromValueOrNil(turn.RouteDecision),
			ClarificationJson:   mustJSONBytesFromValue(turn.Clarification),
			DryRunJson:          mustJSONBytesFromValue(turn.DryRun),
			ResolvedCandidateID: nilIfBlank(turn.ResolvedCandidateID),
			SelectedCandidateID: nilIfBlank(turn.SelectedCandidateID),
			AmbiguityCount:      int32(turn.AmbiguityCount),
			Confidence:          turn.Confidence,
			ResolutionSource:    nilIfBlank(turn.ResolutionSource),
			PendingDraftSummary: nilIfBlank(turn.PendingDraftSummary),
			MissingFields:       mustJSONBytesFromValue(turn.MissingFields),
			CommitResultJson:    mustJSONBytesFromValueOrNil(turn.CommitResult),
			CommitReply:         mustJSONBytesFromValueOrNil(turn.CommitReply),
			ErrorCode:           nilIfBlank(turn.ErrorCode),
			CreatedAt:           pgtype.Timestamptz{Time: turn.CreatedAt.UTC(), Valid: !turn.CreatedAt.IsZero()},
			UpdatedAt:           pgtype.Timestamptz{Time: turn.UpdatedAt.UTC(), Valid: !turn.UpdatedAt.IsZero()},
		})
	}
	s.transitionRows = make([]cubeboxsqlc.IamCubeboxStateTransition, 0, len(conversation.Transitions))
	for _, transition := range conversation.Transitions {
		s.transitionRows = append(s.transitionRows, cubeboxsqlc.IamCubeboxStateTransition{
			ID:             transition.ID,
			ConversationID: conversation.ConversationID,
			TurnID:         nilIfBlank(transition.TurnID),
			TurnAction:     nilIfBlank(transition.TurnAction),
			RequestID:      transition.RequestID,
			TraceID:        transition.TraceID,
			FromState:      transition.FromState,
			ToState:        transition.ToState,
			FromPhase:      transition.FromPhase,
			ToPhase:        transition.ToPhase,
			ReasonCode:     nilIfBlank(transition.ReasonCode),
			ActorID:        transition.ActorID,
			ChangedAt:      pgtype.Timestamptz{Time: transition.ChangedAt.UTC(), Valid: !transition.ChangedAt.IsZero()},
		})
	}
	return nil
}
func (s *stubConversationReader) CountBlockingTasks(context.Context, string, string) (int64, error) {
	return s.blockingTasks, nil
}
func (s *stubConversationReader) DeleteConversation(context.Context, string, string) (int64, error) {
	return s.deleteRows, nil
}
func (s *stubConversationReader) GetTask(context.Context, string, string) (cubeboxsqlc.IamCubeboxTask, error) {
	return s.taskRow, s.taskErr
}
func (s *stubConversationReader) GetTaskForDispatch(context.Context, string, string) (cubeboxsqlc.IamCubeboxTask, error) {
	return s.taskDispatchRow, s.taskDispatchErr
}
func (s *stubConversationReader) GetTaskActorID(context.Context, string, string) (string, error) {
	return s.taskActorID, nil
}
func (s *stubConversationReader) SubmitTask(context.Context, string, cubeboxsqlc.IamCubeboxTask) (cubeboxsqlc.IamCubeboxTask, bool, error) {
	return s.submitTaskRow, s.submitExisted, s.submitErr
}
func (s *stubConversationReader) CancelTask(context.Context, string, string, time.Time) (cubeboxsqlc.IamCubeboxTask, bool, error) {
	return s.cancelTaskRow, s.cancelAccepted, s.cancelErr
}
func (s *stubConversationReader) ListDispatchOutbox(context.Context, string, string, int32) ([]cubeboxsqlc.IamCubeboxTaskDispatchOutbox, error) {
	return s.dispatchRows, nil
}
func (s *stubConversationReader) UpdateTaskState(_ context.Context, _ string, update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
	if s.updateTaskFn != nil {
		return s.updateTaskFn(update)
	}
	return s.taskDispatchRow, nil
}
func (s *stubConversationReader) InsertTaskEvent(_ context.Context, _ string, event cubeboxdomain.TaskEventRecord) error {
	if s.insertEventFn != nil {
		return s.insertEventFn(event)
	}
	return nil
}
func (s *stubConversationReader) UpdateTaskDispatchOutbox(_ context.Context, _ string, update cubeboxdomain.TaskDispatchOutboxUpdate) error {
	if s.updateOutboxFn != nil {
		return s.updateOutboxFn(update)
	}
	return nil
}

type stubRuntimeProbe struct {
	models    []cubeboxdomain.ModelEntry
	backend   cubeboxdomain.RuntimeComponentStatus
	knowledge cubeboxdomain.RuntimeComponentStatus
	modelGate cubeboxdomain.RuntimeComponentStatus
}

func (s stubRuntimeProbe) BackendStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return s.backend
}
func (s stubRuntimeProbe) KnowledgeRuntimeStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return s.knowledge
}
func (s stubRuntimeProbe) ModelGatewayStatus(context.Context) cubeboxdomain.RuntimeComponentStatus {
	return s.modelGate
}
func (s stubRuntimeProbe) Models(context.Context) ([]cubeboxdomain.ModelEntry, error) {
	return s.models, nil
}

type stubLegacyFacade struct {
	listItems     []cubeboxdomain.ConversationListItem
	listNext      string
	getConv       *cubeboxdomain.Conversation
	createConv    *cubeboxdomain.Conversation
	createTurn    *cubeboxdomain.Conversation
	confirmTurn   *cubeboxdomain.Conversation
	commitReceipt *cubeboxdomain.TaskReceipt
	getTask       *cubeboxdomain.TaskDetail
	reply         map[string]any
	execTask      TaskWorkflowExecutionResult
	execErr       error
	execFn        func(string, Principal, *cubeboxdomain.Conversation, string) (TaskWorkflowExecutionResult, error)
}

func (s stubLegacyFacade) ListConversations(context.Context, string, string, int, string) ([]cubeboxdomain.ConversationListItem, string, error) {
	return s.listItems, s.listNext, nil
}
func (s stubLegacyFacade) GetConversation(context.Context, string, string, string) (*cubeboxdomain.Conversation, error) {
	return s.getConv, nil
}
func (s stubLegacyFacade) CreateConversation(context.Context, string, Principal) (*cubeboxdomain.Conversation, error) {
	return s.createConv, nil
}
func (s stubLegacyFacade) CreateTurn(context.Context, string, Principal, string, string) (*cubeboxdomain.Conversation, error) {
	return s.createTurn, nil
}
func (s stubLegacyFacade) ConfirmTurn(context.Context, string, Principal, string, string, string) (*cubeboxdomain.Conversation, error) {
	return s.confirmTurn, nil
}
func (s stubLegacyFacade) CommitTurn(context.Context, string, Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
	return s.commitReceipt, nil
}
func (s stubLegacyFacade) SubmitTask(context.Context, string, Principal, cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	return nil, nil
}
func (s stubLegacyFacade) GetTask(context.Context, string, Principal, string) (*cubeboxdomain.TaskDetail, error) {
	return s.getTask, nil
}
func (s stubLegacyFacade) CancelTask(context.Context, string, Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
	return nil, nil
}
func (s stubLegacyFacade) ExecuteTaskWorkflow(_ context.Context, tenantID string, principal Principal, conversation *cubeboxdomain.Conversation, turnID string) (TaskWorkflowExecutionResult, error) {
	if s.execFn != nil {
		return s.execFn(tenantID, principal, conversation, turnID)
	}
	return s.execTask, s.execErr
}
func (s stubLegacyFacade) RenderReply(context.Context, string, Principal, string, string, map[string]any) (map[string]any, error) {
	return s.reply, nil
}

type healthyFileStore struct{ err error }

func (s healthyFileStore) List(context.Context, string, string) ([]FileRecord, error) {
	return nil, nil
}
func (s healthyFileStore) Save(context.Context, string, string, string, string, string, io.Reader) (FileRecord, error) {
	return FileRecord{}, nil
}
func (s healthyFileStore) Delete(context.Context, string, string) (bool, error) { return false, nil }
func (s healthyFileStore) Healthy(context.Context) error                        { return s.err }

func TestFacadeListConversations(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	reader := &stubConversationReader{
		listRows: []cubeboxsqlc.IamCubeboxConversation{
			{
				ConversationID: "conv_2",
				ActorID:        "actor-1",
				State:          "validated",
				UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
			},
		},
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_2",
			ActorID:        "actor-1",
			State:          "validated",
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:    "turn_1",
			UserInput: "hello",
			State:     "validated",
			RiskTier:  "low",
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
	}
	facade := NewFacade(reader, nil, nil, nil)
	items, next, err := facade.ListConversations(context.Background(), "tenant-1", "actor-1", 20, "")
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if next != "" {
		t.Fatalf("unexpected next cursor: %q", next)
	}
	if len(items) != 1 || items[0].LastTurn == nil || items[0].LastTurn.TurnID != "turn_1" {
		t.Fatalf("items=%+v", items)
	}
}

func TestFacadeGetConversationFallsBackToLegacy(t *testing.T) {
	facade := NewFacade(&stubConversationReader{getErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
		getConv: &cubeboxdomain.Conversation{ConversationID: "conv_fallback"},
	})
	conv, err := facade.GetConversation(context.Background(), "tenant-1", "actor-1", "conv_fallback")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if conv == nil || conv.ConversationID != "conv_fallback" {
		t.Fatalf("conversation=%+v", conv)
	}
}

func TestFacadeDeleteConversation(t *testing.T) {
	reader := &stubConversationReader{
		getRow:     cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
		deleteRows: 1,
	}
	facade := NewFacade(reader, nil, nil, nil)
	if err := facade.DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1"); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}

	reader.blockingTasks = 1
	if err := facade.DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1"); !errors.Is(err, ErrDeleteBlockedByTask) {
		t.Fatalf("expected blocking task error, got %v", err)
	}
}

func TestFacadeCreateConversationSyncsFormalSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 15, 15, 0, 0, 0, time.UTC)
	reader := &stubConversationReader{}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		createConv: &cubeboxdomain.Conversation{
			ConversationID: "conv_sync",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "validated",
			CurrentPhase:   "idle",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	})
	conv, err := facade.CreateConversation(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"})
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if conv == nil || conv.ConversationID != "conv_sync" {
		t.Fatalf("conversation=%+v", conv)
	}
	if reader.getRow.ConversationID != "conv_sync" || reader.getRow.ActorID != "actor-1" {
		t.Fatalf("synced row=%+v", reader.getRow)
	}
}

func TestFacadeCreateTurnSyncsFormalSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 15, 15, 10, 0, 0, time.UTC)
	reader := &stubConversationReader{}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		createTurn: &cubeboxdomain.Conversation{
			ConversationID: "conv_sync",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "validated",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      now,
			UpdatedAt:      now,
			Turns: []cubeboxdomain.ConversationTurn{{
				TurnID:     "turn_sync",
				UserInput:  "create org",
				State:      "validated",
				Phase:      "await_commit_confirm",
				RiskTier:   "low",
				RequestID:  "assistant_req",
				TraceID:    "trace",
				Plan:       map[string]any{"summary": "create"},
				Intent:     map[string]any{"action": "create_orgunit"},
				Candidates: []map[string]any{},
				DryRun:     map[string]any{"plan_hash": "p"},
				CreatedAt:  now,
				UpdatedAt:  now,
			}},
			Transitions: []cubeboxdomain.StateTransition{{
				TurnID:     "turn_sync",
				RequestID:  "assistant_req",
				TraceID:    "trace",
				FromState:  "init",
				ToState:    "validated",
				FromPhase:  "idle",
				ToPhase:    "await_commit_confirm",
				ReasonCode: "turn_created",
				ActorID:    "actor-1",
				ChangedAt:  now,
			}},
		},
	})
	conv, err := facade.CreateTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_sync", "create org")
	if err != nil {
		t.Fatalf("create turn: %v", err)
	}
	if conv == nil || len(conv.Turns) != 1 || conv.Turns[0].TurnID != "turn_sync" {
		t.Fatalf("conversation=%+v", conv)
	}
	if len(reader.turnRows) != 1 || reader.turnRows[0].TurnID != "turn_sync" {
		t.Fatalf("synced turns=%+v", reader.turnRows)
	}
	if len(reader.transitionRows) != 1 || reader.transitionRows[0].ReasonCode == nil || *reader.transitionRows[0].ReasonCode != "turn_created" {
		t.Fatalf("synced transitions=%+v", reader.transitionRows)
	}
}

func TestFacadeConfirmTurnSyncsFormalSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 15, 15, 20, 0, 0, time.UTC)
	reader := &stubConversationReader{}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		confirmTurn: &cubeboxdomain.Conversation{
			ConversationID: "conv_sync",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      now,
			UpdatedAt:      now,
			Turns: []cubeboxdomain.ConversationTurn{{
				TurnID:              "turn_sync",
				UserInput:           "create org",
				State:               "confirmed",
				Phase:               "await_commit_confirm",
				RiskTier:            "medium",
				RequestID:           "assistant_req",
				TraceID:             "trace",
				ResolvedCandidateID: "cand_1",
				Plan:                map[string]any{"summary": "confirm"},
				Intent:              map[string]any{"action": "create_orgunit"},
				Candidates:          []map[string]any{{"candidate_id": "cand_1"}},
				DryRun:              map[string]any{"plan_hash": "p"},
				CreatedAt:           now,
				UpdatedAt:           now,
			}},
			Transitions: []cubeboxdomain.StateTransition{{
				TurnID:     "turn_sync",
				TurnAction: "confirm",
				RequestID:  "assistant_req",
				TraceID:    "trace",
				FromState:  "validated",
				ToState:    "confirmed",
				FromPhase:  "await_candidate_confirm",
				ToPhase:    "await_commit_confirm",
				ReasonCode: "confirmed",
				ActorID:    "actor-1",
				ChangedAt:  now,
			}},
		},
	})
	conv, err := facade.ConfirmTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_sync", "turn_sync", "cand_1")
	if err != nil {
		t.Fatalf("confirm turn: %v", err)
	}
	if conv == nil || conv.State != "confirmed" {
		t.Fatalf("conversation=%+v", conv)
	}
	if len(reader.transitionRows) != 1 || reader.transitionRows[0].TurnAction == nil || *reader.transitionRows[0].TurnAction != "confirm" {
		t.Fatalf("synced transitions=%+v", reader.transitionRows)
	}
	if len(reader.turnRows) != 1 || reader.turnRows[0].ResolvedCandidateID == nil || *reader.turnRows[0].ResolvedCandidateID != "cand_1" {
		t.Fatalf("synced turns=%+v", reader.turnRows)
	}
}

func TestFacadeCommitTurnUsesFormalSubmitWhenConversationExists(t *testing.T) {
	now := time.Date(2026, 4, 15, 16, 20, 0, 0, time.UTC)
	taskID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	intent := mustJSONBytes(t, map[string]any{
		"action":                "create_orgunit",
		"intent_schema_version": "intent.v1",
		"context_hash":          "ctx",
		"intent_hash":           "intent",
	})
	plan := mustJSONBytes(t, map[string]any{
		"compiler_contract_version": "compiler.v1",
		"capability_map_version":    "cap.v1",
		"skill_manifest_digest":     "skill",
		"knowledge_snapshot_digest": "knowledge.v1",
		"route_catalog_version":     "route.v1",
		"resolver_contract_version": "resolver.v1",
		"context_template_version":  "ctx-template.v1",
		"reply_guidance_version":    "reply.v1",
		"confirm_ttl_seconds":       900,
		"expires_at":                now.Add(5 * time.Minute).Format(time.RFC3339),
	})
	routeDecision := mustJSONBytes(t, map[string]any{
		"knowledge_snapshot_digest": "knowledge.v1",
		"route_catalog_version":     "route.v1",
		"resolver_contract_version": "resolver.v1",
	})
	dryRun := mustJSONBytes(t, map[string]any{
		"plan_hash": "plan",
		"create_orgunit_projection": map[string]any{
			"policy_context": map[string]any{
				"policy_context_digest": "policy-digest",
			},
			"projection": map[string]any{
				"effective_policy_version": "epv1",
				"resolved_setid":           "S2601",
				"setid_source":             "custom",
				"projection_digest":        "projection-digest",
				"mutation_policy_version":  "mutation.v1",
			},
		},
	})
	reader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_1",
			State:             "confirmed",
			RequestID:         "req_1",
			TraceID:           "trace_1",
			IntentJson:        intent,
			PlanJson:          plan,
			RouteDecisionJson: routeDecision,
			DryRunJson:        dryRun,
			CreatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		}},
		submitTaskRow: cubeboxsqlc.IamCubeboxTask{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:    taskTypeAsyncPlan,
			Status:      taskStatusQueued,
			WorkflowID:  "assistant_async_orchestration_v1:tenant-1:conv_1:turn_1:req_1",
			RequestID:   "req_1",
			RequestHash: "hash",
			SubmittedAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
	}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		commitReceipt: &cubeboxdomain.TaskReceipt{TaskID: "legacy-task"},
	})
	facade.nowFn = func() time.Time { return now }
	receipt, err := facade.CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1")
	if err != nil {
		t.Fatalf("commit turn: %v", err)
	}
	if receipt == nil || receipt.TaskID != taskID.String() {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestFacadeCommitTurnFallsBackToLegacyWhenFormalConversationMissing(t *testing.T) {
	facade := NewFacade(&stubConversationReader{getErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
		commitReceipt: &cubeboxdomain.TaskReceipt{TaskID: "fallback-task"},
	})
	receipt, err := facade.CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_fallback", "turn_1")
	if err != nil {
		t.Fatalf("commit turn: %v", err)
	}
	if receipt == nil || receipt.TaskID != "fallback-task" {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestFacadeGetTaskMapsPollFreeContract(t *testing.T) {
	taskID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	reader := &stubConversationReader{
		taskRow: cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                "assistant_async_plan",
			Status:                  "queued",
			DispatchStatus:          "pending",
			WorkflowID:              "wf",
			RequestID:               "req",
			ConversationID:          "conv_1",
			TurnID:                  "turn_1",
			IntentSchemaVersion:     "v1",
			CompilerContractVersion: "v1",
			CapabilityMapVersion:    "v1",
			SkillManifestDigest:     "skill",
			ContextHash:             "ctx",
			IntentHash:              "intent",
			PlanHash:                "plan",
			SubmittedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:               pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		},
		taskActorID: "actor-1",
	}
	facade := NewFacade(reader, nil, nil, nil)
	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.TaskID != taskID.String() {
		t.Fatalf("task=%+v", task)
	}
}

func TestFacadeGetTaskRejectsOtherActor(t *testing.T) {
	taskID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	reader := &stubConversationReader{
		taskRow: cubeboxsqlc.IamCubeboxTask{
			TaskID:         pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:       "assistant_async_plan",
			Status:         "queued",
			WorkflowID:     "wf",
			RequestID:      "req",
			ConversationID: "conv_1",
			TurnID:         "turn_1",
			SubmittedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		},
		taskActorID: "actor-2",
	}
	facade := NewFacade(reader, nil, nil, nil)
	if _, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String()); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected task not found, got %v", err)
	}
}

func TestFacadeConversationTurnRetainsFormalJSONFields(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	intent := mustJSONBytes(t, map[string]any{"action": "create_orgunit"})
	plan := mustJSONBytes(t, map[string]any{"summary": "create org"})
	candidates := mustJSONBytes(t, []map[string]any{{"candidate_id": "cand_1"}})
	dryRun := mustJSONBytes(t, map[string]any{"explain": "ok"})
	commitReply := mustJSONBytes(t, map[string]any{"outcome": "success"})
	commitResult := mustJSONBytes(t, map[string]any{"org_code": "ORG-A"})
	reader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
			State:          "validated",
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:           "turn_1",
			UserInput:        "hello",
			State:            "validated",
			RiskTier:         "low",
			IntentJson:       intent,
			PlanJson:         plan,
			CandidatesJson:   candidates,
			DryRunJson:       dryRun,
			CommitReply:      commitReply,
			CommitResultJson: commitResult,
			CreatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
		}},
	}
	facade := NewFacade(reader, nil, nil, nil)
	conv, err := facade.GetConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if conv == nil || len(conv.Turns) != 1 {
		t.Fatalf("conversation=%+v", conv)
	}
	turn := conv.Turns[0]
	if turn.Plan["summary"] != "create org" || turn.Intent["action"] != "create_orgunit" {
		t.Fatalf("turn=%+v", turn)
	}
	if len(turn.Candidates) != 1 || turn.Candidates[0]["candidate_id"] != "cand_1" {
		t.Fatalf("turn=%+v", turn)
	}
	if turn.DryRun["explain"] != "ok" || turn.CommitReply["outcome"] != "success" || turn.CommitResult["org_code"] != "ORG-A" {
		t.Fatalf("turn=%+v", turn)
	}
}

func TestFacadeSubmitTaskUsesFormalStore(t *testing.T) {
	now := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	taskID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	intent := mustJSONBytes(t, map[string]any{
		"action":                "create_orgunit",
		"intent_schema_version": "intent.v1",
		"context_hash":          "ctx",
		"intent_hash":           "intent",
	})
	plan := mustJSONBytes(t, map[string]any{
		"compiler_contract_version": "compiler.v1",
		"capability_map_version":    "cap.v1",
		"skill_manifest_digest":     "skill",
		"knowledge_snapshot_digest": "knowledge.v1",
		"route_catalog_version":     "route.v1",
		"resolver_contract_version": "resolver.v1",
		"context_template_version":  "ctx-template.v1",
		"reply_guidance_version":    "reply.v1",
	})
	routeDecision := mustJSONBytes(t, map[string]any{
		"knowledge_snapshot_digest": "knowledge.v1",
		"route_catalog_version":     "route.v1",
		"resolver_contract_version": "resolver.v1",
	})
	dryRun := mustJSONBytes(t, map[string]any{
		"plan_hash": "plan",
		"create_orgunit_projection": map[string]any{
			"policy_context": map[string]any{
				"policy_context_digest": "policy-digest",
			},
			"projection": map[string]any{
				"effective_policy_version": "epv1",
				"resolved_setid":           "S2601",
				"setid_source":             "custom",
				"projection_digest":        "projection-digest",
				"mutation_policy_version":  "mutation.v1",
			},
		},
	})
	reader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_1",
			IntentJson:        intent,
			PlanJson:          plan,
			RouteDecisionJson: routeDecision,
			DryRunJson:        dryRun,
		}},
		submitTaskRow: cubeboxsqlc.IamCubeboxTask{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:    "assistant_async_plan",
			Status:      "queued",
			WorkflowID:  "assistant_async_orchestration_v1:tenant-1:conv_1:turn_1:req_1",
			SubmittedAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
	}
	facade := NewFacade(reader, nil, nil, nil)
	facade.nowFn = func() time.Time { return now }

	receipt, err := facade.SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, cubeboxdomain.TaskSubmitRequest{
		ConversationID: "conv_1",
		TurnID:         "turn_1",
		TaskType:       "assistant_async_plan",
		RequestID:      "req_1",
		TraceID:        "trace_1",
		ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:      "intent.v1",
			CompilerContractVersion:  "compiler.v1",
			CapabilityMapVersion:     "cap.v1",
			SkillManifestDigest:      "skill",
			ContextHash:              "ctx",
			IntentHash:               "intent",
			PlanHash:                 "plan",
			KnowledgeSnapshotDigest:  "knowledge.v1",
			RouteCatalogVersion:      "route.v1",
			ResolverContractVersion:  "resolver.v1",
			ContextTemplateVersion:   "ctx-template.v1",
			ReplyGuidanceVersion:     "reply.v1",
			PolicyContextDigest:      "policy-digest",
			EffectivePolicyVersion:   "epv1",
			ResolvedSetID:            "S2601",
			SetIDSource:              "custom",
			PrecheckProjectionDigest: "projection-digest",
			MutationPolicyVersion:    "mutation.v1",
		},
	})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	if receipt == nil || receipt.TaskID != taskID.String() || receipt.PollURI != "/internal/cubebox/tasks/"+taskID.String() {
		t.Fatalf("receipt=%+v", receipt)
	}
}

func TestFacadeCancelTaskRejectsOtherActor(t *testing.T) {
	reader := &stubConversationReader{taskActorID: "actor-2"}
	facade := NewFacade(reader, nil, nil, nil)
	if _, err := facade.CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "55555555-5555-5555-5555-555555555555"); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestFacadeGetTaskDispatchesPendingFormalTask(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	taskID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		DispatchAttempt:         0,
		Attempt:                 0,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req",
		ConversationID:          "conv_1",
		TurnID:                  "turn_1",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		getRow:          cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1", ActorRole: "tenant-admin"},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_1",
			IntentJson:        mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:          mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson:        mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			RouteDecisionJson: nil,
		}},
	}
	var updates []cubeboxdomain.TaskStateUpdate
	var events []cubeboxdomain.TaskEventRecord
	var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		updates = append(updates, update)
		taskRow.Status = update.Status
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.DispatchAttempt = int32(update.DispatchAttempt)
		taskRow.Attempt = int32(update.Attempt)
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	reader.insertEventFn = func(event cubeboxdomain.TaskEventRecord) error {
		events = append(events, event)
		return nil
	}
	reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
		outboxes = append(outboxes, update)
		return nil
	}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != taskStatusSucceeded {
		t.Fatalf("task=%+v", task)
	}
	if len(updates) < 2 || updates[len(updates)-1].Status != taskStatusSucceeded {
		t.Fatalf("updates=%+v", updates)
	}
	if len(events) < 2 || events[len(events)-1].EventType != taskStatusSucceeded {
		t.Fatalf("events=%+v", events)
	}
	if len(outboxes) != 1 || outboxes[0].Status != taskDispatchStarted {
		t.Fatalf("outboxes=%+v", outboxes)
	}
}

func TestFacadeDispatchTaskPassesFormalConversationSnapshotToLegacyExecutor(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 2, 0, 0, time.UTC)
	taskID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		DispatchAttempt:         0,
		Attempt:                 0,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req_1",
		ConversationID:          "conv_formal",
		TurnID:                  "turn_formal",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_formal",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:              "turn_formal",
			UserInput:           "create org",
			State:               "confirmed",
			Phase:               "await_commit_confirm",
			RiskTier:            "high",
			RequestID:           "req_1",
			TraceID:             "trace_1",
			PolicyVersion:       "policy.v1",
			CompositionVersion:  "composition.v1",
			MappingVersion:      "mapping.v1",
			IntentJson:          mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:            mustJSONBytes(t, map[string]any{"summary": "formal summary", "compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson:          mustJSONBytes(t, map[string]any{"plan_hash": "plan", "explain": "formal explain"}),
			ResolvedCandidateID: nilIfBlank("cand_1"),
			CreatedAt:           pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:           pgtype.Timestamptz{Time: now, Valid: true},
		}},
		transitionRows: []cubeboxsqlc.IamCubeboxStateTransition{{
			ID:             1,
			ConversationID: "conv_formal",
			TurnID:         nilIfBlank("turn_formal"),
			TurnAction:     nilIfBlank("confirm"),
			RequestID:      "req_1",
			TraceID:        "trace_1",
			FromState:      "validated",
			ToState:        "confirmed",
			FromPhase:      "await_candidate_confirm",
			ToPhase:        "await_commit_confirm",
			ReasonCode:     nilIfBlank("confirmed"),
			ActorID:        "actor-1",
			ChangedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		}},
	}
	var capturedConversation *cubeboxdomain.Conversation
	var capturedPrincipal Principal
	var capturedTenant string
	var capturedTurnID string
	var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		taskRow.Status = update.Status
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.DispatchAttempt = int32(update.DispatchAttempt)
		taskRow.Attempt = int32(update.Attempt)
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
		outboxes = append(outboxes, update)
		return nil
	}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		execFn: func(tenantID string, principal Principal, conversation *cubeboxdomain.Conversation, turnID string) (TaskWorkflowExecutionResult, error) {
			capturedTenant = tenantID
			capturedPrincipal = principal
			capturedTurnID = turnID
			capturedConversation = conversation
			return TaskWorkflowExecutionResult{}, nil
		},
	})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != taskStatusSucceeded {
		t.Fatalf("task=%+v", task)
	}
	if capturedTenant != "tenant-1" || capturedPrincipal.ID != "actor-1" || capturedPrincipal.RoleSlug != "tenant-admin" || capturedTurnID != "turn_formal" {
		t.Fatalf("captured tenant/principal/turn mismatch: tenant=%q principal=%+v turn=%q", capturedTenant, capturedPrincipal, capturedTurnID)
	}
	if capturedConversation == nil || capturedConversation.ConversationID != "conv_formal" || len(capturedConversation.Turns) != 1 {
		t.Fatalf("captured conversation=%+v", capturedConversation)
	}
	if capturedConversation.Turns[0].TurnID != "turn_formal" || capturedConversation.Turns[0].Plan["summary"] != "formal summary" || capturedConversation.Turns[0].DryRun["explain"] != "formal explain" {
		t.Fatalf("captured turn=%+v", capturedConversation.Turns[0])
	}
	if len(outboxes) != 1 || outboxes[0].Status != taskDispatchStarted {
		t.Fatalf("outboxes=%+v", outboxes)
	}
}

func TestFacadeDispatchTaskSyncsExecutedFormalConversationSnapshot(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 12, 0, 0, time.UTC)
	taskID := uuid.MustParse("12121212-1212-1212-1212-121212121212")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		DispatchAttempt:         0,
		Attempt:                 0,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req_1",
		ConversationID:          "conv_exec",
		TurnID:                  "turn_exec",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_exec",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_exec",
			UserInput:         "create org",
			State:             "confirmed",
			Phase:             "await_commit_confirm",
			RequestID:         "req_1",
			TraceID:           "trace_1",
			IntentJson:        mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:          mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson:        mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			RouteDecisionJson: nil,
			CreatedAt:         pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		}},
	}
	var updates []cubeboxdomain.TaskStateUpdate
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		updates = append(updates, update)
		taskRow.Status = update.Status
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.DispatchAttempt = int32(update.DispatchAttempt)
		taskRow.Attempt = int32(update.Attempt)
		taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		execTask: TaskWorkflowExecutionResult{
			Conversation: &cubeboxdomain.Conversation{
				ConversationID: "conv_exec",
				TenantID:       "tenant-1",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
				State:          "committed",
				CurrentPhase:   "completed",
				CreatedAt:      now.Add(-time.Minute),
				UpdatedAt:      now.Add(30 * time.Second),
				Turns: []cubeboxdomain.ConversationTurn{{
					TurnID:       "turn_exec",
					UserInput:    "create org",
					State:        "committed",
					Phase:        "completed",
					RequestID:    "req_1",
					TraceID:      "trace_1",
					CommitResult: map[string]any{"org_code": "ORG-A"},
					CommitReply:  map[string]any{"summary": "created"},
					CreatedAt:    now.Add(-time.Minute),
					UpdatedAt:    now.Add(30 * time.Second),
				}},
			},
		},
	})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != taskStatusSucceeded {
		t.Fatalf("task=%+v", task)
	}
	if len(reader.turnRows) != 1 || reader.turnRows[0].State != "committed" || string(reader.turnRows[0].CommitResultJson) == "" {
		t.Fatalf("formal turns not synced: %+v", reader.turnRows)
	}
	if reader.getRow.State != "committed" || reader.getRow.CurrentPhase != "completed" {
		t.Fatalf("conversation snapshot not synced: %+v", reader.getRow)
	}
	if len(updates) < 2 || updates[len(updates)-1].Status != taskStatusSucceeded {
		t.Fatalf("updates=%+v", updates)
	}
}

func TestFacadeDispatchTaskSyncsApplyErrorConversationSnapshotBeforeManualTakeover(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 15, 0, 0, time.UTC)
	taskID := uuid.MustParse("13131313-1313-1313-1313-131313131313")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req_1",
		ConversationID:          "conv_exec",
		TurnID:                  "turn_exec",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_exec",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_exec",
			UserInput:         "create org",
			State:             "confirmed",
			Phase:             "await_commit_confirm",
			RequestID:         "req_1",
			TraceID:           "trace_1",
			IntentJson:        mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:          mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson:        mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			RouteDecisionJson: nil,
			CreatedAt:         pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		}},
	}
	var updates []cubeboxdomain.TaskStateUpdate
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		updates = append(updates, update)
		taskRow.Status = update.Status
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.DispatchAttempt = int32(update.DispatchAttempt)
		taskRow.Attempt = int32(update.Attempt)
		taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		execTask: TaskWorkflowExecutionResult{
			ApplyErrorCode: "orgunit_code_conflict",
			Conversation: &cubeboxdomain.Conversation{
				ConversationID: "conv_exec",
				TenantID:       "tenant-1",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
				State:          "manual_takeover_required",
				CurrentPhase:   "completed",
				CreatedAt:      now.Add(-time.Minute),
				UpdatedAt:      now.Add(30 * time.Second),
				Turns: []cubeboxdomain.ConversationTurn{{
					TurnID:    "turn_exec",
					UserInput: "create org",
					State:     "manual_takeover_required",
					Phase:     "completed",
					RequestID: "req_1",
					TraceID:   "trace_1",
					ErrorCode: "orgunit_code_conflict",
					CreatedAt: now.Add(-time.Minute),
					UpdatedAt: now.Add(30 * time.Second),
				}},
			},
		},
	})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != taskStatusManualTakeover {
		t.Fatalf("task=%+v", task)
	}
	if reader.getRow.State != "manual_takeover_required" || len(reader.turnRows) != 1 || reader.turnRows[0].ErrorCode == nil || *reader.turnRows[0].ErrorCode != "orgunit_code_conflict" {
		t.Fatalf("formal snapshot not synced before manual takeover: conversation=%+v turns=%+v", reader.getRow, reader.turnRows)
	}
	if len(updates) < 2 || updates[len(updates)-1].Status != taskStatusManualTakeover || updates[len(updates)-1].LastErrorCode != "orgunit_code_conflict" {
		t.Fatalf("updates=%+v", updates)
	}
}

func TestFacadeDispatchTaskMarksManualTakeoverWhenConversationSnapshotSyncFails(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 18, 0, 0, time.UTC)
	taskID := uuid.MustParse("14141414-1414-1414-1414-141414141414")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req_1",
		ConversationID:          "conv_exec",
		TurnID:                  "turn_exec",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_exec",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
			State:          "confirmed",
			CurrentPhase:   "await_commit_confirm",
			CreatedAt:      pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:            "turn_exec",
			UserInput:         "create org",
			State:             "confirmed",
			Phase:             "await_commit_confirm",
			RequestID:         "req_1",
			TraceID:           "trace_1",
			IntentJson:        mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:          mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson:        mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			RouteDecisionJson: nil,
			CreatedAt:         pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			UpdatedAt:         pgtype.Timestamptz{Time: now, Valid: true},
		}},
		syncFn: func(cubeboxdomain.Conversation) error {
			return errors.New("sync failed")
		},
	}
	var updates []cubeboxdomain.TaskStateUpdate
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		updates = append(updates, update)
		taskRow.Status = update.Status
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.DispatchAttempt = int32(update.DispatchAttempt)
		taskRow.Attempt = int32(update.Attempt)
		taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	execCalls := 0
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{
		execFn: func(string, Principal, *cubeboxdomain.Conversation, string) (TaskWorkflowExecutionResult, error) {
			execCalls++
			return TaskWorkflowExecutionResult{
				Conversation: &cubeboxdomain.Conversation{
					ConversationID: "conv_exec",
					TenantID:       "tenant-1",
					ActorID:        "actor-1",
					ActorRole:      "tenant-admin",
					State:          "committed",
					CurrentPhase:   "completed",
					CreatedAt:      now.Add(-time.Minute),
					UpdatedAt:      now.Add(30 * time.Second),
				},
			}, nil
		},
	})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if execCalls != 1 {
		t.Fatalf("expected single execution attempt, got %d", execCalls)
	}
	if task == nil || task.Status != taskStatusManualTakeover {
		t.Fatalf("task=%+v", task)
	}
	if len(updates) < 2 || updates[len(updates)-1].LastErrorCode != ErrConversationSnapshotSyncFailed.Error() {
		t.Fatalf("updates=%+v", updates)
	}
}

func TestFacadeGetTaskMarksSnapshotMismatchManualTakeover(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC)
	taskID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	taskRow := cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		ConversationID:          "conv_1",
		TurnID:                  "turn_1",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(5 * time.Minute), Valid: true},
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	}
	reader := &stubConversationReader{
		dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
			TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
			Status:      taskDispatchPending,
			Attempt:     0,
			NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
		}},
		taskDispatchRow: taskRow,
		taskRow:         taskRow,
		taskActorID:     "actor-1",
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:     "turn_1",
			IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v2", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
		}},
	}
	var events []cubeboxdomain.TaskEventRecord
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxsqlc.IamCubeboxTask, error) {
		taskRow.Status = update.Status
		taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return taskRow, nil
	}
	reader.insertEventFn = func(event cubeboxdomain.TaskEventRecord) error {
		events = append(events, event)
		return nil
	}
	reader.updateOutboxFn = func(cubeboxdomain.TaskDispatchOutboxUpdate) error { return nil }
	facade := NewFacade(reader, nil, nil, stubLegacyFacade{})
	facade.nowFn = func() time.Time { return now }

	task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task == nil || task.Status != taskStatusManualTakeover {
		t.Fatalf("task=%+v", task)
	}
	if task.LastErrorCode != ErrPlanContractMismatch.Error() {
		t.Fatalf("task=%+v", task)
	}
	if len(events) != 3 {
		t.Fatalf("events=%+v", events)
	}
	if events[1].EventType != "manual_takeover_required" || !strings.Contains(events[1].ErrorCode, "cubebox_plan_contract_version_mismatch") {
		t.Fatalf("events=%+v", events)
	}
}

func TestFacadeRuntimeStatus(t *testing.T) {
	facade := NewFacade(nil, stubRuntimeProbe{
		backend:   cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy},
		knowledge: cubeboxdomain.RuntimeComponentStatus{Healthy: healthDegraded, Reason: "knowledge_runtime_unavailable"},
		modelGate: cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy},
	}, NewFileService(healthyFileStore{}), nil)
	facade.nowFn = func() time.Time { return time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) }
	status := facade.RuntimeStatus(context.Background())
	if status.Status != healthDegraded {
		t.Fatalf("status=%+v", status)
	}
	if status.FileStore.Healthy != healthHealthy {
		t.Fatalf("status=%+v", status)
	}
}

func TestConversationCursorRoundTrip(t *testing.T) {
	cursor := encodeConversationCursor(conversationCursor{
		UpdatedAt:      time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		ConversationID: "conv_1",
	}, "tenant-1", "actor-1")
	decoded, err := decodeConversationCursor(cursor, "tenant-1", "actor-1")
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}
	if decoded == nil || decoded.ConversationID != "conv_1" {
		t.Fatalf("decoded=%+v", decoded)
	}
	if _, err := decodeConversationCursor(cursor, "tenant-x", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid cursor, got %v", err)
	}
}

func mustJSONBytes(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return raw
}

func mustJSONBytesFromValue(value any) []byte {
	if value == nil {
		return []byte("{}")
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return []byte("{}")
	}
	return raw
}

func mustJSONBytesFromValueOrNil(value any) []byte {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return raw
}

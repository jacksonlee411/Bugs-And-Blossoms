package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	cubeboxdomain "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/domain"
	cubeboxsqlc "github.com/jacksonlee411/Bugs-And-Blossoms/modules/cubebox/infrastructure/sqlc/gen"
)

type stubConversationReader struct {
	listRows        []cubeboxsqlc.IamCubeboxConversation
	listErr         error
	getRow          cubeboxsqlc.IamCubeboxConversation
	getErr          error
	turnRows        []cubeboxsqlc.IamCubeboxTurn
	turnErr         error
	transitionRows  []cubeboxsqlc.IamCubeboxStateTransition
	transitionErr   error
	syncFn          func(cubeboxdomain.Conversation) error
	blockingTasks   int64
	blockingErr     error
	deleteRows      int64
	deleteErr       error
	taskRow         cubeboxsqlc.IamCubeboxTask
	taskErr         error
	taskDispatchRow cubeboxsqlc.IamCubeboxTask
	taskDispatchErr error
	taskActorID     string
	taskActorErr    error
	submitTaskRow   cubeboxsqlc.IamCubeboxTask
	submitExisted   bool
	submitErr       error
	cancelTaskRow   cubeboxsqlc.IamCubeboxTask
	cancelAccepted  bool
	cancelErr       error
	dispatchRows    []cubeboxsqlc.IamCubeboxTaskDispatchOutbox
	dispatchErr     error
	updateTaskFn    func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error)
	insertEventFn   func(cubeboxdomain.TaskEventRecord) error
	updateOutboxFn  func(cubeboxdomain.TaskDispatchOutboxUpdate) error
}

func (s *stubConversationReader) ListConversations(context.Context, string, string, int32, time.Time, string) ([]cubeboxdomain.ConversationRecord, error) {
	return sqlcConversationRecords(s.listRows), s.listErr
}
func (s *stubConversationReader) GetConversation(context.Context, string, string) (cubeboxdomain.ConversationRecord, error) {
	return sqlcConversationRecord(s.getRow), s.getErr
}
func (s *stubConversationReader) ListConversationTurns(context.Context, string, string) ([]cubeboxdomain.ConversationTurnRecord, error) {
	return sqlcTurnRecords(s.turnRows), s.turnErr
}
func (s *stubConversationReader) ListConversationStateTransitions(context.Context, string, string) ([]cubeboxdomain.StateTransitionRecord, error) {
	return sqlcTransitionRecords(s.transitionRows), s.transitionErr
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
	return s.blockingTasks, s.blockingErr
}
func (s *stubConversationReader) DeleteConversation(context.Context, string, string) (int64, error) {
	return s.deleteRows, s.deleteErr
}
func (s *stubConversationReader) GetTask(context.Context, string, string) (cubeboxdomain.TaskRecord, error) {
	return sqlcTaskRecord(s.taskRow), s.taskErr
}
func (s *stubConversationReader) GetTaskForDispatch(context.Context, string, string) (cubeboxdomain.TaskRecord, error) {
	return sqlcTaskRecord(s.taskDispatchRow), s.taskDispatchErr
}
func (s *stubConversationReader) GetTaskActorID(context.Context, string, string) (string, error) {
	return s.taskActorID, s.taskActorErr
}
func (s *stubConversationReader) SubmitTask(context.Context, string, cubeboxdomain.TaskRecord) (cubeboxdomain.TaskRecord, bool, error) {
	return sqlcTaskRecord(s.submitTaskRow), s.submitExisted, s.submitErr
}
func (s *stubConversationReader) CancelTask(context.Context, string, string, time.Time) (cubeboxdomain.TaskRecord, bool, error) {
	return sqlcTaskRecord(s.cancelTaskRow), s.cancelAccepted, s.cancelErr
}
func (s *stubConversationReader) ListDispatchOutbox(context.Context, string, string, int32) ([]cubeboxdomain.TaskDispatchOutboxRecord, error) {
	return sqlcDispatchOutboxRecords(s.dispatchRows), s.dispatchErr
}
func (s *stubConversationReader) UpdateTaskState(_ context.Context, _ string, update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
	if s.updateTaskFn != nil {
		return s.updateTaskFn(update)
	}
	return sqlcTaskRecord(s.taskDispatchRow), nil
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
	createErr     error
	createTurn    *cubeboxdomain.Conversation
	createTurnErr error
	confirmTurn   *cubeboxdomain.Conversation
	confirmErr    error
	commitReceipt *cubeboxdomain.TaskReceipt
	submitReceipt *cubeboxdomain.TaskReceipt
	getTask       *cubeboxdomain.TaskDetail
	getTaskErr    error
	cancelResp    *cubeboxdomain.TaskCancelResponse
	reply         map[string]any
	replyErr      error
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
	return s.createConv, s.createErr
}
func (s stubLegacyFacade) CreateTurn(context.Context, string, Principal, string, string) (*cubeboxdomain.Conversation, error) {
	return s.createTurn, s.createTurnErr
}
func (s stubLegacyFacade) ConfirmTurn(context.Context, string, Principal, string, string, string) (*cubeboxdomain.Conversation, error) {
	return s.confirmTurn, s.confirmErr
}
func (s stubLegacyFacade) CommitTurn(context.Context, string, Principal, string, string) (*cubeboxdomain.TaskReceipt, error) {
	return s.commitReceipt, nil
}
func (s stubLegacyFacade) SubmitTask(context.Context, string, Principal, cubeboxdomain.TaskSubmitRequest) (*cubeboxdomain.TaskReceipt, error) {
	return s.submitReceipt, nil
}
func (s stubLegacyFacade) GetTask(context.Context, string, Principal, string) (*cubeboxdomain.TaskDetail, error) {
	return s.getTask, s.getTaskErr
}
func (s stubLegacyFacade) CancelTask(context.Context, string, Principal, string) (*cubeboxdomain.TaskCancelResponse, error) {
	return s.cancelResp, nil
}
func (s stubLegacyFacade) ExecuteTaskWorkflow(_ context.Context, tenantID string, principal Principal, conversation *cubeboxdomain.Conversation, turnID string) (TaskWorkflowExecutionResult, error) {
	if s.execFn != nil {
		return s.execFn(tenantID, principal, conversation, turnID)
	}
	return s.execTask, s.execErr
}
func (s stubLegacyFacade) RenderReply(context.Context, string, Principal, string, string, map[string]any) (map[string]any, error) {
	return s.reply, s.replyErr
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

type runtimeHealthyFileRepo struct{ err error }

func (s runtimeHealthyFileRepo) ListFiles(context.Context, string, string, int32) ([]FileMetadata, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) ListFileLinks(context.Context, string, string) ([]FileLinkRef, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) ListTenantFileLinks(context.Context, string) ([]FileLinkRef, error) {
	return nil, nil
}
func (s runtimeHealthyFileRepo) GetFile(context.Context, string, string) (FileMetadata, error) {
	return FileMetadata{}, nil
}
func (s runtimeHealthyFileRepo) ConversationExists(context.Context, string, string) (bool, error) {
	return false, nil
}
func (s runtimeHealthyFileRepo) CreateFile(context.Context, string, FileObject, string, string, string, time.Time) (FileMetadata, []FileLinkRef, error) {
	return FileMetadata{}, nil, nil
}
func (s runtimeHealthyFileRepo) CountFileLinks(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s runtimeHealthyFileRepo) DeleteFile(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (s runtimeHealthyFileRepo) InsertFileCleanupJob(context.Context, string, FileCleanupJob, time.Time) error {
	return nil
}
func (s runtimeHealthyFileRepo) Healthy(context.Context, string) error { return s.err }

type runtimeHealthyObjectStore struct{ err error }

func (s runtimeHealthyObjectStore) SaveObject(context.Context, string, string, string, string, io.Reader) (FileObject, error) {
	return FileObject{}, nil
}
func (s runtimeHealthyObjectStore) DeleteObject(context.Context, string) error { return nil }
func (s runtimeHealthyObjectStore) Healthy(context.Context) error              { return s.err }

type stubFacadeFileStore struct {
	listRecords []FileRecord
	saveRecord  FileRecord
	saveBody    string
	deleteOK    bool
	listErr     error
	saveErr     error
	deleteErr   error
}

func (s *stubFacadeFileStore) List(context.Context, string, string) ([]FileRecord, error) {
	return s.listRecords, s.listErr
}

func (s *stubFacadeFileStore) Save(_ context.Context, _ string, _ string, _ string, _ string, _ string, body io.Reader) (FileRecord, error) {
	if body != nil {
		raw, _ := io.ReadAll(body)
		s.saveBody = string(raw)
	}
	return s.saveRecord, s.saveErr
}

func (s *stubFacadeFileStore) Delete(context.Context, string, string) (bool, error) {
	return s.deleteOK, s.deleteErr
}

func (s *stubFacadeFileStore) Healthy(context.Context) error { return nil }

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

func TestFacadeListConversationsSupportsCursorClampAndFallback(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 30, 0, 0, time.UTC)
	reader := &stubConversationReader{
		listRows: []cubeboxsqlc.IamCubeboxConversation{
			{
				ConversationID: "conv_3",
				ActorID:        "actor-1",
				State:          "confirmed",
				UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
			},
			{
				ConversationID: "conv_2",
				ActorID:        "actor-1",
				State:          "validated",
				UpdatedAt:      pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			},
		},
		getErr: errors.New("missing detail"),
	}
	facade := NewFacade(reader, nil, nil, nil)

	items, next, err := facade.ListConversations(context.Background(), "tenant-1", "actor-1", 1, "")
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if len(items) != 1 || items[0].ConversationID != "conv_3" || items[0].LastTurn != nil {
		t.Fatalf("items=%+v", items)
	}
	if next == "" {
		t.Fatal("expected next cursor")
	}

	nextItems, nextNext, err := facade.ListConversations(context.Background(), "tenant-1", "actor-1", 999, next)
	if err != nil {
		t.Fatalf("list conversations with cursor: %v", err)
	}
	if len(nextItems) != 2 || nextNext != "" {
		t.Fatalf("nextItems=%+v nextNext=%q", nextItems, nextNext)
	}

	legacy := stubLegacyFacade{
		listItems: []cubeboxdomain.ConversationListItem{{ConversationID: "fallback_conv"}},
		listNext:  "fallback_next",
	}
	items, next, err = NewFacade(nil, nil, nil, legacy).ListConversations(context.Background(), "tenant-1", "actor-1", 10, "")
	if err != nil {
		t.Fatalf("legacy list conversations: %v", err)
	}
	if len(items) != 1 || items[0].ConversationID != "fallback_conv" || next != "fallback_next" {
		t.Fatalf("legacy items=%+v next=%q", items, next)
	}
	if _, _, err := NewFacade(nil, nil, nil, legacy).ListConversations(context.Background(), "tenant-1", "actor-1", 10, "bad-cursor"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid cursor before legacy fallback, got %v", err)
	}

	if _, _, err := facade.ListConversations(context.Background(), "tenant-1", "actor-1", 10, "bad-cursor"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid cursor, got %v", err)
	}

	items, next, err = NewFacade(nil, nil, nil, nil).ListConversations(context.Background(), "tenant-1", "actor-1", 10, "")
	if err != nil || items != nil || next != "" {
		t.Fatalf("expected nil legacy list result, got items=%+v next=%q err=%v", items, next, err)
	}

	if _, _, err := NewFacade(&stubConversationReader{listErr: errors.New("list failed")}, nil, nil, nil).
		ListConversations(context.Background(), "tenant-1", "actor-1", 10, ""); err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("expected reader list error, got %v", err)
	}

	t.Run("default page size and overflow rows are truncated", func(t *testing.T) {
		reader := &stubConversationReader{
			listRows: []cubeboxsqlc.IamCubeboxConversation{
				{ConversationID: "conv_1", ActorID: "actor-1", UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}},
				{ConversationID: "conv_2", ActorID: "actor-1", UpdatedAt: pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}},
				{ConversationID: "conv_3", ActorID: "actor-1", UpdatedAt: pgtype.Timestamptz{Time: now.Add(-2 * time.Minute), Valid: true}},
			},
			getErr: errors.New("detail missing"),
		}
		items, next, err := NewFacade(reader, nil, nil, nil).ListConversations(context.Background(), "tenant-1", "actor-1", 1, "")
		if err != nil {
			t.Fatalf("list conversations with truncated overflow: %v", err)
		}
		if len(items) != 1 || items[0].ConversationID != "conv_1" || next == "" {
			t.Fatalf("items=%+v next=%q", items, next)
		}
		items, next, err = NewFacade(reader, nil, nil, nil).ListConversations(context.Background(), "tenant-1", "actor-1", 0, "")
		if err != nil {
			t.Fatalf("list conversations with default page size: %v", err)
		}
		if len(items) != 3 || next != "" {
			t.Fatalf("items=%+v next=%q", items, next)
		}
	})
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

func TestFacadeLoadConversationFormalBranches(t *testing.T) {
	t.Run("formal actor mismatch returns forbidden", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow: cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-2"},
		}
		conv, err := NewFacade(reader, nil, nil, nil).loadConversation(context.Background(), "tenant-1", "actor-1", "conv_1", false)
		if !errors.Is(err, ErrConversationForbidden) || conv != nil {
			t.Fatalf("conversation=%+v err=%v", conv, err)
		}
	})

	t.Run("formal turns error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow:  cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
			turnErr: errors.New("turn load failed"),
		}
		conv, err := NewFacade(reader, nil, nil, nil).loadConversation(context.Background(), "tenant-1", "actor-1", "conv_1", false)
		if err == nil || !strings.Contains(err.Error(), "turn load failed") || conv != nil {
			t.Fatalf("conversation=%+v err=%v", conv, err)
		}
	})

	t.Run("formal transitions error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow:        cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
			transitionErr: errors.New("transition load failed"),
		}
		conv, err := NewFacade(reader, nil, nil, nil).loadConversation(context.Background(), "tenant-1", "actor-1", "conv_1", false)
		if err == nil || !strings.Contains(err.Error(), "transition load failed") || conv != nil {
			t.Fatalf("conversation=%+v err=%v", conv, err)
		}
	})

	t.Run("reader miss without legacy returns not found", func(t *testing.T) {
		conv, err := NewFacade(&stubConversationReader{getErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
			getConv: &cubeboxdomain.Conversation{ConversationID: "fallback_conv"},
		}).loadConversation(context.Background(), "tenant-1", "actor-1", "conv_1", false)
		if !errors.Is(err, ErrConversationNotFound) || conv != nil {
			t.Fatalf("conversation=%+v err=%v", conv, err)
		}
	})

	t.Run("reader miss with allowed legacy falls back", func(t *testing.T) {
		conv, err := NewFacade(&stubConversationReader{getErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
			getConv: &cubeboxdomain.Conversation{ConversationID: "fallback_conv"},
		}).loadConversation(context.Background(), "tenant-1", "actor-1", "conv_1", true)
		if err != nil || conv == nil || conv.ConversationID != "fallback_conv" {
			t.Fatalf("conversation=%+v err=%v", conv, err)
		}
	})
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

func TestFacadeDeleteConversationErrorBranches(t *testing.T) {
	t.Run("conversation lookup miss returns not found", func(t *testing.T) {
		reader := &stubConversationReader{getErr: errors.New("lookup failed")}
		err := NewFacade(reader, nil, nil, nil).DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected conversation not found, got %v", err)
		}
	})

	t.Run("reader missing after conversation lookup is tolerated", func(t *testing.T) {
		err := NewFacade(nil, nil, nil, stubLegacyFacade{
			getConv: &cubeboxdomain.Conversation{ConversationID: "conv_1"},
		}).DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
		if err != nil {
			t.Fatalf("expected nil delete with reader missing, got %v", err)
		}
	})

	t.Run("count blocking tasks error", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow:      cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
			blockingErr: errors.New("count failed"),
		}
		err := NewFacade(reader, nil, nil, nil).DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
		if err == nil || !strings.Contains(err.Error(), "count failed") {
			t.Fatalf("expected count error, got %v", err)
		}
	})

	t.Run("delete returns not found", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow:     cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
			deleteRows: 0,
		}
		err := NewFacade(reader, nil, nil, nil).DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
		if !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("delete error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			getRow:    cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1"},
			deleteErr: errors.New("delete failed"),
		}
		err := NewFacade(reader, nil, nil, nil).DeleteConversation(context.Background(), "tenant-1", "actor-1", "conv_1")
		if err == nil || !strings.Contains(err.Error(), "delete failed") {
			t.Fatalf("expected delete error, got %v", err)
		}
	})
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

func TestFacadeCreateConversationNilLegacyAndSyncErrors(t *testing.T) {
	conv, err := NewFacade(nil, nil, nil, nil).CreateConversation(context.Background(), "tenant-1", Principal{ID: "actor-1"})
	if err != nil {
		t.Fatalf("nil legacy create conversation: %v", err)
	}
	if conv != nil {
		t.Fatalf("conversation=%+v", conv)
	}

	if _, err := NewFacade(nil, nil, nil, stubLegacyFacade{createErr: errors.New("legacy failed")}).
		CreateConversation(context.Background(), "tenant-1", Principal{ID: "actor-1"}); err == nil || !strings.Contains(err.Error(), "legacy failed") {
		t.Fatalf("expected legacy error, got %v", err)
	}

	reader := &stubConversationReader{
		syncFn: func(cubeboxdomain.Conversation) error {
			return errors.New("sync failed")
		},
	}
	_, err = NewFacade(reader, nil, nil, stubLegacyFacade{
		createConv: &cubeboxdomain.Conversation{ConversationID: "conv_sync", ActorID: "actor-1"},
	}).CreateConversation(context.Background(), "tenant-1", Principal{ID: "actor-1"})
	if err == nil || !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("expected sync error, got %v", err)
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

func TestFacadeCreateTurnNilLegacyAndSyncErrors(t *testing.T) {
	conv, err := NewFacade(nil, nil, nil, nil).CreateTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "hello")
	if err != nil {
		t.Fatalf("nil legacy create turn: %v", err)
	}
	if conv != nil {
		t.Fatalf("conversation=%+v", conv)
	}

	if _, err := NewFacade(nil, nil, nil, stubLegacyFacade{createTurnErr: errors.New("legacy failed")}).
		CreateTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "hello"); err == nil || !strings.Contains(err.Error(), "legacy failed") {
		t.Fatalf("expected legacy error, got %v", err)
	}

	reader := &stubConversationReader{
		syncFn: func(cubeboxdomain.Conversation) error {
			return errors.New("sync failed")
		},
	}
	_, err = NewFacade(reader, nil, nil, stubLegacyFacade{
		createTurn: &cubeboxdomain.Conversation{ConversationID: "conv_sync", ActorID: "actor-1"},
	}).CreateTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_sync", "hello")
	if err == nil || !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("expected sync error, got %v", err)
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

func TestFacadeConfirmTurnNilLegacyAndSyncErrors(t *testing.T) {
	conv, err := NewFacade(nil, nil, nil, nil).ConfirmTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", "cand_1")
	if err != nil {
		t.Fatalf("nil legacy confirm turn: %v", err)
	}
	if conv != nil {
		t.Fatalf("conversation=%+v", conv)
	}

	if _, err := NewFacade(nil, nil, nil, stubLegacyFacade{confirmErr: errors.New("legacy failed")}).
		ConfirmTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", "cand_1"); err == nil || !strings.Contains(err.Error(), "legacy failed") {
		t.Fatalf("expected legacy error, got %v", err)
	}

	reader := &stubConversationReader{
		syncFn: func(cubeboxdomain.Conversation) error {
			return errors.New("sync failed")
		},
	}
	_, err = NewFacade(reader, nil, nil, stubLegacyFacade{
		confirmTurn: &cubeboxdomain.Conversation{ConversationID: "conv_sync", ActorID: "actor-1"},
	}).ConfirmTurn(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_sync", "turn_1", "cand_1")
	if err == nil || !strings.Contains(err.Error(), "sync failed") {
		t.Fatalf("expected sync error, got %v", err)
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

func TestFacadeCommitTurnRejectsInvalidFormalState(t *testing.T) {
	now := time.Date(2026, 4, 15, 16, 30, 0, 0, time.UTC)
	baseReader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
		},
	}

	t.Run("actor mismatch", func(t *testing.T) {
		reader := *baseReader
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-2", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrAuthSnapshotExpired) {
			t.Fatalf("expected auth snapshot expired, got %v", err)
		}
	})

	t.Run("role drift", func(t *testing.T) {
		reader := *baseReader
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "viewer"}, "conv_1", "turn_1"); !errors.Is(err, ErrRoleDriftDetected) {
			t.Fatalf("expected role drift, got %v", err)
		}
	})

	t.Run("turn missing", func(t *testing.T) {
		reader := *baseReader
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_missing"); !errors.Is(err, ErrTurnNotFound) {
			t.Fatalf("expected turn not found, got %v", err)
		}
	})

	t.Run("validated turn requires confirmation", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:    "turn_1",
			State:     "validated",
			PlanJson:  mustJSONBytes(t, map[string]any{"confirm_ttl_seconds": 300}),
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}}
		facade := NewFacade(&reader, nil, nil, nil)
		facade.nowFn = func() time.Time { return now.Add(time.Minute) }
		if _, err := facade.CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrConfirmationRequired) {
			t.Fatalf("expected confirmation required, got %v", err)
		}
	})

	t.Run("validated turn expired", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:    "turn_1",
			State:     "validated",
			PlanJson:  mustJSONBytes(t, map[string]any{"confirm_ttl_seconds": 1}),
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}}
		facade := NewFacade(&reader, nil, nil, nil)
		facade.nowFn = func() time.Time { return now.Add(2 * time.Second) }
		if _, err := facade.CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrConfirmationExpired) {
			t.Fatalf("expected confirmation expired, got %v", err)
		}
	})

	t.Run("committed turn invalid", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{TurnID: "turn_1", State: "committed"}}
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrTaskStateInvalid) {
			t.Fatalf("expected task state invalid, got %v", err)
		}
	})

	t.Run("expired turn invalid", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{TurnID: "turn_1", State: "expired"}}
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrConversationStateInvalid) {
			t.Fatalf("expected conversation state invalid, got %v", err)
		}
	})

	t.Run("unknown turn state invalid", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{TurnID: "turn_1", State: "draft"}}
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); !errors.Is(err, ErrConversationStateInvalid) {
			t.Fatalf("expected conversation state invalid, got %v", err)
		}
	})
}

func TestFacadeCommitTurnAdditionalBranches(t *testing.T) {
	now := time.Date(2026, 4, 15, 16, 40, 0, 0, time.UTC)
	baseReader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
			ActorRole:      "tenant-admin",
		},
	}

	t.Run("formal turn lookup error is returned", func(t *testing.T) {
		reader := *baseReader
		reader.turnErr = errors.New("turn list failed")
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "turn list failed") {
			t.Fatalf("expected turn list error, got %v", err)
		}
	})

	t.Run("formal commit request build error is returned", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:     "turn_1",
			State:      "confirmed",
			IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		}}
		if _, err := NewFacade(&reader, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1"); err == nil || !strings.Contains(err.Error(), "request_id required") {
			t.Fatalf("expected request_id error, got %v", err)
		}
	})

	t.Run("formal miss without legacy returns nil receipt", func(t *testing.T) {
		receipt, err := NewFacade(&stubConversationReader{getErr: errors.New("missing")}, nil, nil, nil).CommitTurn(context.Background(), "tenant-1", Principal{ID: "actor-1", RoleSlug: "tenant-admin"}, "conv_1", "turn_1")
		if err != nil || receipt != nil {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})
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

func TestFacadeGetTaskFallbackAndErrorBranches(t *testing.T) {
	taskID := uuid.MustParse("23232323-2323-2323-2323-232323232323")

	t.Run("formal actor lookup error", func(t *testing.T) {
		reader := &stubConversationReader{
			taskRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:    taskTypeAsyncPlan,
				Status:      taskStatusQueued,
				WorkflowID:  "wf",
				RequestID:   "req",
				SubmittedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			taskActorErr: errors.New("actor lookup failed"),
		}
		if _, err := NewFacade(reader, nil, nil, nil).GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String()); err == nil || !strings.Contains(err.Error(), "actor lookup failed") {
			t.Fatalf("expected actor lookup error, got %v", err)
		}
	})

	t.Run("formal missing falls back to legacy", func(t *testing.T) {
		task, err := NewFacade(&stubConversationReader{taskErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
			getTask: &cubeboxdomain.TaskDetail{TaskID: "legacy-task"},
		}).GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if err != nil {
			t.Fatalf("legacy get task: %v", err)
		}
		if task == nil || task.TaskID != "legacy-task" {
			t.Fatalf("task=%+v", task)
		}
	})

	t.Run("legacy error is returned", func(t *testing.T) {
		if _, err := NewFacade(&stubConversationReader{taskErr: errors.New("missing")}, nil, nil, stubLegacyFacade{
			getTaskErr: ErrTaskNotFound,
		}).GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String()); !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("expected task not found, got %v", err)
		}
	})

	t.Run("no legacy returns not found", func(t *testing.T) {
		if _, err := NewFacade(nil, nil, nil, nil).GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String()); !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("expected task not found, got %v", err)
		}
	})
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

func TestFacadeRenderReplyBranches(t *testing.T) {
	reply, err := NewFacade(nil, nil, nil, nil).RenderReply(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", map[string]any{"locale": "zh"})
	if err != nil {
		t.Fatalf("nil legacy render reply: %v", err)
	}
	if reply != nil {
		t.Fatalf("reply=%+v", reply)
	}

	reply, err = NewFacade(nil, nil, nil, stubLegacyFacade{
		reply: map[string]any{"text": "ok"},
	}).RenderReply(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", map[string]any{"locale": "zh"})
	if err != nil {
		t.Fatalf("render reply: %v", err)
	}
	if reply["text"] != "ok" {
		t.Fatalf("reply=%+v", reply)
	}

	if _, err := NewFacade(nil, nil, nil, stubLegacyFacade{replyErr: errors.New("reply failed")}).
		RenderReply(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", nil); err == nil || !strings.Contains(err.Error(), "reply failed") {
		t.Fatalf("expected reply error, got %v", err)
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

func TestFacadeSubmitTaskErrorAndFallbackPaths(t *testing.T) {
	now := time.Date(2026, 4, 15, 13, 30, 0, 0, time.UTC)
	req := cubeboxdomain.TaskSubmitRequest{
		ConversationID: "conv_1",
		TurnID:         "turn_1",
		TaskType:       taskTypeAsyncPlan,
		RequestID:      "req_1",
		ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:     "intent.v1",
			CompilerContractVersion: "compiler.v1",
			CapabilityMapVersion:    "cap.v1",
			SkillManifestDigest:     "skill",
			ContextHash:             "ctx",
			IntentHash:              "intent",
			PlanHash:                "plan",
		},
	}
	baseReader := &stubConversationReader{
		getRow: cubeboxsqlc.IamCubeboxConversation{
			ConversationID: "conv_1",
			ActorID:        "actor-1",
		},
		turnRows: []cubeboxsqlc.IamCubeboxTurn{{
			TurnID:     "turn_1",
			IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
		}},
	}

	t.Run("conversation missing", func(t *testing.T) {
		reader := *baseReader
		reader.getErr = errors.New("missing")
		if _, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req); !errors.Is(err, ErrConversationNotFound) {
			t.Fatalf("expected conversation not found, got %v", err)
		}
	})

	t.Run("conversation forbidden", func(t *testing.T) {
		reader := *baseReader
		if _, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-2"}, req); !errors.Is(err, ErrConversationForbidden) {
			t.Fatalf("expected conversation forbidden, got %v", err)
		}
	})

	t.Run("turn missing", func(t *testing.T) {
		reader := *baseReader
		reader.turnRows = nil
		if _, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req); !errors.Is(err, ErrTurnNotFound) {
			t.Fatalf("expected turn not found, got %v", err)
		}
	})

	t.Run("snapshot mismatch", func(t *testing.T) {
		reader := *baseReader
		badReq := req
		badReq.ContractSnapshot.PlanHash = "other-plan"
		if _, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, badReq); !errors.Is(err, ErrPlanContractMismatch) {
			t.Fatalf("expected contract mismatch, got %v", err)
		}
	})

	t.Run("idempotency conflict", func(t *testing.T) {
		reader := *baseReader
		reader.submitExisted = true
		reader.submitTaskRow = cubeboxsqlc.IamCubeboxTask{
			TaskID:      pgtype.UUID{Bytes: uuid.MustParse("45454545-4545-4545-4545-454545454545"), Valid: true},
			TaskType:    taskTypeAsyncPlan,
			Status:      taskStatusQueued,
			RequestHash: "different-hash",
			WorkflowID:  "wf",
			SubmittedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		}
		facade := NewFacade(&reader, nil, nil, nil)
		facade.nowFn = func() time.Time { return now }
		if _, err := facade.SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("expected idempotency conflict, got %v", err)
		}
	})

	t.Run("falls back to legacy", func(t *testing.T) {
		receipt, err := NewFacade(nil, nil, nil, stubLegacyFacade{
			submitReceipt: &cubeboxdomain.TaskReceipt{TaskID: "legacy-task"},
		}).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req)
		if err != nil {
			t.Fatalf("legacy submit task: %v", err)
		}
		if receipt == nil || receipt.TaskID != "legacy-task" {
			t.Fatalf("receipt=%+v", receipt)
		}
	})

	t.Run("formal turn lookup error is returned", func(t *testing.T) {
		reader := *baseReader
		reader.turnErr = errors.New("turn list failed")
		if _, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req); err == nil || !strings.Contains(err.Error(), "turn list failed") {
			t.Fatalf("expected turn list error, got %v", err)
		}
	})

	t.Run("validation error is returned before reader path", func(t *testing.T) {
		badReq := req
		badReq.RequestID = ""
		if _, err := NewFacade(baseReader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, badReq); err == nil || !strings.Contains(err.Error(), "request_id required") {
			t.Fatalf("expected request_id required, got %v", err)
		}
	})

	t.Run("formal submit error without legacy returns nil receipt", func(t *testing.T) {
		reader := *baseReader
		reader.submitErr = errors.New("submit failed")
		receipt, err := NewFacade(&reader, nil, nil, nil).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req)
		if err != nil || receipt != nil {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})

	t.Run("formal submit error falls back to legacy", func(t *testing.T) {
		reader := *baseReader
		reader.submitErr = errors.New("submit failed")
		receipt, err := NewFacade(&reader, nil, nil, stubLegacyFacade{
			submitReceipt: &cubeboxdomain.TaskReceipt{TaskID: "legacy-submit"},
		}).SubmitTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, req)
		if err != nil || receipt == nil || receipt.TaskID != "legacy-submit" {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
	})
}

func TestFacadeCancelTaskRejectsOtherActor(t *testing.T) {
	reader := &stubConversationReader{taskActorID: "actor-2"}
	facade := NewFacade(reader, nil, nil, nil)
	if _, err := facade.CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "55555555-5555-5555-5555-555555555555"); !errors.Is(err, ErrConversationForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestFacadeCancelTaskAcceptsFormalAndFallsBack(t *testing.T) {
	taskID := uuid.MustParse("56565656-5656-5656-5656-565656565656")
	now := time.Date(2026, 4, 15, 14, 5, 0, 0, time.UTC)

	t.Run("formal accepted", func(t *testing.T) {
		reader := &stubConversationReader{
			taskActorID:    "actor-1",
			cancelAccepted: true,
			cancelTaskRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:    taskTypeAsyncPlan,
				Status:      taskStatusCanceled,
				WorkflowID:  "wf",
				RequestID:   "req_1",
				SubmittedAt: pgtype.Timestamptz{Time: now, Valid: true},
				UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		facade.nowFn = func() time.Time { return now }
		resp, err := facade.CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if err != nil {
			t.Fatalf("cancel task: %v", err)
		}
		if resp == nil || !resp.CancelAccepted || resp.TaskID != taskID.String() {
			t.Fatalf("resp=%+v", resp)
		}
	})

	t.Run("task missing falls back to legacy", func(t *testing.T) {
		resp, err := NewFacade(&stubConversationReader{taskActorID: "actor-1", cancelErr: errors.New("cancel failed")}, nil, nil, stubLegacyFacade{
			cancelResp: &cubeboxdomain.TaskCancelResponse{TaskDetail: cubeboxdomain.TaskDetail{TaskID: "legacy-task"}},
		}).CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if err != nil {
			t.Fatalf("legacy cancel task: %v", err)
		}
		if resp == nil || resp.TaskID != "legacy-task" {
			t.Fatalf("resp=%+v", resp)
		}
	})

	t.Run("actor lookup error returns task not found", func(t *testing.T) {
		_, err := NewFacade(&stubConversationReader{taskActorErr: errors.New("actor lookup failed")}, nil, nil, nil).CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if !errors.Is(err, ErrTaskNotFound) {
			t.Fatalf("expected task not found, got %v", err)
		}
	})

	t.Run("cancel error without legacy returns nil response", func(t *testing.T) {
		resp, err := NewFacade(&stubConversationReader{taskActorID: "actor-1", cancelErr: errors.New("cancel failed")}, nil, nil, nil).CancelTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if err != nil || resp != nil {
			t.Fatalf("resp=%+v err=%v", resp, err)
		}
	})
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
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
		return sqlcTaskRecord(taskRow), nil
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
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
		return sqlcTaskRecord(taskRow), nil
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
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
		return sqlcTaskRecord(taskRow), nil
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
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
		return sqlcTaskRecord(taskRow), nil
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
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
		return sqlcTaskRecord(taskRow), nil
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
	reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
		taskRow.Status = update.Status
		taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
		taskRow.DispatchStatus = update.DispatchStatus
		taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
		if update.CompletedAt != nil {
			taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
		}
		reader.taskRow = taskRow
		reader.taskDispatchRow = taskRow
		return sqlcTaskRecord(taskRow), nil
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
	}, NewFileService(runtimeHealthyFileRepo{}, runtimeHealthyObjectStore{}), nil)
	facade.nowFn = func() time.Time { return time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) }
	status := facade.RuntimeStatus(context.Background())
	if status.Status != healthDegraded {
		t.Fatalf("status=%+v", status)
	}
	if status.FileStore.Healthy != healthHealthy {
		t.Fatalf("status=%+v", status)
	}
}

func TestFacadeRuntimeStatusUnavailableBranches(t *testing.T) {
	t.Run("runtime and file store missing", func(t *testing.T) {
		facade := NewFacade(nil, nil, nil, nil)
		facade.nowFn = func() time.Time { return time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) }
		status := facade.RuntimeStatus(context.Background())
		if status.Status != healthUnavailable ||
			status.Backend.Reason != "assistant_service_missing" ||
			status.FileStore.Reason != "file_store_missing" ||
			status.KnowledgeRuntime.Reason != "knowledge_runtime_missing" ||
			status.ModelGateway.Reason != "model_gateway_missing" {
			t.Fatalf("status=%+v", status)
		}
	})

	t.Run("file store unhealthy and empty runtime components", func(t *testing.T) {
		facade := NewFacade(nil, stubRuntimeProbe{
			backend: cubeboxdomain.RuntimeComponentStatus{Healthy: healthHealthy},
		}, NewFileService(runtimeHealthyFileRepo{err: errors.New("repo down")}, runtimeHealthyObjectStore{}), nil)
		facade.nowFn = func() time.Time { return time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC) }
		status := facade.RuntimeStatus(context.Background())
		if status.Status != healthUnavailable ||
			status.FileStore.Reason != "file_store_unavailable" ||
			status.KnowledgeRuntime.Reason != "knowledge_runtime_missing" ||
			status.ModelGateway.Reason != "model_gateway_missing" {
			t.Fatalf("status=%+v", status)
		}
	})
}

func TestTaskSubmitAndSnapshotHelpers(t *testing.T) {
	t.Run("validate task submit request", func(t *testing.T) {
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{}); err == nil || !strings.Contains(err.Error(), "conversation_id required") {
			t.Fatalf("expected conversation_id required, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{ConversationID: "conv"}); err == nil || !strings.Contains(err.Error(), "turn_id required") {
			t.Fatalf("expected turn_id required, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{ConversationID: "conv", TurnID: "turn"}); err == nil || !strings.Contains(err.Error(), "task_type required") {
			t.Fatalf("expected task_type required, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{ConversationID: "conv", TurnID: "turn", TaskType: "bad", RequestID: "req"}); err == nil || !strings.Contains(err.Error(), "task_type invalid") {
			t.Fatalf("expected task_type invalid, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{ConversationID: "conv", TurnID: "turn", TaskType: taskTypeAsyncPlan}); err == nil || !strings.Contains(err.Error(), "request_id required") {
			t.Fatalf("expected request_id required, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{
			ConversationID: "conv",
			TurnID:         "turn",
			TaskType:       taskTypeAsyncPlan,
			RequestID:      "req",
			ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
				PlanHash:                "plan",
				PolicyContextDigest:     "policy",
			},
		}); err == nil || !strings.Contains(err.Error(), "contract_snapshot incomplete") {
			t.Fatalf("expected incomplete contract snapshot, got %v", err)
		}
		if err := validateTaskSubmitRequest(cubeboxdomain.TaskSubmitRequest{
			ConversationID: "conv",
			TurnID:         "turn",
			TaskType:       taskTypeAsyncPlan,
			RequestID:      "req",
			ContractSnapshot: cubeboxdomain.TaskContractSnapshot{
				IntentSchemaVersion:     "intent.v1",
				CompilerContractVersion: "compiler.v1",
				CapabilityMapVersion:    "cap.v1",
				SkillManifestDigest:     "skill",
				ContextHash:             "ctx",
				IntentHash:              "intent",
			},
		}); err == nil || !strings.Contains(err.Error(), "contract_snapshot incomplete") {
			t.Fatalf("expected missing plan hash to be incomplete, got %v", err)
		}
	})

	t.Run("task snapshot compatibility", func(t *testing.T) {
		current := cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:     "intent.v1",
			CompilerContractVersion: "compiler.v1",
			CapabilityMapVersion:    "cap.v1",
			SkillManifestDigest:     "skill",
			ContextHash:             "ctx",
			IntentHash:              "intent",
			PlanHash:                "plan",
			RouteCatalogVersion:     "route.v1",
		}
		stored := current
		if !taskSnapshotCompatible(current, stored) {
			t.Fatal("expected compatible snapshots")
		}
		stored.RouteCatalogVersion = "route.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected route catalog mismatch")
		}
		stored = current
		stored.PlanHash = "plan-x"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected plan hash mismatch")
		}

		stored = current
		stored.KnowledgeSnapshotDigest = "knowledge.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected knowledge snapshot mismatch")
		}

		stored = current
		stored.PolicyContextDigest = "policy.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected policy context mismatch")
		}

		stored = current
		stored.MutationPolicyVersion = "mutation.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected mutation policy mismatch")
		}

		stored = current
		stored.ResolverContractVersion = "resolver.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected resolver contract mismatch")
		}

		stored = current
		stored.ContextTemplateVersion = "ctx-template.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected context template mismatch")
		}

		stored = current
		stored.ReplyGuidanceVersion = "reply.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected reply guidance mismatch")
		}

		stored = current
		stored.EffectivePolicyVersion = "policy.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected effective policy mismatch")
		}

		stored = current
		stored.ResolvedSetID = "S2602"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected resolved setid mismatch")
		}

		stored = current
		stored.SetIDSource = "tenant_default"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected setid source mismatch")
		}

		stored = current
		stored.PrecheckProjectionDigest = "projection.v2"
		if taskSnapshotCompatible(current, stored) {
			t.Fatal("expected projection digest mismatch")
		}
	})

	t.Run("build commit task submit request", func(t *testing.T) {
		turn := cubeboxdomain.ConversationTurnRecord{
			TurnID:     "turn_1",
			RequestID:  "req_1",
			TraceID:    "trace_1",
			IntentJSON: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
			PlanJSON:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
			DryRunJSON: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			UpdatedAt:  time.Now().UTC(),
			CreatedAt:  time.Now().UTC(),
		}
		req, err := buildCommitTaskSubmitRequest("conv_1", turn)
		if err != nil {
			t.Fatalf("build commit task submit request: %v", err)
		}
		if req.TaskType != taskTypeAsyncPlan || req.RequestID != "req_1" || req.ContractSnapshot.PlanHash != "plan" {
			t.Fatalf("req=%+v", req)
		}

		turn.DryRunJSON = mustJSONBytes(t, map[string]any{})
		if _, err := buildCommitTaskSubmitRequest("conv_1", turn); !errors.Is(err, ErrPlanDeterminismViolation) {
			t.Fatalf("expected plan determinism violation, got %v", err)
		}

		turn.DryRunJSON = mustJSONBytes(t, map[string]any{"plan_hash": "plan"})
		turn.RequestID = ""
		if _, err := buildCommitTaskSubmitRequest("conv_1", turn); err == nil || !strings.Contains(err.Error(), "request_id required") {
			t.Fatalf("expected request_id required, got %v", err)
		}
	})

	t.Run("build commit task submit request rejects invalid snapshot json", func(t *testing.T) {
		turn := cubeboxdomain.ConversationTurnRecord{
			TurnID:     "turn_bad",
			RequestID:  "req_bad",
			IntentJSON: []byte(`{bad`),
			CreatedAt:  time.Now().UTC(),
		}
		if _, err := buildCommitTaskSubmitRequest("conv_1", turn); err == nil {
			t.Fatal("expected invalid snapshot json error")
		}
	})
}

func TestTaskSnapshotProjectionAndValidationBranches(t *testing.T) {
	t.Run("policy contract values from orgunit version projection", func(t *testing.T) {
		values, ok := policyContractValuesFromDryRun(turnDryRunPayload{
			OrgUnitVersionProjection: &orgUnitVersionProjectionWrap{
				PolicyContext: policyContextPayload{PolicyContextDigest: "policy-digest"},
				Projection: projectionPayload{
					EffectivePolicyVersion: "epv1",
					ResolvedSetID:          "S2601",
					SetIDSource:            "custom",
					ProjectionDigest:       "projection-digest",
					MutationPolicyVersion:  "mutation.v1",
				},
			},
		})
		if !ok || values.PolicyContextDigest != "policy-digest" || values.PrecheckProjectionDigest != "projection-digest" {
			t.Fatalf("values=%+v ok=%v", values, ok)
		}
	})

	t.Run("policy contract values absent", func(t *testing.T) {
		if values, ok := policyContractValuesFromDryRun(turnDryRunPayload{}); ok || values != (cubeboxdomain.TaskContractSnapshot{}) {
			t.Fatalf("values=%+v ok=%v", values, ok)
		}
	})

	t.Run("route audit versions require complete plan metadata", func(t *testing.T) {
		consistent := turnRouteAuditVersionsConsistent(
			cubeboxdomain.ConversationTurnRecord{RouteDecisionJSON: mustJSONBytes(t, map[string]any{"resolver_contract_version": "resolver.v1"})},
			turnPlanPayload{
				KnowledgeSnapshotDigest: "knowledge.v1",
				RouteCatalogVersion:     "route.v1",
				ResolverContractVersion: "resolver.v1",
			},
			routeDecisionPayload{
				KnowledgeSnapshotDigest: "knowledge.v1",
				RouteCatalogVersion:     "route.v1",
				ResolverContractVersion: "resolver.v1",
			},
		)
		if consistent {
			t.Fatal("expected inconsistent route audit versions when context/reply metadata missing")
		}
	})

	t.Run("task snapshot from turn detects route audit mismatch", func(t *testing.T) {
		turn := cubeboxdomain.ConversationTurnRecord{
			IntentJSON: mustJSONBytes(t, map[string]any{
				"action":                intentPlanOnly,
				"intent_schema_version": "intent.v1",
				"context_hash":          "ctx",
				"intent_hash":           "intent",
			}),
			PlanJSON: mustJSONBytes(t, map[string]any{
				"compiler_contract_version": "compiler.v1",
				"capability_map_version":    "cap.v1",
				"skill_manifest_digest":     "skill",
				"knowledge_snapshot_digest": "knowledge.v1",
				"route_catalog_version":     "route.v1",
				"resolver_contract_version": "resolver.v1",
				"context_template_version":  "ctx-template.v1",
				"reply_guidance_version":    "reply.v1",
			}),
			DryRunJSON: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			RouteDecisionJSON: mustJSONBytes(t, map[string]any{
				"knowledge_snapshot_digest": "knowledge.v1",
				"route_catalog_version":     "route.v2",
				"resolver_contract_version": "resolver.v1",
			}),
		}
		if _, err := taskSnapshotFromTurn(turn); !errors.Is(err, ErrPlanContractMismatch) {
			t.Fatalf("expected plan contract mismatch, got %v", err)
		}
	})

	t.Run("task snapshot from turn requires policy projection for orgunit actions", func(t *testing.T) {
		turn := cubeboxdomain.ConversationTurnRecord{
			IntentJSON: mustJSONBytes(t, map[string]any{
				"action":                intentCreateOrgUnit,
				"intent_schema_version": "intent.v1",
				"context_hash":          "ctx",
				"intent_hash":           "intent",
			}),
			PlanJSON: mustJSONBytes(t, map[string]any{
				"compiler_contract_version": "compiler.v1",
				"capability_map_version":    "cap.v1",
				"skill_manifest_digest":     "skill",
			}),
			DryRunJSON: mustJSONBytes(t, map[string]any{
				"plan_hash": "plan",
				"create_orgunit_projection": map[string]any{
					"policy_context": map[string]any{"policy_context_digest": "policy-digest"},
					"projection": map[string]any{
						"effective_policy_version": "epv1",
						"resolved_setid":           "S2601",
					},
				},
			}),
		}
		if _, err := taskSnapshotFromTurn(turn); !errors.Is(err, ErrPlanContractMismatch) {
			t.Fatalf("expected plan contract mismatch, got %v", err)
		}
	})

	t.Run("validate task snapshot against turn requires complete policy snapshot", func(t *testing.T) {
		turn := cubeboxdomain.ConversationTurnRecord{
			IntentJSON: mustJSONBytes(t, map[string]any{
				"action":                intentCreateOrgUnit,
				"intent_schema_version": "intent.v1",
				"context_hash":          "ctx",
				"intent_hash":           "intent",
			}),
			PlanJSON: mustJSONBytes(t, map[string]any{
				"compiler_contract_version": "compiler.v1",
				"capability_map_version":    "cap.v1",
				"skill_manifest_digest":     "skill",
				"knowledge_snapshot_digest": "knowledge.v1",
				"route_catalog_version":     "route.v1",
				"resolver_contract_version": "resolver.v1",
				"context_template_version":  "ctx-template.v1",
				"reply_guidance_version":    "reply.v1",
			}),
			DryRunJSON: mustJSONBytes(t, map[string]any{
				"plan_hash": "plan",
				"create_orgunit_projection": map[string]any{
					"policy_context": map[string]any{"policy_context_digest": "policy-digest"},
					"projection": map[string]any{
						"effective_policy_version": "epv1",
						"resolved_setid":           "S2601",
						"setid_source":             "custom",
						"projection_digest":        "projection-digest",
						"mutation_policy_version":  "mutation.v1",
					},
				},
			}),
		}
		err := validateTaskSnapshotAgainstTurn(cubeboxdomain.TaskContractSnapshot{
			IntentSchemaVersion:     "intent.v1",
			CompilerContractVersion: "compiler.v1",
			CapabilityMapVersion:    "cap.v1",
			SkillManifestDigest:     "skill",
			ContextHash:             "ctx",
			IntentHash:              "intent",
			PlanHash:                "plan",
			PolicyContextDigest:     "policy-digest",
		}, turn)
		if !errors.Is(err, ErrPlanContractMismatch) {
			t.Fatalf("expected plan contract mismatch, got %v", err)
		}
	})

	t.Run("task snapshot from turn rejects invalid json payloads", func(t *testing.T) {
		tests := []struct {
			name string
			turn cubeboxdomain.ConversationTurnRecord
		}{
			{
				name: "intent json invalid",
				turn: cubeboxdomain.ConversationTurnRecord{
					IntentJSON: []byte(`{bad`),
				},
			},
			{
				name: "plan json invalid",
				turn: cubeboxdomain.ConversationTurnRecord{
					IntentJSON: mustJSONBytes(t, map[string]any{
						"action":                intentPlanOnly,
						"intent_schema_version": "intent.v1",
						"context_hash":          "ctx",
						"intent_hash":           "intent",
					}),
					PlanJSON: []byte(`{bad`),
				},
			},
			{
				name: "dry run json invalid",
				turn: cubeboxdomain.ConversationTurnRecord{
					IntentJSON: mustJSONBytes(t, map[string]any{
						"action":                intentPlanOnly,
						"intent_schema_version": "intent.v1",
						"context_hash":          "ctx",
						"intent_hash":           "intent",
					}),
					PlanJSON: mustJSONBytes(t, map[string]any{
						"compiler_contract_version": "compiler.v1",
						"capability_map_version":    "cap.v1",
						"skill_manifest_digest":     "skill",
					}),
					DryRunJSON: []byte(`{bad`),
				},
			},
			{
				name: "route decision json invalid",
				turn: cubeboxdomain.ConversationTurnRecord{
					IntentJSON: mustJSONBytes(t, map[string]any{
						"action":                intentPlanOnly,
						"intent_schema_version": "intent.v1",
						"context_hash":          "ctx",
						"intent_hash":           "intent",
					}),
					PlanJSON: mustJSONBytes(t, map[string]any{
						"compiler_contract_version": "compiler.v1",
						"capability_map_version":    "cap.v1",
						"skill_manifest_digest":     "skill",
						"knowledge_snapshot_digest": "knowledge.v1",
						"route_catalog_version":     "route.v1",
						"resolver_contract_version": "resolver.v1",
						"context_template_version":  "ctx-template.v1",
						"reply_guidance_version":    "reply.v1",
					}),
					DryRunJSON:        mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
					RouteDecisionJSON: []byte(`{bad`),
				},
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := taskSnapshotFromTurn(tc.turn); err == nil {
					t.Fatal("expected invalid json error")
				}
			})
		}
	})

	t.Run("validate task snapshot against turn bubbles snapshot parse error", func(t *testing.T) {
		err := validateTaskSnapshotAgainstTurn(cubeboxdomain.TaskContractSnapshot{}, cubeboxdomain.ConversationTurnRecord{
			IntentJSON: []byte(`{bad`),
		})
		if err == nil {
			t.Fatal("expected snapshot parse error")
		}
	})
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
	if _, err := decodeConversationCursor("%%%bad%%%", "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid base64 cursor, got %v", err)
	}
	if _, err := decodeConversationCursor(base64.RawURLEncoding.EncodeToString([]byte("tenant-1|actor-1|only-four|parts")), "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid parts cursor, got %v", err)
	}
	if _, err := decodeConversationCursor(base64.RawURLEncoding.EncodeToString([]byte("tenant-1|actor-1|not-a-time|conv_1|sig")), "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid time cursor, got %v", err)
	}
	if _, err := decodeConversationCursor(base64.RawURLEncoding.EncodeToString([]byte("tenant-1|actor-1|2026-04-15T00:00:00Z| |sig")), "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected invalid blank conversation cursor, got %v", err)
	}
	if got := encodeConversationCursor(conversationCursor{}, "tenant-1", "actor-1"); got != "" {
		t.Fatalf("expected blank cursor for zero value, got %q", got)
	}
	if got := encodeConversationCursor(conversationCursor{
		UpdatedAt:      time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		ConversationID: "   ",
	}, "tenant-1", "actor-1"); got != "" {
		t.Fatalf("expected blank cursor for blank conversation id, got %q", got)
	}

	invalidTimeBase := "tenant-1|actor-1|not-a-time|conv_1"
	invalidTimeSig := hashText(invalidTimeBase + "|" + conversationCursorSalt)
	if _, err := decodeConversationCursor(base64.RawURLEncoding.EncodeToString([]byte(invalidTimeBase+"|"+invalidTimeSig)), "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected signed invalid time cursor, got %v", err)
	}

	blankConversationBase := "tenant-1|actor-1|2026-04-15T00:00:00Z| "
	blankConversationSig := hashText(blankConversationBase + "|" + conversationCursorSalt)
	if _, err := decodeConversationCursor(base64.RawURLEncoding.EncodeToString([]byte(blankConversationBase+"|"+blankConversationSig)), "tenant-1", "actor-1"); !errors.Is(err, ErrConversationCursorInvalid) {
		t.Fatalf("expected signed blank conversation cursor, got %v", err)
	}
}

func TestFacadeFileMethodsAndHelpers(t *testing.T) {
	fileStore := &stubFacadeFileStore{
		listRecords: []FileRecord{{FileID: "file_1"}},
		saveRecord:  FileRecord{FileID: "file_2"},
		deleteOK:    true,
	}
	facade := NewFacade(nil, stubRuntimeProbe{
		models: []cubeboxdomain.ModelEntry{{Provider: "openai", Model: "gpt-5.4"}},
	}, NewFileService(fileStore), stubLegacyFacade{
		reply: map[string]any{"message": "ok"},
	})

	files, err := facade.ListFiles(context.Background(), "tenant-1", "conv_1")
	if err != nil || len(files) != 1 || files[0].FileID != "file_1" {
		t.Fatalf("files=%+v err=%v", files, err)
	}

	saved, err := facade.SaveFile(context.Background(), "tenant-1", "actor-1", "conv_1", "a.txt", "text/plain", strings.NewReader("hello"))
	if err != nil || saved.FileID != "file_2" || fileStore.saveBody != "hello" {
		t.Fatalf("saved=%+v body=%q err=%v", saved, fileStore.saveBody, err)
	}

	deleted, err := facade.DeleteFile(context.Background(), "tenant-1", "file_2")
	if err != nil || !deleted {
		t.Fatalf("deleted=%v err=%v", deleted, err)
	}

	reply, err := facade.RenderReply(context.Background(), "tenant-1", Principal{ID: "actor-1"}, "conv_1", "turn_1", map[string]any{"x": 1})
	if err != nil || reply["message"] != "ok" {
		t.Fatalf("reply=%+v err=%v", reply, err)
	}

	models, err := facade.Models(context.Background())
	if err != nil || len(models) != 1 || models[0].Model != "gpt-5.4" {
		t.Fatalf("models=%+v err=%v", models, err)
	}

	if _, err := (*Facade)(nil).Models(context.Background()); err != nil {
		t.Fatalf("nil facade models err=%v", err)
	}
	if got, err := (*Facade)(nil).ListFiles(context.Background(), "tenant-1", ""); err != nil || got != nil {
		t.Fatalf("nil facade list files got=%+v err=%v", got, err)
	}
	if got, err := (*Facade)(nil).SaveFile(context.Background(), "tenant-1", "actor-1", "", "a.txt", "text/plain", nil); err != nil || got.FileID != "" {
		t.Fatalf("nil facade save file got=%+v err=%v", got, err)
	}
	if got, err := (*Facade)(nil).DeleteFile(context.Background(), "tenant-1", "file_1"); err != nil || got {
		t.Fatalf("nil facade delete file got=%v err=%v", got, err)
	}

	if !taskStatusCancellable(taskStatusQueued) || !taskStatusCancellable(taskStatusRunning) || !taskStatusCancellable(taskStatusManualTakeover) || taskStatusCancellable(taskStatusSucceeded) {
		t.Fatal("unexpected cancellable status evaluation")
	}
	if taskDispatchBackoff(0) != 300*time.Millisecond || taskDispatchBackoff(2) != 600*time.Millisecond || taskDispatchBackoff(5) != 2*time.Second {
		t.Fatalf("unexpected backoff values: %v %v %v", taskDispatchBackoff(0), taskDispatchBackoff(2), taskDispatchBackoff(5))
	}
	if taskErrorCode(nil) != "" || taskErrorCode(errors.New(" boom ")) != "boom" {
		t.Fatal("unexpected task error code mapping")
	}
	if taskTerminalErrorCode(" x ") != "x" || taskTerminalErrorCode("") != ErrTaskStateInvalid.Error() {
		t.Fatal("unexpected task terminal error code")
	}
	if got := bytesToStringSlice([]byte(`["a","b"]`)); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected bytes to string slice: %+v", got)
	}
	if got := bytesToStringSlice([]byte(`[ , ]`)); got != nil {
		t.Fatalf("expected nil bytesToStringSlice for blank items, got %+v", got)
	}
	if got := bytesToStringSlice([]byte("null")); got != nil {
		t.Fatalf("expected nil bytesToStringSlice, got %+v", got)
	}
	if got := bytesToStringSlice([]byte("[]")); got != nil {
		t.Fatalf("expected nil empty bytesToStringSlice, got %+v", got)
	}
	if got := jsonObject([]byte("{bad")); got != nil {
		t.Fatalf("expected nil jsonObject on bad json, got %+v", got)
	}
	if got := jsonObject([]byte("{}")); got != nil {
		t.Fatalf("expected nil jsonObject on empty object, got %+v", got)
	}
	if got := jsonObjectSlice([]byte("[bad")); got != nil {
		t.Fatalf("expected nil jsonObjectSlice on bad json, got %+v", got)
	}
	if got := jsonObjectSlice([]byte("[]")); got != nil {
		t.Fatalf("expected nil jsonObjectSlice on empty slice, got %+v", got)
	}
	record := cubeboxdomain.TaskRecord{
		TaskID:      "task_1",
		TaskType:    taskTypeAsyncPlan,
		Status:      taskStatusQueued,
		WorkflowID:  "wf",
		RequestID:   "req_1",
		SubmittedAt: time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if got := mapTaskCancelResponse(record, false); got == nil || got.CancelAccepted {
		t.Fatalf("expected cancel response with accepted=false, got %+v", got)
	}
	if got := mapTaskCancelResponse(record, true); !got.CancelAccepted {
		t.Fatalf("expected cancel accepted, got %+v", got)
	}
	if got := aggregateRuntimeStatus(cubeboxdomain.RuntimeComponentStatus{Healthy: healthUnavailable}); got != healthUnavailable {
		t.Fatalf("expected unavailable aggregate, got %q", got)
	}
	nilFacadeTime := (*Facade)(nil).now()
	if nilFacadeTime.IsZero() {
		t.Fatal("expected nil facade now to fallback to current time")
	}
	if err := (*Facade)(nil).syncConversationSnapshot(context.Background(), "tenant-1", nil); err != nil {
		t.Fatalf("nil facade sync conversation snapshot err=%v", err)
	}
	if err := NewFacade(nil, nil, nil, nil).syncConversationSnapshot(context.Background(), "tenant-1", &cubeboxdomain.Conversation{ConversationID: "conv_1"}); err != nil {
		t.Fatalf("readerless sync conversation snapshot err=%v", err)
	}
}

func TestFacadeTurnDeadlineHelpersAndCancelResponse(t *testing.T) {
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	turn := cubeboxdomain.ConversationTurnRecord{
		State:     "validated",
		PlanJSON:  mustJSONBytes(t, map[string]any{"confirm_ttl_seconds": 60}),
		CreatedAt: now,
	}
	deadline, ok := turnConfirmDeadline(turn)
	if !ok || !deadline.Equal(now.Add(time.Minute)) {
		t.Fatalf("deadline=%v ok=%v", deadline, ok)
	}
	if !turnConfirmationExpired(turn, now.Add(2*time.Minute)) {
		t.Fatal("expected confirmation expired")
	}
	if turnConfirmationExpired(cubeboxdomain.ConversationTurnRecord{State: "confirmed"}, now) {
		t.Fatal("confirmed turn should not expire")
	}
	if turnConfirmationExpired(cubeboxdomain.ConversationTurnRecord{
		State:    "validated",
		PlanJSON: mustJSONBytes(t, map[string]any{"confirm_ttl_seconds": 60}),
	}, time.Time{}) {
		t.Fatal("zero now should not treat zero-base turn as expired")
	}
	if turnConfirmationExpired(cubeboxdomain.ConversationTurnRecord{
		State:    "validated",
		PlanJSON: []byte("{bad json"),
	}, now) {
		t.Fatal("invalid plan json should not expire")
	}
	if turnConfirmationExpired(cubeboxdomain.ConversationTurnRecord{State: "validated"}, now) {
		t.Fatal("validated turn without deadline should not expire")
	}
	explicit, ok := turnConfirmDeadline(cubeboxdomain.ConversationTurnRecord{
		State:    "validated",
		PlanJSON: mustJSONBytes(t, map[string]any{"expires_at": now.Add(30 * time.Second).Format(time.RFC3339)}),
	})
	if !ok || !explicit.Equal(now.Add(30*time.Second)) {
		t.Fatalf("explicit deadline=%v ok=%v", explicit, ok)
	}
	if _, ok := turnConfirmDeadline(cubeboxdomain.ConversationTurnRecord{
		PlanJSON: []byte("{bad json"),
	}); ok {
		t.Fatal("expected invalid plan json to fail")
	}
	defaultDeadline, ok := turnConfirmDeadline(cubeboxdomain.ConversationTurnRecord{
		State:     "validated",
		UpdatedAt: now,
	})
	if !ok || !defaultDeadline.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("default deadline=%v ok=%v", defaultDeadline, ok)
	}
	if _, ok := turnConfirmDeadline(cubeboxdomain.ConversationTurnRecord{State: "validated"}); ok {
		t.Fatal("missing timestamps should not produce deadline")
	}

	taskID := uuid.MustParse("17171717-1717-1717-1717-171717171717")
	record := sqlcTaskRecord(cubeboxsqlc.IamCubeboxTask{
		TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:    taskTypeAsyncPlan,
		Status:      taskStatusCanceled,
		WorkflowID:  "wf",
		RequestID:   "req_1",
		SubmittedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})
	cancel := mapTaskCancelResponse(record, true)
	if cancel == nil || !cancel.CancelAccepted || cancel.TaskID != taskID.String() {
		t.Fatalf("cancel=%+v", cancel)
	}

	cancelRequestedAt := now.Add(30 * time.Second)
	completedAt := now.Add(time.Minute)
	detail := mapTask(cubeboxdomain.TaskRecord{
		TaskID:                  taskID.String(),
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusSucceeded,
		DispatchStatus:          taskDispatchStarted,
		Attempt:                 2,
		MaxAttempts:             3,
		LastErrorCode:           "none",
		WorkflowID:              "wf",
		RequestID:               "req_1",
		TraceID:                 "trace_1",
		ConversationID:          "conv_1",
		TurnID:                  "turn_1",
		IntentSchemaVersion:     "intent.v1",
		CompilerContractVersion: "compiler.v1",
		CapabilityMapVersion:    "cap.v1",
		SkillManifestDigest:     "skill",
		ContextHash:             "ctx",
		IntentHash:              "intent",
		PlanHash:                "plan",
		SubmittedAt:             now,
		UpdatedAt:               now,
		CancelRequestedAt:       &cancelRequestedAt,
		CompletedAt:             &completedAt,
	})
	if detail == nil || detail.CancelRequestedAt == nil || detail.CompletedAt == nil ||
		!detail.CancelRequestedAt.Equal(cancelRequestedAt.UTC()) || !detail.CompletedAt.Equal(completedAt.UTC()) {
		t.Fatalf("detail=%+v", detail)
	}
}

func TestFacadeDispatchFailureAndDeadlineBranches(t *testing.T) {
	now := time.Date(2026, 4, 15, 14, 30, 0, 0, time.UTC)
	taskID := uuid.MustParse("18181818-1818-1818-1818-181818181818")
	baseTask := sqlcTaskRecord(cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		DispatchAttempt:         0,
		Attempt:                 0,
		MaxAttempts:             int32(taskDefaultMaxAttempts),
		WorkflowID:              "wf",
		RequestID:               "req_1",
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
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	})

	t.Run("executor failure marks manual takeover", func(t *testing.T) {
		reader := &stubConversationReader{
			dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
				TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
				Status:      taskDispatchPending,
				Attempt:     int32(taskDefaultMaxAttempts - 1),
				NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
			}},
			taskDispatchRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:                taskTypeAsyncPlan,
				Status:                  taskStatusQueued,
				DispatchStatus:          taskDispatchPending,
				Attempt:                 0,
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
				SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
				DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(time.Minute), Valid: true},
				UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
			},
			getRow:      cubeboxsqlc.IamCubeboxConversation{ConversationID: "conv_1", ActorID: "actor-1", ActorRole: "tenant-admin"},
			taskActorID: "actor-1",
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_1",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
		}
		var events []cubeboxdomain.TaskEventRecord
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			task := baseTask
			task.Status = update.Status
			task.DispatchStatus = update.DispatchStatus
			task.DispatchAttempt = update.DispatchAttempt
			task.Attempt = update.Attempt
			task.LastErrorCode = update.LastErrorCode
			task.UpdatedAt = update.UpdatedAt
			task.CompletedAt = update.CompletedAt
			reader.taskRow = cubeboxsqlc.IamCubeboxTask{
				TaskID:          pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:        task.TaskType,
				Status:          task.Status,
				DispatchStatus:  task.DispatchStatus,
				DispatchAttempt: int32(task.DispatchAttempt),
				Attempt:         int32(task.Attempt),
				MaxAttempts:     int32(task.MaxAttempts),
				LastErrorCode:   nilIfBlank(task.LastErrorCode),
				WorkflowID:      task.WorkflowID,
				RequestID:       task.RequestID,
				ConversationID:  task.ConversationID,
				TurnID:          task.TurnID,
				SubmittedAt:     pgtype.Timestamptz{Time: task.SubmittedAt, Valid: true},
				CompletedAt:     pgtype.Timestamptz{Time: task.UpdatedAt, Valid: task.CompletedAt != nil},
				UpdatedAt:       pgtype.Timestamptz{Time: task.UpdatedAt, Valid: true},
			}
			return task, nil
		}
		reader.insertEventFn = func(event cubeboxdomain.TaskEventRecord) error {
			events = append(events, event)
			return nil
		}
		reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
			outboxes = append(outboxes, update)
			return nil
		}
		facade := NewFacade(reader, nil, nil, stubLegacyFacade{
			execErr: errors.New("dispatch failed"),
		})
		facade.nowFn = func() time.Time { return now }

		task, err := facade.GetTask(context.Background(), "tenant-1", Principal{ID: "actor-1"}, taskID.String())
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if task == nil || task.Status != taskStatusManualTakeover {
			t.Fatalf("task=%+v", task)
		}
		if len(events) < 3 || events[len(events)-1].EventType != "dead_lettered" {
			t.Fatalf("events=%+v", events)
		}
		if len(outboxes) != 1 || outboxes[0].Status != taskDispatchFailed {
			t.Fatalf("outboxes=%+v", outboxes)
		}
	})

	t.Run("deadline exceeded marks manual takeover", func(t *testing.T) {
		reader := &stubConversationReader{
			dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
				TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
				Status:      taskDispatchPending,
				Attempt:     0,
				NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
			}},
			taskDispatchRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:                taskTypeAsyncPlan,
				Status:                  taskStatusQueued,
				DispatchStatus:          taskDispatchPending,
				MaxAttempts:             int32(taskDefaultMaxAttempts),
				LastErrorCode:           nilIfBlank("old_error"),
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
				DispatchDeadlineAt:      pgtype.Timestamptz{Time: now.Add(-time.Second), Valid: true},
				UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
			},
			taskRow:     cubeboxsqlc.IamCubeboxTask{TaskID: pgtype.UUID{Bytes: taskID, Valid: true}, Status: taskStatusManualTakeover, LastErrorCode: nilIfBlank("old_error"), UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true}},
			taskActorID: "actor-1",
		}
		var events []cubeboxdomain.TaskEventRecord
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			task := baseTask
			task.Status = update.Status
			task.DispatchStatus = update.DispatchStatus
			task.DispatchAttempt = update.DispatchAttempt
			task.Attempt = update.Attempt
			task.LastErrorCode = update.LastErrorCode
			task.UpdatedAt = update.UpdatedAt
			task.CompletedAt = update.CompletedAt
			reader.taskRow = cubeboxsqlc.IamCubeboxTask{
				TaskID:          pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:        task.TaskType,
				Status:          task.Status,
				DispatchStatus:  task.DispatchStatus,
				DispatchAttempt: int32(task.DispatchAttempt),
				Attempt:         int32(task.Attempt),
				MaxAttempts:     int32(task.MaxAttempts),
				LastErrorCode:   nilIfBlank(task.LastErrorCode),
				WorkflowID:      task.WorkflowID,
				RequestID:       task.RequestID,
				ConversationID:  task.ConversationID,
				TurnID:          task.TurnID,
				SubmittedAt:     pgtype.Timestamptz{Time: task.SubmittedAt, Valid: true},
				CompletedAt:     pgtype.Timestamptz{Time: task.UpdatedAt, Valid: task.CompletedAt != nil},
				UpdatedAt:       pgtype.Timestamptz{Time: task.UpdatedAt, Valid: true},
			}
			return task, nil
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
		if task == nil || task.Status != taskStatusManualTakeover || task.LastErrorCode != "old_error" {
			t.Fatalf("task=%+v", task)
		}
		if len(events) != 2 || events[0].EventType != "manual_takeover_required" || events[1].EventType != "dead_lettered" {
			t.Fatalf("events=%+v", events)
		}
		if len(outboxes) != 1 || outboxes[0].Status != taskDispatchFailed {
			t.Fatalf("outboxes=%+v", outboxes)
		}
	})
}

func TestFacadeDispatchPendingAndSnapshotHelperBranches(t *testing.T) {
	now := time.Date(2026, 4, 15, 15, 0, 0, 0, time.UTC)
	taskID := uuid.MustParse("19191919-1919-1919-1919-191919191919")
	task := sqlcTaskRecord(cubeboxsqlc.IamCubeboxTask{
		TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
		TaskType:                taskTypeAsyncPlan,
		Status:                  taskStatusQueued,
		DispatchStatus:          taskDispatchPending,
		DispatchAttempt:         0,
		Attempt:                 0,
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
		SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
	})

	t.Run("dispatch pending nil facade or reader is no-op", func(t *testing.T) {
		if err := (*Facade)(nil).dispatchPendingTasks(context.Background(), "tenant-1", 1); err != nil {
			t.Fatalf("nil facade dispatch pending: %v", err)
		}
		if err := NewFacade(nil, nil, nil, nil).dispatchPendingTasks(context.Background(), "tenant-1", 0); err != nil {
			t.Fatalf("nil reader dispatch pending: %v", err)
		}
	})

	t.Run("dispatch pending returns list error", func(t *testing.T) {
		reader := &stubConversationReader{dispatchErr: errors.New("list failed")}
		if err := NewFacade(reader, nil, nil, nil).dispatchPendingTasks(context.Background(), "tenant-1", 0); err == nil || !strings.Contains(err.Error(), "list failed") {
			t.Fatalf("expected list error, got %v", err)
		}
	})

	t.Run("dispatch pending propagates dispatch task error", func(t *testing.T) {
		reader := &stubConversationReader{
			dispatchRows: []cubeboxsqlc.IamCubeboxTaskDispatchOutbox{{
				TaskID:      pgtype.UUID{Bytes: taskID, Valid: true},
				Status:      taskDispatchPending,
				Attempt:     0,
				NextRetryAt: pgtype.Timestamptz{Time: now, Valid: true},
			}},
			taskDispatchErr: errors.New("dispatch load failed"),
		}
		if err := NewFacade(reader, nil, nil, nil).dispatchPendingTasks(context.Background(), "tenant-1", 1); err == nil || !strings.Contains(err.Error(), "dispatch load failed") {
			t.Fatalf("expected dispatch error, got %v", err)
		}
	})

	t.Run("validate snapshot turn missing marks manual takeover", func(t *testing.T) {
		reader := &stubConversationReader{}
		var updates []cubeboxdomain.TaskStateUpdate
		var events []cubeboxdomain.TaskEventRecord
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			updates = append(updates, update)
			task.Status = update.Status
			task.LastErrorCode = update.LastErrorCode
			return task, nil
		}
		reader.insertEventFn = func(event cubeboxdomain.TaskEventRecord) error {
			events = append(events, event)
			return nil
		}
		facade := NewFacade(reader, nil, nil, nil)
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", task, taskStatusRunning, now)
		if err != nil {
			t.Fatalf("validate dispatch snapshot: %v", err)
		}
		if !handled || len(updates) != 1 || updates[0].Status != taskStatusManualTakeover {
			t.Fatalf("handled=%v updates=%+v", handled, updates)
		}
		if len(events) != 2 || events[0].EventType != "manual_takeover_required" {
			t.Fatalf("events=%+v", events)
		}
	})

	t.Run("validate snapshot list turns error", func(t *testing.T) {
		reader := &stubConversationReader{turnErr: errors.New("turn list failed")}
		facade := NewFacade(reader, nil, nil, nil)
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", task, taskStatusRunning, now)
		if err == nil || !strings.Contains(err.Error(), "turn list failed") || handled {
			t.Fatalf("handled=%v err=%v", handled, err)
		}
	})

	t.Run("validate snapshot empty stored plan hash marks determinism violation", func(t *testing.T) {
		reader := &stubConversationReader{
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_1",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{}),
			}},
		}
		var updates []cubeboxdomain.TaskStateUpdate
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			updates = append(updates, update)
			task.Status = update.Status
			task.LastErrorCode = update.LastErrorCode
			return task, nil
		}
		reader.insertEventFn = func(cubeboxdomain.TaskEventRecord) error { return nil }
		facade := NewFacade(reader, nil, nil, nil)
		taskWithoutPlanHash := task
		taskWithoutPlanHash.PlanHash = ""
		taskWithoutPlanHash.RouteCatalogVersion = ""
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", taskWithoutPlanHash, taskStatusRunning, now)
		if err != nil || !handled || len(updates) != 1 || updates[0].LastErrorCode != ErrPlanDeterminismViolation.Error() {
			t.Fatalf("handled=%v updates=%+v err=%v", handled, updates, err)
		}
	})

	t.Run("validate snapshot turn missing manual takeover update error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("manual takeover update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", task, taskStatusRunning, now)
		if err == nil || !strings.Contains(err.Error(), "manual takeover update failed") || handled {
			t.Fatalf("handled=%v err=%v", handled, err)
		}
	})

	t.Run("validate snapshot mismatch manual takeover update error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_1",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v2", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("snapshot mismatch update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", task, taskStatusRunning, now)
		if err == nil || !strings.Contains(err.Error(), "snapshot mismatch update failed") || handled {
			t.Fatalf("handled=%v err=%v", handled, err)
		}
	})

	t.Run("validate snapshot determinism manual takeover update error is returned", func(t *testing.T) {
		reader := &stubConversationReader{
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_1",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{}),
			}},
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("determinism update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		taskWithoutPlanHash := task
		taskWithoutPlanHash.PlanHash = ""
		taskWithoutPlanHash.RouteCatalogVersion = ""
		handled, err := facade.validateDispatchSnapshot(context.Background(), "tenant-1", taskWithoutPlanHash, taskStatusRunning, now)
		if err == nil || !strings.Contains(err.Error(), "determinism update failed") || handled {
			t.Fatalf("handled=%v err=%v", handled, err)
		}
	})

	t.Run("mark task running if already running is no-op", func(t *testing.T) {
		facade := NewFacade(&stubConversationReader{}, nil, nil, nil)
		current := task
		current.Status = taskStatusRunning
		updated, fromStatus, err := facade.markTaskRunningIfNeeded(context.Background(), "tenant-1", current, 2, now)
		if err != nil || fromStatus != taskStatusRunning || updated.Status != taskStatusRunning {
			t.Fatalf("updated=%+v fromStatus=%q err=%v", updated, fromStatus, err)
		}
	})

	t.Run("mark task running update error", func(t *testing.T) {
		queuedTask := task
		queuedTask.Status = taskStatusQueued
		reader := &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		_, _, err := facade.markTaskRunningIfNeeded(context.Background(), "tenant-1", queuedTask, 1, now)
		if err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}
	})

	t.Run("mark task running insert event error", func(t *testing.T) {
		queuedTask := task
		queuedTask.Status = taskStatusQueued
		reader := &stubConversationReader{
			updateTaskFn: func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				updated := queuedTask
				updated.Status = update.Status
				return updated, nil
			},
			insertEventFn: func(cubeboxdomain.TaskEventRecord) error {
				return errors.New("event failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		_, _, err := facade.markTaskRunningIfNeeded(context.Background(), "tenant-1", queuedTask, 1, now)
		if err == nil || !strings.Contains(err.Error(), "event failed") {
			t.Fatalf("expected event error, got %v", err)
		}
	})

	t.Run("mark task manual takeover and finalize error branches", func(t *testing.T) {
		baseReader := &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return task, nil
			},
		}

		reader := *baseReader
		reader.insertEventFn = func(event cubeboxdomain.TaskEventRecord) error {
			if event.EventType == "manual_takeover_required" {
				return errors.New("first event failed")
			}
			return nil
		}
		facade := NewFacade(&reader, nil, nil, nil)
		if err := facade.markTaskManualTakeover(context.Background(), "tenant-1", task, taskStatusRunning, "err", now); err == nil || !strings.Contains(err.Error(), "first event failed") {
			t.Fatalf("expected first event error, got %v", err)
		}

		reader = *baseReader
		callCount := 0
		reader.insertEventFn = func(cubeboxdomain.TaskEventRecord) error {
			callCount++
			if callCount == 2 {
				return errors.New("second event failed")
			}
			return nil
		}
		facade = NewFacade(&reader, nil, nil, nil)
		if err := facade.markTaskManualTakeover(context.Background(), "tenant-1", task, taskStatusRunning, "err", now); err == nil || !strings.Contains(err.Error(), "second event failed") {
			t.Fatalf("expected second event error, got %v", err)
		}

		reader = *baseReader
		reader.updateOutboxFn = func(cubeboxdomain.TaskDispatchOutboxUpdate) error {
			return errors.New("outbox failed")
		}
		facade = NewFacade(&reader, nil, nil, nil)
		if err := facade.finalizeTaskManualTakeover(context.Background(), "tenant-1", task, taskStatusRunning, cubeboxdomain.TaskDispatchOutboxRecord{TaskID: task.TaskID, NextRetryAt: now}, 1, now, "err"); err == nil || !strings.Contains(err.Error(), "outbox failed") {
			t.Fatalf("expected outbox error, got %v", err)
		}

		reader = *baseReader
		reader.updateTaskFn = func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			return cubeboxdomain.TaskRecord{}, errors.New("update failed")
		}
		facade = NewFacade(&reader, nil, nil, nil)
		if err := facade.finalizeTaskManualTakeover(context.Background(), "tenant-1", task, taskStatusRunning, cubeboxdomain.TaskDispatchOutboxRecord{TaskID: task.TaskID, NextRetryAt: now}, 1, now, "err"); err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}
	})

	t.Run("mark task dispatch failure error branches", func(t *testing.T) {
		reader := &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		if err := facade.markTaskDispatchFailure(context.Background(), "tenant-1", task, "err", 2, now); err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}

		reader = &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return task, nil
			},
			insertEventFn: func(cubeboxdomain.TaskEventRecord) error {
				return errors.New("event failed")
			},
		}
		facade = NewFacade(reader, nil, nil, nil)
		if err := facade.markTaskDispatchFailure(context.Background(), "tenant-1", task, "err", 2, now); err == nil || !strings.Contains(err.Error(), "event failed") {
			t.Fatalf("expected event error, got %v", err)
		}
	})

	t.Run("mark task dispatch deadline exceeded error branches", func(t *testing.T) {
		reader := &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return cubeboxdomain.TaskRecord{}, errors.New("update failed")
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		if err := facade.markTaskDispatchDeadlineExceeded(context.Background(), "tenant-1", task, 2, now); err == nil || !strings.Contains(err.Error(), "update failed") {
			t.Fatalf("expected update error, got %v", err)
		}

		reader = &stubConversationReader{
			updateTaskFn: func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				return task, nil
			},
			insertEventFn: func(cubeboxdomain.TaskEventRecord) error {
				return errors.New("event failed")
			},
		}
		facade = NewFacade(reader, nil, nil, nil)
		if err := facade.markTaskDispatchDeadlineExceeded(context.Background(), "tenant-1", task, 2, now); err == nil || !strings.Contains(err.Error(), "event failed") {
			t.Fatalf("expected event error, got %v", err)
		}
	})

	t.Run("dispatch task no rows marks outbox failed", func(t *testing.T) {
		reader := &stubConversationReader{
			taskDispatchErr: pgx.ErrNoRows,
		}
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
			outboxes = append(outboxes, update)
			return nil
		}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     2,
			NextRetryAt: now,
		}, now)
		if err != nil {
			t.Fatalf("dispatch task: %v", err)
		}
		if len(outboxes) != 1 || outboxes[0].Status != taskDispatchFailed {
			t.Fatalf("outboxes=%+v", outboxes)
		}
	})

	t.Run("dispatch task without legacy returns task state invalid", func(t *testing.T) {
		reader := &stubConversationReader{
			taskDispatchRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
				TaskType:                taskTypeAsyncPlan,
				Status:                  taskStatusRunning,
				DispatchStatus:          taskDispatchStarted,
				Attempt:                 1,
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
				SubmittedAt:             pgtype.Timestamptz{Time: now, Valid: true},
				UpdatedAt:               pgtype.Timestamptz{Time: now, Valid: true},
			},
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_1",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
		}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if !errors.Is(err, ErrTaskStateInvalid) {
			t.Fatalf("expected task state invalid, got %v", err)
		}
	})

	t.Run("dispatch task terminal task marks outbox failed", func(t *testing.T) {
		reader := &stubConversationReader{
			taskDispatchRow: cubeboxsqlc.IamCubeboxTask{
				TaskID:         pgtype.UUID{Bytes: taskID, Valid: true},
				Status:         taskStatusSucceeded,
				DispatchStatus: taskDispatchStarted,
				UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
			},
		}
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
			outboxes = append(outboxes, update)
			return nil
		}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     1,
			NextRetryAt: now,
		}, now)
		if err != nil || len(outboxes) != 1 || outboxes[0].Status != taskDispatchFailed {
			t.Fatalf("err=%v outboxes=%+v", err, outboxes)
		}
	})

	t.Run("dispatch task unexpected load error bubbles up", func(t *testing.T) {
		reader := &stubConversationReader{taskDispatchErr: errors.New("load task failed")}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "load task failed") {
			t.Fatalf("expected load error, got %v", err)
		}
	})

	t.Run("dispatch task snapshot validation error retries pending", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusQueued,
			DispatchStatus:          taskDispatchPending,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_retry",
			TurnID:                  "turn_retry",
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
			taskDispatchRow: taskRow,
			turnErr:         errors.New("turn list failed"),
		}
		var updates []cubeboxdomain.TaskStateUpdate
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			updates = append(updates, update)
			taskRow.Status = update.Status
			taskRow.DispatchStatus = update.DispatchStatus
			taskRow.DispatchAttempt = int32(update.DispatchAttempt)
			taskRow.Attempt = int32(update.Attempt)
			taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
			taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
			reader.taskDispatchRow = taskRow
			return sqlcTaskRecord(taskRow), nil
		}
		reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
			outboxes = append(outboxes, update)
			return nil
		}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err != nil {
			t.Fatalf("dispatch task: %v", err)
		}
		if len(updates) != 2 || updates[1].DispatchStatus != taskDispatchPending || updates[1].LastErrorCode != "turn list failed" {
			t.Fatalf("updates=%+v", updates)
		}
		if len(outboxes) != 1 || outboxes[0].Status != taskDispatchPending {
			t.Fatalf("outboxes=%+v", outboxes)
		}
	})

	t.Run("dispatch task snapshot validation retry update failure bubbles up", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusQueued,
			DispatchStatus:          taskDispatchPending,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_retry_update_fail",
			TurnID:                  "turn_retry_update_fail",
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
			taskDispatchRow: taskRow,
			turnErr:         errors.New("turn list failed"),
			updateTaskFn: func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				if update.DispatchStatus == taskDispatchPending {
					return cubeboxdomain.TaskRecord{}, errors.New("retry update failed")
				}
				taskRow.Status = update.Status
				taskRow.DispatchStatus = update.DispatchStatus
				taskRow.DispatchAttempt = int32(update.DispatchAttempt)
				taskRow.Attempt = int32(update.Attempt)
				taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
				return sqlcTaskRecord(taskRow), nil
			},
		}
		facade := NewFacade(reader, nil, nil, nil)
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "retry update failed") {
			t.Fatalf("expected retry update error, got %v", err)
		}
	})

	t.Run("dispatch task executor error retries pending", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusQueued,
			DispatchStatus:          taskDispatchPending,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_exec_retry",
			TurnID:                  "turn_exec_retry",
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
			taskDispatchRow: taskRow,
			getRow: cubeboxsqlc.IamCubeboxConversation{
				ConversationID: "conv_exec_retry",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
			},
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_exec_retry",
				State:      "confirmed",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
		}
		var updates []cubeboxdomain.TaskStateUpdate
		var outboxes []cubeboxdomain.TaskDispatchOutboxUpdate
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			updates = append(updates, update)
			taskRow.Status = update.Status
			taskRow.DispatchStatus = update.DispatchStatus
			taskRow.DispatchAttempt = int32(update.DispatchAttempt)
			taskRow.Attempt = int32(update.Attempt)
			taskRow.LastErrorCode = nilIfBlank(update.LastErrorCode)
			taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
			reader.taskDispatchRow = taskRow
			return sqlcTaskRecord(taskRow), nil
		}
		reader.updateOutboxFn = func(update cubeboxdomain.TaskDispatchOutboxUpdate) error {
			outboxes = append(outboxes, update)
			return nil
		}
		facade := NewFacade(reader, nil, nil, stubLegacyFacade{execErr: errors.New("exec failed")})
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err != nil {
			t.Fatalf("dispatch task: %v", err)
		}
		if len(updates) != 2 || updates[1].DispatchStatus != taskDispatchPending || updates[1].LastErrorCode != "exec failed" {
			t.Fatalf("updates=%+v", updates)
		}
		if len(outboxes) != 1 || outboxes[0].Status != taskDispatchPending {
			t.Fatalf("outboxes=%+v", outboxes)
		}
	})

	t.Run("dispatch task executor retry update failure bubbles up", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusQueued,
			DispatchStatus:          taskDispatchPending,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_exec_retry_update_fail",
			TurnID:                  "turn_exec_retry_update_fail",
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
			taskDispatchRow: taskRow,
			getRow: cubeboxsqlc.IamCubeboxConversation{
				ConversationID: "conv_exec_retry_update_fail",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
			},
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_exec_retry_update_fail",
				State:      "confirmed",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
			updateTaskFn: func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
				if update.DispatchStatus == taskDispatchPending {
					return cubeboxdomain.TaskRecord{}, errors.New("exec retry update failed")
				}
				taskRow.Status = update.Status
				taskRow.DispatchStatus = update.DispatchStatus
				taskRow.DispatchAttempt = int32(update.DispatchAttempt)
				taskRow.Attempt = int32(update.Attempt)
				taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
				return sqlcTaskRecord(taskRow), nil
			},
		}
		facade := NewFacade(reader, nil, nil, stubLegacyFacade{execErr: errors.New("exec failed")})
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "exec retry update failed") {
			t.Fatalf("expected exec retry update error, got %v", err)
		}
	})

	t.Run("dispatch task success update and event failures bubble up", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusRunning,
			DispatchStatus:          taskDispatchStarted,
			Attempt:                 1,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_success_fail",
			TurnID:                  "turn_success_fail",
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
		baseReader := &stubConversationReader{
			taskDispatchRow: taskRow,
			getRow: cubeboxsqlc.IamCubeboxConversation{
				ConversationID: "conv_success_fail",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
			},
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_success_fail",
				State:      "confirmed",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
		}

		reader := *baseReader
		reader.updateTaskFn = func(cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			return cubeboxdomain.TaskRecord{}, errors.New("success update failed")
		}
		facade := NewFacade(&reader, nil, nil, stubLegacyFacade{})
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "success update failed") {
			t.Fatalf("expected success update error, got %v", err)
		}

		reader = *baseReader
		reader.updateTaskFn = func(update cubeboxdomain.TaskStateUpdate) (cubeboxdomain.TaskRecord, error) {
			taskRow.Status = update.Status
			taskRow.DispatchStatus = update.DispatchStatus
			taskRow.DispatchAttempt = int32(update.DispatchAttempt)
			taskRow.Attempt = int32(update.Attempt)
			taskRow.UpdatedAt = pgtype.Timestamptz{Time: update.UpdatedAt.UTC(), Valid: true}
			if update.CompletedAt != nil {
				taskRow.CompletedAt = pgtype.Timestamptz{Time: update.CompletedAt.UTC(), Valid: true}
			}
			return sqlcTaskRecord(taskRow), nil
		}
		reader.insertEventFn = func(cubeboxdomain.TaskEventRecord) error { return errors.New("success event failed") }
		facade = NewFacade(&reader, nil, nil, stubLegacyFacade{})
		err = facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "success event failed") {
			t.Fatalf("expected success event error, got %v", err)
		}
	})

	t.Run("dispatch task conversation lookup error is returned", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusRunning,
			DispatchStatus:          taskDispatchStarted,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_lookup_err",
			TurnID:                  "turn_lookup_err",
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
			taskDispatchRow: taskRow,
			getErr:          errors.New("conversation lookup failed"),
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_lookup_err",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
		}
		facade := NewFacade(reader, nil, nil, stubLegacyFacade{})
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "conversation lookup failed") {
			t.Fatalf("expected conversation lookup error, got %v", err)
		}
	})

	t.Run("dispatch task formal snapshot load error is returned", func(t *testing.T) {
		taskRow := cubeboxsqlc.IamCubeboxTask{
			TaskID:                  pgtype.UUID{Bytes: taskID, Valid: true},
			TaskType:                taskTypeAsyncPlan,
			Status:                  taskStatusRunning,
			DispatchStatus:          taskDispatchStarted,
			MaxAttempts:             int32(taskDefaultMaxAttempts),
			ConversationID:          "conv_snapshot_err",
			TurnID:                  "turn_snapshot_err",
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
			taskDispatchRow: taskRow,
			getRow: cubeboxsqlc.IamCubeboxConversation{
				ConversationID: "conv_snapshot_err",
				ActorID:        "actor-1",
				ActorRole:      "tenant-admin",
			},
			turnRows: []cubeboxsqlc.IamCubeboxTurn{{
				TurnID:     "turn_snapshot_err",
				IntentJson: mustJSONBytes(t, map[string]any{"action": "plan_only", "intent_schema_version": "intent.v1", "context_hash": "ctx", "intent_hash": "intent"}),
				PlanJson:   mustJSONBytes(t, map[string]any{"compiler_contract_version": "compiler.v1", "capability_map_version": "cap.v1", "skill_manifest_digest": "skill"}),
				DryRunJson: mustJSONBytes(t, map[string]any{"plan_hash": "plan"}),
			}},
			transitionErr: errors.New("transition load failed"),
		}
		facade := NewFacade(reader, nil, nil, stubLegacyFacade{})
		err := facade.dispatchTask(context.Background(), "tenant-1", cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:      taskID.String(),
			Attempt:     0,
			NextRetryAt: now,
		}, now)
		if err == nil || !strings.Contains(err.Error(), "transition load failed") {
			t.Fatalf("expected snapshot load error, got %v", err)
		}
	})
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

func sqlcConversationRecord(item cubeboxsqlc.IamCubeboxConversation) cubeboxdomain.ConversationRecord {
	return cubeboxdomain.ConversationRecord{
		ConversationID: item.ConversationID,
		ActorID:        item.ActorID,
		ActorRole:      item.ActorRole,
		State:          item.State,
		CurrentPhase:   item.CurrentPhase,
		CreatedAt:      item.CreatedAt.Time.UTC(),
		UpdatedAt:      item.UpdatedAt.Time.UTC(),
	}
}

func sqlcConversationRecords(items []cubeboxsqlc.IamCubeboxConversation) []cubeboxdomain.ConversationRecord {
	out := make([]cubeboxdomain.ConversationRecord, 0, len(items))
	for _, item := range items {
		out = append(out, sqlcConversationRecord(item))
	}
	return out
}

func sqlcTurnRecord(item cubeboxsqlc.IamCubeboxTurn) cubeboxdomain.ConversationTurnRecord {
	return cubeboxdomain.ConversationTurnRecord{
		TurnID:              item.TurnID,
		UserInput:           item.UserInput,
		State:               item.State,
		Phase:               item.Phase,
		RiskTier:            item.RiskTier,
		RequestID:           item.RequestID,
		TraceID:             item.TraceID,
		PolicyVersion:       item.PolicyVersion,
		CompositionVersion:  item.CompositionVersion,
		MappingVersion:      item.MappingVersion,
		IntentJSON:          append([]byte(nil), item.IntentJson...),
		RouteDecisionJSON:   append([]byte(nil), item.RouteDecisionJson...),
		ClarificationJSON:   append([]byte(nil), item.ClarificationJson...),
		CandidatesJSON:      append([]byte(nil), item.CandidatesJson...),
		PlanJSON:            append([]byte(nil), item.PlanJson...),
		DryRunJSON:          append([]byte(nil), item.DryRunJson...),
		ResolvedCandidateID: stringValue(item.ResolvedCandidateID),
		SelectedCandidateID: stringValue(item.SelectedCandidateID),
		AmbiguityCount:      int(item.AmbiguityCount),
		Confidence:          item.Confidence,
		ResolutionSource:    stringValue(item.ResolutionSource),
		PendingDraftSummary: stringValue(item.PendingDraftSummary),
		MissingFieldsJSON:   append([]byte(nil), item.MissingFields...),
		CommitResultJSON:    append([]byte(nil), item.CommitResultJson...),
		CommitReplyJSON:     append([]byte(nil), item.CommitReply...),
		ErrorCode:           stringValue(item.ErrorCode),
		CreatedAt:           item.CreatedAt.Time.UTC(),
		UpdatedAt:           item.UpdatedAt.Time.UTC(),
	}
}

func sqlcTurnRecords(items []cubeboxsqlc.IamCubeboxTurn) []cubeboxdomain.ConversationTurnRecord {
	out := make([]cubeboxdomain.ConversationTurnRecord, 0, len(items))
	for _, item := range items {
		out = append(out, sqlcTurnRecord(item))
	}
	return out
}

func sqlcTransitionRecord(item cubeboxsqlc.IamCubeboxStateTransition) cubeboxdomain.StateTransitionRecord {
	return cubeboxdomain.StateTransitionRecord{
		ID:         item.ID,
		TurnID:     stringValue(item.TurnID),
		TurnAction: stringValue(item.TurnAction),
		RequestID:  item.RequestID,
		TraceID:    item.TraceID,
		FromState:  item.FromState,
		ToState:    item.ToState,
		FromPhase:  item.FromPhase,
		ToPhase:    item.ToPhase,
		ReasonCode: stringValue(item.ReasonCode),
		ActorID:    item.ActorID,
		ChangedAt:  item.ChangedAt.Time.UTC(),
	}
}

func sqlcTransitionRecords(items []cubeboxsqlc.IamCubeboxStateTransition) []cubeboxdomain.StateTransitionRecord {
	out := make([]cubeboxdomain.StateTransitionRecord, 0, len(items))
	for _, item := range items {
		out = append(out, sqlcTransitionRecord(item))
	}
	return out
}

func sqlcTaskRecord(item cubeboxsqlc.IamCubeboxTask) cubeboxdomain.TaskRecord {
	record := cubeboxdomain.TaskRecord{
		TaskID:                   item.TaskID.String(),
		ConversationID:           item.ConversationID,
		TurnID:                   item.TurnID,
		TaskType:                 item.TaskType,
		RequestID:                item.RequestID,
		RequestHash:              item.RequestHash,
		WorkflowID:               item.WorkflowID,
		Status:                   item.Status,
		DispatchStatus:           item.DispatchStatus,
		DispatchAttempt:          int(item.DispatchAttempt),
		Attempt:                  int(item.Attempt),
		MaxAttempts:              int(item.MaxAttempts),
		LastErrorCode:            stringValue(item.LastErrorCode),
		TraceID:                  stringValue(item.TraceID),
		IntentSchemaVersion:      item.IntentSchemaVersion,
		CompilerContractVersion:  item.CompilerContractVersion,
		CapabilityMapVersion:     item.CapabilityMapVersion,
		SkillManifestDigest:      item.SkillManifestDigest,
		ContextHash:              item.ContextHash,
		IntentHash:               item.IntentHash,
		PlanHash:                 item.PlanHash,
		KnowledgeSnapshotDigest:  stringValue(item.KnowledgeSnapshotDigest),
		RouteCatalogVersion:      stringValue(item.RouteCatalogVersion),
		ResolverContractVersion:  stringValue(item.ResolverContractVersion),
		ContextTemplateVersion:   stringValue(item.ContextTemplateVersion),
		ReplyGuidanceVersion:     stringValue(item.ReplyGuidanceVersion),
		PolicyContextDigest:      stringValue(item.PolicyContextDigest),
		EffectivePolicyVersion:   stringValue(item.EffectivePolicyVersion),
		ResolvedSetID:            stringValue(item.ResolvedSetid),
		SetIDSource:              stringValue(item.SetidSource),
		PrecheckProjectionDigest: stringValue(item.PrecheckProjectionDigest),
		MutationPolicyVersion:    stringValue(item.MutationPolicyVersion),
		SubmittedAt:              item.SubmittedAt.Time.UTC(),
		CreatedAt:                item.CreatedAt.Time.UTC(),
		UpdatedAt:                item.UpdatedAt.Time.UTC(),
	}
	if item.DispatchDeadlineAt.Valid {
		record.DispatchDeadlineAt = timePtr(item.DispatchDeadlineAt.Time.UTC())
	}
	if item.CancelRequestedAt.Valid {
		record.CancelRequestedAt = timePtr(item.CancelRequestedAt.Time.UTC())
	}
	if item.CompletedAt.Valid {
		record.CompletedAt = timePtr(item.CompletedAt.Time.UTC())
	}
	return record
}

func sqlcDispatchOutboxRecords(items []cubeboxsqlc.IamCubeboxTaskDispatchOutbox) []cubeboxdomain.TaskDispatchOutboxRecord {
	out := make([]cubeboxdomain.TaskDispatchOutboxRecord, 0, len(items))
	for _, item := range items {
		record := cubeboxdomain.TaskDispatchOutboxRecord{
			TaskID:  item.TaskID.String(),
			Status:  item.Status,
			Attempt: int(item.Attempt),
		}
		if item.NextRetryAt.Valid {
			record.NextRetryAt = item.NextRetryAt.Time.UTC()
		}
		out = append(out, record)
	}
	return out
}

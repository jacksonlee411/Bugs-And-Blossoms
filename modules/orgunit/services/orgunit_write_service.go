package services

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/fieldmeta"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	dictpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/dict"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

const (
	errOrgCodeInvalid                 = "ORG_CODE_INVALID"
	errOrgCodeNotFound                = "ORG_CODE_NOT_FOUND"
	errOrgInvalidArgument             = "ORG_INVALID_ARGUMENT"
	errEffectiveDateInvalid           = "EFFECTIVE_DATE_INVALID"
	errPatchFieldNotAllowed           = "PATCH_FIELD_NOT_ALLOWED"
	errPatchRequired                  = "PATCH_REQUIRED"
	errOrgEventNotFound               = "ORG_EVENT_NOT_FOUND"
	errParentNotFoundAsOf             = "PARENT_NOT_FOUND_AS_OF"
	errManagerPernrInvalid            = "MANAGER_PERNR_INVALID"
	errManagerPernrNotFound           = "MANAGER_PERNR_NOT_FOUND"
	errManagerPernrInactive           = "MANAGER_PERNR_INACTIVE"
	errOrgRequestIDConflict           = "ORG_REQUEST_ID_CONFLICT"
	errOrgRootDeleteForbidden         = "ORG_ROOT_DELETE_FORBIDDEN"
	errOrgHasChildrenCannotDelete     = "ORG_HAS_CHILDREN_CANNOT_DELETE"
	errOrgHasDependenciesCannotDelete = "ORG_HAS_DEPENDENCIES_CANNOT_DELETE"
	errOrgEventRescinded              = "ORG_EVENT_RESCINDED"
	errFieldNotMaintainable           = "FIELD_NOT_MAINTAINABLE"
	errDefaultRuleRequired            = "DEFAULT_RULE_REQUIRED"
	errDefaultRuleEvalFailed          = "DEFAULT_RULE_EVAL_FAILED"
	errFieldPolicyExprInvalid         = "FIELD_POLICY_EXPR_INVALID"
	errOrgCodeExhausted               = "ORG_CODE_EXHAUSTED"
	errOrgCodeConflict                = "ORG_CODE_CONFLICT"
)

var (
	newUUID                             = uuidv7.NewString
	marshalJSON                         = json.Marshal
	resolveOrgUnitMutationPolicyInWrite = ResolvePolicy
	resolveDictLabelInWrite             = func(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
		return globalDictResolver{}.ResolveValueLabel(ctx, tenantID, asOf, dictCode, code)
	}
	celCompileCache   sync.Map
	nextOrgCodeRuleRe = regexp.MustCompile(`^next_org_code\(\s*"([^"]*)"\s*,\s*([0-9]+)\s*\)$`)
)

type OrgUnitWriteService interface {
	// Write is the DEV-PLAN-108 unified write entrypoint:
	// - create_org -> CREATE
	// - add_version/insert_version -> UPDATE
	// - correct -> CORRECT_EVENT (keeps target_event_uuid audit chain; replay may interpret as UPDATE)
	Write(ctx context.Context, tenantID string, req WriteOrgUnitRequest) (OrgUnitWriteResult, error)

	Create(ctx context.Context, tenantID string, req CreateOrgUnitRequest) (types.OrgUnitResult, error)
	Rename(ctx context.Context, tenantID string, req RenameOrgUnitRequest) error
	Move(ctx context.Context, tenantID string, req MoveOrgUnitRequest) error
	Disable(ctx context.Context, tenantID string, req DisableOrgUnitRequest) error
	Enable(ctx context.Context, tenantID string, req EnableOrgUnitRequest) error
	SetBusinessUnit(ctx context.Context, tenantID string, req SetBusinessUnitRequest) error
	Correct(ctx context.Context, tenantID string, req CorrectOrgUnitRequest) (types.OrgUnitResult, error)
	CorrectStatus(ctx context.Context, tenantID string, req CorrectStatusOrgUnitRequest) (types.OrgUnitResult, error)
	RescindRecord(ctx context.Context, tenantID string, req RescindRecordOrgUnitRequest) (types.OrgUnitResult, error)
	RescindOrg(ctx context.Context, tenantID string, req RescindOrgUnitRequest) (types.OrgUnitResult, error)
}

type OrgUnitWriteIntent string

const (
	OrgUnitWriteIntentCreateOrg     OrgUnitWriteIntent = "create_org"
	OrgUnitWriteIntentAddVersion    OrgUnitWriteIntent = "add_version"
	OrgUnitWriteIntentInsertVersion OrgUnitWriteIntent = "insert_version"
	OrgUnitWriteIntentCorrect       OrgUnitWriteIntent = "correct"
)

type OrgUnitWritePatch struct {
	Name           *string
	ParentOrgCode  *string
	Status         *string
	IsBusinessUnit *bool
	ManagerPernr   *string
	Ext            map[string]any
}

type WriteOrgUnitRequest struct {
	Intent              string
	OrgCode             string
	EffectiveDate       string
	TargetEffectiveDate string
	RequestID           string
	Patch               OrgUnitWritePatch
	InitiatorUUID       string
}

type OrgUnitWriteResult struct {
	OrgCode       string         `json:"org_code"`
	EffectiveDate string         `json:"effective_date"`
	EventType     string         `json:"event_type"`
	EventUUID     string         `json:"event_uuid"`
	Fields        map[string]any `json:"fields,omitempty"`
}

type CreateOrgUnitRequest struct {
	EffectiveDate  string
	OrgCode        string
	Name           string
	ParentOrgCode  string
	IsBusinessUnit bool
	ManagerPernr   string
	Ext            map[string]any
	InitiatorUUID  string
}

type RenameOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
	NewName       string
	Ext           map[string]any
	InitiatorUUID string
}

type MoveOrgUnitRequest struct {
	EffectiveDate    string
	OrgCode          string
	NewParentOrgCode string
	Ext              map[string]any
	InitiatorUUID    string
}

type DisableOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
	Ext           map[string]any
	InitiatorUUID string
}

type EnableOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
	Ext           map[string]any
	InitiatorUUID string
}

type SetBusinessUnitRequest struct {
	EffectiveDate  string
	OrgCode        string
	IsBusinessUnit bool
	Ext            map[string]any
	InitiatorUUID  string
}

type CorrectOrgUnitRequest struct {
	OrgCode             string
	TargetEffectiveDate string
	Patch               OrgUnitCorrectionPatch
	RequestID           string
	InitiatorUUID       string
}

type CorrectStatusOrgUnitRequest struct {
	OrgCode             string
	TargetEffectiveDate string
	TargetStatus        string
	RequestID           string
	InitiatorUUID       string
}

type RescindRecordOrgUnitRequest struct {
	OrgCode             string
	TargetEffectiveDate string
	RequestID           string
	Reason              string
	InitiatorUUID       string
}

type RescindOrgUnitRequest struct {
	OrgCode       string
	RequestID     string
	Reason        string
	InitiatorUUID string
}

type OrgUnitCorrectionPatch struct {
	EffectiveDate  *string
	Name           *string
	ParentOrgCode  *string
	IsBusinessUnit *bool
	ManagerPernr   *string
	Ext            map[string]any
}

type orgUnitWriteService struct {
	store ports.OrgUnitWriteStore
}

type orgUnitRequestIDEventReader interface {
	FindEventByRequestID(ctx context.Context, tenantID string, requestID string) (types.OrgUnitEvent, bool, error)
}

type orgUnitFieldPolicyResolver interface {
	ResolveTenantFieldPolicy(
		ctx context.Context,
		tenantID string,
		fieldKey string,
		scopeType string,
		scopeKey string,
		asOf string,
	) (types.TenantFieldPolicy, bool, error)
}

type orgUnitCreateAutoCodeSubmitter interface {
	SubmitCreateEventWithGeneratedCode(
		ctx context.Context,
		tenantID string,
		eventUUID string,
		effectiveDate string,
		payload json.RawMessage,
		requestID string,
		initiatorUUID string,
		prefix string,
		width int,
	) (int64, string, error)
}

type orgUnitRuleRuntimeContext struct {
	tenantID      string
	effectiveDate string
}

type orgUnitAutoCodeSpec struct {
	Prefix string
	Width  int
}

const (
	orgUnitPolicyScopeGlobal    = "GLOBAL"
	orgUnitPolicyScopeForm      = "FORM"
	orgUnitPolicyScopeGlobalKey = "global"
	orgUnitCreateDialogScopeKey = "orgunit.create_dialog"
	orgUnitDefaultModeNone      = "NONE"
	orgUnitDefaultModeCEL       = "CEL"
	orgUnitNextOrgCodeFuncName  = "next_org_code"
	orgUnitDefaultOrgCodePrefix = "O"
	orgUnitDefaultOrgCodeWidth  = 6
	orgUnitPolicyDisabledStatus = "disabled"
	orgUnitPolicyEnabledStatus  = "active"
)

type globalDictResolver struct{}

func (globalDictResolver) ResolveValueLabel(ctx context.Context, tenantID string, asOf string, dictCode string, code string) (string, bool, error) {
	return dictpkg.ResolveValueLabel(ctx, tenantID, asOf, dictCode, code)
}

func (globalDictResolver) ListOptions(ctx context.Context, tenantID string, asOf string, dictCode string, keyword string, limit int) ([]dictpkg.Option, error) {
	return dictpkg.ListOptions(ctx, tenantID, asOf, dictCode, keyword, limit)
}

func NewOrgUnitWriteService(store ports.OrgUnitWriteStore) OrgUnitWriteService {
	return &orgUnitWriteService{store: store}
}

func (s *orgUnitWriteService) Write(ctx context.Context, tenantID string, req WriteOrgUnitRequest) (OrgUnitWriteResult, error) {
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		return OrgUnitWriteResult{}, httperr.NewBadRequest("intent is required")
	}
	writeIntent := OrgUnitWriteIntent(intent)

	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return OrgUnitWriteResult{}, err
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return OrgUnitWriteResult{}, httperr.NewBadRequest("request_id is required")
	}

	if writeIntent == OrgUnitWriteIntentCreateOrg {
		if result, ok, err := s.resolveCreateByRequestID(ctx, tenantID, requestID); err != nil {
			return OrgUnitWriteResult{}, err
		} else if ok {
			return result, nil
		}
	}

	// Resolve ext configs as-of the final effective date (DEV-PLAN-108 §16.3).
	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return OrgUnitWriteResult{}, err
	}
	_ = enabledExtFieldKeys // reserved for future policy checks (capabilities is the UI SSOT).
	var autoCodeSpec *orgUnitAutoCodeSpec

	if writeIntent == OrgUnitWriteIntentCreateOrg {
		autoCodeSpec, err = s.applyCreatePolicyDefaults(ctx, tenantID, effectiveDate, fieldConfigs, &req)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}
	}

	orgCodeRaw := strings.TrimSpace(req.OrgCode)
	orgCode := ""
	if orgCodeRaw != "" {
		orgCode, err = normalizeOrgCode(orgCodeRaw)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}
	} else if writeIntent != OrgUnitWriteIntentCreateOrg {
		return OrgUnitWriteResult{}, httperr.NewBadRequest(errOrgCodeInvalid)
	}

	payload := make(map[string]any)
	fields := make(map[string]any)

	// parent_org_code -> parent_id (kernel payload)
	if req.Patch.ParentOrgCode != nil {
		parentCodeRaw := strings.TrimSpace(*req.Patch.ParentOrgCode)
		if parentCodeRaw == "" {
			return OrgUnitWriteResult{}, httperr.NewBadRequest("parent_org_code is required")
		}
		parentCode, err := normalizeOrgCode(parentCodeRaw)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}
		parentID, err := s.store.ResolveOrgID(ctx, tenantID, parentCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				return OrgUnitWriteResult{}, errors.New(errParentNotFoundAsOf)
			}
			return OrgUnitWriteResult{}, err
		}
		payload["parent_id"] = parentID
		fields["parent_org_code"] = parentCode
	}

	// name
	if req.Patch.Name != nil {
		name := strings.TrimSpace(*req.Patch.Name)
		if name == "" {
			return OrgUnitWriteResult{}, httperr.NewBadRequest("name is required")
		}
		payload["name"] = name
		fields["name"] = name
	}

	// status
	if req.Patch.Status != nil {
		status := strings.TrimSpace(*req.Patch.Status)
		if status == "" {
			return OrgUnitWriteResult{}, httperr.NewBadRequest("status is required")
		}
		payload["status"] = status
		fields["status"] = status
	}

	// is_business_unit
	if req.Patch.IsBusinessUnit != nil {
		payload["is_business_unit"] = *req.Patch.IsBusinessUnit
		fields["is_business_unit"] = *req.Patch.IsBusinessUnit
	}

	// manager_pernr -> manager_uuid
	if req.Patch.ManagerPernr != nil {
		pernrInput := strings.TrimSpace(*req.Patch.ManagerPernr)
		if pernrInput == "" {
			// Allow explicit clear.
			payload["manager_uuid"] = ""
			payload["manager_pernr"] = ""
			fields["manager_pernr"] = ""
			fields["manager_name"] = ""
		} else {
			pernr, managerUUID, managerName, err := s.resolveManager(ctx, tenantID, pernrInput)
			if err != nil {
				return OrgUnitWriteResult{}, err
			}
			payload["manager_uuid"] = managerUUID
			payload["manager_pernr"] = pernr
			fields["manager_pernr"] = pernr
			fields["manager_name"] = managerName
		}
	}

	// ext payload + dict labels snapshot
	if len(req.Patch.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Patch.Ext, fieldConfigs)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}
		if len(extPayload) > 0 {
			payload["ext"] = extPayload
			fields["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payload["ext_labels_snapshot"] = extLabels
		}
	}

	eventUUID, err := newUUID()
	if err != nil {
		return OrgUnitWriteResult{}, err
	}

	initiatorUUID := resolveInitiatorUUID(req.InitiatorUUID, tenantID)

	switch writeIntent {
	case OrgUnitWriteIntentCreateOrg:
		// Create supports setting status in the same CREATE payload (DEV-PLAN-108 §6.4).
		autoGenerateOrgCode := orgCode == ""
		if !autoGenerateOrgCode {
			payload["org_code"] = orgCode
		}
		if _, ok := payload["name"]; !ok {
			return OrgUnitWriteResult{}, httperr.NewBadRequest("name is required")
		}

		payloadJSON, err := marshalJSON(payload)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}

		if autoGenerateOrgCode {
			if autoCodeSpec == nil {
				return OrgUnitWriteResult{}, errors.New(errDefaultRuleRequired)
			}
			submitter, ok := s.store.(orgUnitCreateAutoCodeSubmitter)
			if !ok {
				return OrgUnitWriteResult{}, errors.New(errDefaultRuleEvalFailed)
			}
			eventID, generatedOrgCode, err := submitter.SubmitCreateEventWithGeneratedCode(
				ctx,
				tenantID,
				eventUUID,
				effectiveDate,
				payloadJSON,
				requestID,
				initiatorUUID,
				autoCodeSpec.Prefix,
				autoCodeSpec.Width,
			)
			if err != nil {
				return OrgUnitWriteResult{}, mapCreateAutoCodeError(err)
			}
			_ = eventID // event_id is committed but not needed on API response.
			orgCode = generatedOrgCode
			fields["org_code"] = generatedOrgCode
		} else {
			if _, err := s.store.SubmitEvent(ctx, tenantID, eventUUID, nil, string(types.OrgUnitEventCreate), effectiveDate, payloadJSON, requestID, initiatorUUID); err != nil {
				return OrgUnitWriteResult{}, err
			}
		}

		return OrgUnitWriteResult{
			OrgCode:       orgCode,
			EffectiveDate: effectiveDate,
			EventType:     string(types.OrgUnitEventCreate),
			EventUUID:     eventUUID,
			Fields:        fields,
		}, nil

	case OrgUnitWriteIntentAddVersion, OrgUnitWriteIntentInsertVersion:
		if len(payload) == 0 {
			return OrgUnitWriteResult{}, httperr.NewBadRequest(errPatchRequired)
		}

		orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				return OrgUnitWriteResult{}, errors.New(errOrgCodeNotFound)
			}
			return OrgUnitWriteResult{}, err
		}

		payloadJSON, err := marshalJSON(payload)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}

		if _, err := s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventUpdate), effectiveDate, payloadJSON, requestID, initiatorUUID); err != nil {
			return OrgUnitWriteResult{}, err
		}

		return OrgUnitWriteResult{
			OrgCode:       orgCode,
			EffectiveDate: effectiveDate,
			EventType:     string(types.OrgUnitEventUpdate),
			EventUUID:     eventUUID,
			Fields:        fields,
		}, nil

	case OrgUnitWriteIntentCorrect:
		targetDate, err := validateDate(req.TargetEffectiveDate)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}
		orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				return OrgUnitWriteResult{}, errors.New(errOrgCodeNotFound)
			}
			return OrgUnitWriteResult{}, err
		}

		// DEV-PLAN-108: correct uses CORRECT_EVENT; effective-date correction is represented as patch.effective_date.
		if effectiveDate != targetDate {
			payload["effective_date"] = effectiveDate
		}

		if len(payload) == 0 {
			return OrgUnitWriteResult{}, httperr.NewBadRequest(errPatchRequired)
		}

		patchJSON, err := marshalJSON(payload)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}

		correctionUUID, err := s.store.SubmitCorrection(ctx, tenantID, orgID, targetDate, patchJSON, requestID, initiatorUUID)
		if err != nil {
			return OrgUnitWriteResult{}, err
		}

		return OrgUnitWriteResult{
			OrgCode:       orgCode,
			EffectiveDate: effectiveDate,
			EventType:     "CORRECT_EVENT",
			EventUUID:     correctionUUID,
			Fields:        fields,
		}, nil

	default:
		return OrgUnitWriteResult{}, httperr.NewBadRequest("intent not supported")
	}
}

func (s *orgUnitWriteService) Create(ctx context.Context, tenantID string, req CreateOrgUnitRequest) (types.OrgUnitResult, error) {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("name is required")
	}

	// Fail-closed: creating an existing org_code should be rejected early.
	if _, err := s.store.ResolveOrgID(ctx, tenantID, orgCode); err == nil {
		return types.OrgUnitResult{}, errors.New("ORG_ALREADY_EXISTS")
	} else if !errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
		return types.OrgUnitResult{}, err
	}

	var parentID *int
	var parentCode string
	if strings.TrimSpace(req.ParentOrgCode) != "" {
		parentCode, err = normalizeOrgCode(req.ParentOrgCode)
		if err != nil {
			return types.OrgUnitResult{}, err
		}
		parentIDValue, err := s.store.ResolveOrgID(ctx, tenantID, parentCode)
		if err != nil {
			if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
				return types.OrgUnitResult{}, errors.New(errParentNotFoundAsOf)
			}
			return types.OrgUnitResult{}, err
		}
		parentID = &parentIDValue
	}

	var managerUUID string
	var managerPernr string
	var managerName string
	if strings.TrimSpace(req.ManagerPernr) != "" {
		managerPernr, managerUUID, managerName, err = s.resolveManager(ctx, tenantID, req.ManagerPernr)
		if err != nil {
			return types.OrgUnitResult{}, err
		}
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionCreate,
		EmittedEventType: OrgUnitEmittedCreate,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return types.OrgUnitResult{}, err
	}
	if !decision.Enabled {
		return types.OrgUnitResult{}, httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := map[string]any{
		"org_code":         orgCode,
		"name":             name,
		"is_business_unit": req.IsBusinessUnit,
	}
	if parentID != nil {
		payload["parent_id"] = *parentID
	}
	if managerUUID != "" {
		payload["manager_uuid"] = managerUUID
		payload["manager_pernr"] = managerPernr
	}

	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return types.OrgUnitResult{}, err
		}
		if len(extPayload) > 0 {
			payload["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payload["ext_labels_snapshot"] = extLabels
		}
	}

	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitEvent(ctx, tenantID, eventUUID, nil, string(types.OrgUnitEventCreate), effectiveDate, payloadJSON, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID)); err != nil {
		return types.OrgUnitResult{}, err
	}

	event, err := s.store.FindEventByUUID(ctx, tenantID, eventUUID)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	fields := map[string]any{
		"name":             name,
		"is_business_unit": req.IsBusinessUnit,
	}
	if parentCode != "" {
		fields["parent_org_code"] = parentCode
	}
	if managerPernr != "" {
		fields["manager_pernr"] = managerPernr
		fields["manager_name"] = managerName
	}

	return types.OrgUnitResult{
		OrgID:         strconv.Itoa(event.OrgID),
		OrgCode:       orgCode,
		EffectiveDate: effectiveDate,
		Fields:        fields,
	}, nil
}

func (s *orgUnitWriteService) Rename(ctx context.Context, tenantID string, req RenameOrgUnitRequest) error {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return err
	}

	newName := strings.TrimSpace(req.NewName)
	if newName == "" {
		return httperr.NewBadRequest("new_name is required")
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errOrgCodeNotFound)
		}
		return err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedRename,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return err
	}
	if !decision.Enabled {
		return httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := map[string]any{"new_name": newName}
	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return err
		}
		if len(extPayload) > 0 {
			payload["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payload["ext_labels_snapshot"] = extLabels
		}
	}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventRename), effectiveDate, payloadJSON, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	return err
}

func (s *orgUnitWriteService) Move(ctx context.Context, tenantID string, req MoveOrgUnitRequest) error {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return err
	}

	if strings.TrimSpace(req.NewParentOrgCode) == "" {
		return httperr.NewBadRequest("new_parent_org_code is required")
	}

	parentCode, err := normalizeOrgCode(req.NewParentOrgCode)
	if err != nil {
		return err
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errOrgCodeNotFound)
		}
		return err
	}

	parentID, err := s.store.ResolveOrgID(ctx, tenantID, parentCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errParentNotFoundAsOf)
		}
		return err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedMove,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		IsRoot:              strings.EqualFold(orgCode, "ROOT"),
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return err
	}
	if !decision.Enabled {
		if len(decision.DenyReasons) > 0 {
			return errors.New(decision.DenyReasons[0])
		}
		return httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := map[string]any{"new_parent_id": parentID}
	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return err
		}
		if len(extPayload) > 0 {
			payload["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payload["ext_labels_snapshot"] = extLabels
		}
	}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventMove), effectiveDate, payloadJSON, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	return err
}

func (s *orgUnitWriteService) Disable(ctx context.Context, tenantID string, req DisableOrgUnitRequest) error {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return err
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errOrgCodeNotFound)
		}
		return err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedDisable,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return err
	}
	if !decision.Enabled {
		return httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := json.RawMessage(`{}`)
	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return err
		}
		payloadMap := map[string]any{}
		if len(extPayload) > 0 {
			payloadMap["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payloadMap["ext_labels_snapshot"] = extLabels
		}
		if len(payloadMap) > 0 {
			b, err := marshalJSON(payloadMap)
			if err != nil {
				return err
			}
			payload = b
		}
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventDisable), effectiveDate, payload, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	return err
}

func (s *orgUnitWriteService) Enable(ctx context.Context, tenantID string, req EnableOrgUnitRequest) error {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return err
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errOrgCodeNotFound)
		}
		return err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedEnable,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return err
	}
	if !decision.Enabled {
		return httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := json.RawMessage(`{}`)
	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return err
		}
		payloadMap := map[string]any{}
		if len(extPayload) > 0 {
			payloadMap["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payloadMap["ext_labels_snapshot"] = extLabels
		}
		if len(payloadMap) > 0 {
			b, err := marshalJSON(payloadMap)
			if err != nil {
				return err
			}
			payload = b
		}
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventEnable), effectiveDate, payload, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	return err
}

func (s *orgUnitWriteService) SetBusinessUnit(ctx context.Context, tenantID string, req SetBusinessUnitRequest) error {
	effectiveDate, err := validateDate(req.EffectiveDate)
	if err != nil {
		return err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return err
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return errors.New(errOrgCodeNotFound)
		}
		return err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, effectiveDate)
	if err != nil {
		return err
	}
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:       OrgUnitActionEventUpdate,
		EmittedEventType: OrgUnitEmittedSetBusinessUnit,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		TreeInitialized:     true,
		TargetExistsAsOf:    true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return err
	}
	if !decision.Enabled {
		return httperr.NewBadRequest(errOrgInvalidArgument)
	}

	payload := map[string]any{"is_business_unit": req.IsBusinessUnit}
	if len(req.Ext) > 0 {
		extPayload, extLabels, err := buildExtPayloadWithContext(ctx, tenantID, effectiveDate, req.Ext, fieldConfigs)
		if err != nil {
			return err
		}
		if len(extPayload) > 0 {
			payload["ext"] = extPayload
		}
		if len(extLabels) > 0 {
			payload["ext_labels_snapshot"] = extLabels
		}
	}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventSetBusinessUnit), effectiveDate, payloadJSON, eventUUID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	return err
}

func (s *orgUnitWriteService) Correct(ctx context.Context, tenantID string, req CorrectOrgUnitRequest) (types.OrgUnitResult, error) {
	targetEffectiveDate, err := validateDate(req.TargetEffectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("request_id is required")
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return types.OrgUnitResult{}, errors.New(errOrgCodeNotFound)
		}
		return types.OrgUnitResult{}, err
	}

	event, err := s.store.FindEventByEffectiveDate(ctx, tenantID, orgID, targetEffectiveDate)
	if err != nil {
		if errors.Is(err, ports.ErrOrgEventNotFound) {
			return types.OrgUnitResult{}, errors.New(errOrgEventNotFound)
		}
		return types.OrgUnitResult{}, err
	}

	fieldConfigs, enabledExtFieldKeys, err := s.listEnabledExtFieldConfigs(ctx, tenantID, targetEffectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	targetEventType := event.EventType
	decision, err := resolveOrgUnitMutationPolicyInWrite(OrgUnitMutationPolicyKey{
		ActionKind:               OrgUnitActionCorrectEvent,
		EmittedEventType:         OrgUnitEmittedCorrectEvent,
		TargetEffectiveEventType: &targetEventType,
	}, OrgUnitMutationPolicyFacts{
		CanAdmin:            true,
		EnabledExtFieldKeys: enabledExtFieldKeys,
	})
	if err != nil {
		return types.OrgUnitResult{}, err
	}
	if err := ValidatePatch(targetEffectiveDate, decision, req.Patch); err != nil {
		return types.OrgUnitResult{}, err
	}

	patch, fields, correctedDate, err := s.buildCorrectionPatch(ctx, tenantID, event, req.Patch, fieldConfigs)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	patchJSON, err := marshalJSON(patch)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitCorrection(ctx, tenantID, orgID, targetEffectiveDate, patchJSON, requestID, resolveInitiatorUUID(req.InitiatorUUID, tenantID)); err != nil {
		return types.OrgUnitResult{}, err
	}

	if correctedDate == "" {
		correctedDate = targetEffectiveDate
	}

	return types.OrgUnitResult{
		OrgID:         strconv.Itoa(orgID),
		OrgCode:       orgCode,
		EffectiveDate: correctedDate,
		Fields:        fields,
	}, nil
}

func (s *orgUnitWriteService) CorrectStatus(ctx context.Context, tenantID string, req CorrectStatusOrgUnitRequest) (types.OrgUnitResult, error) {
	targetEffectiveDate, err := validateDate(req.TargetEffectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	targetStatus, err := normalizeTargetStatus(req.TargetStatus)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("request_id is required")
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return types.OrgUnitResult{}, errors.New(errOrgCodeNotFound)
		}
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitStatusCorrection(ctx, tenantID, orgID, targetEffectiveDate, targetStatus, requestID, resolveInitiatorUUID(req.InitiatorUUID, tenantID)); err != nil {
		return types.OrgUnitResult{}, err
	}

	fields := map[string]any{
		"operation":     "CORRECT_STATUS",
		"target_status": targetStatus,
		"request_id":    requestID,
	}

	return types.OrgUnitResult{
		OrgID:         strconv.Itoa(orgID),
		OrgCode:       orgCode,
		EffectiveDate: targetEffectiveDate,
		Fields:        fields,
	}, nil
}

func (s *orgUnitWriteService) RescindRecord(ctx context.Context, tenantID string, req RescindRecordOrgUnitRequest) (types.OrgUnitResult, error) {
	targetEffectiveDate, err := validateDate(req.TargetEffectiveDate)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("request_id is required")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("reason is required")
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return types.OrgUnitResult{}, errors.New(errOrgCodeNotFound)
		}
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitRescindEvent(ctx, tenantID, orgID, targetEffectiveDate, reason, requestID, resolveInitiatorUUID(req.InitiatorUUID, tenantID)); err != nil {
		return types.OrgUnitResult{}, err
	}

	return types.OrgUnitResult{
		OrgID:         strconv.Itoa(orgID),
		OrgCode:       orgCode,
		EffectiveDate: targetEffectiveDate,
		Fields: map[string]any{
			"operation":  "RESCIND_EVENT",
			"request_id": requestID,
		},
	}, nil
}

func (s *orgUnitWriteService) RescindOrg(ctx context.Context, tenantID string, req RescindOrgUnitRequest) (types.OrgUnitResult, error) {
	orgCode, err := normalizeOrgCode(req.OrgCode)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("request_id is required")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return types.OrgUnitResult{}, httperr.NewBadRequest("reason is required")
	}

	orgID, err := s.store.ResolveOrgID(ctx, tenantID, orgCode)
	if err != nil {
		if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
			return types.OrgUnitResult{}, errors.New(errOrgCodeNotFound)
		}
		return types.OrgUnitResult{}, err
	}

	rescindedEvents, err := s.store.SubmitRescindOrg(ctx, tenantID, orgID, reason, requestID, resolveInitiatorUUID(req.InitiatorUUID, tenantID))
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	return types.OrgUnitResult{
		OrgID:         strconv.Itoa(orgID),
		OrgCode:       orgCode,
		EffectiveDate: "",
		Fields: map[string]any{
			"operation":        "RESCIND_ORG",
			"request_id":       requestID,
			"rescinded_events": rescindedEvents,
		},
	}, nil
}

func (s *orgUnitWriteService) buildCorrectionPatch(ctx context.Context, tenantID string, event types.OrgUnitEvent, patch OrgUnitCorrectionPatch, fieldConfigs []types.TenantFieldConfig) (map[string]any, map[string]any, string, error) {
	patchMap := make(map[string]any)
	fields := make(map[string]any)
	var correctedDate string

	if patch.EffectiveDate != nil {
		value, err := validateDate(*patch.EffectiveDate)
		if err != nil {
			return nil, nil, "", err
		}
		correctedDate = value
		patchMap["effective_date"] = value
	}

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return nil, nil, "", httperr.NewBadRequest("name is required")
		}
		key, ok := namePatchKey(event.EventType)
		if !ok {
			return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		patchMap[key] = name
		fields["name"] = name
	}

	if patch.ParentOrgCode != nil {
		parentCodeRaw := strings.TrimSpace(*patch.ParentOrgCode)
		key, ok := parentPatchKey(event.EventType)
		if !ok {
			return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
		}

		if parentCodeRaw == "" {
			if event.EventType == types.OrgUnitEventMove {
				return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
			}
			patchMap[key] = ""
			fields["parent_org_code"] = ""
		} else {
			parentCode, err := normalizeOrgCode(parentCodeRaw)
			if err != nil {
				return nil, nil, "", err
			}
			parentID, err := s.store.ResolveOrgID(ctx, tenantID, parentCode)
			if err != nil {
				if errors.Is(err, orgunitpkg.ErrOrgCodeNotFound) {
					return nil, nil, "", errors.New(errParentNotFoundAsOf)
				}
				return nil, nil, "", err
			}
			patchMap[key] = parentID
			fields["parent_org_code"] = parentCode
		}
	}

	if patch.IsBusinessUnit != nil {
		if event.EventType != types.OrgUnitEventCreate && event.EventType != types.OrgUnitEventSetBusinessUnit {
			return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		patchMap["is_business_unit"] = *patch.IsBusinessUnit
		fields["is_business_unit"] = *patch.IsBusinessUnit
	}

	if patch.ManagerPernr != nil {
		if event.EventType != types.OrgUnitEventCreate {
			return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		pernr, managerUUID, managerName, err := s.resolveManager(ctx, tenantID, *patch.ManagerPernr)
		if err != nil {
			return nil, nil, "", err
		}
		patchMap["manager_uuid"] = managerUUID
		patchMap["manager_pernr"] = pernr
		fields["manager_pernr"] = pernr
		fields["manager_name"] = managerName
	}

	if len(patch.Ext) > 0 {
		cfgByKey := make(map[string]types.TenantFieldConfig, len(fieldConfigs))
		for _, cfg := range fieldConfigs {
			key := strings.TrimSpace(cfg.FieldKey)
			if key == "" {
				continue
			}
			cfgByKey[key] = cfg
		}

		extPatch := make(map[string]any, len(patch.Ext))
		extLabels := make(map[string]string)
		asOfForDict := strings.TrimSpace(event.EffectiveDate)
		if strings.TrimSpace(correctedDate) != "" {
			asOfForDict = strings.TrimSpace(correctedDate)
		}
		for rawKey, rawValue := range patch.Ext {
			fieldKey := strings.TrimSpace(rawKey)
			if fieldKey == "" {
				return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
			}
			cfg, ok := cfgByKey[fieldKey]
			if !ok {
				return nil, nil, "", httperr.NewBadRequest(errPatchFieldNotAllowed)
			}
			if err := validateExtFieldKeyEnabled(fieldKey, cfg); err != nil {
				return nil, nil, "", err
			}

			extPatch[fieldKey] = rawValue

			if strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
				if rawValue == nil {
					continue
				}
				value, ok := rawValue.(string)
				if !ok {
					return nil, nil, "", httperr.NewBadRequest(errOrgInvalidArgument)
				}
				value = strings.TrimSpace(value)
				if value == "" {
					return nil, nil, "", httperr.NewBadRequest(errOrgInvalidArgument)
				}
				dictCode, ok := fieldmeta.DictCodeFromDataSourceConfig(cfg.DataSourceConfig)
				if !ok {
					return nil, nil, "", httperr.NewBadRequest(errOrgInvalidArgument)
				}
				label, ok, err := resolveDictLabelInWrite(ctx, tenantID, asOfForDict, dictCode, value)
				if err != nil || !ok {
					return nil, nil, "", httperr.NewBadRequest(errOrgInvalidArgument)
				}
				extLabels[fieldKey] = label
			}
		}

		if len(extPatch) > 0 {
			patchMap["ext"] = extPatch
			fields["ext"] = extPatch
		}
		if len(extLabels) > 0 {
			patchMap["ext_labels_snapshot"] = extLabels
		}
	}

	if len(patchMap) == 0 {
		return nil, nil, "", httperr.NewBadRequest(errPatchRequired)
	}

	return patchMap, fields, correctedDate, nil
}

func (s *orgUnitWriteService) listEnabledExtFieldConfigs(ctx context.Context, tenantID string, asOf string) ([]types.TenantFieldConfig, []string, error) {
	cfgs, err := s.store.ListEnabledTenantFieldConfigsAsOf(ctx, tenantID, asOf)
	if err != nil {
		return nil, nil, err
	}
	outCfgs := make([]types.TenantFieldConfig, 0, len(cfgs))
	keys := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		key := strings.TrimSpace(cfg.FieldKey)
		if key == "" {
			continue
		}
		if isReservedExtFieldKey(key) {
			continue
		}
		if _, ok := fieldmeta.LookupFieldDefinition(key); !ok && !fieldmeta.IsCustomPlainFieldKey(key) && !fieldmeta.IsCustomDictFieldKey(key) {
			continue
		}

		// Defense-in-depth for dict namespace keys: ensure key <-> config consistency.
		if fieldmeta.IsCustomDictFieldKey(key) {
			if !strings.EqualFold(strings.TrimSpace(cfg.ValueType), "text") {
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
				continue
			}
			wantDictCode, _ := fieldmeta.DictCodeFromDictFieldKey(key)
			gotDictCode, ok := fieldmeta.DictCodeFromDataSourceConfig(cfg.DataSourceConfig)
			if !ok || !strings.EqualFold(strings.TrimSpace(gotDictCode), strings.TrimSpace(wantDictCode)) {
				continue
			}
		}

		cfg.FieldKey = key
		outCfgs = append(outCfgs, cfg)
		keys = append(keys, key)
	}
	return outCfgs, keys, nil
}

func buildExtPayload(ext map[string]any, fieldConfigs []types.TenantFieldConfig) (map[string]any, map[string]string, error) {
	return buildExtPayloadWithContext(context.Background(), "", "", ext, fieldConfigs)
}

func validateExtFieldKeyEnabled(fieldKey string, cfg types.TenantFieldConfig) error {
	if _, ok := fieldmeta.LookupFieldDefinition(fieldKey); ok {
		return nil
	}
	if fieldmeta.IsCustomDictFieldKey(fieldKey) {
		if !strings.EqualFold(strings.TrimSpace(cfg.ValueType), "text") {
			return httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		if !strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
			return httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		wantDictCode, _ := fieldmeta.DictCodeFromDictFieldKey(fieldKey)
		gotDictCode, ok := fieldmeta.DictCodeFromDataSourceConfig(cfg.DataSourceConfig)
		if !ok || !strings.EqualFold(strings.TrimSpace(gotDictCode), strings.TrimSpace(wantDictCode)) {
			return httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		return nil
	}
	if !fieldmeta.IsCustomPlainFieldKey(fieldKey) {
		return httperr.NewBadRequest(errPatchFieldNotAllowed)
	}
	if !isAllowedExtValueType(cfg.ValueType) {
		return httperr.NewBadRequest(errPatchFieldNotAllowed)
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "PLAIN") {
		return httperr.NewBadRequest(errPatchFieldNotAllowed)
	}
	return nil
}

func isAllowedExtValueType(valueType string) bool {
	switch strings.ToLower(strings.TrimSpace(valueType)) {
	case "text", "int", "uuid", "bool", "date", "numeric":
		return true
	default:
		return false
	}
}

func buildExtPayloadWithContext(ctx context.Context, tenantID string, asOf string, ext map[string]any, fieldConfigs []types.TenantFieldConfig) (map[string]any, map[string]string, error) {
	cfgByKey := make(map[string]types.TenantFieldConfig, len(fieldConfigs))
	for _, cfg := range fieldConfigs {
		key := strings.TrimSpace(cfg.FieldKey)
		if key == "" {
			continue
		}
		cfgByKey[key] = cfg
	}

	extPatch := make(map[string]any, len(ext))
	extLabels := make(map[string]string)
	for rawKey, rawValue := range ext {
		fieldKey := strings.TrimSpace(rawKey)
		if fieldKey == "" {
			return nil, nil, httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		if isReservedExtFieldKey(fieldKey) {
			return nil, nil, httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		cfg, ok := cfgByKey[fieldKey]
		if !ok {
			return nil, nil, httperr.NewBadRequest(errPatchFieldNotAllowed)
		}
		if err := validateExtFieldKeyEnabled(fieldKey, cfg); err != nil {
			return nil, nil, err
		}

		extPatch[fieldKey] = rawValue

		if strings.EqualFold(strings.TrimSpace(cfg.DataSourceType), "DICT") {
			if rawValue == nil {
				continue
			}
			value, ok := rawValue.(string)
			if !ok {
				return nil, nil, httperr.NewBadRequest(errOrgInvalidArgument)
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, nil, httperr.NewBadRequest(errOrgInvalidArgument)
			}
			dictCode, ok := fieldmeta.DictCodeFromDataSourceConfig(cfg.DataSourceConfig)
			if !ok {
				return nil, nil, httperr.NewBadRequest(errOrgInvalidArgument)
			}
			label, ok, err := resolveDictLabelInWrite(ctx, tenantID, asOf, dictCode, value)
			if err != nil || !ok {
				return nil, nil, httperr.NewBadRequest(errOrgInvalidArgument)
			}
			extLabels[fieldKey] = label
		}
	}

	return extPatch, extLabels, nil
}

func (s *orgUnitWriteService) resolveCreateByRequestID(ctx context.Context, tenantID string, requestID string) (OrgUnitWriteResult, bool, error) {
	reader, ok := s.store.(orgUnitRequestIDEventReader)
	if !ok {
		return OrgUnitWriteResult{}, false, nil
	}
	event, found, err := reader.FindEventByRequestID(ctx, tenantID, requestID)
	if err != nil {
		return OrgUnitWriteResult{}, false, err
	}
	if !found {
		return OrgUnitWriteResult{}, false, nil
	}
	if string(event.EventType) != string(types.OrgUnitEventCreate) {
		return OrgUnitWriteResult{}, false, errors.New(errOrgRequestIDConflict)
	}

	orgCode, err := orgCodeFromEventPayload(event.Payload)
	if err != nil {
		return OrgUnitWriteResult{}, false, err
	}
	if orgCode == "" {
		resolved, resolveErr := s.store.ResolveOrgCode(ctx, tenantID, event.OrgID)
		if resolveErr != nil {
			return OrgUnitWriteResult{}, false, resolveErr
		}
		orgCode = resolved
	}
	return OrgUnitWriteResult{
		OrgCode:       orgCode,
		EffectiveDate: event.EffectiveDate,
		EventType:     string(event.EventType),
		EventUUID:     event.EventUUID,
		Fields: map[string]any{
			"org_code": orgCode,
		},
	}, true, nil
}

func orgCodeFromEventPayload(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	value, _ := payload["org_code"].(string)
	return strings.TrimSpace(value), nil
}

func (s *orgUnitWriteService) applyCreatePolicyDefaults(
	ctx context.Context,
	tenantID string,
	effectiveDate string,
	fieldConfigs []types.TenantFieldConfig,
	req *WriteOrgUnitRequest,
) (*orgUnitAutoCodeSpec, error) {
	_ = fieldConfigs // M1 runtime scope: org_code only.

	resolver, ok := s.store.(orgUnitFieldPolicyResolver)
	if !ok {
		return nil, nil
	}
	policy := types.TenantFieldPolicy{
		FieldKey:     "org_code",
		ScopeType:    orgUnitPolicyScopeGlobal,
		ScopeKey:     orgUnitPolicyScopeGlobalKey,
		Maintainable: true,
		DefaultMode:  orgUnitDefaultModeNone,
	}
	resolved, found, err := resolver.ResolveTenantFieldPolicy(
		ctx,
		tenantID,
		"org_code",
		orgUnitPolicyScopeForm,
		orgUnitCreateDialogScopeKey,
		effectiveDate,
	)
	if err != nil {
		return nil, err
	}
	if found {
		policy = resolved
	}

	provided := strings.TrimSpace(req.OrgCode) != ""
	if !policy.Maintainable {
		if provided {
			return nil, errors.New(errFieldNotMaintainable)
		}
	}

	mode := strings.ToUpper(strings.TrimSpace(policy.DefaultMode))
	switch mode {
	case "", orgUnitDefaultModeNone:
		if !policy.Maintainable {
			return nil, errors.New(errDefaultRuleRequired)
		}
		return nil, nil
	case orgUnitDefaultModeCEL:
		if provided {
			return nil, nil
		}
		if policy.DefaultRuleExpr == nil || strings.TrimSpace(*policy.DefaultRuleExpr) == "" {
			return nil, errors.New(errDefaultRuleRequired)
		}
		spec, err := parseNextOrgCodeRule(*policy.DefaultRuleExpr)
		if err != nil {
			return nil, err
		}
		req.OrgCode = ""
		return spec, nil
	default:
		if !policy.Maintainable {
			return nil, errors.New(errDefaultRuleRequired)
		}
		return nil, errors.New(errDefaultRuleEvalFailed)
	}
}

func parseNextOrgCodeRule(expr string) (*orgUnitAutoCodeSpec, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, errors.New(errFieldPolicyExprInvalid)
	}
	if err := compileCELExpr(expr); err != nil {
		return nil, errors.New(errFieldPolicyExprInvalid)
	}

	matches := nextOrgCodeRuleRe.FindStringSubmatch(expr)
	if len(matches) != 3 {
		return nil, errors.New(errDefaultRuleEvalFailed)
	}
	prefix := strings.TrimSpace(matches[1])
	if prefix == "" {
		return nil, errors.New(errDefaultRuleEvalFailed)
	}
	width, err := strconv.Atoi(strings.TrimSpace(matches[2]))
	if err != nil || width <= 0 || width > 12 {
		return nil, errors.New(errDefaultRuleEvalFailed)
	}
	return &orgUnitAutoCodeSpec{Prefix: prefix, Width: width}, nil
}

func orgUnitWriteCELNextOrgCode(_ ...ref.Val) ref.Val {
	return celtypes.String("")
}

var newOrgUnitWriteCELEnv = func() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Function(
			orgUnitNextOrgCodeFuncName,
			cel.Overload(
				"next_org_code_string_int",
				[]*cel.Type{cel.StringType, cel.IntType},
				cel.StringType,
				cel.FunctionBinding(orgUnitWriteCELNextOrgCode),
			),
		),
	)
}

func compileCELExpr(expr string) error {
	if _, ok := celCompileCache.Load(expr); ok {
		return nil
	}
	env, err := newOrgUnitWriteCELEnv()
	if err != nil {
		return err
	}
	ast, iss := env.Compile(expr)
	if iss != nil && iss.Err() != nil {
		return iss.Err()
	}
	if ast.OutputType() != cel.StringType {
		return errors.New("default expression must return string")
	}
	celCompileCache.Store(expr, struct{}{})
	return nil
}

func mapCreateAutoCodeError(err error) error {
	msg := strings.ToUpper(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, errOrgCodeExhausted):
		return errors.New(errOrgCodeExhausted)
	case strings.Contains(msg, "ORG_REQUEST_ID_CONFLICT"):
		return errors.New(errOrgRequestIDConflict)
	case strings.Contains(msg, "DUPLICATE KEY VALUE") && strings.Contains(msg, "ORG_UNIT_CODES_ORG_CODE_UNIQUE"):
		return errors.New(errOrgCodeConflict)
	default:
		return err
	}
}

func namePatchKey(eventType types.OrgUnitEventType) (string, bool) {
	switch eventType {
	case types.OrgUnitEventCreate:
		return "name", true
	case types.OrgUnitEventRename:
		return "new_name", true
	default:
		return "", false
	}
}

func parentPatchKey(eventType types.OrgUnitEventType) (string, bool) {
	switch eventType {
	case types.OrgUnitEventCreate:
		return "parent_id", true
	case types.OrgUnitEventMove:
		return "new_parent_id", true
	default:
		return "", false
	}
}

var pernrDigitsMax8Re = regexp.MustCompile(`^[0-9]{1,8}$`)

func (s *orgUnitWriteService) resolveManager(ctx context.Context, tenantID string, pernrInput string) (string, string, string, error) {
	pernr, err := normalizePernr(pernrInput)
	if err != nil {
		return "", "", "", err
	}

	person, err := s.store.FindPersonByPernr(ctx, tenantID, pernr)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return "", "", "", errors.New(errManagerPernrNotFound)
		}
		return "", "", "", err
	}

	if strings.ToLower(person.Status) != "active" {
		return "", "", "", errors.New(errManagerPernrInactive)
	}

	return pernr, person.UUID, person.DisplayName, nil
}

func normalizePernr(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", httperr.NewBadRequest(errManagerPernrInvalid)
	}
	if !pernrDigitsMax8Re.MatchString(raw) {
		return "", httperr.NewBadRequest(errManagerPernrInvalid)
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		raw = "0"
	}
	return raw, nil
}

func normalizeOrgCode(raw string) (string, error) {
	normalized, err := orgunitpkg.NormalizeOrgCode(raw)
	if err != nil {
		return "", httperr.NewBadRequest(errOrgCodeInvalid)
	}
	return normalized, nil
}

func normalizeTargetStatus(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "enabled", "有效":
		return "active", nil
	case "inactive", "无效":
		return "disabled", nil
	case "active", "disabled":
		return value, nil
	default:
		return "", httperr.NewBadRequest("target_status invalid")
	}
}

func validateDate(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", httperr.NewBadRequest(errEffectiveDateInvalid)
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", httperr.NewBadRequest(errEffectiveDateInvalid)
	}
	return value, nil
}

func resolveInitiatorUUID(candidate string, tenantID string) string {
	value := strings.TrimSpace(candidate)
	if value != "" {
		return value
	}
	return strings.TrimSpace(tenantID)
}

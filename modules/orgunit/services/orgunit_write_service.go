package services

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/ports"
	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/uuidv7"
)

const (
	errOrgCodeInvalid                 = "ORG_CODE_INVALID"
	errOrgCodeNotFound                = "ORG_CODE_NOT_FOUND"
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
	errOrgReplayFailed                = "ORG_REPLAY_FAILED"
	errOrgEventRescinded              = "ORG_EVENT_RESCINDED"
)

var (
	newUUID     = uuidv7.NewString
	marshalJSON = json.Marshal
)

type OrgUnitWriteService interface {
	Create(ctx context.Context, tenantID string, req CreateOrgUnitRequest) (types.OrgUnitResult, error)
	Rename(ctx context.Context, tenantID string, req RenameOrgUnitRequest) error
	Move(ctx context.Context, tenantID string, req MoveOrgUnitRequest) error
	Disable(ctx context.Context, tenantID string, req DisableOrgUnitRequest) error
	Enable(ctx context.Context, tenantID string, req EnableOrgUnitRequest) error
	SetBusinessUnit(ctx context.Context, tenantID string, req SetBusinessUnitRequest) error
	Correct(ctx context.Context, tenantID string, req CorrectOrgUnitRequest) (types.OrgUnitResult, error)
	RescindRecord(ctx context.Context, tenantID string, req RescindRecordOrgUnitRequest) (types.OrgUnitResult, error)
	RescindOrg(ctx context.Context, tenantID string, req RescindOrgUnitRequest) (types.OrgUnitResult, error)
}

type CreateOrgUnitRequest struct {
	EffectiveDate  string
	OrgCode        string
	Name           string
	ParentOrgCode  string
	IsBusinessUnit bool
	ManagerPernr   string
}

type RenameOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
	NewName       string
}

type MoveOrgUnitRequest struct {
	EffectiveDate    string
	OrgCode          string
	NewParentOrgCode string
}

type DisableOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
}

type EnableOrgUnitRequest struct {
	EffectiveDate string
	OrgCode       string
}

type SetBusinessUnitRequest struct {
	EffectiveDate  string
	OrgCode        string
	IsBusinessUnit bool
}

type CorrectOrgUnitRequest struct {
	OrgCode             string
	TargetEffectiveDate string
	Patch               OrgUnitCorrectionPatch
	RequestID           string
}

type RescindRecordOrgUnitRequest struct {
	OrgCode             string
	TargetEffectiveDate string
	RequestID           string
	Reason              string
}

type RescindOrgUnitRequest struct {
	OrgCode   string
	RequestID string
	Reason    string
}

type OrgUnitCorrectionPatch struct {
	EffectiveDate  *string
	Name           *string
	ParentOrgCode  *string
	IsBusinessUnit *bool
	ManagerPernr   *string
}

type orgUnitWriteService struct {
	store ports.OrgUnitWriteStore
}

func NewOrgUnitWriteService(store ports.OrgUnitWriteStore) OrgUnitWriteService {
	return &orgUnitWriteService{store: store}
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

	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitEvent(ctx, tenantID, eventUUID, nil, string(types.OrgUnitEventCreate), effectiveDate, payloadJSON, eventUUID, tenantID); err != nil {
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

	payload := map[string]any{"new_name": newName}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventRename), effectiveDate, payloadJSON, eventUUID, tenantID)
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

	payload := map[string]any{"new_parent_id": parentID}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventMove), effectiveDate, payloadJSON, eventUUID, tenantID)
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

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventDisable), effectiveDate, json.RawMessage(`{}`), eventUUID, tenantID)
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

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventEnable), effectiveDate, json.RawMessage(`{}`), eventUUID, tenantID)
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

	payload := map[string]any{"is_business_unit": req.IsBusinessUnit}
	payloadJSON, err := marshalJSON(payload)
	if err != nil {
		return err
	}

	eventUUID, err := newUUID()
	if err != nil {
		return err
	}

	_, err = s.store.SubmitEvent(ctx, tenantID, eventUUID, &orgID, string(types.OrgUnitEventSetBusinessUnit), effectiveDate, payloadJSON, eventUUID, tenantID)
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

	patch, fields, correctedDate, err := s.buildCorrectionPatch(ctx, tenantID, event, req.Patch)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	patchJSON, err := marshalJSON(patch)
	if err != nil {
		return types.OrgUnitResult{}, err
	}

	if _, err := s.store.SubmitCorrection(ctx, tenantID, orgID, targetEffectiveDate, patchJSON, requestID, tenantID); err != nil {
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

	if _, err := s.store.SubmitRescindEvent(ctx, tenantID, orgID, targetEffectiveDate, reason, requestID, tenantID); err != nil {
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

	rescindedEvents, err := s.store.SubmitRescindOrg(ctx, tenantID, orgID, reason, requestID, tenantID)
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

func (s *orgUnitWriteService) buildCorrectionPatch(ctx context.Context, tenantID string, event types.OrgUnitEvent, patch OrgUnitCorrectionPatch) (map[string]any, map[string]any, string, error) {
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

	if len(patchMap) == 0 {
		return nil, nil, "", httperr.NewBadRequest(errPatchRequired)
	}

	return patchMap, fields, correctedDate, nil
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

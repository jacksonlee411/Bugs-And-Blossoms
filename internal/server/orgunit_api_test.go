package server

import (
	"context"

	orgunittypes "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/domain/types"
	orgunitservices "github.com/jacksonlee411/Bugs-And-Blossoms/modules/orgunit/services"
)

type resolveOrgCodeStore struct {
	resolveID  int
	resolveErr error

	setErr  error
	setArgs []string

	listChildrenByNodeKeyArg string
	detailsByNodeKeyArg      string
	versionsByNodeKeyArg     string
	auditByNodeKeyArg        string
	resolveCodeByNodeKeyArg  string

	listNodes    []OrgUnitNode
	listNodesErr error

	listChildren    []OrgUnitChild
	listChildrenErr error

	resolveCodes    map[int]string
	resolveCodesErr error

	getNodeDetails    OrgUnitNodeDetails
	getNodeDetailsErr error

	searchNodeResult    OrgUnitSearchResult
	searchNodeErr       error
	searchCandidates    []OrgUnitSearchCandidate
	searchCandidatesErr error

	listNodeVersions    []OrgUnitNodeVersion
	listNodeVersionsErr error

	auditEvents    []OrgUnitNodeAuditEvent
	auditEventsErr error
}

func (s *resolveOrgCodeStore) ListNodesCurrent(context.Context, string, string) ([]OrgUnitNode, error) {
	if s.listNodesErr != nil {
		return nil, s.listNodesErr
	}
	return append([]OrgUnitNode(nil), s.listNodes...), nil
}

func (s *resolveOrgCodeStore) CreateNodeCurrent(context.Context, string, string, string, string, string, bool) (OrgUnitNode, error) {
	return OrgUnitNode{}, nil
}

func (s *resolveOrgCodeStore) RenameNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}

func (s *resolveOrgCodeStore) MoveNodeCurrent(context.Context, string, string, string, string) error {
	return nil
}

func (s *resolveOrgCodeStore) DisableNodeCurrent(context.Context, string, string, string) error {
	return nil
}

func (s *resolveOrgCodeStore) SetBusinessUnitCurrent(_ context.Context, tenantID string, effectiveDate string, orgID string, _ bool, requestID string) error {
	s.setArgs = []string{tenantID, effectiveDate, orgID, requestID}
	return s.setErr
}

func (s *resolveOrgCodeStore) ResolveOrgID(context.Context, string, string) (int, error) {
	if s.resolveErr != nil {
		return 0, s.resolveErr
	}
	return s.resolveID, nil
}

func (s *resolveOrgCodeStore) ResolveOrgNodeKeyByCode(_ context.Context, _ string, _ string) (string, error) {
	if s.resolveErr != nil {
		return "", s.resolveErr
	}
	if s.resolveID == 0 {
		return "", nil
	}
	return encodeOrgNodeKeyFromID(s.resolveID)
}

func (s *resolveOrgCodeStore) ResolveOrgCode(_ context.Context, _ string, orgID int) (string, error) {
	if s.resolveCodes != nil {
		if code, ok := s.resolveCodes[orgID]; ok {
			return code, nil
		}
	}
	return "", nil
}

func (s *resolveOrgCodeStore) ResolveOrgCodeByNodeKey(ctx context.Context, tenantID string, orgNodeKey string) (string, error) {
	s.resolveCodeByNodeKeyArg = orgNodeKey
	orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
	if err != nil {
		return "", err
	}
	return s.ResolveOrgCode(ctx, tenantID, orgID)
}

func (s *resolveOrgCodeStore) ResolveOrgCodes(context.Context, string, []int) (map[int]string, error) {
	if s.resolveCodesErr != nil {
		return nil, s.resolveCodesErr
	}
	if s.resolveCodes != nil {
		return s.resolveCodes, nil
	}
	return map[int]string{}, nil
}

func (s *resolveOrgCodeStore) ResolveOrgCodesByNodeKeys(ctx context.Context, tenantID string, orgNodeKeys []string) (map[string]string, error) {
	if s.resolveCodesErr != nil {
		return nil, s.resolveCodesErr
	}
	out := make(map[string]string, len(orgNodeKeys))
	for _, orgNodeKey := range orgNodeKeys {
		orgID, err := decodeOrgNodeKeyToID(orgNodeKey)
		if err != nil {
			return nil, err
		}
		code, err := s.ResolveOrgCode(ctx, tenantID, orgID)
		if err != nil {
			return nil, err
		}
		out[orgNodeKey] = code
	}
	return out, nil
}

func (s *resolveOrgCodeStore) ListChildren(context.Context, string, int, string) ([]OrgUnitChild, error) {
	if s.listChildrenErr != nil {
		return nil, s.listChildrenErr
	}
	return append([]OrgUnitChild(nil), s.listChildren...), nil
}

func (s *resolveOrgCodeStore) ListChildrenByNodeKey(_ context.Context, _ string, parentOrgNodeKey string, _ string) ([]OrgUnitChild, error) {
	s.listChildrenByNodeKeyArg = parentOrgNodeKey
	if s.listChildrenErr != nil {
		return nil, s.listChildrenErr
	}
	return append([]OrgUnitChild(nil), s.listChildren...), nil
}

func (s *resolveOrgCodeStore) GetNodeDetails(context.Context, string, int, string) (OrgUnitNodeDetails, error) {
	if s.getNodeDetailsErr != nil {
		return OrgUnitNodeDetails{}, s.getNodeDetailsErr
	}
	return s.getNodeDetails, nil
}

func (s *resolveOrgCodeStore) GetNodeDetailsByNodeKey(_ context.Context, _ string, orgNodeKey string, _ string) (OrgUnitNodeDetails, error) {
	s.detailsByNodeKeyArg = orgNodeKey
	if s.getNodeDetailsErr != nil {
		return OrgUnitNodeDetails{}, s.getNodeDetailsErr
	}
	return s.getNodeDetails, nil
}

func (s *resolveOrgCodeStore) SearchNode(context.Context, string, string, string) (OrgUnitSearchResult, error) {
	if s.searchNodeErr != nil {
		return OrgUnitSearchResult{}, s.searchNodeErr
	}
	return s.searchNodeResult, nil
}

func (s *resolveOrgCodeStore) SearchNodeCandidates(context.Context, string, string, string, int) ([]OrgUnitSearchCandidate, error) {
	if s.searchCandidatesErr != nil {
		return nil, s.searchCandidatesErr
	}
	return append([]OrgUnitSearchCandidate(nil), s.searchCandidates...), nil
}

func (s *resolveOrgCodeStore) ListNodeVersions(context.Context, string, int) ([]OrgUnitNodeVersion, error) {
	if s.listNodeVersionsErr != nil {
		return nil, s.listNodeVersionsErr
	}
	return append([]OrgUnitNodeVersion(nil), s.listNodeVersions...), nil
}

func (s *resolveOrgCodeStore) ListNodeVersionsByNodeKey(_ context.Context, _ string, orgNodeKey string) ([]OrgUnitNodeVersion, error) {
	s.versionsByNodeKeyArg = orgNodeKey
	if s.listNodeVersionsErr != nil {
		return nil, s.listNodeVersionsErr
	}
	return append([]OrgUnitNodeVersion(nil), s.listNodeVersions...), nil
}

func (s *resolveOrgCodeStore) ListNodeAuditEvents(context.Context, string, int, int) ([]OrgUnitNodeAuditEvent, error) {
	if s.auditEventsErr != nil {
		return nil, s.auditEventsErr
	}
	return append([]OrgUnitNodeAuditEvent(nil), s.auditEvents...), nil
}

func (s *resolveOrgCodeStore) ListNodeAuditEventsByNodeKey(_ context.Context, _ string, orgNodeKey string, _ int) ([]OrgUnitNodeAuditEvent, error) {
	s.auditByNodeKeyArg = orgNodeKey
	if s.auditEventsErr != nil {
		return nil, s.auditEventsErr
	}
	return append([]OrgUnitNodeAuditEvent(nil), s.auditEvents...), nil
}

func (s *resolveOrgCodeStore) MaxEffectiveDateOnOrBefore(context.Context, string, string) (string, bool, error) {
	return "", false, nil
}

func (s *resolveOrgCodeStore) MinEffectiveDate(context.Context, string) (string, bool, error) {
	return "", false, nil
}

type orgUnitListPageReaderStore struct {
	*resolveOrgCodeStore
	items       []orgUnitListItem
	total       int
	err         error
	capturedReq orgUnitListPageRequest
}

func (s *orgUnitListPageReaderStore) ListOrgUnitsPage(_ context.Context, _ string, req orgUnitListPageRequest) ([]orgUnitListItem, int, error) {
	s.capturedReq = req
	if s.err != nil {
		return nil, 0, s.err
	}
	return append([]orgUnitListItem(nil), s.items...), s.total, nil
}

type orgUnitDetailsExtStoreStub struct {
	*resolveOrgCodeStore
	cfgs                 []orgUnitTenantFieldConfig
	cfgErr               error
	snapshot             orgUnitVersionExtSnapshot
	snapshotErr          error
	snapshotOrgIDArg     int
	snapshotByNodeKeyArg string
}

func (s *orgUnitDetailsExtStoreStub) ListEnabledTenantFieldConfigsAsOf(_ context.Context, _ string, _ string) ([]orgUnitTenantFieldConfig, error) {
	if s.cfgErr != nil {
		return nil, s.cfgErr
	}
	return append([]orgUnitTenantFieldConfig(nil), s.cfgs...), nil
}

func (s *orgUnitDetailsExtStoreStub) GetOrgUnitVersionExtSnapshot(_ context.Context, _ string, orgID int, _ string) (orgUnitVersionExtSnapshot, error) {
	s.snapshotOrgIDArg = orgID
	if s.snapshotErr != nil {
		return orgUnitVersionExtSnapshot{}, s.snapshotErr
	}
	return s.snapshot, nil
}

func (s *orgUnitDetailsExtStoreStub) GetOrgUnitVersionExtSnapshotByNodeKey(_ context.Context, _ string, orgNodeKey string, _ string) (orgUnitVersionExtSnapshot, error) {
	s.snapshotByNodeKeyArg = orgNodeKey
	if s.snapshotErr != nil {
		return orgUnitVersionExtSnapshot{}, s.snapshotErr
	}
	return s.snapshot, nil
}

type orgUnitWriteServiceStub struct {
	writeFn           func(context.Context, string, orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error)
	createFn          func(context.Context, string, orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	renameFn          func(context.Context, string, orgunitservices.RenameOrgUnitRequest) error
	moveFn            func(context.Context, string, orgunitservices.MoveOrgUnitRequest) error
	disableFn         func(context.Context, string, orgunitservices.DisableOrgUnitRequest) error
	enableFn          func(context.Context, string, orgunitservices.EnableOrgUnitRequest) error
	setBusinessUnitFn func(context.Context, string, orgunitservices.SetBusinessUnitRequest) error
	correctFn         func(context.Context, string, orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	correctStatusFn   func(context.Context, string, orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	rescindRecordFn   func(context.Context, string, orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
	rescindOrgFn      func(context.Context, string, orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error)
}

func (s orgUnitWriteServiceStub) Write(ctx context.Context, tenantID string, req orgunitservices.WriteOrgUnitRequest) (orgunitservices.OrgUnitWriteResult, error) {
	if s.writeFn == nil {
		return orgunitservices.OrgUnitWriteResult{}, nil
	}
	return s.writeFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Create(ctx context.Context, tenantID string, req orgunitservices.CreateOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.createFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.createFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Rename(ctx context.Context, tenantID string, req orgunitservices.RenameOrgUnitRequest) error {
	if s.renameFn == nil {
		return nil
	}
	return s.renameFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Move(ctx context.Context, tenantID string, req orgunitservices.MoveOrgUnitRequest) error {
	if s.moveFn == nil {
		return nil
	}
	return s.moveFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Disable(ctx context.Context, tenantID string, req orgunitservices.DisableOrgUnitRequest) error {
	if s.disableFn == nil {
		return nil
	}
	return s.disableFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Enable(ctx context.Context, tenantID string, req orgunitservices.EnableOrgUnitRequest) error {
	if s.enableFn == nil {
		return nil
	}
	return s.enableFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) SetBusinessUnit(ctx context.Context, tenantID string, req orgunitservices.SetBusinessUnitRequest) error {
	if s.setBusinessUnitFn == nil {
		return nil
	}
	return s.setBusinessUnitFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) Correct(ctx context.Context, tenantID string, req orgunitservices.CorrectOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.correctFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.correctFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) CorrectStatus(ctx context.Context, tenantID string, req orgunitservices.CorrectStatusOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.correctStatusFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.correctStatusFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) RescindRecord(ctx context.Context, tenantID string, req orgunitservices.RescindRecordOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.rescindRecordFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.rescindRecordFn(ctx, tenantID, req)
}

func (s orgUnitWriteServiceStub) RescindOrg(ctx context.Context, tenantID string, req orgunitservices.RescindOrgUnitRequest) (orgunittypes.OrgUnitResult, error) {
	if s.rescindOrgFn == nil {
		return orgunittypes.OrgUnitResult{}, nil
	}
	return s.rescindOrgFn(ctx, tenantID, req)
}

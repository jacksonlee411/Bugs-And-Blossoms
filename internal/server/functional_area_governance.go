package server

import (
	"strings"
	"sync"
)

const (
	functionalAreaMissingCode     = "FUNCTIONAL_AREA_MISSING"
	functionalAreaDisabledCode    = "FUNCTIONAL_AREA_DISABLED"
	functionalAreaNotActiveCode   = "FUNCTIONAL_AREA_NOT_ACTIVE"
	functionalAreaLifecycleActive = "active"
)

var functionalAreaLifecycleByKey = map[string]string{
	"org_foundation": functionalAreaLifecycleActive,
	"staffing":       functionalAreaLifecycleActive,
	"jobcatalog":     functionalAreaLifecycleActive,
	"person":         functionalAreaLifecycleActive,
	"iam_platform":   functionalAreaLifecycleActive,
	"compensation":   "reserved",
	"benefits":       "reserved",
}

type functionalAreaSwitchStore struct {
	mu       sync.RWMutex
	disabled map[string]map[string]struct{}
}

func newFunctionalAreaSwitchStore() *functionalAreaSwitchStore {
	return &functionalAreaSwitchStore{
		disabled: make(map[string]map[string]struct{}),
	}
}

func (s *functionalAreaSwitchStore) isEnabled(tenantID string, functionalAreaKey string) bool {
	tenantID = strings.TrimSpace(tenantID)
	functionalAreaKey = strings.TrimSpace(functionalAreaKey)
	if tenantID == "" || functionalAreaKey == "" {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	areas, ok := s.disabled[tenantID]
	if !ok {
		return true
	}
	_, blocked := areas[functionalAreaKey]
	return !blocked
}

func (s *functionalAreaSwitchStore) setEnabled(tenantID string, functionalAreaKey string, enabled bool) {
	tenantID = strings.TrimSpace(tenantID)
	functionalAreaKey = strings.TrimSpace(functionalAreaKey)
	if tenantID == "" || functionalAreaKey == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if enabled {
		areas, ok := s.disabled[tenantID]
		if !ok {
			return
		}
		delete(areas, functionalAreaKey)
		if len(areas) == 0 {
			delete(s.disabled, tenantID)
		}
		return
	}
	areas, ok := s.disabled[tenantID]
	if !ok {
		areas = make(map[string]struct{})
		s.disabled[tenantID] = areas
	}
	areas[functionalAreaKey] = struct{}{}
}

var defaultFunctionalAreaSwitchStore = newFunctionalAreaSwitchStore()

func resetFunctionalAreaSwitchStoreForTest() {
	defaultFunctionalAreaSwitchStore = newFunctionalAreaSwitchStore()
}

func evaluateFunctionalAreaGate(tenantID string, capabilityKey string) (functionalAreaKey string, reasonCode string, allowed bool) {
	definition, ok := capabilityDefinitionForKey(capabilityKey)
	if !ok {
		return "", functionalAreaMissingCode, false
	}
	functionalAreaKey = strings.TrimSpace(definition.FunctionalAreaKey)
	if functionalAreaKey == "" {
		return "", functionalAreaMissingCode, false
	}
	lifecycle, ok := functionalAreaLifecycleByKey[functionalAreaKey]
	if !ok || strings.TrimSpace(lifecycle) == "" {
		return functionalAreaKey, functionalAreaMissingCode, false
	}
	if lifecycle != functionalAreaLifecycleActive {
		return functionalAreaKey, functionalAreaNotActiveCode, false
	}
	if strings.TrimSpace(definition.Status) != routeCapabilityStatusActive {
		return functionalAreaKey, functionalAreaDisabledCode, false
	}
	if !defaultFunctionalAreaSwitchStore.isEnabled(tenantID, functionalAreaKey) {
		return functionalAreaKey, functionalAreaDisabledCode, false
	}
	return functionalAreaKey, "", true
}

func functionalAreaErrorMessage(reasonCode string) string {
	switch strings.TrimSpace(reasonCode) {
	case functionalAreaMissingCode:
		return "functional area missing"
	case functionalAreaDisabledCode:
		return "functional area disabled"
	case functionalAreaNotActiveCode:
		return "functional area not active"
	default:
		return "functional area blocked"
	}
}

func resolveFunctionalAreaKey(capabilityKey string) string {
	definition, ok := capabilityDefinitionForKey(capabilityKey)
	if !ok {
		return ""
	}
	return strings.TrimSpace(definition.FunctionalAreaKey)
}

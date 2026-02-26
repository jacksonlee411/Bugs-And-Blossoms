package server

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

type capabilityCatalogMetadata struct {
	TargetObject string
	Surface      string
	Intent       string
}

type capabilityCatalogEntry struct {
	Module        string   `json:"module"`
	OwnerModule   string   `json:"owner_module"`
	TargetObject  string   `json:"target_object"`
	Surface       string   `json:"surface"`
	Intent        string   `json:"intent"`
	CapabilityKey string   `json:"capability_key"`
	RouteClass    string   `json:"route_class,omitempty"`
	Actions       []string `json:"actions,omitempty"`
	Status        string   `json:"status"`
}

type capabilityCatalogFilter struct {
	OwnerModule   string
	TargetObject  string
	Surface       string
	Intent        string
	CapabilityKey string
}

type capabilityCatalogResponse struct {
	Items []capabilityCatalogEntry `json:"items"`
}

var capabilityCatalogMetadataByKey = map[string]capabilityCatalogMetadata{
	"staffing.assignment_create.field_policy": {
		TargetObject: "assignment",
		Surface:      "api_write",
		Intent:       "create",
	},
	"org.policy_activation.manage": {
		TargetObject: "policy_activation",
		Surface:      "activation_console",
		Intent:       "manage",
	},
	"org.orgunit_write.field_policy": {
		TargetObject: "orgunit",
		Surface:      "api_write",
		Intent:       "write_all",
	},
	"org.orgunit_create.field_policy": {
		TargetObject: "orgunit",
		Surface:      "create_dialog",
		Intent:       "create_org",
	},
	"org.orgunit_add_version.field_policy": {
		TargetObject: "orgunit",
		Surface:      "details_dialog",
		Intent:       "add_version",
	},
	"org.orgunit_insert_version.field_policy": {
		TargetObject: "orgunit",
		Surface:      "details_dialog",
		Intent:       "insert_version",
	},
	"org.orgunit_correct.field_policy": {
		TargetObject: "orgunit",
		Surface:      "details_dialog",
		Intent:       "correct",
	},
}

var capabilityCatalogEntries = buildCapabilityCatalogEntries(capabilityDefinitions, capabilityRouteBindings, capabilityCatalogMetadataByKey)
var capabilityCatalogByCapabilityKey = buildCapabilityCatalogByKey(capabilityCatalogEntries)

func buildCapabilityCatalogEntries(
	definitions []capabilityDefinition,
	bindings []capabilityRouteBinding,
	metadataByKey map[string]capabilityCatalogMetadata,
) []capabilityCatalogEntry {
	type routeMeta struct {
		routeClass string
		actions    []string
	}
	routeIndex := make(map[string]routeMeta)
	for _, binding := range bindings {
		key := strings.ToLower(strings.TrimSpace(binding.CapabilityKey))
		if key == "" {
			continue
		}
		rm := routeIndex[key]
		if rm.routeClass == "" {
			rm.routeClass = strings.TrimSpace(binding.RouteClass)
		}
		action := strings.TrimSpace(binding.Action)
		if action != "" && !slices.Contains(rm.actions, action) {
			rm.actions = append(rm.actions, action)
			slices.Sort(rm.actions)
		}
		routeIndex[key] = rm
	}

	entries := make([]capabilityCatalogEntry, 0, len(definitions))
	seenObjectIntent := make(map[string]string)
	for _, definition := range definitions {
		capabilityKey := strings.ToLower(strings.TrimSpace(definition.CapabilityKey))
		if capabilityKey == "" {
			continue
		}
		metadata, ok := metadataByKey[capabilityKey]
		if !ok {
			continue
		}
		ownerModule := strings.ToLower(strings.TrimSpace(definition.OwnerModule))
		entry := capabilityCatalogEntry{
			Module:        ownerModule,
			OwnerModule:   ownerModule,
			TargetObject:  strings.ToLower(strings.TrimSpace(metadata.TargetObject)),
			Surface:       strings.ToLower(strings.TrimSpace(metadata.Surface)),
			Intent:        strings.ToLower(strings.TrimSpace(metadata.Intent)),
			CapabilityKey: capabilityKey,
			Status:        strings.ToLower(strings.TrimSpace(definition.Status)),
		}
		if routeMeta, ok := routeIndex[capabilityKey]; ok {
			entry.RouteClass = strings.TrimSpace(routeMeta.routeClass)
			entry.Actions = append([]string(nil), routeMeta.actions...)
		}
		objectIntentKey := strings.Join([]string{entry.TargetObject, entry.Surface, entry.Intent}, "|")
		if existingCapability, exists := seenObjectIntent[objectIntentKey]; exists {
			// 配置冲突时 fail-closed：只保留第一条，后续由测试与门禁阻断。
			if existingCapability != entry.CapabilityKey {
				continue
			}
		}
		seenObjectIntent[objectIntentKey] = entry.CapabilityKey
		entries = append(entries, entry)
	}

	slices.SortFunc(entries, func(a, b capabilityCatalogEntry) int {
		return cmp.Or(
			strings.Compare(a.OwnerModule, b.OwnerModule),
			strings.Compare(a.TargetObject, b.TargetObject),
			strings.Compare(a.Surface, b.Surface),
			strings.Compare(a.Intent, b.Intent),
			strings.Compare(a.CapabilityKey, b.CapabilityKey),
		)
	})
	return entries
}

func buildCapabilityCatalogByKey(entries []capabilityCatalogEntry) map[string]capabilityCatalogEntry {
	index := make(map[string]capabilityCatalogEntry, len(entries))
	for _, entry := range entries {
		index[strings.ToLower(strings.TrimSpace(entry.CapabilityKey))] = entry
	}
	return index
}

func capabilityCatalogEntryForCapabilityKey(capabilityKey string) (capabilityCatalogEntry, bool) {
	entry, ok := capabilityCatalogByCapabilityKey[strings.ToLower(strings.TrimSpace(capabilityKey))]
	return entry, ok
}

func listCapabilityCatalog(filter capabilityCatalogFilter) []capabilityCatalogEntry {
	ownerModule := strings.ToLower(strings.TrimSpace(filter.OwnerModule))
	targetObject := strings.ToLower(strings.TrimSpace(filter.TargetObject))
	surface := strings.ToLower(strings.TrimSpace(filter.Surface))
	intent := strings.ToLower(strings.TrimSpace(filter.Intent))
	capabilityKey := strings.ToLower(strings.TrimSpace(filter.CapabilityKey))

	items := make([]capabilityCatalogEntry, 0, len(capabilityCatalogEntries))
	for _, entry := range capabilityCatalogEntries {
		if ownerModule != "" && entry.OwnerModule != ownerModule {
			continue
		}
		if targetObject != "" && entry.TargetObject != targetObject {
			continue
		}
		if surface != "" && entry.Surface != surface {
			continue
		}
		if intent != "" && entry.Intent != intent {
			continue
		}
		if capabilityKey != "" && entry.CapabilityKey != capabilityKey {
			continue
		}
		items = append(items, entry)
	}
	return items
}

func parseCapabilityCatalogFilter(r *http.Request) (capabilityCatalogFilter, bool) {
	ownerModule := strings.TrimSpace(r.URL.Query().Get("owner_module"))
	module := strings.TrimSpace(r.URL.Query().Get("module"))
	if ownerModule != "" && module != "" && !strings.EqualFold(ownerModule, module) {
		return capabilityCatalogFilter{}, false
	}
	if ownerModule == "" {
		ownerModule = module
	}
	return capabilityCatalogFilter{
		OwnerModule:   ownerModule,
		TargetObject:  strings.TrimSpace(r.URL.Query().Get("target_object")),
		Surface:       strings.TrimSpace(r.URL.Query().Get("surface")),
		Intent:        strings.TrimSpace(r.URL.Query().Get("intent")),
		CapabilityKey: strings.TrimSpace(r.URL.Query().Get("capability_key")),
	}, true
}

func handleCapabilityCatalogAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := currentTenant(r.Context()); !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	filter, ok := parseCapabilityCatalogFilter(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "module and owner_module mismatch")
		return
	}
	items := listCapabilityCatalog(filter)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(capabilityCatalogResponse{Items: items})
}

func handleCapabilityCatalogByIntentAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if _, ok := currentTenant(r.Context()); !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}
	filter, ok := parseCapabilityCatalogFilter(r)
	if !ok {
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, "invalid_request", "module and owner_module mismatch")
		return
	}
	items := listCapabilityCatalog(filter)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(capabilityCatalogResponse{Items: items})
}

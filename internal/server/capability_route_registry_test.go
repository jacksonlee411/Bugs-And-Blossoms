package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/authz"
)

type capabilityRouteContract struct {
	Capabilities []capabilityDefinition   `json:"capabilities"`
	Routes       []capabilityRouteBinding `json:"routes"`
}

func TestCapabilityRouteBindingKey(t *testing.T) {
	key := capabilityRouteBindingKey(" post ", " /internal/rules/evaluate ")
	if key != "POST /internal/rules/evaluate" {
		t.Fatalf("key=%q", key)
	}
}

func TestCapabilityRouteBindingForRoute(t *testing.T) {
	if definition, ok := capabilityDefinitionForKey(" org.orgunit_write.field_policy "); !ok || definition.FunctionalAreaKey != "org_foundation" {
		t.Fatalf("definition=%+v ok=%v", definition, ok)
	}
	if _, ok := capabilityDefinitionForKey("unknown.key"); ok {
		t.Fatal("expected unknown capability missing")
	}

	binding, ok := capabilityRouteBindingForRoute("GET", "/org/api/setid-strategy-registry")
	if !ok {
		t.Fatal("expected mapping found")
	}
	if binding.CapabilityKey != orgUnitWriteFieldPolicyCapabilityKey || binding.Action != authz.ActionRead {
		t.Fatalf("binding=%+v", binding)
	}

	if _, ok := capabilityRouteBindingForRoute("DELETE", "/org/api/setid-strategy-registry"); ok {
		t.Fatal("expected mapping missing")
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/org/api/setid-strategy-registry:disable"); !ok || binding.Action != authz.ActionAdmin {
		t.Fatalf("expected disable mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/conversations/conv_001"); !ok || binding.CapabilityKey != "org.assistant_conversation.manage" {
		t.Fatalf("expected assistant conversation template mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/conversations/conv_001/turns/turn_001:confirm"); !ok || binding.CapabilityKey != "org.assistant_conversation.manage" {
		t.Fatalf("expected assistant turn action template mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/conversations"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant conversation list mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/tasks"); !ok || binding.CapabilityKey != "org.assistant_conversation.manage" {
		t.Fatalf("expected assistant tasks submit mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/tasks/123"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant task detail mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/tasks/123:cancel"); !ok || binding.Action != authz.ActionAdmin {
		t.Fatalf("expected assistant task cancel mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/model-providers"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant model providers mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/model-providers:validate"); !ok || binding.Action != authz.ActionAdmin {
		t.Fatalf("expected assistant model providers validate mapping found, got=%+v ok=%v", binding, ok)
	}
	if _, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/model-providers:apply"); ok {
		t.Fatal("expected retired assistant model providers apply mapping missing")
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/models"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant models mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/runtime-status"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant runtime status mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/cubebox/conversations"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected cubebox conversation list mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/cubebox/conversations/conv_001/turns/turn_001:commit"); !ok || binding.Action != authz.ActionAdmin {
		t.Fatalf("expected cubebox turn action mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/cubebox/runtime-status"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected cubebox runtime status mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/ui-bootstrap"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant ui bootstrap mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("GET", "/internal/assistant/session"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant session mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/session/refresh"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant session refresh mapping found, got=%+v ok=%v", binding, ok)
	}
	if binding, ok := capabilityRouteBindingForRoute("POST", "/internal/assistant/session/logout"); !ok || binding.Action != authz.ActionRead {
		t.Fatalf("expected assistant session logout mapping found, got=%+v ok=%v", binding, ok)
	}
}

func TestCapabilityAuthzRequirementForBinding(t *testing.T) {
	object, action, ok := capabilityAuthzRequirementForBinding(capabilityRouteBinding{
		Action: authz.ActionAdmin,
	})
	if !ok || object != authz.ObjectOrgSetIDCapability || action != authz.ActionAdmin {
		t.Fatalf("ok=%v object=%q action=%q", ok, object, action)
	}

	if _, _, ok := capabilityAuthzRequirementForBinding(capabilityRouteBinding{Action: "write"}); ok {
		t.Fatal("expected invalid action rejected")
	}

	if _, _, ok := capabilityAuthzRequirementForRoute(httpMethodDelete, "/internal/rules/evaluate"); ok {
		t.Fatal("expected unsupported route method rejected")
	}
}

func TestCapabilityRouteRegistryContract(t *testing.T) {
	contractPath := filepath.Clean(filepath.Join("..", "..", "config", "capability", "route-capability-map.v1.json"))
	data, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read contract=%v", err)
	}
	var contract capabilityRouteContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("unmarshal contract=%v", err)
	}
	if len(contract.Routes) == 0 {
		t.Fatal("expected contract routes")
	}
	if len(contract.Capabilities) == 0 {
		t.Fatal("expected contract capabilities")
	}

	got := make([]capabilityRouteBinding, len(capabilityRouteBindings))
	copy(got, capabilityRouteBindings)
	slices.SortFunc(got, func(a capabilityRouteBinding, b capabilityRouteBinding) int {
		if a.Method != b.Method {
			if a.Method < b.Method {
				return -1
			}
			return 1
		}
		if a.Path != b.Path {
			if a.Path < b.Path {
				return -1
			}
			return 1
		}
		return 0
	})

	want := make([]capabilityRouteBinding, len(contract.Routes))
	copy(want, contract.Routes)
	slices.SortFunc(want, func(a capabilityRouteBinding, b capabilityRouteBinding) int {
		if a.Method != b.Method {
			if a.Method < b.Method {
				return -1
			}
			return 1
		}
		if a.Path != b.Path {
			if a.Path < b.Path {
				return -1
			}
			return 1
		}
		return 0
	})

	if len(got) != len(want) {
		t.Fatalf("route count mismatch got=%d want=%d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("route mismatch index=%d got=%+v want=%+v", i, got[i], want[i])
		}
	}

	gotCapabilities := make([]capabilityDefinition, len(capabilityDefinitions))
	copy(gotCapabilities, capabilityDefinitions)
	slices.SortFunc(gotCapabilities, func(a capabilityDefinition, b capabilityDefinition) int {
		if a.CapabilityKey < b.CapabilityKey {
			return -1
		}
		if a.CapabilityKey > b.CapabilityKey {
			return 1
		}
		return 0
	})
	wantCapabilities := make([]capabilityDefinition, len(contract.Capabilities))
	copy(wantCapabilities, contract.Capabilities)
	slices.SortFunc(wantCapabilities, func(a capabilityDefinition, b capabilityDefinition) int {
		if a.CapabilityKey < b.CapabilityKey {
			return -1
		}
		if a.CapabilityKey > b.CapabilityKey {
			return 1
		}
		return 0
	})
	if len(gotCapabilities) != len(wantCapabilities) {
		t.Fatalf("capability count mismatch got=%d want=%d", len(gotCapabilities), len(wantCapabilities))
	}
	for i := range gotCapabilities {
		if gotCapabilities[i] != wantCapabilities[i] {
			t.Fatalf("capability mismatch index=%d got=%+v want=%+v", i, gotCapabilities[i], wantCapabilities[i])
		}
	}
}

const httpMethodDelete = "DELETE"

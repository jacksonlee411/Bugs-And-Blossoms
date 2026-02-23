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
	Routes []capabilityRouteBinding `json:"routes"`
}

func TestCapabilityRouteBindingKey(t *testing.T) {
	key := capabilityRouteBindingKey(" post ", " /internal/rules/evaluate ")
	if key != "POST /internal/rules/evaluate" {
		t.Fatalf("key=%q", key)
	}
}

func TestCapabilityRouteBindingForRoute(t *testing.T) {
	binding, ok := capabilityRouteBindingForRoute("GET", "/org/api/setid-strategy-registry")
	if !ok {
		t.Fatal("expected mapping found")
	}
	if binding.CapabilityKey != "staffing.assignment_create.field_policy" || binding.Action != authz.ActionRead {
		t.Fatalf("binding=%+v", binding)
	}

	if _, ok := capabilityRouteBindingForRoute("DELETE", "/org/api/setid-strategy-registry"); ok {
		t.Fatal("expected mapping missing")
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
}

const httpMethodDelete = "DELETE"

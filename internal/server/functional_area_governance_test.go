package server

import "testing"

func TestFunctionalAreaSwitchStore(t *testing.T) {
	store := newFunctionalAreaSwitchStore()
	if !store.isEnabled("t1", "staffing") {
		t.Fatal("expected default enabled")
	}
	if !store.isEnabled("", "staffing") {
		t.Fatal("expected empty tenant enabled")
	}
	if !store.isEnabled("t1", "") {
		t.Fatal("expected empty functional area enabled")
	}

	store.setEnabled("t1", "staffing", false)
	if store.isEnabled("t1", "staffing") {
		t.Fatal("expected disabled")
	}

	store.setEnabled("t1", "jobcatalog", false)
	store.setEnabled("t1", "staffing", true)
	if !store.isEnabled("t1", "staffing") {
		t.Fatal("expected re-enabled")
	}
	if store.isEnabled("t1", "jobcatalog") {
		t.Fatal("expected jobcatalog still disabled")
	}

	store.setEnabled("t2", "staffing", true)

	store.setEnabled("", "staffing", false)
	store.setEnabled("t1", "", false)
}

func TestEvaluateFunctionalAreaGate(t *testing.T) {
	resetFunctionalAreaSwitchStoreForTest()
	t.Cleanup(resetFunctionalAreaSwitchStoreForTest)

	if _, reason, ok := evaluateFunctionalAreaGate("t1", "unknown.key"); ok || reason != functionalAreaMissingCode {
		t.Fatalf("ok=%v reason=%q", ok, reason)
	}

	missingAreaKey := "staffing.assignment_create.missing_area"
	capabilityDefinitionByKey[missingAreaKey] = capabilityDefinition{
		CapabilityKey:     missingAreaKey,
		FunctionalAreaKey: "",
		Status:            routeCapabilityStatusActive,
	}
	t.Cleanup(func() { delete(capabilityDefinitionByKey, missingAreaKey) })
	if _, reason, ok := evaluateFunctionalAreaGate("t1", missingAreaKey); ok || reason != functionalAreaMissingCode {
		t.Fatalf("ok=%v reason=%q", ok, reason)
	}

	missingLifecycleKey := "staffing.assignment_create.unknown_area"
	capabilityDefinitionByKey[missingLifecycleKey] = capabilityDefinition{
		CapabilityKey:     missingLifecycleKey,
		FunctionalAreaKey: "unknown_area",
		Status:            routeCapabilityStatusActive,
	}
	t.Cleanup(func() { delete(capabilityDefinitionByKey, missingLifecycleKey) })
	if area, reason, ok := evaluateFunctionalAreaGate("t1", missingLifecycleKey); ok || reason != functionalAreaMissingCode || area != "unknown_area" {
		t.Fatalf("ok=%v area=%q reason=%q", ok, area, reason)
	}

	reservedAreaKey := "staffing.assignment_create.reserved_area"
	capabilityDefinitionByKey[reservedAreaKey] = capabilityDefinition{
		CapabilityKey:     reservedAreaKey,
		FunctionalAreaKey: "benefits",
		Status:            routeCapabilityStatusActive,
	}
	t.Cleanup(func() { delete(capabilityDefinitionByKey, reservedAreaKey) })
	if area, reason, ok := evaluateFunctionalAreaGate("t1", reservedAreaKey); ok || reason != functionalAreaNotActiveCode || area != "benefits" {
		t.Fatalf("ok=%v area=%q reason=%q", ok, area, reason)
	}

	disabledStatusKey := "staffing.assignment_create.disabled_status"
	capabilityDefinitionByKey[disabledStatusKey] = capabilityDefinition{
		CapabilityKey:     disabledStatusKey,
		FunctionalAreaKey: "staffing",
		Status:            "deprecated",
	}
	t.Cleanup(func() { delete(capabilityDefinitionByKey, disabledStatusKey) })
	if area, reason, ok := evaluateFunctionalAreaGate("t1", disabledStatusKey); ok || reason != functionalAreaDisabledCode || area != "staffing" {
		t.Fatalf("ok=%v area=%q reason=%q", ok, area, reason)
	}

	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", false)
	if area, reason, ok := evaluateFunctionalAreaGate("t1", "staffing.assignment_create.field_policy"); ok || reason != functionalAreaDisabledCode || area != "staffing" {
		t.Fatalf("ok=%v area=%q reason=%q", ok, area, reason)
	}
	defaultFunctionalAreaSwitchStore.setEnabled("t1", "staffing", true)

	if area, reason, ok := evaluateFunctionalAreaGate("t1", "staffing.assignment_create.field_policy"); !ok || reason != "" || area != "staffing" {
		t.Fatalf("ok=%v area=%q reason=%q", ok, area, reason)
	}
}

func TestFunctionalAreaErrorMessage(t *testing.T) {
	if msg := functionalAreaErrorMessage(functionalAreaMissingCode); msg != "functional area missing" {
		t.Fatalf("msg=%q", msg)
	}
	if msg := functionalAreaErrorMessage(functionalAreaDisabledCode); msg != "functional area disabled" {
		t.Fatalf("msg=%q", msg)
	}
	if msg := functionalAreaErrorMessage(functionalAreaNotActiveCode); msg != "functional area not active" {
		t.Fatalf("msg=%q", msg)
	}
	if msg := functionalAreaErrorMessage("unknown"); msg != "functional area blocked" {
		t.Fatalf("msg=%q", msg)
	}
}

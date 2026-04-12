package server

import "testing"

func TestAssistantTaskSnapshotCompatible_KnowledgeFields(t *testing.T) {
	current := assistantTaskContractSnapshot{
		IntentSchemaVersion:     "v1",
		CompilerContractVersion: "v1",
		CapabilityMapVersion:    "v1",
		SkillManifestDigest:     "d",
		ContextHash:             "c",
		IntentHash:              "i",
		PlanHash:                "p",
		KnowledgeSnapshotDigest: "k1",
		RouteCatalogVersion:     "r1",
		ResolverContractVersion: "resolver_contract_v1",
		ContextTemplateVersion:  "plan_context_v1",
		ReplyGuidanceVersion:    "reply_v1",
	}
	compat := assistantTaskContractSnapshot{
		IntentSchemaVersion:     "v1",
		CompilerContractVersion: "v1",
		CapabilityMapVersion:    "v1",
		SkillManifestDigest:     "d",
		ContextHash:             "c",
		IntentHash:              "i",
		PlanHash:                "p",
	}
	if !assistantTaskSnapshotCompatible(current, compat) {
		t.Fatal("compat snapshot should remain compatible when new fields are empty")
	}
	strict := compat
	strict.KnowledgeSnapshotDigest = "wrong"
	if assistantTaskSnapshotCompatible(current, strict) {
		t.Fatal("snapshot should be incompatible when non-empty knowledge field mismatches")
	}

	cases := []struct {
		name   string
		mutate func(snapshot *assistantTaskContractSnapshot)
	}{
		{name: "intent schema mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.IntentSchemaVersion = "x" }},
		{name: "compiler contract mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.CompilerContractVersion = "x" }},
		{name: "capability map mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.CapabilityMapVersion = "x" }},
		{name: "skill manifest mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.SkillManifestDigest = "x" }},
		{name: "context hash mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.ContextHash = "x" }},
		{name: "intent hash mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.IntentHash = "x" }},
		{name: "plan hash mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.PlanHash = "x" }},
		{name: "route catalog mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.RouteCatalogVersion = "x" }},
		{name: "resolver contract mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.ResolverContractVersion = "x" }},
		{name: "context template mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.ContextTemplateVersion = "x" }},
		{name: "reply guidance mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.ReplyGuidanceVersion = "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := current
			tc.mutate(&stored)
			if assistantTaskSnapshotCompatible(current, stored) {
				t.Fatalf("expected mismatch for case=%s", tc.name)
			}
		})
	}
}

func TestAssistantTaskSnapshotCompatible_PolicyFields(t *testing.T) {
	current := assistantTaskContractSnapshot{
		IntentSchemaVersion:      "v1",
		CompilerContractVersion:  "v1",
		CapabilityMapVersion:     "v1",
		SkillManifestDigest:      "d",
		ContextHash:              "c",
		IntentHash:               "i",
		PlanHash:                 "p",
		PolicyContextDigest:      "ctx",
		EffectivePolicyVersion:   "epv1",
		ResolvedSetID:            "S2601",
		SetIDSource:              "custom",
		PrecheckProjectionDigest: "proj",
		MutationPolicyVersion:    "mpv1",
	}
	if !assistantTaskSnapshotCompatible(current, assistantTaskContractSnapshot{
		IntentSchemaVersion:     "v1",
		CompilerContractVersion: "v1",
		CapabilityMapVersion:    "v1",
		SkillManifestDigest:     "d",
		ContextHash:             "c",
		IntentHash:              "i",
		PlanHash:                "p",
	}) {
		t.Fatal("empty policy fields should remain forward-compatible")
	}

	cases := []struct {
		name   string
		mutate func(snapshot *assistantTaskContractSnapshot)
	}{
		{name: "policy version mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.EffectivePolicyVersion = "x" }},
		{name: "resolved setid mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.ResolvedSetID = "x" }},
		{name: "setid source mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.SetIDSource = "x" }},
		{name: "projection digest mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.PrecheckProjectionDigest = "x" }},
		{name: "mutation policy mismatch", mutate: func(snapshot *assistantTaskContractSnapshot) { snapshot.MutationPolicyVersion = "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := current
			tc.mutate(&stored)
			if assistantTaskSnapshotCompatible(current, stored) {
				t.Fatalf("expected mismatch for case=%s", tc.name)
			}
		})
	}
}

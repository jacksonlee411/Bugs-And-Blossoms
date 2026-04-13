package fieldpolicy.setid_strategy

import rego.v1

error_policy_conflict := "policy_conflict_ambiguous"
error_policy_missing := "policy_missing"
error_default_rule_missing := "FIELD_DEFAULT_RULE_MISSING"
error_policy_mode_invalid := "policy_mode_invalid"
error_policy_mode_combination := "policy_mode_combination_invalid"

priority_mode_blend_custom_first := "blend_custom_first"
priority_mode_blend_deflt_first := "blend_deflt_first"
priority_mode_deflt_unsubscribed := "deflt_unsubscribed"

local_override_mode_allow := "allow"
local_override_mode_no_override := "no_override"
local_override_mode_no_local := "no_local"

org_applicability_tenant := "tenant"
org_applicability_business_unit := "business_unit"

source_type_baseline := "baseline"
source_type_intent_override := "intent_override"

bucket_intent_setid_business_unit_exact := "intent_setid_exact_business_unit_exact"
bucket_intent_setid_wildcard := "intent_setid_exact_business_unit_wildcard"
bucket_intent_wildcard := "intent_setid_wildcard_business_unit_wildcard"
bucket_baseline_setid_business_unit := "baseline_setid_exact_business_unit_exact"
bucket_baseline_setid_wildcard := "baseline_setid_exact_business_unit_wildcard"
bucket_baseline_wildcard := "baseline_setid_wildcard_business_unit_wildcard"

default result := {"ok": false, "error": "policy_missing"}

result := {"ok": false, "error": mode_error} if {
	eval := evaluation
	mode_error := mode_validation_error(eval.primary.priority_mode, eval.primary.local_override_mode)
	mode_error != ""
}

result := {"ok": false, "error": final.conflict_error} if {
	final := finalized
	final.conflict_error != ""
}

result := {"ok": true, "decision": final.decision} if {
	final := finalized
	final.conflict_error == ""
}

finalized := final if {
	eval := evaluation
	mode_validation_error(eval.primary.priority_mode, eval.primary.local_override_mode) == ""
	allowed_value_codes := merged_allowed_codes(
		eval.primary.allowed_value_codes,
		eval.fallback_allowed_value_codes,
		eval.bucket.setid_exact,
		eval.primary.priority_mode,
		eval.primary.local_override_mode,
	)
	resolved_default := resolved_default_value(
		eval.primary.default_value,
		eval.fallbacks,
		allowed_value_codes,
		eval.bucket.setid_exact,
	)
	conflict := conflict_error_for(eval.primary, allowed_value_codes, resolved_default)
	mode_trace := sprintf(
		"mode:%s/%s:allowed=%s",
		[eval.primary.priority_mode, eval.primary.local_override_mode, joined_allowed_value_codes(allowed_value_codes)],
	)
	decision := {
		"capability_key": eval.primary.capability_key,
		"field_key": eval.primary.field_key,
		"source_type": eval.bucket.source_type,
		"required": eval.primary.required,
		"visible": eval.primary.visible,
		"maintainable": eval.primary.maintainable,
		"default_rule_ref": eval.primary.default_rule_ref,
		"resolved_default_value": resolved_default,
		"allowed_value_codes": allowed_value_codes,
		"priority_mode": eval.primary.priority_mode,
		"local_override_mode": eval.primary.local_override_mode,
		"matched_bucket": eval.bucket.name,
		"primary_policy_id": eval.primary.policy_id,
		"winner_policy_ids": eval.winner_policy_ids,
		"matched_policy_ids": eval.matched_policy_ids,
		"resolution_trace": array.concat(array.concat(eval.bucket_trace, eval.fallback_trace), [mode_trace]),
	}
	final := {
		"conflict_error": conflict,
		"decision": decision,
	}
}

evaluation := output if {
	buckets := bucket_specs
	bucket_idx := first_hit_bucket_index(buckets)
	bucket := buckets[bucket_idx]
	matched_indices := matching_indices(bucket)
	count(matched_indices) > 0
	primary := input.records[matched_indices[0]]
	matched_policy_ids := normalized_strings([input.records[i].policy_id | i := matched_indices[_]])
	fallbacks := fallback_winners(buckets, bucket_idx)
	fallback_allowed_value_codes := [code |
		i := array_indices(fallbacks)[_]
		j := array_indices(fallbacks[i].record.allowed_value_codes)[_]
		code := fallbacks[i].record.allowed_value_codes[j]
	]
	fallback_trace := [sprintf("fallback:%s:%s", [fallbacks[i].bucket_name, fallbacks[i].record.policy_id]) |
		i := array_indices(fallbacks)[_]
	]
	fallback_policy_ids := [fallbacks[i].record.policy_id | i := array_indices(fallbacks)[_]]
	winner_policy_ids := normalized_strings(array.concat([primary.policy_id], fallback_policy_ids))
	bucket_trace := [bucket_trace_item(buckets, i) |
		i := array_indices(buckets)[_]
		i <= bucket_idx
	]
	output := {
		"bucket": bucket,
		"bucket_trace": bucket_trace,
		"fallback_allowed_value_codes": fallback_allowed_value_codes,
		"fallback_trace": fallback_trace,
		"fallbacks": fallbacks,
		"matched_policy_ids": matched_policy_ids,
		"primary": primary,
		"winner_policy_ids": winner_policy_ids,
	}
}

bucket_specs := [
	{
		"name": bucket_intent_setid_business_unit_exact,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": input.ctx.business_unit_node_key,
		"setid_exact": true,
		"business_unit_exact": true,
	},
	{
		"name": bucket_intent_setid_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": "",
		"setid_exact": true,
		"business_unit_exact": false,
	},
	{
		"name": bucket_intent_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": "",
		"business_unit_node_key": "",
		"setid_exact": false,
		"business_unit_exact": false,
	},
] if {
	baseline := trim_space(input.baseline_capability_key)
	baseline == ""
}

bucket_specs := [
	{
		"name": bucket_intent_setid_business_unit_exact,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": input.ctx.business_unit_node_key,
		"setid_exact": true,
		"business_unit_exact": true,
	},
	{
		"name": bucket_intent_setid_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": "",
		"setid_exact": true,
		"business_unit_exact": false,
	},
	{
		"name": bucket_intent_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": "",
		"business_unit_node_key": "",
		"setid_exact": false,
		"business_unit_exact": false,
	},
] if {
	baseline := trim_space(input.baseline_capability_key)
	baseline == input.ctx.capability_key
}

bucket_specs := [
	{
		"name": bucket_intent_setid_business_unit_exact,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": input.ctx.business_unit_node_key,
		"setid_exact": true,
		"business_unit_exact": true,
	},
	{
		"name": bucket_intent_setid_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": "",
		"setid_exact": true,
		"business_unit_exact": false,
	},
	{
		"name": bucket_intent_wildcard,
		"source_type": source_type_intent_override,
		"capability_key": input.ctx.capability_key,
		"resolved_setid": "",
		"business_unit_node_key": "",
		"setid_exact": false,
		"business_unit_exact": false,
	},
	{
		"name": bucket_baseline_setid_business_unit,
		"source_type": source_type_baseline,
		"capability_key": input.baseline_capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": input.ctx.business_unit_node_key,
		"setid_exact": true,
		"business_unit_exact": true,
	},
	{
		"name": bucket_baseline_setid_wildcard,
		"source_type": source_type_baseline,
		"capability_key": input.baseline_capability_key,
		"resolved_setid": input.ctx.resolved_setid,
		"business_unit_node_key": "",
		"setid_exact": true,
		"business_unit_exact": false,
	},
	{
		"name": bucket_baseline_wildcard,
		"source_type": source_type_baseline,
		"capability_key": input.baseline_capability_key,
		"resolved_setid": "",
		"business_unit_node_key": "",
		"setid_exact": false,
		"business_unit_exact": false,
	},
] if {
	baseline := trim_space(input.baseline_capability_key)
	baseline != ""
	baseline != input.ctx.capability_key
}

matching_indices(bucket) := sort([i |
	some i
	record := input.records[i]
	record_matches_bucket(record, bucket)
])

record_matches_bucket(record, bucket) if {
	record.capability_key == bucket.capability_key
	record.field_key == input.ctx.field_key
	bucket.business_unit_exact
	record.org_applicability == org_applicability_business_unit
	record.resolved_setid == bucket.resolved_setid
	record.business_unit_node_key == bucket.business_unit_node_key
}

record_matches_bucket(record, bucket) if {
	record.capability_key == bucket.capability_key
	record.field_key == input.ctx.field_key
	not bucket.business_unit_exact
	bucket.setid_exact
	record.org_applicability == org_applicability_tenant
	record.business_unit_node_key == ""
	record.resolved_setid == bucket.resolved_setid
}

record_matches_bucket(record, bucket) if {
	record.capability_key == bucket.capability_key
	record.field_key == input.ctx.field_key
	not bucket.business_unit_exact
	not bucket.setid_exact
	record.org_applicability == org_applicability_tenant
	record.business_unit_node_key == ""
	record.resolved_setid == ""
}

first_hit_bucket_index(buckets) := hit_indices[0] if {
	hit_indices := sort([i |
		some i
		_ := buckets[i]
		count(matching_indices(buckets[i])) > 0
	])
	count(hit_indices) > 0
}

fallback_winners(buckets, current_idx) := [winner |
	i := sort([j |
		some j
		_ := buckets[j]
		j > current_idx
		count(matching_indices(buckets[j])) > 0
	])[_]
	indices := matching_indices(buckets[i])
	winner := {
		"bucket_name": buckets[i].name,
		"record": input.records[indices[0]],
	}
]

bucket_trace_item(buckets, i) := sprintf("bucket:%s:hit:%d", [bucket.name, count(indices)]) if {
	bucket := buckets[i]
	indices := matching_indices(bucket)
	count(indices) > 0
}

bucket_trace_item(buckets, i) := sprintf("bucket:%s:miss", [bucket.name]) if {
	bucket := buckets[i]
	indices := matching_indices(bucket)
	count(indices) == 0
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(local_codes) if {
	not local_exact
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(local_codes) if {
	local_exact
	count(fallback_codes) == 0
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := unique_concat(local_codes, fallback_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_custom_first
	local_override_mode == local_override_mode_allow
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := unique_concat(fallback_codes, local_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_custom_first
	local_override_mode == local_override_mode_no_override
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(fallback_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_custom_first
	local_override_mode == local_override_mode_no_local
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := unique_concat(fallback_codes, local_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_deflt_first
	local_override_mode == local_override_mode_allow
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := unique_concat(fallback_codes, local_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_deflt_first
	local_override_mode == local_override_mode_no_override
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(fallback_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_blend_deflt_first
	local_override_mode == local_override_mode_no_local
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(local_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_deflt_unsubscribed
	local_override_mode == local_override_mode_allow
}

merged_allowed_codes(local_codes, fallback_codes, local_exact, priority_mode, local_override_mode) := normalized_strings(local_codes) if {
	local_exact
	count(fallback_codes) > 0
	priority_mode == priority_mode_deflt_unsubscribed
	local_override_mode == local_override_mode_no_override
}

resolved_default_value(primary_default, fallbacks, allowed, local_exact) := primary_default if {
	primary_default_selected(primary_default, allowed)
}

resolved_default_value(primary_default, fallbacks, allowed, local_exact) := primary_default if {
	not primary_default_selected(primary_default, allowed)
	not local_exact
}

resolved_default_value(primary_default, fallbacks, allowed, local_exact) := candidates[0] if {
	not primary_default_selected(primary_default, allowed)
	local_exact
	candidates := fallback_default_candidates(fallbacks, allowed)
	count(candidates) > 0
}

resolved_default_value(primary_default, fallbacks, allowed, local_exact) := primary_default if {
	not primary_default_selected(primary_default, allowed)
	local_exact
	candidates := fallback_default_candidates(fallbacks, allowed)
	count(candidates) == 0
}

primary_default_selected(primary_default, allowed) if {
	primary_default != ""
	count(allowed) == 0
}

primary_default_selected(primary_default, allowed) if {
	primary_default != ""
	array_contains(allowed, primary_default)
}

fallback_default_candidates(fallbacks, allowed) := fallback_default_candidates_all(fallbacks) if {
	count(allowed) == 0
}

fallback_default_candidates(fallbacks, allowed) := fallback_default_candidates_allowed(fallbacks, allowed) if {
	count(allowed) > 0
}

fallback_default_candidates_all(fallbacks) := [candidate |
	i := array_indices(fallbacks)[_]
	candidate := fallbacks[i].record.default_value
	candidate != ""
]

fallback_default_candidates_allowed(fallbacks, allowed) := [candidate |
	i := array_indices(fallbacks)[_]
	candidate := fallbacks[i].record.default_value
	candidate != ""
	array_contains(allowed, candidate)
]

conflict_error_for(primary, allowed, default_value) := error_policy_conflict if {
	required_hidden_conflict(primary)
}

conflict_error_for(primary, allowed, default_value) := error_default_rule_missing if {
	not required_hidden_conflict(primary)
	default_rule_missing_conflict(primary, default_value)
}

conflict_error_for(primary, allowed, default_value) := error_policy_conflict if {
	not required_hidden_conflict(primary)
	not default_rule_missing_conflict(primary, default_value)
	required_allowed_empty_conflict(primary, allowed)
}

conflict_error_for(primary, allowed, default_value) := error_policy_conflict if {
	not required_hidden_conflict(primary)
	not default_rule_missing_conflict(primary, default_value)
	not required_allowed_empty_conflict(primary, allowed)
	default_not_allowed_conflict(allowed, default_value)
}

conflict_error_for(primary, allowed, default_value) := "" if {
	not required_hidden_conflict(primary)
	not default_rule_missing_conflict(primary, default_value)
	not required_allowed_empty_conflict(primary, allowed)
	not default_not_allowed_conflict(allowed, default_value)
}

required_hidden_conflict(primary) if {
	primary.required
	not primary.visible
}

default_rule_missing_conflict(primary, default_value) if {
	not primary.maintainable
	primary.default_rule_ref == ""
	default_value == ""
}

required_allowed_empty_conflict(primary, allowed) if {
	primary.required
	count(primary.allowed_value_codes) > 0
	count(allowed) == 0
}

default_not_allowed_conflict(allowed, default_value) if {
	default_value != ""
	count(allowed) > 0
	not array_contains(allowed, default_value)
}

mode_validation_error(priority_mode, local_override_mode) := error_policy_mode_invalid if {
	not valid_priority_mode(priority_mode)
}

mode_validation_error(priority_mode, local_override_mode) := error_policy_mode_invalid if {
	valid_priority_mode(priority_mode)
	not valid_local_override_mode(local_override_mode)
}

mode_validation_error(priority_mode, local_override_mode) := error_policy_mode_combination if {
	valid_priority_mode(priority_mode)
	valid_local_override_mode(local_override_mode)
	invalid_mode_combination(priority_mode, local_override_mode)
}

mode_validation_error(priority_mode, local_override_mode) := "" if {
	valid_priority_mode(priority_mode)
	valid_local_override_mode(local_override_mode)
	not invalid_mode_combination(priority_mode, local_override_mode)
}

valid_priority_mode(priority_mode) if priority_mode == priority_mode_blend_custom_first
valid_priority_mode(priority_mode) if priority_mode == priority_mode_blend_deflt_first
valid_priority_mode(priority_mode) if priority_mode == priority_mode_deflt_unsubscribed

valid_local_override_mode(local_override_mode) if local_override_mode == local_override_mode_allow
valid_local_override_mode(local_override_mode) if local_override_mode == local_override_mode_no_override
valid_local_override_mode(local_override_mode) if local_override_mode == local_override_mode_no_local

invalid_mode_combination(priority_mode, local_override_mode) if {
	priority_mode == priority_mode_deflt_unsubscribed
	local_override_mode == local_override_mode_no_local
}

array_indices(arr) := sort([i |
	some i
	_ := arr[i]
])

normalized_strings(values) := [trim_space(values[i]) |
	i := sort([j |
		some j
		_ := values[j]
		trim_space(values[j]) != ""
		not has_prior_trimmed_value(values, trim_space(values[j]), j)
	])[_]
]

has_prior_trimmed_value(values, value, idx) if {
	some i
	_ := values[i]
	i < idx
	trim_space(values[i]) == value
}

unique_concat(primary, secondary) := normalized_strings(array.concat(primary, secondary))

array_contains(values, value) if {
	values[_] == value
}

joined_allowed_value_codes(values) := "" if {
	count(values) == 0
}

joined_allowed_value_codes(values) := concat(",", values) if {
	count(values) > 0
}

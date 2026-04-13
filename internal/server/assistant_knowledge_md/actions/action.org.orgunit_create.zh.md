---
id: action.org.orgunit_create
title: 创建组织动作说明
locale: zh
kind: action
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - create_orgunit
summary: 在指定父组织下创建部门，提交前需要确认候选主键。
action_key: create_orgunit
required_checks:
  - strict_decode
  - boundary_lint
  - candidate_confirmation
  - dry_run
proposal_template: 上级组织：{parent_ref_text}；新建组织：{entity_name}；生效日期：{effective_date}
reply_refs:
  - reply.missing_fields
  - reply.candidate_confirm
  - reply.confirm_summary
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
field_display_map:
  - field: parent_ref_text
    label: 上级组织
  - field: entity_name
    label: 部门名称
  - field: effective_date
    label: 生效日期
missing_field_guidance:
  - error_code: missing_parent_ref_text
    text: 请补充上级组织（例如：鲜花组织）。
  - error_code: missing_entity_name
    text: 请补充部门名称（例如：运营部）。
  - error_code: missing_effective_date
    text: 请补充生效日期（YYYY-MM-DD）。
  - error_code: invalid_effective_date_format
    text: 生效日期格式不正确，请使用 YYYY-MM-DD。
  - error_code: parent_candidate_not_found
    text: 未找到匹配上级组织，请补充更准确的组织名称或编码。
  - error_code: candidate_confirmation_required
    text: 检测到多个上级组织候选，请先确认候选主键。
  - error_code: FIELD_REQUIRED_VALUE_MISSING
    text: 当前创建策略缺少默认值，请联系管理员补齐字段策略。
  - error_code: PATCH_FIELD_NOT_ALLOWED
    text: 当前租户未启用创建所需字段配置，请联系管理员启用后重试。
field_examples:
  - field: parent_ref_text
    example: 鲜花组织
  - field: entity_name
    example: 运营部
  - field: effective_date
    example: 2026-01-01
candidate_explanation_templates:
  - 候选组织：{candidate_name}（{candidate_code}）
confirmation_summary_templates:
  - 上级组织：{parent_ref_text}；新建组织：{entity_name}；生效日期：{effective_date}
template_fields:
  - action_view_pack.summary
  - field_display_map
  - missing_field_guidance
  - contract_projection.required_fields_view
  - contract_projection.action_spec_summary
  - conversation_snapshot.current_phase
---
创建组织的正式写入仍由 authoritative backend 执行，这里只保留说明性知识与模板。

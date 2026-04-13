---
id: action.org.orgunit_correct
title: 更正组织动作说明
locale: zh
kind: action
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - correct_orgunit
summary: 更正指定组织版本的字段内容，提交前进行字段校验。
action_key: correct_orgunit
required_checks:
  - strict_decode
  - boundary_lint
  - candidate_confirmation
  - dry_run
proposal_template: 目标组织：{org_code}；目标生效日期：{target_effective_date}
reply_refs:
  - reply.missing_fields
  - reply.confirm_summary
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
field_display_map:
  - field: org_code
    label: 组织编码
  - field: target_effective_date
    label: 目标生效日期
missing_field_guidance:
  - error_code: missing_org_code
    text: 请补充目标组织编码。
  - error_code: missing_target_effective_date
    text: 请补充要更正的目标生效日期（YYYY-MM-DD）。
  - error_code: invalid_target_effective_date_format
    text: 目标生效日期格式不正确，请使用 YYYY-MM-DD。
  - error_code: missing_change_fields
    text: 请补充需要更正的字段内容。
field_examples:
  - field: org_code
    example: FLOWER-C
  - field: target_effective_date
    example: 2026-01-01
confirmation_summary_templates:
  - 目标组织：{org_code}；目标生效日期：{target_effective_date}
template_fields:
  - action_view_pack.summary
  - field_display_map
  - missing_field_guidance
  - contract_projection.required_fields_view
  - contract_projection.action_spec_summary
---
更正动作需要对齐目标版本和字段 patch，Markdown 只描述说明性知识。

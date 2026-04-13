---
id: action.org.orgunit_disable
title: 停用组织动作说明
locale: zh
kind: action
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - disable_orgunit
summary: 停用指定组织，提交前进行字段校验。
action_key: disable_orgunit
required_checks:
  - strict_decode
  - boundary_lint
  - dry_run
proposal_template: 目标组织：{org_code}；停用生效日期：{effective_date}
reply_refs:
  - reply.missing_fields
  - reply.confirm_summary
tool_refs:
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
field_display_map:
  - field: org_code
    label: 组织编码
  - field: effective_date
    label: 生效日期
missing_field_guidance:
  - error_code: missing_org_code
    text: 请补充目标组织编码。
  - error_code: missing_effective_date
    text: 请补充停用生效日期（YYYY-MM-DD）。
  - error_code: invalid_effective_date_format
    text: 生效日期格式不正确，请使用 YYYY-MM-DD。
field_examples:
  - field: org_code
    example: FLOWER-C
  - field: effective_date
    example: 2026-04-01
confirmation_summary_templates:
  - 目标组织：{org_code}；停用生效日期：{effective_date}
template_fields:
  - action_view_pack.summary
  - field_display_map
  - missing_field_guidance
  - contract_projection.required_fields_view
  - contract_projection.action_spec_summary
---
停用动作不再维护独立知识真相源，所有说明从 Markdown runtime 提供。

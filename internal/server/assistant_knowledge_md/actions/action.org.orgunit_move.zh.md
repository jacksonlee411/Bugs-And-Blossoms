---
id: action.org.orgunit_move
title: 移动组织动作说明
locale: zh
kind: action
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - move_orgunit
summary: 将组织移动到新的上级组织，提交前需要确认候选主键。
action_key: move_orgunit
required_checks:
  - strict_decode
  - boundary_lint
  - candidate_confirmation
  - dry_run
proposal_template: 目标组织：{org_code}；新上级：{new_parent_ref_text}；生效日期：{effective_date}
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
  - field: org_code
    label: 组织编码
  - field: new_parent_ref_text
    label: 新上级组织
  - field: effective_date
    label: 生效日期
missing_field_guidance:
  - error_code: missing_org_code
    text: 请补充目标组织编码。
  - error_code: missing_new_parent_ref_text
    text: 请补充新的上级组织。
  - error_code: parent_candidate_not_found
    text: 未找到匹配的新上级组织，请补充更准确的组织名称或编码。
  - error_code: candidate_confirmation_required
    text: 检测到多个新上级组织候选，请先确认候选主键。
  - error_code: missing_effective_date
    text: 请补充移动生效日期（YYYY-MM-DD）。
  - error_code: invalid_effective_date_format
    text: 生效日期格式不正确，请使用 YYYY-MM-DD。
field_examples:
  - field: org_code
    example: FLOWER-C
  - field: new_parent_ref_text
    example: 共享服务中心
  - field: effective_date
    example: 2026-04-01
candidate_explanation_templates:
  - 候选组织：{candidate_name}（{candidate_code}）
confirmation_summary_templates:
  - 目标组织：{org_code}；新上级：{new_parent_ref_text}；生效日期：{effective_date}
template_fields:
  - action_view_pack.summary
  - field_display_map
  - missing_field_guidance
  - contract_projection.required_fields_view
---
移动组织需要候选确认与 precheck，Markdown 只描述说明性知识和模板。

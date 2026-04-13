---
id: org.orgunit_move
title: 移动组织意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于移动组织请求，会先做候选确认与 dry-run，不会直接提交。
route_kind: business_action
action_key: move_orgunit
intent_classes:
  - business_action
required_slots:
  - org_code
  - effective_date
  - new_parent_ref_text
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_move.v1
    text: 请补充目标组织编码、新的上级组织和生效日期，我会先生成草案并做校验。
keywords:
  - 移动
  - 调整上级
  - move
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
移动组织意图只提供运行时解释与候选确认上下文。

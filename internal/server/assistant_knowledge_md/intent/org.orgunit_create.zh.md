---
id: org.orgunit_create
title: 创建组织意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于创建组织请求，会先做候选确认与 dry-run，不会直接提交。
route_kind: business_action
action_key: create_orgunit
intent_classes:
  - business_action
required_slots:
  - parent_ref_text
  - entity_name
  - effective_date
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_create.v1
    text: 请补充上级组织、组织名称和生效日期，我会先生成草案并做校验。
keywords:
  - 新建
  - 创建
  - 部门
  - 组织
  - create
  - department
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
创建组织属于业务动作意图，知识 runtime 只负责解释与装配上下文，正式 contract 与提交边界仍由 350 主线裁决。

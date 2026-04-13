---
id: org.orgunit_add_version
title: 新增版本意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于新增组织版本请求，会先做字段校验与 dry-run，不会直接提交。
route_kind: business_action
action_key: add_orgunit_version
intent_classes:
  - business_action
required_slots:
  - org_code
  - effective_date
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_add_version.v1
    text: 请补充目标组织编码与新增版本生效日期，我会先整理草案并做校验。
keywords:
  - 新增版本
  - 新增组织版本
  - add version
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
新增版本意图只负责运行时解释与路由，正式 contract 仍由 350 主线裁决。

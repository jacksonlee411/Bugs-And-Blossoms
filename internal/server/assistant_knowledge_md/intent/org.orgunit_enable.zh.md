---
id: org.orgunit_enable
title: 启用组织意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于启用组织请求，会先做字段校验与 dry-run，不会直接提交。
route_kind: business_action
action_key: enable_orgunit
intent_classes:
  - business_action
required_slots:
  - org_code
  - effective_date
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_enable.v1
    text: 请补充目标组织编码和启用生效日期，我会先生成草案并做校验。
keywords:
  - 启用
  - 恢复
  - enable
tool_refs:
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
启用组织意图只提供说明性运行时知识。

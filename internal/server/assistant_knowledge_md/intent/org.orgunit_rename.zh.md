---
id: org.orgunit_rename
title: 重命名组织意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于重命名组织请求，会先做字段校验与 dry-run，不会直接提交。
route_kind: business_action
action_key: rename_orgunit
intent_classes:
  - business_action
required_slots:
  - org_code
  - effective_date
  - new_name
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_rename.v1
    text: 请补充目标组织编码、新名称和生效日期，我会先生成草案并做校验。
keywords:
  - 重命名
  - 改名
  - rename
tool_refs:
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
重命名组织意图只提供说明性知识，不反向定义写入 contract。

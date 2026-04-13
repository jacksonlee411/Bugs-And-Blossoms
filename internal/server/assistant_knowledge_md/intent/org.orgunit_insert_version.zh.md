---
id: org.orgunit_insert_version
title: 插入版本意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于插入组织历史版本请求，会先做字段校验与 dry-run，不会直接提交。
route_kind: business_action
action_key: insert_orgunit_version
intent_classes:
  - business_action
required_slots:
  - org_code
  - effective_date
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_insert_version.v1
    text: 请补充目标组织编码与插入版本生效日期，我会先生成草案并做校验。
keywords:
  - 插入版本
  - 历史版本
  - insert version
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
插入版本意图只提供运行时知识，不反向定义写入 contract。

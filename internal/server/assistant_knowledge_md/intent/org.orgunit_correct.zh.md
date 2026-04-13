---
id: org.orgunit_correct
title: 更正组织意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/350-assistant-tooling-alignment-with-unified-policy-model-plan.md
applies_to:
  - business_action
summary: 当前轮属于更正组织版本请求，会先做字段校验与 dry-run，不会直接提交。
route_kind: business_action
action_key: correct_orgunit
intent_classes:
  - business_action
required_slots:
  - org_code
  - target_effective_date
min_confidence: 0.55
clarification_prompts:
  - template_id: clarify.org.orgunit_correct.v1
    text: 请补充目标组织编码与要更正的目标生效日期，我会先生成草案并做校验。
keywords:
  - 更正
  - 修正
  - correct
tool_refs:
  - tool.orgunit_candidate_lookup
  - tool.orgunit_candidate_snapshot
  - tool.orgunit_action_precheck
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
更正组织意图只提供运行时知识，不裁决 patch 权限与提交边界。

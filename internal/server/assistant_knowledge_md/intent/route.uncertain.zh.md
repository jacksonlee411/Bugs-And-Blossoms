---
id: route.uncertain
title: 未确定意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md
applies_to:
  - uncertain
summary: 当前轮语义仍不确定，仅保留澄清投影，不触发业务提交。
route_kind: uncertain
intent_classes:
  - uncertain
required_slots: []
min_confidence: 0.2
clarification_prompts:
  - template_id: clarify.route.uncertain.v1
    text: 我还不能稳定判断你的目标，请补充你想查询说明还是执行组织变更。
keywords:
  - 不确定
  - 没想好
  - maybe
  - not sure
tool_refs: []
wiki_refs:
  - wiki.assistant_runtime
---
未确定意图用于 fail-closed，避免非稳定语义误入业务提交链。

---
id: knowledge.general_qa
title: 知识问答意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md
applies_to:
  - knowledge_qa
summary: 当前轮属于知识问答，只返回说明，不触发业务提交。
route_kind: knowledge_qa
intent_classes:
  - knowledge_qa
required_slots: []
min_confidence: 0.4
clarification_prompts:
  - template_id: clarify.knowledge.general_qa.v1
    text: 这是知识问答场景，我会直接回答，不会进入 confirm 或 commit。
keywords:
  - 功能
  - 如何
  - 怎么
  - 什么
  - 哪些
  - why
  - how
  - what
tool_refs:
  - tool.orgunit_field_explain
wiki_refs:
  - wiki.assistant_runtime
---
知识问答只提供说明性回答，不会构造业务写入计划。

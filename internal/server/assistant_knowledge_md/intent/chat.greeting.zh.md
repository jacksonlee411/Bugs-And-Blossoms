---
id: chat.greeting
title: 闲聊意图
locale: zh
kind: intent
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - docs/dev-plans/370-assistant-api-first-and-markdown-knowledge-runtime-plan.md
applies_to:
  - chitchat
summary: 当前轮属于闲聊响应，不触发业务提交。
route_kind: chitchat
intent_classes:
  - chitchat
required_slots: []
min_confidence: 0.4
clarification_prompts:
  - template_id: clarify.chat.greeting.v1
    text: 这是闲聊场景，我会直接回应，不会进入业务动作链。
keywords:
  - 你好
  - 您好
  - hi
  - hello
  - thanks
  - 谢谢
tool_refs: []
wiki_refs:
  - wiki.assistant_runtime
---
闲聊属于非业务请求，只保留说明性回复。

---
id: wiki.assistant_runtime
title: Assistant Runtime 说明
locale: zh
kind: wiki
version: 2026-04-13.v1
status: active
source_refs:
  - docs/dev-plans/370a-assistant-markdown-knowledge-runtime-phase1-query-and-compiler-plan.md
  - docs/dev-plans/375-assistant-mainline-implementation-roadmap-350-370.md
applies_to:
  - assistant_runtime
summary: 说明 Assistant runtime 的知识主源、fail-closed 边界与 route/plan 关系。
topic_key: assistant.runtime
retrieval_terms:
  - runtime
  - markdown knowledge
  - fail closed
related_topics:
  - route_kind
  - knowledge_snapshot_digest
---
Assistant runtime 只消费 Markdown 主源知识，不再依赖 checked-in JSON 快照。

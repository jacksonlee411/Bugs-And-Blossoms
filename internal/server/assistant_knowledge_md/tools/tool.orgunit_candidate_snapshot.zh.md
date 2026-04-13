---
id: tool.orgunit_candidate_snapshot
title: 组织候选快照工具
locale: zh
kind: tool
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - business_action
summary: 读取当前候选组织快照，用于解释候选和生成确认摘要。
tool_name: orgunit_candidate_snapshot
allowed_route_kinds:
  - business_action
input_schema_ref: internal/server/assistant_action_registry.go
output_schema_ref: internal/server/assistant_action_registry.go
usage_constraints:
  - 只读
  - 不回写业务状态
---
候选快照仅用于说明和确认上下文。

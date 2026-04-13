---
id: tool.orgunit_field_explain
title: 组织字段说明工具
locale: zh
kind: tool
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - business_action
  - knowledge_qa
summary: 解释组织字段、缺失字段和字段策略约束。
tool_name: orgunit_field_explain
allowed_route_kinds:
  - business_action
  - knowledge_qa
input_schema_ref: internal/server/assistant_action_registry.go
output_schema_ref: internal/server/assistant_action_registry.go
usage_constraints:
  - 只读说明
  - 不生成写入指令
---
字段说明工具只用于解释字段语义和限制。

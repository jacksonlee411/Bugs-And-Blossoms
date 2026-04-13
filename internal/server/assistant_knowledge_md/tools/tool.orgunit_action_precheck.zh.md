---
id: tool.orgunit_action_precheck
title: 组织动作预检工具
locale: zh
kind: tool
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
applies_to:
  - business_action
summary: 对目标组织动作执行只读 precheck，用于 dry-run、风险说明和 fail-closed 校验。
tool_name: orgunit_action_precheck
allowed_route_kinds:
  - business_action
input_schema_ref: internal/server/assistant_action_registry.go
output_schema_ref: internal/server/assistant_action_registry.go
usage_constraints:
  - 只读预检
  - 不代替正式提交
---
动作预检只用于写前解释和策略校验。

---
id: tool.orgunit_candidate_lookup
title: 组织候选查找工具
locale: zh
kind: tool
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_action_registry.go
  - internal/server/assistant_semantic_contract.go
applies_to:
  - business_action
summary: 根据组织名称或编码检索候选组织，用于候选确认与 parent/new parent 解析。
tool_name: orgunit_candidate_lookup
allowed_route_kinds:
  - business_action
input_schema_ref: internal/server/assistant_semantic_contract.go
output_schema_ref: internal/server/assistant_semantic_contract.go
usage_constraints:
  - 只用于只读候选检索
  - 不得视为提交授权
---
候选查找是只读工具，不直接决定最终写入。

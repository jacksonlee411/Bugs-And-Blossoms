---
id: reply.missing_fields
title: 缺失字段回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - missing_fields
reply_kind: missing_fields
guidance_templates:
  - template_id: reply.missing_fields.v1
    text: 请先补充这些信息：{missing_fields}。
tone_constraints:
  - 简洁
  - 明确指出缺失项
error_codes:
  - missing_parent_ref_text
  - missing_new_parent_ref_text
  - missing_entity_name
  - missing_new_name
  - missing_org_code
  - missing_effective_date
  - invalid_effective_date_format
  - missing_target_effective_date
  - invalid_target_effective_date_format
  - missing_change_fields
---
缺失字段回复必须明确指出缺哪项，不得暗示可以直接提交。

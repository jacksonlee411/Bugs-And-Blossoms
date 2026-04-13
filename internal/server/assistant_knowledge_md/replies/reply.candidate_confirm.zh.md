---
id: reply.candidate_confirm
title: 候选确认回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - candidate_confirm
reply_kind: candidate_confirm
guidance_templates:
  - template_id: reply.candidate_confirm.v1
    text: 请先确认候选组织，再继续生成可提交草案。
tone_constraints:
  - 明确要求确认
error_codes:
  - candidate_confirmation_required
---
候选确认是 fail-closed 保护，不可省略。

---
id: reply.clarification_required
title: 需要澄清回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - clarification_required
reply_kind: clarification_required
guidance_templates:
  - template_id: reply.clarification_required.v1
    text: 我还需要补充信息才能继续，请先回答当前澄清问题。
tone_constraints:
  - 先澄清
---
需要澄清时不得跳过问题直接进入提交链。

---
id: reply.manual_takeover
title: 人工接管回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - manual_takeover
reply_kind: manual_takeover
guidance_templates:
  - template_id: reply.manual_takeover.v1
    text: 当前场景需要人工接管处理，请联系管理员或在页面上手动完成后续操作。
tone_constraints:
  - 明确提示转人工
---
人工接管用于 fail-closed，不得伪造自动能力。

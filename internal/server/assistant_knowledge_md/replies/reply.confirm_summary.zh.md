---
id: reply.confirm_summary
title: 确认摘要回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - confirm_summary
reply_kind: confirm_summary
guidance_templates:
  - template_id: reply.confirm_summary.v1
    text: 我已整理好草案：{summary}。如果确认无误，请继续提交。
tone_constraints:
  - 只总结已知草案
---
确认摘要只允许复述当前草案，不允许扩展新业务事实。

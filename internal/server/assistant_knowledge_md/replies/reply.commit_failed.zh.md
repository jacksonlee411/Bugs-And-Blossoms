---
id: reply.commit_failed
title: 提交失败回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - commit_failed
reply_kind: commit_failed
guidance_templates:
  - template_id: reply.commit_failed.v1
    text: 本次提交未成功，请根据错误提示修正后重试。
tone_constraints:
  - 明确失败
negative_examples:
  - 不要把失败说成成功
---
提交失败回复不得弱化 authoritative backend 返回的失败结论。

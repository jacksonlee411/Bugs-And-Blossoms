---
id: reply.task_waiting
title: 任务等待回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - task_waiting
reply_kind: task_waiting
guidance_templates:
  - template_id: reply.task_waiting.v1
    text: 任务仍在处理中，请稍后刷新查看状态。
tone_constraints:
  - 明确仍在等待
---
任务等待回复只说明状态，不补充未经确认的业务事实。

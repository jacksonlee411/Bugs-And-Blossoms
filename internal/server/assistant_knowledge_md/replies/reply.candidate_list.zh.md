---
id: reply.candidate_list
title: 候选列表回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - candidate_list
reply_kind: candidate_list
guidance_templates:
  - template_id: reply.candidate_list.v1
    text: 我找到了多个候选组织：{candidate_list}。请告诉我你要选择哪一个。
tone_constraints:
  - 列清候选
  - 不自动替用户选择
---
候选列表回复必须要求用户确认，不得自动提交。

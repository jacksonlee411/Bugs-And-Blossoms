---
id: reply.commit_success
title: 提交成功回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - commit_success
reply_kind: commit_success
guidance_templates:
  - template_id: reply.commit_success.v1
    text: 已完成提交，相关组织变更已经进入正式链路。
tone_constraints:
  - 简洁
  - 不夸大成功范围
---
提交成功回复只报告 authoritative backend 已接受的结果。

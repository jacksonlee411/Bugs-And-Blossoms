---
id: reply.non_business_route
title: 非业务路由回复
locale: zh
kind: reply
version: 2026-04-13.v1
status: active
source_refs:
  - internal/server/assistant_reply_nlg.go
applies_to:
  - non_business_route
reply_kind: non_business_route
guidance_templates:
  - template_id: reply.non_business_route.v1
    text: 这是说明性请求，不会触发业务提交；如果你想执行组织变更，请明确告诉我动作和对象。
tone_constraints:
  - 明确非业务
error_codes:
  - non_business_route
  - assistant_unsupported_intent
---
非业务路由回复必须阻断 confirm/commit 叙事。

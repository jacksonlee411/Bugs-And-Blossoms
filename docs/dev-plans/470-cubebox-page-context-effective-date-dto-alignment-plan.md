# DEV-PLAN-470：CubeBox `page_context` 历史日期 DTO 对齐方案

## 1. 背景

`DEV-PLAN-468 Slice E / P1` 已把前端受控 `page_context` 接入 `/internal/cubebox/turns:stream`，但当前 DTO 仍沿用 `page_context.view.as_of`。这与组织详情页真实历史锚点 `effective_date` 已出现语义分叉：

- 页面自身的历史详情入口以 `effective_date` 为主；
- `page_context` 为了兼容当前服务端/DTO，只能把页面日期事实重新映射回 `view.as_of`；
- 这会让“页面真实锚点”和“对话侧页面事实”长期维持双口径。

本计划用于单独承接 `page_context.view.as_of -> effective_date` 的 DTO 契约变更，不将其混入 `468` 的回归修复。

## 2. 问题定义

当前存在以下不一致：

1. 页面真实 URL 与详情页内部状态已经支持 `effective_date`；
2. `CubeBoxPageContext` / `modules.cubebox.PageViewContext` / `/internal/cubebox/turns:stream` 仍只认 `as_of`；
3. 前端只能做兼容映射，而不能让模型侧直接看到页面真实日期键名；
4. 若后续第二个业务模块接入，也可能继续复制“页面真实键名”和“page_context 兼容键名”双轨。

## 3. 目标

1. 将 `page_context` 中的历史日期 canonical 字段从 `view.as_of` 收敛为 `view.effective_date`。
2. 前后端 DTO、服务端规范化、prompt-view、测试与文档一次性对齐。
3. 保持 fail-closed：未知字段、不合法日期、不兼容 payload 不得 silently 放行。
4. 明确迁移窗口与兼容策略，避免一次改名打断现有页面或测试。

## 4. 非目标

1. 不在本计划内改变组织详情页自己的业务日期解析逻辑。
2. 不在本计划内引入新的页面事实字段或扩展对象结构。
3. 不在本计划内扩大 `page_context` 到更多模块页面。

## 5. 方案

### 5.1 Canonical DTO 改名

- 前端 `CubeBoxPageContext.view` 从：
  - `as_of`
- 收敛为：
  - `effective_date`

- 服务端 `modules/cubebox.PageViewContext` 同步改为：
  - `EffectiveDate string \`json:"effective_date,omitempty"\``

### 5.2 兼容窗口

迁移分两步：

1. 先让服务端同时接受 `effective_date` 与 legacy `as_of`，但规范化输出只保留 `effective_date`。
2. 前端与服务端测试全部完成后，删除 legacy `as_of` 兼容读取与文档口径。

### 5.3 Prompt/View 对齐

- planner / clarifier / narrator 的 `page_context` 只再暴露 `effective_date`。
- 不允许在 prompt-view 中继续同时暴露 `effective_date` 与 `as_of`，避免第二套时间语义并存。

## 6. 验收

1. `/internal/cubebox/turns:stream` 请求体中的 `page_context.view` 只发送 `effective_date`。
2. 服务端规范化后的 `PageContext` 只输出 `effective_date`。
3. 组织详情页、组织列表页相关前端测试全部切到 `effective_date`。
4. `468` / readiness / 相关 plan 文档中的 `page_context` 日期字段口径完成回写。
5. legacy `as_of` 兼容删除后，测试与门禁仍通过。

## 7. 测试与验证

- 前端：
  - `apps/web/src/pages/cubebox/CubeBoxProvider.test.tsx`
  - `apps/web/src/pages/cubebox/api.test.ts`
- Go：
  - `modules/cubebox/page_context.go` 相关测试
  - `internal/server/cubebox_api_test.go`
- 文档：
  - `make check doc`

## 8. Stopline

1. 不得把此次 DTO 改名与新的页面事实扩容绑在一起。
2. 不得在前端保留长期双写 `effective_date + as_of`。
3. 不得在 prompt-view 中继续暴露 legacy `as_of`。

## 9. 当前状态

- [x] 新建计划文档，作为 `page_context.view.as_of -> effective_date` 的唯一 owner。
- [ ] DTO 改名实现
- [ ] 兼容窗口落地
- [ ] 测试与文档回写

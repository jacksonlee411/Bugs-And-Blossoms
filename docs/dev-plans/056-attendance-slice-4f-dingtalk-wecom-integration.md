# DEV-PLAN-056：考勤 Slice 4F——生态集成闭环（钉钉 Stream / 企微 Poller）

**状态**: 草拟中（2026-01-09 02:17 UTC）

## 1. 背景与上下文

- 本计划在考勤内核/读模稳定后接入外部来源（钉钉/企微），避免平台差异污染内核，并确保外部数据与手工数据同口径可重算（对齐 `DEV-PLAN-050` Slice 4F）。
- 生态集成的重点不是“拿到平台给的计算结果”，而是**拿到可信输入**（timestamp + 外部身份），其余计算全部由本仓库的规则体系完成。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] **接入能力**：
  - 钉钉：Stream 模式接入（WebSocket），接收实时打卡事件。
  - 企微：Poller 拉取增量打卡（周期可配置）。
- [ ] **身份映射**：将外部 userId/unionId 映射到 `person_uuid`（映射策略与落点需对齐模块边界；不得在 attendance 子域里自造跨模块回退规则）。
- [ ] **幂等与去重**：对外部事件必须实现幂等写入（`source_provider + source_event_id` 等），并与 `request_id` 幂等互补；不得通过回写事件表标记处理进度。
- [ ] **验收闭环**：外部事件进入后可在“打卡流水/日结果/余额”页面可见，并与手工事件同口径。

### 2.2 非目标（本计划不做）

- 不做“对外回写平台”的回执/对账闭环（如需另立计划）。
- 不引入新的消息队列作为权威写路径；限流/重试/回执属于副作用链路，不得写权威读模（对齐 `AGENTS.md` §3.6）。

## 3. 关键设计决策（草案）

- **外部字段不作为权威输入**：忽略平台侧 `timeResult`（正常/迟到/早退）等计算字段；仅保留原始 payload 作为审计/排障输入。
- **幂等键落地方式**：
  - 若事件表需要分区（按 `punch_time`），则外部幂等键的全局唯一需通过“包含分区键的约束”或“独立去重表”实现；不得牺牲正确性以求省事。
- **限流与可靠性**：限流/重试优先使用现有 Redis 能力与最小可复现配置，不引入额外运维组件。

## 4. 数据模型（草案；落地迁移前需手工确认）

> 红线：新增数据库表/迁移落地前必须获得你手工确认。

- 候选表（示意）：
  - `external_identity_map`（外部身份 → person_uuid，落点需评审：`iam`/`person`/`staffing`）
  - `time_punch_dedup`（外部幂等键去重表：`tenant_id + source_provider + source_event_id` unique）

## 5. 工具链与门禁（SSOT 引用）

### 5.1 触发器（勾选本计划命中的项）

- [ ] Go 代码
- [ ] DB 迁移 / Schema（Atlas+Goose）
- [ ] sqlc
- [ ] 路由治理（Routing）
- [ ] Authz（Casbin）
- [ ] `.templ` / Tailwind（UI 资源）

### 5.2 SSOT 链接

- 触发器矩阵：`AGENTS.md`
- CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
- RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 路由：`docs/dev-plans/017-routing-strategy.md`
- Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
- 上游切片：`docs/dev-plans/055-attendance-slice-4e-corrections-audit-recalc.md`
- 蓝图：`docs/dev-plans/050-hrms-attendance-blueprint.md`

## 6. 实施步骤

1. [ ] 明确外部身份映射的所有权与落点（对齐 `DEV-PLAN-016` 的跨模块契约），并评审（落迁移前获得手工确认）。
2. [ ] 设计外部事件幂等策略（含分区 unique 限制的解决方案），并评审。
3. [ ] 实现 DingTalk Stream adapter（最小链路：接收→解析→映射 person_uuid→调用 `submit_*_event`）。
4. [ ] 实现 WeCom Poller（最小链路：拉取→解析→映射→写入），并加入限流（令牌桶）与重试策略。
5. [ ] sqlc/DB：如需去重表或映射表，落迁移并按 024 闭环；写路径仍只允许走 `submit_*_event`。
6. [ ] UI：最小可发现入口（例如“集成状态/最近同步时间/错误摘要”，可选），并确保外部事件在流水/日结果页可见。
7. [ ] 测试：使用可复现的 stub/fixture 验证幂等、跨租户隔离、错误重试不会写坏权威读模。
8. [ ] 运行门禁并记录结果（必要时补 `docs/dev-records/`）。

## 7. 验收标准

- [ ] 外部事件进入后，流水/日结果/余额页面可见且与手工事件同口径。
- [ ] 幂等成立：重复投递不会产生重复事件或重复影响读模。
- [ ] 安全成立：RLS/Authz/路由/生成物门禁全绿。

## 8. 开放问题

- [ ] 外部身份映射的 UX：映射缺失时如何引导用户补齐（而不是隐式丢弃/回退）。
- [ ] Stream/Poller 的运行形态：与现有服务编排（DevHub/compose）如何对齐并保持可复现。


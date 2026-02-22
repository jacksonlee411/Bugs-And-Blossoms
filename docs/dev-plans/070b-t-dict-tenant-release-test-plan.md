# DEV-PLAN-070B-T：070B 系列目标达成测试方案（字典租户本地发布）

**状态**: 开发验证完成（T1-T3，2026-02-22 10:10 UTC）

## 1. 背景与范围

- 本方案承接 `DEV-PLAN-070B`（后端与迁移链路）和 `DEV-PLAN-070B1`（UI 可视化操作），目标是验证“共享改发布、运行时 tenant-only、无 legacy 回退”是否真正达成。
- 本方案是测试契约，不新增业务能力；若发现契约缺口，先回写相应 dev-plan，再实施代码。

## 2. 测试目标与非目标

### 2.1 测试目标（DoD）
- [x] 证明运行时字典读取链路已 tenant-only，不再触发 `global_tenant` fallback。
- [x] 证明“预检 -> 发布”链路可用且可审计（`release_id/request_id/operator/source_tenant_id/as_of`）。
- [x] 证明冲突与异常路径 fail-closed（不误发布、不旁路写）。
- [x] 证明权限边界一致（UI 权限态与后端鉴权一致）。
- [x] 证明 `STD-001` / `STD-002` 在 070B 场景落地。
- [x] 证明 070B1 页面满足“可发现、可操作、可解释”的用户可见性原则。
- [ ] 证明切流窗口（T-1/T0/T+1）可执行并可回溯。

### 2.2 非目标
- 不做性能压测与长期运维监控（对齐 `AGENTS.md` §3.6）。
- 不覆盖 scope package 全域发布测试（另由后续 070B2 承接）。
- 不在本计划引入新迁移或新表。

## 3. 关联契约与标准（SSOT）

- 主契约：
  - `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
  - `docs/dev-plans/070b1-dict-release-ui-operations-plan.md`
- 前置契约：
  - `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
  - `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- 评审与标准：
  - `docs/dev-plans/003-simple-not-easy-review-guide.md`
  - `docs/dev-plans/005-project-standards-and-spec-adoption.md`
  - `docs/dev-plans/002-ui-design-guidelines.md`
- 门禁入口：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`

## 4. 测试分层与覆盖矩阵

| 测试层 | 覆盖目标 | 关键断言 |
| --- | --- | --- |
| L1 单元测试 | 参数与状态机正确 | 输入校验、状态转移、防重入、错误映射稳定 |
| L2 集成测试（API+Store+DB） | 发布能力与租户边界正确 | preview/release、冲突阻断、幂等/冲突、审计字段完整 |
| L3 UI 集成测试 | 070B1 可操作闭环 | 表单输入、预检结果、冲突展示、按钮禁用/解锁 |
| L4 E2E（浏览器） | 用户可见性与真实链路 | `/dicts` 可发现，完整“预检->发布->结果”路径 |
| L5 工具与切流演练 | 迁移可执行与可回溯 | 回填脚本/对账脚本/runbook 证据齐全 |

## 5. 核心用例清单（围绕 070B 目标）

### 5.1 运行时 tenant-only（无 global fallback）
- [x] `GET /iam/api/dicts`：仅返回当前租户数据；空结果不回退 global。
- [x] `GET /iam/api/dicts/values`：tenant-only；无跨租户读。
- [x] `ResolveValueLabel/ListOptions`：tenant 无数据返回稳定错误，不隐式共享读。
- [x] grep/静态门禁确认无新增 `global_tenant` 读取分支。

### 5.2 发布链路与幂等语义
- [x] preview 成功返回可发布结果（HTTP 200）。
- [x] preview 冲突返回 HTTP 409 + 冲突样例。
- [x] release 成功返回 HTTP 201 + 结果统计。
- [x] 同 `(tenant, request_id, payload)` 重试幂等成功（`was_retry` 可观测）。
- [x] 同 `(tenant, request_id)` 不同 payload 返回冲突（fail-closed）。

### 5.3 基线未就绪与失败路径
- [x] 新租户未导入基线时，写入口返回 `dict_baseline_not_ready`。
- [x] 读侧参数非法（如 `as_of`）返回 `invalid_as_of`。
- [x] 写侧 `effective_date` 缺失/非法时返回 `invalid_effective_date`（覆盖字典写 API）。
- [x] 任何失败场景不允许回退旧链路或旁路写入（No Legacy）。

### 5.4 权限边界
- [x] 无 `iam.dict_release/admin` 时，release/preview API 必须 403。
- [x] 070B1 UI 中“执行发布”按发布权限控制（禁用或隐藏，并有说明）。
- [x] 业务读写权限回归：`dict.read` 可读、`dict.admin` 可写、无权限 fail-closed。
- [x] 不出现“UI 可点 -> 后端 403”长期漂移（需有联调用例）。

### 5.5 审计可追溯
- [x] 发布结果可回溯：`release_id/request_id/operator/source_tenant_id/as_of`。
- [x] 事件链可解释“谁在何时向哪个租户发布了哪些字典值”。

### 5.6 标准对齐（DEV-PLAN-005）
- [x] `STD-001`：对外与内部统一使用 `request_id`，不得新增/外泄 `request_code` 命名。
- [x] `STD-001`：`trace_id` 与幂等语义分离。
- [x] `STD-002`：`as_of/effective_date` 显式输入且无 default today；错误码口径一致。

### 5.7 一致性校验（与 070B §5.2 对齐）
- [ ] 回填后按租户校验 `dict_code` 数量与基线期望一致。
- [ ] 关键字典（首批 `org_type`）按双 `as_of` 日期抽样比对一致。
- [ ] `enabled_on/disabled_on` 窗口边界抽样一致（至少 1 组启用、1 组停用样本）。
- [ ] 切流后业务 API 调用链不访问 global 字典路径。

## 6. 070B1 UI 专项验收（对齐 002/003）

### 6.1 信息架构与页面关系
- [x] 不新增路由；在现有 `/dicts` 页面内可发现“字典发布”操作区。
- [x] 页面至少支持一次完整发布闭环，不是只读展示。

### 6.2 交互状态机
- [x] 覆盖 `idle/previewing/conflict/ready/releasing/success/fail` 七态。
- [x] `conflict` 态不可执行发布；`releasing` 态不可重复提交。

### 6.3 按钮层级/A11y/i18n
- [x] 同任务域只有一个 Primary（执行发布）；预检为 Secondary。
- [x] 键盘可达、焦点可见、`aria-label` 完整、错误可读。
- [x] 新增文案仅走 `en/zh` i18n key，无硬编码。

## 7. 测试数据与环境

### 7.1 租户与角色
- [x] 准备 `global_tenant`（基线源）与至少 2 个业务租户（目标租户 + 对照租户）。
- [x] 准备 `tenant-admin`、`tenant-viewer`（无写权限）、`release-admin`（有发布权限）角色组合。

### 7.2 字典数据集
- [ ] 基线源租户包含最小可验证字典集（建议包含 `org_type` 与至少 1 条停用记录）。
- [ ] 目标租户准备三类初始态：空租户、已有一致数据、已有冲突覆盖数据。

### 7.3 时间切片
- [x] 固定至少两个 `as_of` 日期，验证跨日重放一致性。
- [x] 固定至少一个 `effective_date` 错误样本，用于 `invalid_effective_date` 用例。

## 8. 分阶段执行与退出准则

### T1：契约与单元阶段
- [x] 完成 L1 用例并通过。
- [x] 退出准则：参数/状态机/错误映射无阻塞缺陷。

### T2：集成阶段
- [x] 完成 L2 用例并通过。
- [x] 退出准则：tenant-only、幂等、冲突、审计核心链路均有证据；切流一致性对账留待 T4/T5。

### T3：UI/E2E 阶段
- [x] 完成 L3+L4 用例并通过。
- [x] 退出准则：070B1 用户可见性闭环成立，权限与按钮层级符合 002。

### T4：切流演练阶段
- [ ] 完成 L5 脚本演练（回填、对账、runbook）。
- [ ] 退出准则：可按 runbook 执行“停写->最终增量发布->验收->恢复写入”。

### T5：发布窗口验证阶段（新增）
- [ ] T-1：完成变更评审记录（范围、责任人、回滚口径、通知名单）。
- [ ] T0：完成停写窗口实操记录（停写、最终增量发布、部署、验收、恢复写入）。
- [ ] T+1：完成 24h 抽样巡检记录（重点租户、错误码、审计链、异常闭环）。
- [ ] 退出准则：T-1/T0/T+1 三段证据齐全且无 P0 未闭环。

## 9. 门禁与证据记录

> 具体命令不在本计划复制，统一引用 SSOT；以下只声明命中项与证据类型。

- [x] Go 变更门禁：格式化、静态检查、单测通过。
- [x] Authz 门禁：策略打包/测试/lint 通过。
- [x] Routing 门禁：若路由/allowlist 变更则通过。
- [x] E2E 门禁：发布链路用例通过。
- [x] 文档门禁：`make check doc` 通过。

证据归档：
- [x] 在 `docs/dev-records/dev-plan-070b-execution-log.md` 增补测试批次记录（时间、环境、命中门禁、结果、缺陷链接）。
- [x] 对每个阻塞缺陷给出分类：`BUG` / `CONTRACT_DRIFT` / `CONTRACT_MISSING` / `ENV_DRIFT`。
- [x] 对关键通过项记录“输入参数 + 期望 + 实际 + 证据链接”，保证可复查。

## 10. 070B 目标覆盖映射（评审用）

| 070B 目标/场景 | 070B-T 对应章节 |
| --- | --- |
| 运行时取消 global 读取 | §5.1、§5.7 |
| 共享改发布单链路 | §5.2、§5.5 |
| One Door / No Legacy / fail-closed | §5.3、§5.4 |
| 迁移分期与切流验证 | §8（T1~T5） |
| 一致性校验（dict_code/org_type/窗口边界） | §5.7 |
| 102B 时间口径（as_of/effective_date） | §5.3、§5.6、§7.3 |
| 070B1 UI 可见可操作 | §6 |

## 11. 风险与缓解

- 风险：联调环境角色不齐，导致权限用例无法收敛。  
  缓解：提前冻结最小角色矩阵，缺口按 `CONTRACT_MISSING` 立项。
- 风险：迁移数据冲突规模大，影响用例稳定性。  
  缓解：先跑 preview 冲突分层样本，再扩到全量租户。
- 风险：UI 按钮层级与后端契约不同步。  
  缓解：将 `permissionKey` 与 object/action 映射纳入必测断言。
- 风险：历史遗留文档或提示语出现 `request_code` 命名，导致验收歧义。  
  缓解：按 §5.6 做静态检索与提示语回归，统一收敛到 `request_id`。

## 12. 交付物

1. 本测试方案文档：`docs/dev-plans/070b-t-dict-tenant-release-test-plan.md`
2. 测试执行记录（增补）：`docs/dev-records/dev-plan-070b-execution-log.md`
3. 缺陷与偏差清单：Issue/PR 链接（在执行记录中汇总）

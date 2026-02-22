# DEV-PLAN-102B：070/071 时间口径强制显式化与历史回放稳定性收敛

**状态**: 草拟中（2026-02-22 02:22 UTC）

## 1. 背景与上下文 (Context)
- **需求来源**：针对 070/071 系列方案落地后的复盘反馈：部分接口仍会在缺省参数时默认“今天（`current_date` / `time.Now().UTC()`）”，导致同一问题在不同日期回看时结果漂移。
- **当前痛点**：
  1) 文档契约存在冲突：
     - 070 明确“`as_of_date` 必填、禁止默认 `current_date`”；
     - 071 仍在多个 API/算法处声明“缺省默认 `current_date`”。
  2) 实现层存在隐式默认：070/071 相关 handler、controller、SQL 函数仍有“缺省即今天”的回填。
  3) 测试层已固化默认行为：存在“default as_of/default effective_date”测试，放大了口径漂移风险。
- **业务价值**：统一“显式时间上下文”后，保证同一输入（包含时间参数）在未来任意时点可重放、可审计、可解释，消除“今天看一个结果、回看历史又是另一个”的不确定性。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [ ] 070/071 全链路收敛为**显式时间上下文**：读接口强制 `as_of`，写接口强制 `effective_date`（或等价业务生效日字段）。
- [ ] 移除 070/071 范围内所有隐式“今天”默认（Go/API/SQL），并以 fail-closed 返回明确错误码。
- [ ] 修订 070/071/071A/071B/102 文档契约，消除“可选默认 today”与“必填禁止默认”冲突。
- [ ] 新增回归门禁：阻断新增 `as_of/effective_date` 的隐式默认逻辑回漂。
- [ ] 建立“历史回放一致性”测试集：同一请求在不同运行日执行，结果仅由显式时间参数决定。
- [ ] 输出 readiness 证据与执行日志，形成可审计交付。

### 2.2 非目标 (Out of Scope)
- 不重做 070/071 的业务边界（SetID/Scope Package/Subscription 语义不变）。
- 不引入 Feature Flag、灰度双链路或 legacy fallback。
- 不在本计划内扩展到与 070/071 无关的全部模块（仅处理被 070/071 直接约束和依赖的入口）。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（勾选本计划命中的项）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind（若涉及 UI 生成物则补跑 `make generate && make css`）
  - [ ] 多语言 JSON（`make check tr`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] 路由治理（`make check routing`）
  - [x] DB 迁移 / Schema（`make orgunit plan/lint/migrate up`，按需补 jobcatalog/staffing）
  - [ ] sqlc（如有 schema/query 变更则 `make sqlc-generate`）
  - [x] 文档（`make check doc`）
- **SSOT 链接**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 关键设计决策 (ADR 摘要)
- **决策 1：时间参数从“可缺省”改为“显式必填”**
  - 选项 A：保留默认 today，仅补提示。缺点：历史回放仍不稳定。
  - 选项 B（选定）：接口缺少时间参数即 400/422 fail-closed。优点：行为确定、可审计。
- **决策 2：禁止在服务端推断 today 作为业务时间**
  - 选项 A：由 handler/controller 自动回填。缺点：同请求跨天结果漂移。
  - 选项 B（选定）：仅允许客户端显式传参；服务端仅校验与透传。
- **决策 3：Bootstrap/回填时间锚点显式化**
  - 选项 A：继续 `current_date`。缺点：同逻辑重放日期不可控。
  - 选项 B（选定）：必须使用显式 `effective_date`，且需通过“租户 root BU 在该日有效”校验；禁止固定常量日期兜底。
- **决策 4：一次性切口，不保留兼容默认路径**
  - 选项 A：兼容窗口双路径。缺点：违反 No Legacy，增加歧义。
  - 选项 B（选定）：同版本完成契约与实现同步收口。
- **决策 5：错误码口径冻结为 `invalid_*`**
  - 选项 A：新增 `missing_*` 与 `invalid_*` 双口径并存。缺点：调用方处理分叉。
  - 选项 B（选定）：统一返回 `invalid_as_of` / `invalid_effective_date`，其中“缺失”通过 message 明确 `... required`。

### 3.2 口径冻结（本计划 SSOT）
- `as_of`：读模型切片时间，**必须显式提供**。
- `effective_date`：写入生效日期，**必须显式提供**。
- 任何 `target_effective_date` / `enabled_on` / `disabled_on` / `correction_day` 等业务日字段继续保持必填，不得回退到 today。
- 仅审计时间（`created_at/updated_at/transaction_time`）允许使用系统时钟；业务有效时间一律来自请求显式参数。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 SQL / Kernel 收敛点
- [ ] `orgunit.submit_setid_event`：`CREATE/BOOTSTRAP` 时 `payload.effective_date` 必填；移除 `v_effective_date := current_date` 兜底。
- [ ] `orgunit.ensure_setid_bootstrap`：消除对 `current_date` 的逻辑分支依赖；root BU 校验与 bootstrap 链路统一使用显式锚点日。
- [ ] `orgunit.submit_setid_event` 内订阅存在性判断：`s.validity @> v_effective_date`，禁止 `@> current_date`。
- [ ] Scope Package Disable 写链路收敛：停用事件生效日必须显式输入（`effective_date`），禁止在服务端/store 生成 today。
- [ ] 复核 `orgunit.resolve_scope_package` / `assert_scope_package_active_as_of`：保持 `p_as_of_date IS NULL` 即报错，不新增默认分支。

### 4.2 迁移策略
- [ ] 如函数签名需变更，采用“新增函数签名 + 调用方切换 + 清理旧签名”的前向迁移。
- [ ] bootstrap/backfill 脚本必须要求调用方显式提供 `effective_date` 参数；参数需满足“root BU 在该日有效”，不满足即 fail-closed。
- [ ] 如涉及 `POST /org/api/scope-packages/{package_id}/disable` 请求体变更（新增 `effective_date`），同步完成 API 合同、前端调用、E2E 与错误码对齐。

## 5. 接口契约 (API Contracts)
### 5.1 070/071 相关读接口（统一改为 as_of 必填）
- [ ] `GET /org/api/setid-bindings?as_of=YYYY-MM-DD`
- [ ] `GET /org/api/owned-scope-packages?scope_code=...&as_of=YYYY-MM-DD`
- [ ] `GET /org/api/scope-subscriptions?setid=...&scope_code=...&as_of=YYYY-MM-DD`
- [ ] `GET /jobcatalog/api/catalog?as_of=YYYY-MM-DD&...`
- [ ] `GET /org/api/positions?as_of=YYYY-MM-DD`
- [ ] `GET /org/api/positions:options?as_of=YYYY-MM-DD&org_code=...`
- [ ] `GET /org/api/assignments?as_of=YYYY-MM-DD&person_uuid=...`

缺失参数统一返回：
- `400 invalid_as_of` + message `as_of required`（冻结口径，不再引入 `missing_as_of`）。

### 5.2 070/071 相关写接口（统一改为 effective_date 必填）
- [ ] `POST /org/api/setids`（`effective_date` 必填）
- [ ] `POST /org/api/scope-packages`（`effective_date` 必填）
- [ ] `POST /org/api/scope-packages/{package_id}/disable`（`effective_date` 必填，不允许服务端生成 today）
- [ ] `POST /org/api/global-scope-packages`（`effective_date` 必填）
- [ ] `POST /jobcatalog/api/catalog/actions`（`effective_date` 必填）
- [ ] `POST /org/api/positions`（`effective_date` 必填；不再从 `as_of` 回填）
- [ ] `POST /org/api/assignments`（`effective_date` 必填；不再从 `as_of` 回填）
- [ ] `POST /org/api/scope-subscriptions` / `POST /org/api/setid-bindings` 继续保持 `effective_date` 必填（已有约束不得回退）。

缺失参数统一返回：
- `400 invalid_effective_date` + message `effective_date required`（冻结口径，不再引入 `missing_effective_date`）。

### 5.3 UI 交互约束
- [ ] `/app/org/setid`、`/app/jobcatalog`、`/app/staffing/positions`、`/app/staffing/assignments` 页面请求必须显式携带时间参数；页面内部可以预填“今天”，但提交/请求前必须落到 URL 或 payload。
- [ ] 禁止“页面未显式日期但后端自动 today”的隐式行为。

## 6. 核心逻辑与算法 (Business Logic & Algorithms)
### 6.1 统一日期解析器（服务端）
1. 读取参数（query/body）。
2. 判空：空值直接返回 `invalid_as_of` / `invalid_effective_date` + `... required`。
3. 解析：仅接受 `YYYY-MM-DD`。
4. 透传到 service/store/kernel，不做 today 回填。

### 6.2 历史回放确定性规则
1. 任何读写行为必须可由 `(tenant, route, payload/query, as_of/effective_date)` 完全重放。
2. 业务结果不得依赖执行当日。
3. 回放测试在两个不同系统日执行，断言结果一致（除审计时间戳字段）。

## 7. 安全与鉴权 (Security & Authz)
- 继续遵守 No Tx, No RLS / One Door / shared read switch 规则，不因时间参数收口放宽任何权限边界。
- 时间参数缺失一律 fail-closed，不以默认 today 替代，避免“无意放大可见范围/误读历史”。

## 8. 依赖与里程碑 (Dependencies & Milestones)
### 8.1 依赖
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`

### 8.2 里程碑
1. [ ] **M1 契约冻结**：修订 070/071/071A/071B/102 的时间参数规则，移除“默认 today”描述。
2. [ ] **M2 API 收口**：移除 handler/controller 默认回填，统一 `invalid_* + required message`；含 `scope-packages/{package_id}/disable`。
3. [ ] **M3 Kernel 收口**：移除 SQL 函数内 `current_date` 业务口径分支，改为显式日期。
4. [ ] **M4 测试重构**：删除“default as_of/effective_date”用例，新增“missing required date -> fail”与“跨天重放一致性”用例。
5. [ ] **M5 门禁落地**：新增 `make check as-of-explicit`（名称可调整）并接入 CI，阻断回漂。
6. [ ] **M6 证据归档**：更新 `docs/dev-records/dev-plan-102b-execution-log.md`，记录命令、结果、风险、回滚说明。

## 9. 测试与验收标准 (Acceptance Criteria)
### 9.1 单元/接口测试
- [ ] 070/071 涉及的读接口：缺失 `as_of` 必须失败（400），非法日期必须失败。
- [ ] 070/071 涉及的写接口：缺失 `effective_date` 必须失败（400），非法日期必须失败。
- [ ] `POST /org/api/scope-packages/{package_id}/disable`：缺失/非法 `effective_date` 必须失败（400）；显式同日请求可稳定重放。
- [ ] 原“默认 today 成功”的测试全部替换为显式参数测试。

### 9.2 集成/回放测试
- [ ] 构造固定数据集，分别在两次不同执行日运行同一批请求（参数含显式日期），结果一致。
- [ ] 订阅/包停用/历史切片场景：`as_of` 指定历史日能稳定回放，未来日 fail-closed 行为稳定。

### 9.3 门禁与静态检查
- [ ] 新增检查脚本扫描 070/071 范围内禁止模式：
  - `if asOf == "" { asOf = time.Now().UTC().Format("2006-01-02") }`
  - `if req.EffectiveDate == "" { req.EffectiveDate = ... }`
  - `effectiveDate := time.Now().UTC().Format("2006-01-02")`（用于业务生效日）
  - SQL 中以 `current_date` 参与 070/071 业务有效期判断（白名单除外）。
- [ ] CI 门禁可阻断新增隐式默认逻辑。

### 9.4 完成判定
- [ ] 文档契约与代码行为一致，不再存在“文档说必填、实现却默认 today”的冲突。
- [ ] 用户复盘同一问题时，显式同一日期参数得到稳定一致结果。

## 10. 运维与发布策略 (Ops & Release)
- 不引入功能开关，不保留 legacy 兼容分支。
- 采用前向修复：合同、实现、测试、门禁同版本收口。
- 若发布后出现误用（调用方漏传日期）：按 fail-closed 返回明确错误码，调用方修复后重试。
- 回滚策略：仅允许环境级保护（只读/停写）+ 前向补丁，不回退到“默认 today”旧行为。

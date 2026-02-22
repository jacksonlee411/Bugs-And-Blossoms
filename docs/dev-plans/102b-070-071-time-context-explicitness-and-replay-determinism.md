# DEV-PLAN-102B：070/071 时间口径强制显式化与历史回放稳定性收敛

**状态**: 已完成（2026-02-22 13:30 UTC，M1-M6 已完成并合并 PR #399）

## 0. 主计划定位（Plan of Record）
- 本计划是 070/071 时间参数收敛的 **Plan of Record（PoR）**，后续实现与验收以本计划为准。
- 相关方案（070/071/071A/071B/102/026A/063）中的冲突口径按本计划与 `STD-002` 修订，不再并列解释。
- 实施顺序：先完成文档契约收口（M1）→ 再执行 API/Kernel/测试与门禁改造（M2-M5）→ 最后归档证据（M6）。

## 1. 背景与上下文 (Context)
- **需求来源**：针对 070/071 系列方案落地后的复盘反馈：部分接口仍会在缺省参数时默认“今天（`current_date` / `time.Now().UTC()`）”，导致同一问题在不同日期回看时结果漂移。
- **当前痛点**：
  1) 文档契约存在冲突：
     - 070 明确“`as_of_date` 必填、禁止默认 `current_date`”；
     - 071 仍在多个 API/算法处声明“缺省默认 `current_date`”。
  2) 实现层存在隐式默认：070/071 相关 handler、controller、SQL 函数仍有“缺省即今天”的回填。
  3) 测试层已固化默认行为：存在“default as_of/default effective_date”测试，放大了口径漂移风险。
- **业务价值**：统一“显式时间上下文”后，保证同一输入（包含时间参数）在未来任意时点可重放、可审计、可解释，消除“今天看一个结果、回看历史又是另一个”的不确定性。

### 1.1 调查结论（来源追溯，2026-02-22）
- **结论 A（基线）**：`DEV-PLAN-070` 已明确“`as_of_date` 必填、禁止默认 `current_date`”，并非缺省 today 的来源（见 070 §6.1/§3.1）。
- **结论 B（直接引入点）**：`DEV-PLAN-071` 在 API/算法/回填说明中多处引入“空值默认 `current_date`”，是 070/071 口径冲突的直接来源（如 071 §5.1、§5.2、§6.1、§6.3、迁移回填段）。
- **结论 C（历史上游）**：`DEV-PLAN-026A` 更早出现“`as_of/effective_date` 可选且缺省当日 UTC”的合同描述，属于同类口径来源（需标注为历史文档口径，避免继续外溢）。
- **结论 D（后续固化）**：`DEV-PLAN-102` 路由矩阵冻结了多处“缺省回退当天/302 补齐当天”的行为，放大了与 070 的冲突。
- **时间线（精确日期）**：
  - `2026-01-29`：070 文档冻结“禁止默认 today”；
  - `2026-02-01`：071/026A 文档出现“缺省 current_date/当日 UTC”描述；
  - `2026-02-14`：102 文档将“缺省回退当天”写入矩阵并标记完成。

## 2. 目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [x] 070/071 全链路收敛为**显式时间上下文**：读接口强制 `as_of`，写接口强制 `effective_date`（或等价业务生效日字段）。
- [x] 移除 070/071 范围内所有隐式“今天”默认（Go/API/SQL），并以 fail-closed 返回明确错误码。
- [x] 修订 070/071/071A/071B/102 文档契约，消除“可选默认 today”与“必填禁止默认”冲突。
- [x] 新增回归门禁：阻断新增 `as_of/effective_date` 的隐式默认逻辑回漂。
- [ ] 建立“历史回放一致性”测试集：同一请求在不同运行日执行，结果仅由显式时间参数决定（后续增强，见 §9.2.1）。
- [x] 输出 readiness 证据与执行日志，形成可审计交付。

### 2.2 非目标 (Out of Scope)
- 不重做 070/071 的业务边界（SetID/Scope Package/Subscription 语义不变）。
- 不引入 Feature Flag、灰度双链路或 legacy fallback。
- 不在本计划内扩展到与 070/071 无关的全部模块（仅处理被 070/071 直接约束和依赖的入口）。
- 不重写 026A 的历史目标；仅修订其与 070/071 冲突的时间参数表述，避免继续被引用为“默认 today”依据。

## 2.3 工具链与门禁（SSOT 引用）
- **触发器清单（勾选本计划命中的项）**：
  - [x] Go 代码（`go fmt ./... && go vet ./... && make check lint && make test`）
  - [ ] `.templ` / Tailwind（若涉及 UI 生成物则补跑 `make generate && make css`）
  - [ ] 多语言 JSON（`make check tr`）
  - [x] Authz（`make authz-pack && make authz-test && make authz-lint`）
  - [x] 路由治理（`make check routing`）
  - [x] DB 迁移 / Schema（`make orgunit plan/lint/migrate up`，按需补 jobcatalog/staffing）
  - [x] sqlc（如有 schema/query 变更则 `make sqlc-generate`）
  - [x] 文档（`make check doc`）
- **SSOT 链接**：`AGENTS.md`、`Makefile`、`.github/workflows/quality-gates.yml`、`docs/dev-plans/012-ci-quality-gates.md`
- **标准入口**：`docs/dev-plans/005-project-standards-and-spec-adoption.md`（`STD-002`）

## 3. 架构与关键决策 (Architecture & Decisions)
### 3.1 关键设计决策 (ADR 摘要)
- **决策 1：时间参数从“可缺省”改为“显式必填”**
  - 选项 A：保留默认 today，仅补提示。缺点：历史回放仍不稳定。
  - 选项 B（选定）：接口缺少时间参数即 400 fail-closed。优点：行为确定、可审计。
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

### 3.3 错误响应冻结（避免 400/422 分叉）
- HTTP 状态码统一使用 `400`，不在本计划范围内引入 `422`。
- 错误码统一使用：
  - 读接口缺失/非法：`invalid_as_of`
  - 写接口缺失/非法：`invalid_effective_date`
- 缺失参数通过 message 明确 `as_of required` / `effective_date required`。
- `STD-002` 为唯一规范来源；出现历史文档差异时，以本节为准并在执行日志登记修订证据。

## 4. 数据模型与约束 (Data Model & Constraints)
### 4.1 SQL / Kernel 收敛点
- [x] `orgunit.submit_setid_event`：`CREATE/BOOTSTRAP` 时 `payload.effective_date` 必填；移除 `v_effective_date := current_date` 兜底。
- [x] `orgunit.ensure_setid_bootstrap`：消除对 `current_date` 的逻辑分支依赖；root BU 校验与 bootstrap 链路统一使用显式锚点日。
- [x] `orgunit.submit_setid_event` 内订阅存在性判断：`s.validity @> v_effective_date`，禁止 `@> current_date`。
- [x] Scope Package Disable 写链路收敛：停用事件生效日必须显式输入（`effective_date`），禁止在服务端/store 生成 today。
- [x] 复核 `orgunit.resolve_scope_package` / `assert_scope_package_active_as_of`：保持 `p_as_of_date IS NULL` 即报错，不新增默认分支。

### 4.2 迁移策略
- [x] 如函数签名需变更，采用“新增函数签名 + 调用方切换 + 清理旧签名”的前向迁移。
- [x] bootstrap/backfill 脚本必须要求调用方显式提供 `effective_date` 参数；参数需满足“root BU 在该日有效”，不满足即 fail-closed。
- [x] 如涉及 `POST /org/api/scope-packages/{package_id}/disable` 请求体变更（新增 `effective_date`），同步完成 API 合同、前端调用、E2E 与错误码对齐。

## 5. 接口契约 (API Contracts)
### 5.1 070/071 相关读接口（统一改为 as_of 必填）
- [x] `GET /org/api/setid-bindings?as_of=YYYY-MM-DD`
- [x] `GET /org/api/owned-scope-packages?scope_code=...&as_of=YYYY-MM-DD`
- [x] `GET /org/api/scope-subscriptions?setid=...&scope_code=...&as_of=YYYY-MM-DD`
- [x] `GET /jobcatalog/api/catalog?as_of=YYYY-MM-DD&...`
- [x] `GET /org/api/positions?as_of=YYYY-MM-DD`
- [x] `GET /org/api/positions:options?as_of=YYYY-MM-DD&org_code=...`
- [x] `GET /org/api/assignments?as_of=YYYY-MM-DD&person_uuid=...`

缺失参数统一返回：
- `400 invalid_as_of` + message `as_of required`（冻结口径，不再引入 `missing_as_of`）。

### 5.2 070/071 相关写接口（统一改为 effective_date 必填）
- [x] `POST /org/api/setids`（`effective_date` 必填）
- [x] `POST /org/api/scope-packages`（`effective_date` 必填）
- [x] `POST /org/api/scope-packages/{package_id}/disable`（`effective_date` 必填，不允许服务端生成 today）
- [x] `POST /org/api/global-scope-packages`（`effective_date` 必填）
- [x] `POST /jobcatalog/api/catalog/actions`（`effective_date` 必填）
- [x] `POST /org/api/positions`（`effective_date` 必填；不再从 `as_of` 回填）
- [x] `POST /org/api/assignments`（`effective_date` 必填；不再从 `as_of` 回填）
- [x] `POST /org/api/scope-subscriptions` / `POST /org/api/setid-bindings` 继续保持 `effective_date` 必填（已有约束不得回退）。

缺失参数统一返回：
- `400 invalid_effective_date` + message `effective_date required`（冻结口径，不再引入 `missing_effective_date`）。

### 5.3 UI 交互约束
- [x] `/app/org/setid`、`/app/jobcatalog`、`/app/staffing/positions`、`/app/staffing/assignments` 页面请求必须显式携带时间参数；页面内部可以预填“今天”，但提交/请求前必须落到 URL 或 payload。
- [x] 禁止“页面未显式日期但后端自动 today”的隐式行为。

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
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/archive/dev-plans/071b-field-config-and-dict-config-setid-boundary-implementation.md`
- `docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- `docs/archive/dev-plans/026a-orgunit-id-uuid-code-naming.md`（历史合同中存在“缺省当日 UTC”表述，需要最小修订）
- `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`（测试合同中存在“`effective_date` 缺省默认为 `as_of`”）

### 8.2 对相关计划的影响评估（调查结论落表）
1. **高影响（必须同版本收口）**
   - `DEV-PLAN-071`：删除/替换“可选+默认 today”合同；同步 API、算法、回填说明。
   - `DEV-PLAN-102`：将 §4.5 矩阵降级为“历史存档（非现行）”，并显式跳转到 `DEV-PLAN-102B`；避免形成双重权威。
2. **中高影响（承接计划需同步）**
   - `DEV-PLAN-071A`：其“保持 071 现有契约”的引用需随 071 改写；治理页/业务页提交参数改为显式必填。
3. **中影响（测试合同修订）**
   - `DEV-PLAN-063`：去除“`effective_date` 缺省默认为 `as_of`”描述，改为“缺失即 `invalid_effective_date`”。
4. **低影响（对齐/补充）**
   - `DEV-PLAN-071B`：主方向已一致（禁止隐式当前日），补充错误码与验收措辞对齐即可。
   - `DEV-PLAN-070`：无需改核心语义，仅补充“本计划已消除与 071/102 的冲突”引用证据。
   - `DEV-PLAN-026A`：作为历史文档执行最小文字修订，避免继续传播“默认当日 UTC”。

### 8.3 里程碑
1. [x] **M1 契约冻结**：修订 070/071/071A/071B/102/026A/063 的时间参数规则，移除“默认 today”描述并记录冲突消解表（2026-02-22 已完成文档收口）。
2. [x] **M2 API 收口**：移除 handler/controller 默认回填，统一 `invalid_* + required message`；含 `scope-packages/{package_id}/disable`。
3. [x] **M3 Kernel 收口**：移除 SQL 函数内 `current_date` 业务口径分支，改为显式日期。
4. [x] **M4 测试重构**：删除“default as_of/effective_date”用例，新增“missing required date -> fail”并补齐显式日期成功路径覆盖。
5. [x] **M5 门禁落地**：新增 `make check as-of-explicit`（名称可调整）并接入 CI，阻断回漂。
6. [x] **M6 证据归档**：更新 `docs/dev-records/dev-plan-102b-execution-log.md`，记录命令、结果、风险、回滚说明。

### 8.4 文档收口记录（2026-02-22）
- [x] `DEV-PLAN-005`：新增 `STD-002`（`as_of`/`effective_date` 语义标准）。
- [x] `DEV-PLAN-102B`：明确主实施计划（PoR）定位与 `STD-002` 绑定。
- [x] `DEV-PLAN-071`：移除 `as_of/effective_date` 默认 `current_date` 合同，补齐 disable 的 `effective_date` 必填。
- [x] `DEV-PLAN-071A`：订阅/包治理接口说明对齐 `DEV-PLAN-102B`（显式日期必填）。
- [x] `DEV-PLAN-071B`：补充 `STD-002` 引用，明确时间参数口径来源。
- [x] `DEV-PLAN-102`：将旧矩阵标注为历史附录（非现行），并增加“现行以 `DEV-PLAN-102B` 为准”的显式跳转说明。
- [x] `DEV-PLAN-063`：移除“`effective_date` 缺省默认为 `as_of`”测试合同。
- [x] `DEV-PLAN-026A`：增加历史合同勘误注记，冲突口径以 `STD-002`/`DEV-PLAN-102B` 为准。

## 9. 测试与验收标准 (Acceptance Criteria)
### 9.1 单元/接口测试
- [x] 070/071 涉及的读接口：缺失 `as_of` 必须失败（400），非法日期必须失败。
- [x] 070/071 涉及的写接口：缺失 `effective_date` 必须失败（400），非法日期必须失败。
- [x] `POST /org/api/scope-packages/{package_id}/disable`：缺失/非法 `effective_date` 必须失败（400）；显式同日请求可稳定重放。
- [x] 原“默认 today 成功”的测试全部替换为显式参数测试。

### 9.2 集成/回放测试
- [x] 订阅/包停用/历史切片场景：`as_of` 指定历史日能稳定回放，未来日 fail-closed 行为稳定（通过现有 E2E 与接口回归覆盖）。
- [ ] 构造固定数据集，分别在两次不同执行日运行同一批请求（参数含显式日期），结果一致（后续增强）。

### 9.2.1 回放测试机制（可执行化）
- [ ] 引入统一测试夹具：同一份数据库快照 + 同一批请求脚本 + 同一显式日期参数集（后续增强）。
- [ ] CI 中串行执行两轮回放（示例运行日锚点：`2026-03-01`、`2026-03-20`），仅允许审计字段（`created_at/updated_at/transaction_time`）差异（后续增强）。
- [ ] 输出结构化对比报告：请求键为 `(tenant, route, payload/query, as_of/effective_date)`；差异字段超出审计白名单即失败（后续增强）。
- [ ] 对比报告作为 readiness 证据归档到 `docs/dev-records/dev-plan-102b-execution-log.md`（后续增强）。

### 9.3 门禁与静态检查
- [x] 新增分层门禁 `make check as-of-explicit`（名称可调整）：
  - **L1（契约门禁）**：接口缺失日期参数时必须返回 `400 invalid_*`（契约测试，不依赖实现细节）。
  - **L2（Go 实现门禁）**：在 070/071 相关目录做规则扫描（正则），阻断“空值回填 today/互相回填”的代码路径。
  - **L3（SQL 门禁）**：阻断 `current_date` 参与 070/071 业务有效期判断；仅允许审计时间与显式白名单。
  - **L4（文档门禁）**：新增文档中若出现“as_of/effective_date 缺省 today”表述则失败（后续增强）。
- [x] 示例反例（用于规则单测）：
  - `if asOf == "" { asOf = time.Now().UTC().Format("2006-01-02") }`
  - `if req.EffectiveDate == "" { req.EffectiveDate = ... }`
  - `effectiveDate := time.Now().UTC().Format("2006-01-02")`（用于业务生效日）
- [x] CI 门禁可阻断新增隐式默认逻辑（基于规则扫描）。

### 9.4 完成判定
- [x] 文档契约与代码行为一致，不再存在“文档说必填、实现却默认 today”的冲突。
- [x] 用户复盘同一问题时，显式同一日期参数得到稳定一致结果。
- [x] 调查结论涉及的冲突文档（071/102/071A/026A/063）均完成改写，并在执行日志附“改写前后对照”。
- [x] `DEV-PLAN-102` 的旧矩阵已明确标注“历史存档（非现行）”，不存在与本计划并列生效的双重权威。

## 10. 运维与发布策略 (Ops & Release)
- 不引入功能开关，不保留 legacy 兼容分支。
- 采用前向修复：合同、实现、测试、门禁同版本收口。
- 若发布后出现误用（调用方漏传日期）：按 fail-closed 返回明确错误码，调用方修复后重试。
- 回滚策略：仅允许环境级保护（只读/停写）+ 前向补丁，不回退到“默认 today”旧行为。

### 10.1 故障处置 Runbook（No-Legacy）
1. **触发条件**（任一命中即进入处置）
   - 发布后 `invalid_as_of` / `invalid_effective_date` 异常激增，影响关键业务流程；
   - 回放一致性对比报告出现非审计字段差异；
   - 门禁发现新增隐式日期默认逻辑已进入主分支。
2. **责任分工**
   - 值班 owner：Org 平台值班（主责执行只读/停写与恢复判定）。
   - 修复 owner：对应模块开发 owner（主责补丁与回归）。
3. **处置步骤**
   - 先启用环境级保护：受影响写接口进入只读/停写；
   - 保留失败请求样本并定位缺参来源（调用方/服务端）；
   - 提交前向补丁，禁止引入 legacy fallback；
   - 执行 M2-M5 对应回归与回放测试；
   - 满足恢复判定后解除只读/停写。
4. **恢复判定**
   - 关键接口错误率恢复到基线；
   - 回放一致性报告仅剩审计字段差异；
   - `make check as-of-explicit`、`make check routing`、`make test` 通过并已留档。

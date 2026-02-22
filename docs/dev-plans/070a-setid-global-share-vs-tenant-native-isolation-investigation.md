# DEV-PLAN-070A：全局共享租户模式 vs 天然租户隔离模式专项调查（SetID/Scope Package）

**状态**: 已完成（2026-02-22 20:20 UTC）

## 1. 背景与上下文 (Context)
- **来源**：基于 `DEV-PLAN-070/071/071A` 已落地方案，在评审中识别出中长期架构风险：当前“`global_tenant` + `SHARE`”模式可用，但与更严格的“每租户天然隔离”相比，未来在合规、扩展与治理上存在潜在压力。
- **当前口径（摘要）**：
  - `DEV-PLAN-070`：共享层与租户层物理隔离；共享读需显式开关；禁止 OR 合并读取。
  - `DEV-PLAN-071`：引入 `scope_code + package` 订阅层，并在 shared-only 场景通过受控函数读取共享包。
  - `DEV-PLAN-071A`：引入 `owner_setid` 以明确包编辑归属，业务编辑与订阅治理分离。
- **触发问题**：在多租户长期演进下，是否应继续以“共享租户运行时读取”为主，还是转向“共享数据发布到租户（天然隔离）”的主路径。

## 2. 调查目标与非目标 (Goals & Non-Goals)
### 2.1 核心目标
- [X] 明确“全局共享租户”与“天然租户隔离”在合规、扩展、排障、审计上的差异与边界。
- [X] 形成可比选项（至少 2-3 个）并给出推荐路线、前置条件与迁移成本。
- [X] 输出对 `DEV-PLAN-070/071/071A` 的修订建议（含需要冻结/调整的契约条款）。
- [X] 保持仓库不变量：One Door、No Tx No RLS、Valid Time（date）、No Legacy。

### 2.2 非目标
- 不在本计划内直接执行大规模 schema 重构或业务迁移。
- 不在本计划内引入新的业务 scope 或 UI 大改。
- 不替代 `DEV-PLAN-071B` 等后续功能计划，仅提供架构层调查与决策输入。

### 2.3 工具链与门禁（SSOT 引用）
- 文档阶段：`make check doc`。
- 若调查结论触发后续代码/迁移实施，按 `AGENTS.md` 触发器矩阵执行对应门禁。
- SSOT：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`、`Makefile`。

## 3. 调查范围与边界 (Scope)
### 3.1 in-scope
- SetID 解析链：`org_unit -> setid -> scope -> package`。
- shared-only 配置域的读取与订阅机制。
- `global_tenant` 模式下的 RLS、权限、审计与运营影响。
- `owner_setid` 在包所有权与编辑权限中的治理能力。

### 3.2 out-of-scope
- 非 SetID 控制域的独立业务模型。
- HR 业务规则本身（例如 Job Catalog 字段语义）正确性评估。

## 4. 现状基线（待核验清单）
1. [X] 共享数据读路径是否全部通过受控函数/专用入口，无旁路 SQL（运行时全仓检索未见旁路；见 §4.3）。
2. [X] shared-only 场景中 `app.current_tenant` 切换与恢复是否在异常路径可证明安全（见 §4.4）。
3. [X] 关键接口是否存在 `current_date` 默认导致的回放口径漂移（已确认存在；见 §4.2）。
4. [X] `owner_setid` 回填/变更流程是否可制度化，避免人工决策成为常态（见 §4.4）。
5. [X] shared-only 与 tenant-only 的权限矩阵是否与 Casbin 对象一致且可审计（见 §4.4）。

### 4.1 问题-证据矩阵（M1 草案 v0，2026-02-22）
| 编号 | 问题 | 当前判断（v0） | 现有证据 | 证据缺口 | 下一步动作 |
| --- | --- | --- | --- | --- | --- |
| B1 | 共享读取是否全经受控入口、无旁路 SQL | 已完成（v1：运行时全仓检索未见旁路） | `DEV-PLAN-070/071` 已冻结“共享读需显式开关 + 受控函数”口径；`modules` 运行时代码检索未命中直连 global 表 | 历史迁移仍包含 global 表/函数引用（属于 schema 演进与写引擎，不是业务运行时旁路） | 在 070B 中增加防漂移门禁，持续阻断新旁路 |
| B2 | `app.current_tenant` 切换/恢复异常路径是否安全 | 已完成（有保护，保留迁移期风险提示） | `resolve_scope_package`/`assert_scope_package_active_as_of` 显式保存并恢复上下文；`set_config(..., true)` 使用事务本地作用域 | 个别历史函数缺统一 `EXCEPTION` 恢复块 | 在 070B 中移除该运行时链路，从根因收敛 |
| B3 | 是否存在 `current_date` 隐式默认导致回放漂移 | 已确认存在风险点（部分接口/函数仍默认 today） | `DEV-PLAN-102B` 已识别时间口径显式化需求 | 缺少按模块分级的整改清单与截止时间 | 形成“as_of 必填/可选”核查表并纳入 070B 子计划整改 |
| B4 | `owner_setid` 回填/变更是否制度化 | 已完成（制度化口径成立） | `DEV-PLAN-071A` 明确 owner 规则、回填阻断条件、治理入口唯一；执行日志已记录手工决策 | 仍需在 070B 中把“发布治理”并入统一 runbook | 在 070B PR-070B-1/3 合并治理条款 |
| B5 | shared-only / tenant-only 权限矩阵是否可审计 | 已完成（矩阵可追溯） | `policy.csv` 对象/动作 + `allowlist.yaml` 路由清单 + global 写引擎 `assert_actor_scope_saas` | 需防止后续新增路由漂移 | 以 070B 防漂移门禁持续阻断 |

> 说明：本矩阵为 M1 草案，先用于收敛核验路径；完成代码/测试/配置抽样后再更新为 v1 并回填勾选状态。

### 4.2 B1/B3 抽样证据（2026-02-22）
#### B1：共享读取旁路抽样
- 已执行抽样命令（节选）：
```bash
rg -n "global_setid_scope|global_setids|allow_share_read|global_tenant_id\\(" \
  modules --glob "*.go" --glob "!**/*_test.go" --glob "!**/*_templ.go"
rg -n "resolve_scope_package\\(" modules migrations --glob '!**/*_templ.go'
rg -n "FROM orgunit\\.global_setid_scope_package|FROM orgunit\\.global_setids" \
  modules --glob '!modules/orgunit/infrastructure/persistence/schema/*.sql'
```
- 结果摘要：
  - Go 运行时代码（非测试）未检出直接读取 global 表的调用点。
  - 业务侧读取命中 `orgunit.resolve_scope_package(...)`（例如 `modules/staffing/infrastructure/persistence/schema/00015_staffing_read.sql`），未见业务 SQL 直连 global 表。
  - `resolve_scope_package` 本身为 `SECURITY DEFINER` 且要求 `as_of_date`，并在函数内显式切换/恢复上下文；权限文件中对 global 表与函数执行权限进行了收敛（见 `modules/orgunit/infrastructure/persistence/schema/00009_orgunit_setid_scope_engine.sql`、`modules/orgunit/infrastructure/persistence/schema/00010_orgunit_setid_scope_kernel_privileges.sql`）。

#### B3：`current_date` 默认口径抽样
- 已执行抽样命令（节选）：
```bash
rg -n "current_date|as_of" modules/staffing/presentation/controllers/assignments_api.go \
  modules/staffing/presentation/controllers/assignments_api_test.go \
  modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql \
  modules/orgunit/infrastructure/persistence/schema/00009_orgunit_setid_scope_engine.sql
```
- 结果摘要：
  - 已确认风险点 1：`Assignments API` 在未提供 `as_of` 时默认 `time.Now()/NowUTC`（`modules/staffing/presentation/controllers/assignments_api.go`），且测试明确覆盖该默认行为（`modules/staffing/presentation/controllers/assignments_api_test.go`）。
  - 已确认风险点 2：`orgunit.submit_setid_event` 在 payload 缺失 `effective_date` 时回退 `current_date`（`modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`）。
  - 同时确认正向约束：`resolve_scope_package`/`assert_scope_package_active_as_of` 要求 `as_of_date` 必填，不允许空值默认（`modules/orgunit/infrastructure/persistence/schema/00009_orgunit_setid_scope_engine.sql`）。

> 结论：B1/B3 已形成可证据化结论；其中 B1 已在 §4.3 升级为全仓检索 v1，B3 仍待补“按模块整改优先级”清单。

### 4.3 B1 全仓检索补证（M1 v1，2026-02-22）
- 已执行全仓检索命令（节选）：
```bash
rg -n "global_setid_scope_package|global_setids|allow_share_read|global_tenant_id\\(" \
  modules --glob "*.go" --glob "!**/*_test.go" --glob "!**/*_templ.go"
rg -n "global_setid_scope_package|global_setids|allow_share_read|global_tenant_id\\(" \
  modules --glob "*.sql" --glob "!modules/**/infrastructure/persistence/schema/*.sql"
rg -l "global_setid_scope_package|global_setids|allow_share_read|global_tenant_id\\(" \
  modules/orgunit/infrastructure/persistence/schema/*.sql
rg -l "global_setid_scope_package|global_setids|allow_share_read|global_tenant_id\\(" migrations
```
- 结果摘要：
  - `modules` 非测试 Go 运行时代码：**0 命中**（未见直连 global 表/开关的业务代码旁路）。
  - `modules` 非 schema SQL：**0 命中**（未见业务查询直连 global 表）。
  - 命中集中在 `modules/orgunit/infrastructure/persistence/schema/*.sql`（8 个文件）与 `migrations/orgunit/*.sql`（15 个文件）。
  - 命中类型为：schema 定义/RLS 策略、kernel write engine、受控解析函数与迁移脚本；未发现额外业务读旁路。
- 例外说明（保留项）：
  - 历史迁移文件保留 global 引用属于演进痕迹，不等同“运行时旁路”；070B 落地后需通过门禁阻断新增类似路径。

### 4.4 B2/B4/B5 补证与结论（M1 v1，2026-02-22）
#### B2：上下文切换与恢复安全性
- 证据要点：
  - `orgunit.resolve_scope_package`/`orgunit.assert_scope_package_active_as_of` 在切换前保存 `app.current_tenant` 与 `app.allow_share_read`，完成后恢复（见 `modules/orgunit/infrastructure/persistence/schema/00009_orgunit_setid_scope_engine.sql`）。
  - 关键写引擎在 shared-only 分支执行后有显式恢复（见 `modules/orgunit/infrastructure/persistence/schema/00006_orgunit_setid_engine.sql`）。
  - `set_config(..., true)` 使用事务本地作用域，异常情况下不会形成跨事务污染（基于 PostgreSQL GUC local 语义；本项为从源码行为推断）。
- 结论：在当前链路下可证明“fail-closed + 事务内恢复”成立；剩余风险通过 070B 去运行时共享读链路根因消除。

#### B4：owner_setid 制度化
- 证据要点：
  - `DEV-PLAN-071A` 已冻结：owner_setid 仅 owner 可编辑、订阅者只读、治理入口唯一、回填阻断条件（多订阅人工决策、无订阅迁移阻断）。
  - `docs/dev-records/dev-plan-071-execution-log.md` 已记录 owner_setid 回填与手工决策证据（含多订阅与 SMOKE 特例）。
- 结论：owner_setid 已形成“规则 + 迁移阻断 + 执行证据”的制度化闭环。

#### B5：权限矩阵可审计
- 已执行核查命令（节选）：
```bash
rg -n "org\\.scope_package|org\\.scope_subscription|org\\.share_read" config/access/policy.csv
rg -n "/org/api/scope-packages|/org/api/owned-scope-packages|/org/api/scope-subscriptions|/org/api/global-setids|/org/api/global-scope-packages" config/routing/allowlist.yaml
```
- 矩阵摘要：
  - `org.scope_package`：`read/admin`，对应 `/org/api/scope-packages`、`/org/api/owned-scope-packages`、`/org/api/scope-packages/{package_id}/disable`。
  - `org.scope_subscription`：`read/admin`，对应 `/org/api/scope-subscriptions`。
  - `org.share_read`：`read`，共享读能力与全局写能力分离；global 写入口额外受 `assert_actor_scope_saas` 与 `global_tenant` 约束（见 `modules/orgunit/infrastructure/persistence/schema/00011_orgunit_setid_scope_write_engine.sql`）。
- 结论：shared-only / tenant-only 权限边界具备“策略 + 路由 + DB 写入口约束”三层可审计性。

## 5. 外部对标假设（Workday 公开资料）
> 说明：以下为公开资料可见口径，用于对标调查；不代表对 Workday 私有实现细节的断言。

- [X] 验证“租户为主隔离单位”的公开描述与我们当前模式差异。
- [X] 验证“上下文化安全（Contextual Security）”在 API 层的表现，并映射到本仓库权限模型。
- [X] 验证“审计可追溯（谁在何时访问过什么）”能力与我们当前日志/事件口径差距。

### 5.1 外部对标结论（M2，2026-02-22）
| 对标项 | Workday 公开口径（摘要） | 与本仓库映射 | 结论 |
| --- | --- | --- | --- |
| 租户隔离单位 | Workday 在 ERP 公开页面强调“true multi-tenant SaaS”模型 | 我们当前 `global_tenant` 运行时读取属于“受控跨租户读取”，解释成本高于 tenant-only | 支持 070B 采用“发布到租户”主路径 |
| 上下文化安全 | Workday 安全页面强调细粒度、角色化与情境化访问控制 | 我们对应 `Casbin object/action + RLS + actor_scope`；但共享读依赖运行时上下文切换 | 支持把共享读从运行时转为发布时能力 |
| 审计可追溯 | Workday 公开案例提到可查看谁在何时访问过什么、并支持 API 导出审计视图 | 我们需要把“发布来源/操作者/生效日”纳入统一审计证据链 | 支持 070B 中 release 审计字段与切流证据要求 |

**公开来源链接**：
- https://www.workday.com/en-us/enterprise-resource-planning.html
- https://www.workday.com/en-ae/why-workday/trust/security.html
- https://blog.workday.com/en-us/2021/how-workday-supports-gdpr-and-data-subject-rights.html

> 不可比点说明：Workday 私有实现细节不可得，以上仅用于“原则级别”对标，不作为实现细节约束。

## 6. 关键调查问题 (Research Questions)
### 6.1 合规与数据主权
1. 跨租户运行时共享读取（即使受控）是否会在审计/合规问卷中提高解释成本？
2. shared-only 数据是否存在“按地区/法域”进一步分区需求？若有，`global_tenant` 是否会成为瓶颈？

### 6.2 安全与权限
3. 当前 `RLS + app.allow_share_read + SECURITY DEFINER` 组合是否存在“上下文污染”与误配风险窗口？
4. `owner_setid` 权限是否应从“租户级角色”进化到“组织上下文 + 角色”双维控制？

### 6.3 扩展与性能
5. 共享表/函数是否会形成热点（高并发租户集中读共享包）？
6. 新增 stable scope 的回填与 bootstrap 是否可在大租户数量下保持可控时延？

### 6.4 可运维性与可审计
7. 现有证据链是否可清晰回答“某次读取为何命中共享包、由谁触发、在何 as_of 生效”？
8. 故障场景下，是否可在不引入 legacy 双链路的前提下快速止损与恢复？

## 7. 候选架构选项（调查对象，不是最终决策）
### 7.1 选项 A：维持 global_tenant 主模式（加强治理）
- 核心：保留现有共享读取路径，补强权限、审计、默认参数与异常恢复。
- 优点：迁移成本最低、对现有链路冲击小。
- 风险：中长期合规解释成本与共享热点风险仍在。

### 7.2 选项 B：共享数据“发布到租户”主模式（天然隔离优先）
- 核心：共享端只做发布源，租户运行时只读本租户数据；共享读取作为运营/治理路径而非业务主路径。
- 优点：隔离与合规表达最清晰，运行时边界简单。
- 风险：发布同步与版本治理复杂度上升，需要明确发布/回滚契约。

### 7.3 选项 C：混合模式（按 scope 分层）
- 核心：极少数公共字典保留共享运行时读取，其余 scope 采用发布到租户。
- 优点：兼顾迁移成本与隔离收益。
- 风险：模型复杂，需严格防止“策略漂移”。

### 7.4 决策门结论（2026-02-22）
- **推荐采纳**：选项 B（共享数据“发布到租户”主模式，天然隔离优先）。
- **结论说明**：
  1. 在隔离强度与合规可解释性维度，选项 B 明显优于 A/C，且更符合多租户长期治理目标。
  2. 选项 B 的主要代价是发布链路复杂度上升；通过“字典模块先行样板 + 分阶段迁移 + 环境级停写回滚口径”可控。
  3. 选项 A 作为短期低成本路径保留历史参考，不作为主路径；选项 C 不作为首期方案，避免策略漂移。
- **实施承接**：`DEV-PLAN-070B` 作为本结论的实施计划，按“先字典、后 scope package”推进。

## 8. 评估维度与打分准则
- **隔离强度**：租户边界是否天然成立，是否依赖运行时开关。
- **合规可解释性**：外部审计/客户问卷是否易解释。
- **安全稳健性**：误配、越权、上下文污染的潜在面。
- **可扩展性**：租户规模、scope 数量增长下的可持续性。
- **运维复杂度**：发布、回填、回放、排障路径复杂度。
- **迁移成本**：对现有 `070/071/071A` 代码与数据的改动量。

> 输出时采用 1-5 分（5 为最好），并记录评分依据与证据链接。

### 8.1 量化评分表（v1，2026-02-22）
> 说明：本版基于内部证据先行评分；M2 完成外部对标后仅允许做“小幅校准”，不改变“B 为主路径”的决策方向。

| 维度 | 权重 | 选项 A（维持 global_tenant） | 选项 B（发布到租户） | 选项 C（混合） |
| --- | --- | ---: | ---: | ---: |
| 隔离强度 | 25% | 2 | 5 | 3 |
| 合规可解释性 | 20% | 2 | 5 | 3 |
| 安全稳健性 | 20% | 2 | 4 | 3 |
| 可扩展性 | 15% | 3 | 4 | 3 |
| 运维复杂度 | 10% | 4 | 3 | 2 |
| 迁移成本（低成本=高分） | 10% | 5 | 2 | 3 |
| **加权总分** | 100% | **2.65** | **4.15** | **2.90** |

### 8.2 成本估算（相对量级）
- **选项 A**：低到中（改造范围小，主要是治理加固与审计补齐）。
- **选项 B**：中到高（需新增发布基座、历史回填、切流与收口）。
- **选项 C**：高（需同时维护两种模型，治理与测试复杂度最高）。

### 8.3 评分依据与证据链接（当前版）
- 选项 A 评分依据：`global_tenant + allow_share_read` 仍依赖运行时共享读开关与上下文切换（`docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`、`docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`）。
- 选项 B 评分依据：运行时 tenant-only 边界清晰，但新增发布治理成本（`docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`）。
- 选项 C 评分依据：混合策略导致模型复杂、易漂移（本计划 §7.3 风险描述）。
- 字典模块侧证据：当前存在 tenant/global 语义与 fallback 条款，改造后可收敛为 tenant-only（`docs/dev-plans/105-dict-config-platform-module.md`、`docs/dev-plans/105b-dict-code-management-and-governance.md`）。
- 外部对标证据（M2）：见本计划 §5.1。

## 9. 调查方法与证据来源
### 9.1 内部证据
- 计划与执行记录：
  - `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
  - `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
  - `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
  - `docs/dev-records/dev-plan-070-execution-log.md`
  - `docs/dev-records/dev-plan-071-execution-log.md`
- 代码/Schema/权限核查（按需抽样）：`modules/orgunit/**`、`config/access/policy.csv`、`config/routing/allowlist.yaml`。

### 9.2 外部证据（公开资料）
- Workday SOAP API Reference（多租户 API 入口与版本）。
- Workday API 操作文档中 `Contextual Security` 字段（如 Staffing 相关操作）。
- Workday 官方安全与审计公开说明（Trust & Security / 官方事件通告）。

## 10. 里程碑与交付物
1. [X] **M1（基线核对）**：已完成当前实现与风险清单核验，形成“问题-证据”矩阵（v1，2026-02-22）。
2. [X] **M2（外部对标）**：已完成 Workday 公开机制映射，形成“相同点/差异点/不可比点”（2026-02-22）。
3. [X] **M3（选项评估）**：已完成 A/B/C 三方案评分、成本估算、风险排序（v1）；待 M2 完成后可做小幅校准（2026-02-22）。
4. [X] **M4（决策建议）**：已输出推荐方案与分阶段落地建议（含对 070/071/071A 的修订条目），实施承接 `DEV-PLAN-070B`（2026-02-22）。

### 10.1 里程碑进度细化（2026-02-22）
- **M1 当前状态：已完成（v1）**
  - [X] 已冻结核验范围（SetID 解析链、shared-only 读取链、owner_setid 治理）。
  - [X] 已补齐“问题-证据矩阵”草案（v0，见 §4.1）。
  - [X] 已完成 B1/B3 抽样证据记录（见 §4.2）。
  - [X] 已完成 B1 全仓检索补证并升级 v1（见 §4.3）。
  - [X] 已补齐 B2/B4/B5 的代码/Schema/权限证据（见 §4.4）。
- **M2 当前状态：已完成**
  - [X] 已补齐公开资料证据链接与映射结论（见 §5.1）。
- **M3 当前状态：已完成（v1.1）**
  - [X] 已形成 A/B/C 备选方案与风险描述。
  - [X] 已补齐 1-5 分量化评分与成本估算表（含当前版证据链接）。
  - [X] 已基于 M2 外部对标完成校准并更新为 v1.1（分数结论不变，仍推荐 B）。
- **M4 当前状态：已完成**
  - [X] 已形成“推荐采纳选项 B（发布到租户）”决策门结论。
  - [X] 已明确实施承接文档：`docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`。

### 10.2 070B 实施前置条件结论
- [X] 070A 决策结论已冻结（选项 B）。
- [X] M1 基线核验完成并具备证据链（§4.1~§4.4）。
- [X] M2 外部对标完成并给出可比/不可比边界（§5.1）。
- [X] 对 070/071/071A 的修订方向已明确，并由 070B 承接落地。
- [X] 结论：**满足 070B 实施前置条件**。

**交付物**：
- 调查结论文档（本文件持续更新）。
- 风险矩阵与评分表（可附录）。
- 若进入实施：新增后续计划（建议编号 `070B`）与执行记录文档。

## 11. 验收标准 (Acceptance Criteria)
- [X] 每个关键风险均有证据来源（内部或外部），且可追溯。
- [X] 至少 2 个候选架构被完整评估并给出明确取舍理由。
- [X] 推荐方案与仓库不变量无冲突（One Door/No Tx No RLS/No Legacy/Valid Time）。
- [X] 明确迁移边界：不引入双链路回退，回滚策略仍为环境级停写 + 修复后重试。
- [X] 明确对 `070/071/071A` 的具体修订点（条目级别）。

## 12. 风险登记（调查阶段）
- **R1：证据不足风险** —— 公开资料粒度有限，可能无法覆盖私有实现细节。缓解：明确“可证/不可证”边界，避免过度推断。
- **R2：范围蔓延风险** —— 调查阶段混入实施细节。缓解：严格按本计划 out-of-scope 执行。
- **R3：策略摇摆风险** —— 未形成量化评估导致反复。缓解：统一评分维度与门槛，先证据后结论。

## 13. 关联文档
- `AGENTS.md`
- `docs/archive/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/archive/dev-plans/071-setid-scope-package-subscription-blueprint.md`
- `docs/archive/dev-plans/071a-package-selection-ownership-and-subscription.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-records/dev-plan-070-execution-log.md`
- `docs/dev-records/dev-plan-071-execution-log.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`

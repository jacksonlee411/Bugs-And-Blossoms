# DEV-PLAN-040：薪酬社保（Payroll & Social Insurance）可执行方案

**状态**: 草拟中（2026-01-07 15:00 UTC）

> 来源：`docs/dev-records/薪酬社保模块设计蓝图.docx`（转换为 Markdown）
>
> 本文将原“研究蓝图”收敛为本仓库可执行 dev-plan：明确范围、边界、不变量、失败路径与验收口径（对齐 `AGENTS.md` 与 `docs/dev-plans/003-simple-not-easy-review-guide.md`）。

## 0. 可执行方案（本仓库合同）

> 本节为实施合同；后文（1-9）为设计展开与参考材料。若存在冲突，以本节为准。

### 0.1 背景与上下文
- 本仓库为 Greenfield implementation repo；当前 HR 主线模块为 `orgunit/jobcatalog/staffing/person`（SSOT：`docs/dev-plans/016-greenfield-hr-modules-skeleton.md`）。
- 薪酬社保属于“任职/用工（Staffing）”强关联子域：算薪输入来自任职记录（人员、岗位、FTE、有效期、定薪），输出为工资条/扣缴结果。
- 工程落点：作为 `modules/staffing` 子域实现（不新增第 5 个业务模块）；若未来拆分为独立模块，需先更新 `docs/dev-plans/016-greenfield-hr-modules-skeleton.md` 与架构门禁配置。
- 本计划先交付一个“可见、可操作”的最小闭环（对齐 `AGENTS.md` 的用户可见性原则），并为后续扩展（多城市社保、金税接口、银企直连等）预留清晰边界。

### 0.2 目标与非目标（P0 Slice）
**目标**
- [ ] 提供“薪资周期（pay period）→ 计算 → 审核 → 定稿”的单链路主流程：生成工资条（payslip）并可在 UI 查看。
- [ ] 引入“社保政策（单城市起步）”配置：按 `city_code` + 有效期（Valid Time）选择规则，计算个人/企业缴费。
- [ ] 金额精度全链路无浮点：DB `numeric` + Go `cockroachdb/apd/v3`（实现期禁止 `float64`；舍入通过统一 Context 与枚举口径）。
- [ ] 严格多租户隔离与 fail-closed：所有 payroll 表启用 RLS，访问必须显式事务 + 租户注入（SSOT：`AGENTS.md`、`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。
- [ ] 写入口唯一（One Door）：所有写入通过 DB Kernel `submit_*_event(...)`，同事务同步投射读模型（SSOT：`AGENTS.md`、`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`、`docs/dev-plans/031-greenfield-assignment-job-data.md`）。

**非目标（本计划不交付）**
- 不做生产中的“双轨并行/回退到旧系统/读写双链路”（No Legacy，SSOT：`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`）。
- 不做金税四期直连申报、银企直连、ESOP/LTI、300+ 城市全量规则；这些在后续里程碑另立 dev-plan 承接。

### 0.3 工具链与门禁（SSOT 引用）
- 触发器矩阵与本地必跑：`AGENTS.md`
- 命令入口与 CI：`Makefile`、`.github/workflows/quality-gates.yml`
- DB 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc 规范：`docs/dev-plans/025-sqlc-guidelines.md`
- 路由治理：`docs/dev-plans/017-routing-strategy.md`（并更新 `config/routing/allowlist.yaml`）
- Authz（若引入）：`docs/dev-plans/022-authz-casbin-toolchain.md`
- i18n（仅 `en/zh`）：`docs/dev-plans/020-i18n-en-zh-only.md`（涉及翻译资源时走对应门禁入口）
- 时间语义（Valid Time）：`docs/dev-plans/032-effective-date-day-granularity.md`

### 0.4 关键不变量与失败路径（停止线）
- **No Legacy**：不得引入 `read=legacy/use_legacy` 或任何运行时回退；回滚仅允许“环境级停写/只读/修复后重试”（`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`）。
- **One Door**：任何写入（政策、算薪、定稿）必须走 Kernel `submit_*_event`；禁止直接 `INSERT/UPDATE` 读模型表。
- **No Tx, No RLS**：缺少 tenant context 直接失败（fail-closed）；不得用 superuser/bypass RLS 跑业务链路（对齐 `docs/dev-plans/019-tenant-and-authn.md` 与 `docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`）。
- **Valid Time = date**：有效期仅用 `date`/`daterange`（`[start,end)`）；`timestamptz` 仅用于审计/事务时间（`docs/dev-plans/032-effective-date-day-granularity.md`）。
- **幂等**：所有 `submit_*_event` 必须支持 `event_id`/`request_id` 幂等复用；复用冲突必须抛出稳定错误码（模式参考 `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`）。
- **JSONB 边界**：`rules_config/profile` 仅承载扩展字段；任何参与算薪/审计/对账的关键字段必须是稳定列；禁止“列字段 + JSONB 字段”重复存储同一语义（避免双权威表达）。
- **索引口径**：JSONB 的 **GIN** 仅用于 containment/exists（例如 `@>`、`?`）；数值比较（例如费率阈值分析）必须使用 `numeric` 列或显式表达式 btree 索引，禁止误用 GIN。

#### 0.4.1 JSONB 使用矩阵（防漂移）

> 目的：把“哪些必须关系型、哪些可用 JSONB”冻结为合同口径，避免实现期随手把关键字段塞进 JSONB 导致审计/对账/索引漂移。

| 数据/字段族 | 必须关系型（列/子表） | 可用 JSONB（仅 object） | 示例 | Stopline（触发升级为关系型） |
| --- | --- | --- | --- | --- |
| 核心标识/关联/唯一性 | 是 | 否 | `tenant_id`、`person_id`、`assignment_id`、`pay_period_id`、`policy_id` | 永远禁止用 JSONB 作为主键/外键/唯一性/关联字段 |
| RLS/租户隔离关键列 | 是 | 否 | `tenant_id`、`org_id`/scope | 任何参与 RLS predicate 的字段必须列化 |
| Valid Time（有效期）与互斥约束 | 是 | 否 | `validity daterange`、`EXCLUDE USING gist` | 禁止用 JSONB 维护时间轴或 no-overlap 逻辑 |
| 流程状态机与可筛选字段 | 是 | 否（可存“备注”） | `run_state`、`approved_at`、`posted_at` | 一旦需要 `WHERE/ORDER BY`/列表筛选/分页稳定排序，必须列化 |
| 幂等/事件追踪（可定位） | 是 | 可存“附加元数据” | `event_id`、`request_id`、`last_event_id` | 幂等判定与重放定位用到的键必须列化 |
| 金额/费率/基数/数量（参与计算或对账） | 是 | 否（信息性可用 string） | `base_salary`、`employer_rate`、`base_floor` | 一旦需要 `>` `<` 范围查询、聚合、对账、约束校验，必须列化为 `numeric`/整型 |
| 舍入合同/精度/币种 | 是 | 否（可存“备注”） | `rounding_rule`、`precision`、`currency` | 舍入枚举/舍入点属于合同；禁止埋在 JSONB |
| 工资条/扣缴情景“可对账汇总” | 是 | 可存“解释/展示快照” | `gross_pay`、`net_pay`、`employer_total` | 汇总口径必须可由明细/列字段重算；JSONB 不能成为权威来源 |
| 明细集合（工资项目/险种分项/个税分段等） | 是（子表） | 可存“每行 meta” | `payslip_items`、`social_insurance_items` | 一对多集合与可聚合明细必须子表；禁止用 JSONB array 作为权威明细 |
| 策略/政策配置的低频扩展（不参与计算/对账/约束） | 否 | 是 | `rules_config.flags`、`rules_config.notes` | 一旦扩展字段进入热路径、需要强约束或频繁查询，升级为列字段 |
| 外部系统原始报文/回执/审计附件 | 关键信息需抽列 | 是 | `tax_request_payload`、`tax_receipt_payload` | 若某字段进入查询/关联/幂等/状态机，必须从 payload 抽列 |
| 排障解释/trace（不可依赖查询） | 最小字段列化 | 是 | `calc_trace`、`rule_trace` | 必须限制大小/生命周期；不得把业务口径建立在 trace 上 |
| 导入/草稿/暂存（staging） | 关键信息列化 | 是 | `import_row_raw` | 进入“审核/定稿”等正式链路前必须规范化入关系表 |

**通用约束（适用于所有 JSONB 列）**
- JSONB 列必须是 object：DB 侧加 `CHECK (jsonb_typeof(col) = 'object')`；集合/明细一律用子表表达。
- JSONB 内出现的“数值”一律用 string（例如 `"0.070000"`），并由 Go `apd.Decimal` 解析；禁止 JSON number（避免隐式浮点）。
- 新增/变更 JSONB key 视为“契约变化”：必须先更新本矩阵/对应字段说明，再进入实现与评审。

#### 0.4.2 时间约束矩阵（含期权归属）

> 目的：冻结“时间约束类型 → 数据类型”的选型口径，避免实现期混用 `date/timestamptz` 或把 Valid Time 写成时间戳导致漂移（SSOT：`docs/dev-plans/032-effective-date-day-granularity.md`）。
>
> 期权归属（vesting）行业常见做法：业务通常指定“授予日 + 归属起算/首次归属日 + cliff/频率/期数”等规则参数（或直接列出每次归属日），**归属计划的结束日由规则推导**；但会明确指定“期权到期日（Expiration Date）”以及“离职后行权窗口（Termination Period）”，且行权窗口不得晚于到期日。

| 场景 | 业务常规输入 | 时间约束类型 | 推荐数据类型（PG） | 关键约束/Stopline |
| --- | --- | --- | --- | --- |
| 有效期版本化（任职/定薪/政策） | 通常只填生效日；历史通过新增版本表达 | Valid Time 区间 + no-overlap | `daterange` | 统一 `[start,end)`；同 key 必须 `EXCLUDE ... validity WITH &&`；禁止用 `timestamptz` 表达业务有效期 |
| 固定周期（pay period） | 明确起止（如自然月/自定义周期） | bounded range（区间） | `daterange` | 同 pay_group/tenant 下不得重叠；周期边界统一 `[start,end)` |
| 运行截止（算薪/审批/锁定） | 指定“某时刻前完成” | cutoff（时刻截止） | `timestamptz` | 属于操作/门禁时间，不是 Valid Time；必须显式时区语义（存 UTC，展示按租户时区） |
| 运行窗口（批次允许执行的时间段） | 指定起止时刻（可选） | operational window（时间窗） | `tstzrange`（或 `timestamptz` start/end） | 仅用于运行治理/SLA；不得替代业务有效期 |
| 授予（期权/权益计划） | 授予日（Date of Grant） | 单点业务日 | `date` | 授予日是业务日；审计/审批时间另用 `timestamptz` |
| 归属计划（规则生成：cliff + 月/季归属） | 起算/首次归属日 + cliff 月数 + 频率 + 总期数/总月数 | 规则生成离散序列 | `date`（锚点）+ `int/smallint`（月数/期数） | **禁止业务手填独立 end_date**；结束日=最后一次归属日（推导）；归属序列必须单调递增 |
| 归属计划（显式里程碑：每期数量不同） | 每次归属日 + 数量 | 显式离散集合（子表） | `date`（明细表 `vest_events`） | 明细日期唯一且单调；总归属量不得超过授予量；不得用 JSON 数组作为权威里程碑集合 |
| 加速归属（触发器：并购/解雇等） | 触发日/触发条件 | 事件触发单点或补丁式序列 | `date`（触发日）+（必要时）`date` 明细 | “加速”应表现为新增归属事件或调整未归属余额的规则；必须可审计（事件 SoT） |
| 到期（期权 Expiration） | 到期日（Expiration Date） | hard stop（硬截止） | `date` | 行权不得晚于到期日；到期日应 ≥ 授予日且 ≥ 最后归属日（若合同要求） |
| 离职导致停止归属 | 离职生效日 | stop（停止点） | `date` | 离职日后不再产生新的归属；“离职后行权期”单独计算 |
| 离职后行权期（PTEP） | N 天/个月（Termination Period） | 派生 bounded window（区间） | `daterange`（由 `service_end_date` + `interval`/月数推导） | `end = min(service_end + period, expiration_date)`；窗口语义仍用 `[start,end)`；若出现小时级规则则升级为 `tstzrange` |
| 暂停/顺延（休假/停薪留职影响归属） | 暂停起止（可能多段） | 多段区间（segments） | `daterange`（可多行子表） | “暂停是否顺延”属于合同口径；一旦启用必须测试覆盖，避免隐式顺延漂移 |
| 审计/交易时间（created/updated/tx） | 系统自动 | Audit/Tx Time | `timestamptz` | 仅用于审计/并发与可追溯；不得用于业务有效期计算 |

### 0.5 用户入口（可发现/可操作）
- UI 导航新增“薪酬”入口（归属 `staffing`）：至少包含
  - “薪酬周期/算薪批次”列表与创建表单
  - “社保政策（单城市）”配置页
  - “工资条”列表与详情页（按批次与人员过滤）
- 路由命名空间建议落在 `/org/payroll-*`（与现有 `/org/positions`、`/org/assignments` 一致），并将新路由登记到 `config/routing/allowlist.yaml` 后通过 `make check routing`。

### 0.6 实施步骤（Checklist）
1. [ ] 设计并冻结最小数据模型（见 3.x/5.x 章节修订版）：包含 pay period、payslip、社保政策、YTD 累加器。
2. [ ] 落地 Kernel 写入口：`submit_payroll_*_event`（含 tenant assert、advisory lock、幂等约束）与同步投射。
3. [ ] 落地 UI：创建批次 → 触发计算 → 查看结果 → 定稿（定稿后结果只读）。
4. [ ] 质量门禁对齐：按触发器矩阵运行并在本计划记录证据（时间戳、命令、结果）。

### 0.7 验收标准（Done 口径）
- [ ] 任意租户在 UI 可完成：配置社保政策 → 创建一个 pay period 批次 → 计算 → 查看工资条 → 定稿。
- [ ] 关键约束可审计：RLS fail-closed、唯一写入口、Valid Time 日粒度、金额无浮点、幂等可复现。
- [ ] 新增路由均已登记 allowlist 且通过 routing gate；无 legacy 入口可被 `make check no-legacy` 阻断。

### 0.8 Simple > Easy Review（DEV-PLAN-003）

- **结构（解耦/边界）**：通过 —— Payroll 作为 `modules/staffing` 子域；写入口唯一（Kernel）；Valid Time/Tx Time 分离。
- **演化（规格/确定性）**：需警惕 —— 6/7 章明确为“非目标”，不得在 P0 实现期被顺手带入；回溯重算明确为 M2+ 且需另立子计划收敛。
- **认知（本质/偶然复杂度）**：通过 —— 不变量显式化：No Legacy / One Door / No Tx, No RLS / Valid Time（日粒度）。
- **维护（可理解/可解释）**：待补齐 —— 进入实现前补齐：状态机（run 状态与事件）、稳定错误码枚举、最小路由清单与可复现验收脚本。

### 0.9 本次验证记录（门禁证据）

- [X] `make check doc` —— 通过（2026-01-07 15:29 UTC）
- [X] `make check doc` —— 通过（2026-01-08 00:59 UTC）
- [X] `make check doc` —— 通过（2026-01-08 01:01 UTC）

## 1. 执行摘要：企业级薪酬系统的代际跨越

在中国企业数字化转型的深水区，人力资源管理系统（HCM）正面临着前所未有的挑战与机遇。长期以来，以SAP HCM和Oracle PeopleSoft为代表的国际巨头，凭借其深厚的功能积淀和严谨的逻辑架构，定义了企业级薪酬核算（Payroll）的标准。然而，随着中国“金税四期”工程的全面铺开、个税改革的深化以及企业组织形态的敏捷化演进，基于传统单体架构和专有技术栈（如ABAP、PeopleCode）的遗留系统，在面对高并发实时计算、快速政策迭代以及数据资产化需求时，日益显现出架构上的僵化与滞后<sup>1</sup>。

本报告作为“本项目路线图”的核心纲领，旨在制定一份详尽的中国本地化HR SaaS薪酬社保模块设计蓝图。该蓝图并非对传统系统的简单修补，而是一场基于现代云原生技术栈——**Golang (Go)** 与 **PostgreSQL (PG)** ——的彻底重构。我们主张利用Go语言卓越的并发处理能力（GMP模型）来重塑薪资计算引擎的吞吐性能，利用PostgreSQL先进的时态数据特性（Range Types）和半结构化存储（JSONB）来解决复杂的生效日期管理与政策配置难题<sup>2</sup>。

本方案的核心目标是构建一个既具备SAP级别的逻辑深度与数据严谨性，又拥有互联网级敏捷性与开放性的下一代薪酬平台。通过引入“原子化”的规则引擎、基于Worker Pool的并行计算架构、以及原生支持“金税四期”数智化监管的合规接口，本设计将在保障金融级数据精度的前提下，将十万级员工的算薪耗时从数小时压缩至分钟级，实现从“事后核算”到“实时算薪”的范式转移<sup>2</sup>。

## 2. 传统架构解构与技术范式转移

要构建超越SAP/PeopleSoft的下一代系统，首先必须深刻理解其设计哲学的优劣，并在现代技术语境下找到更优的解法。

### 2.1 从“集群表”到“混合数据模型”

SAP HCM最著名的设计之一是其**集群表（Cluster Tables）**，如PCL2。这种设计将薪资计算结果打包为二进制大对象（BLOB）存储，虽然节省了早期的存储空间，但导致了数据的极度不透明。外部系统若需访问薪资明细，必须调用专用的解包函数（Macro），这使得实时数据分析和跨系统集成变得异常困难<sup>1</sup>。

“本项目”将采用**PostgreSQL的混合关系型-文档型架构**来彻底打破这一黑盒。

- **核心实体关系化**：员工、组织、职位、薪资结构等具有强一致性要求的实体，依然采用严格的第三范式（3NF）关系型表存储，确保引用完整性。

- **计算结果“列化为主，JSONB 为辅”**：工资条/扣缴情景的关键字段（金额/费率/汇总/对账口径）必须列化；允许用 PostgreSQL **JSONB** 存放“解释/展示/扩展”的快照，并且只允许做 containment/exists 查询（不做数值比较/范围分析）<sup>2</sup>。

- **战略意义**：分析类查询（例如“上海地区养老金缴费基数 > 20,000 元”）应基于 `numeric` 列/明细子表完成，而不是把关键数值埋在 JSONB 里，从而避免审计与索引口径漂移<sup>1</sup>。

### 2.2 从“线性批处理”到“高并发流式计算”

传统的薪酬引擎通常采用线性处理模式，或者基于重量级的操作系统线程（如Java Thread）进行有限的并行。在处理“大发薪日”（Big Payday）时，面对数十万员工的复杂规则计算，系统往往面临CPU上下文切换开销大、内存占用高的问题，导致算薪窗口期过长，甚至需要“封网”操作<sup>3</sup>。

Golang的引入是解决这一瓶颈的关键。Go语言的**Goroutine**是一种用户态的轻量级线程，启动成本极低（约2KB栈空间）。“本项目”将设计一个基于**Worker Pool模式**的计算引擎：

- **调度器（Dispatcher）**：负责将待计算的员工ID分批推送到缓冲通道（Buffered Channel）。

- **工作池（Worker Pool）**：启动与CPU核心数成比例（如N\*2）的Worker协程，从通道中抢占式获取任务。

- **背压控制（Backpressure）**：通过限制Worker的数量，系统可以天然地防止数据库连接池被耗尽，确保在高负载下依然保持稳定的吞吐量，而不是因资源争抢而崩溃<sup>4</sup>。

### 2.3 从“软性生效日期”到“数据库级时态约束”

“生效日期”（Effective Dating）是HR系统的灵魂。PeopleSoft通过在应用层维护Effective Date字段来追踪历史。然而，这种依赖应用代码来保证时间连续性的方式是脆弱的。开发人员的疏忽往往导致“时间重叠”（Overlapping）或“时间断裂”（Gaps）的数据脏读，例如一个员工在同一天属于两个不同的部门<sup>6</sup>。

PostgreSQL 提供的 **范围类型（Range Types）** 与 **排他约束（Exclusion Constraints）** 为这一难题提供了数学级的解决方案。我们将使用 `daterange`（date range）存储业务有效期（Valid Time，日粒度），例如 `daterange('2024-01-01', '2024-02-01', '[)')`，并通过 `EXCLUDE USING gist (... validity WITH &&)` 落地 no-overlap（对齐 `docs/dev-plans/032-effective-date-day-granularity.md`）。

## 3. 数据持久层架构：PostgreSQL的深度应用

数据库设计是SaaS产品的基石。本章节详细阐述如何利用PG的高级特性来构建一个高性能、高可用的薪酬社保数据底座。

### 3.1 时态数据建模（Temporal Data Modeling）

在“本项目”中，我们将废弃传统的start_date和end_date双字段设计，转而全面采用daterange。

#### 3.1.1 核心表结构设计

以任职记录（Assignment/Job Data）的“定薪信息”为例：本仓库建议将薪酬相关字段归属 `staffing.assignment_*`（对齐 `docs/dev-plans/031-greenfield-assignment-job-data.md`），并延续现有的 `events -> versions` 模式：

- 事件表：`staffing.assignment_events`（append-only，SoT）
- 版本表：`staffing.assignment_versions`（可重建投射），其中 `validity daterange` 表达 Valid Time（统一 `[start,end)`；见 `docs/dev-plans/032-effective-date-day-granularity.md`）。

在不引入第二套权威表达的前提下，薪酬字段建议采用“稳定列 + JSONB 扩展”：
- 稳定列：`base_salary numeric(15,2)`、`currency char(3)`（金额不允许 float）
- 扩展：`profile jsonb`（object，用于少量非计算扩展键，例如 UI 备注/flags；若为工资项目/金额明细且参与计算/对账，必须下沉为关系型子表，禁止埋在 JSONB）

```sql
-- 示例（拟变更）：为 assignment_versions 增加薪酬相关字段
-- 注意：写入仍必须通过 Kernel submit_*_event，同事务投射到 versions（One Door）
ALTER TABLE staffing.assignment_versions
  ADD COLUMN base_salary numeric(15,2),
  ADD COLUMN currency char(3) NOT NULL DEFAULT 'CNY',
  ADD COLUMN profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  ADD CONSTRAINT assignment_versions_profile_is_object_check CHECK (jsonb_typeof(profile) = 'object');
```

**设计要点**：

- **no-overlap**：`assignment_versions` 已通过 `EXCLUDE USING gist (... validity WITH &&)` 保证同一任职在时间轴上互斥（数学级约束，避免应用层 if/else 校验漂移）。
- **闭开区间标准**：统一采用 `[start,end)`（与现有 `staffing.*_versions_validity_bounds_check` 一致），避免边界重叠。

### 3.2 策略配置的半结构化存储

中国社保政策具有极高的碎片化特征，全国300多个地级市拥有不同的缴费基数上下限、比例和舍入规则。如果使用传统的关系型表设计（如columns: shanghai_pension_rate, beijing_medical_base...），表结构将变得臃肿不堪且难以维护。

我们采用“**稳定列 + JSONB 扩展**”的混合模型来承载策略配置：

- **稳定列（可约束/可索引/热路径）**：费率、基数上下限、舍入规则等关键字段必须列化，保证强类型约束与审计/对账可解释性。
- **JSONB（扩展/低频字段）**：`rules_config` 仅承载少量扩展键（必须是 object）；不得重复存放已列化的关键字段（避免双权威表达）。

**选型理由（简）**
- 合规/审计与排障需要“字段语义稳定 + 可加 DB 约束”，不能把关键字段完全埋在 JSON 结构里。
- 政策碎片化与演进真实存在，JSONB 允许在不做 DDL 的前提下承载少量扩展键，但必须在边界内使用。

前提仍是“写入口唯一 + 可审计”：政策写入走 Kernel `submit_social_insurance_policy_event(...)`，并投射到 `staffing.social_insurance_policy_versions`（有效期 `validity daterange`）。

```sql
-- 示例（核心字段）：社保政策 versions（读模型）
CREATE TABLE staffing.social_insurance_policy_versions (
  id bigserial PRIMARY KEY,
  tenant_id uuid NOT NULL,
  policy_id uuid NOT NULL,
  city_code text NOT NULL,          -- e.g., CN-310000（上海）
  hukou_type text NOT NULL,         -- e.g., local/non_local（P0 可先固定为 'default'）
  insurance_type text NOT NULL,     -- e.g., PENSION, MEDICAL

  -- 关键字段：必须列化（可约束/可索引）
  employer_rate numeric(9,6) NOT NULL,  -- 0..1
  employee_rate numeric(9,6) NOT NULL,  -- 0..1
  base_floor numeric(15,2) NOT NULL,    -- >= 0
  base_ceiling numeric(15,2) NOT NULL,  -- >= base_floor
  rounding_rule text NOT NULL,          -- 枚举（见 4.2 舍入合同）
  precision smallint NOT NULL DEFAULT 2,

  -- 扩展字段：仅承载低频/可选项；数值如需出现必须用 string 表达并由 Go apd 解析
  rules_config jsonb NOT NULL DEFAULT '{}'::jsonb,
  validity daterange NOT NULL,
  last_event_id bigint NOT NULL REFERENCES staffing.social_insurance_policy_events(id),
  CONSTRAINT social_insurance_policy_versions_rules_is_object_check CHECK (jsonb_typeof(rules_config) = 'object'),
  CONSTRAINT social_insurance_policy_versions_rate_check CHECK (
    employer_rate >= 0 AND employer_rate <= 1 AND employee_rate >= 0 AND employee_rate <= 1
  ),
  CONSTRAINT social_insurance_policy_versions_base_check CHECK (
    base_floor >= 0 AND base_ceiling >= base_floor
  ),
  CONSTRAINT social_insurance_policy_versions_validity_check CHECK (NOT isempty(validity)),
  CONSTRAINT social_insurance_policy_versions_validity_bounds_check CHECK (lower_inc(validity) AND NOT upper_inc(validity))
  -- no-overlap：由 (tenant_id, policy_id, validity) 的 EXCLUDE USING gist 实现（模式同 staffing.*_versions）
);
```

**rules_config 示例（扩展字段；数值建议用 string 表达）**：

```json
{
  "notes": "pilot",
  "flags": {
    "allow_zero_base": false
  }
}
```

这种设计的优势在于：

1.  **Schema Evolution**：当社保局发布新政策（例如调整费率/基数上下限/舍入规则）时，只需新增/更新政策版本记录（列字段 + `rules_config`），无需做 DDL（锁表），这对 7x24 小时运行的 SaaS 系统至关重要<sup>12</sup>。

2.  **查询与索引口径清晰**：
    - 运行时按 `(tenant_id, city_code, hukou_type, insurance_type, as_of)` 取政策：走列字段过滤 + `validity @> as_of`（range/约束索引可复用）。
    - 分析类“费率阈值”查询：走 `employer_rate/employee_rate` 的 `numeric` 列（btree/表达式索引），而不是依赖 JSONB GIN。
    - JSONB GIN 仅用于扩展字段的 containment/exists（例如 flags），不承担数值比较。

### 3.3 增量累加器表（Incremental Accumulator）

为了支持中国个税的**累计预扣法**，系统必须能快速获取员工本年度截止上个月的累计收入和累计已纳税额。传统的实时聚合查询（SELECT SUM(...) FROM payroll_results WHERE year=2024）随着月份增加，性能会线性下降。

“本项目”引入**快照式累加器表**（payroll_balances）：

- **设计**：按 `tax_year`、`person_uuid`、`tax_entity_id`（扣缴义务人）等维度组织；P0 先保证正确性，可暂不做物理分区。

- **机制**：每月薪资“过账”（Posting）时，系统自动计算当月数据并更新该表中的累计值（YTD）。

- **效果**：在计算12月份工资时，引擎只需读取该表的一条记录即可获取前11个月的汇总数据，无需扫描历史明细。这保证了1月份和12月份的算薪耗时几乎一致，解决了传统架构中“年底算薪慢”的顽疾<sup>1</sup>。

## 4. 核心计算引擎：基于Golang的高并发设计

薪资计算引擎是“本项目”的心脏。我们将构建一个无状态、纯内存计算的微服务，利用Go语言特性实现极致的性能。

### 4.1 高并发架构：GMP模型与Worker Pool

面对大规模企业客户（如制造业、零售业），系统需在短时间内处理海量计算任务。

**架构设计：**

1.  **任务分发器（Dispatcher）**：作为生产者，负责扫描待计算的员工列表，并将EmployeeID投递到缓冲通道（Buffered Channel）。

2.  **工作池（Worker Pool）**：启动固定数量的Goroutine（建议设置为CPU核心数 \* 2或根据IO等待时间动态调整）。每个Worker是一个消费者，从通道中抢占任务<sup>3</sup>。

    - **资源隔离**：通过固定 Worker 数量，形成了天然的 **背压（Backpressure）** 机制。即使上游涌入十万个计算请求，同时运行的数据库查询和计算任务也始终控制在 Worker 数量范围内，防止数据库连接池耗尽或 CPU 过载。

    - **上下文加载**：Worker接收到ID后，通过errgroup并发加载员工的薪资档案、考勤数据、社保方案等上下文信息（Context），充分利用I/O多路复用。

**性能预期**：基准测试显示，在处理CPU密集型的规则计算时，Go实现的Worker Pool模式比传统的Java线程池模式性能提升约40%，且内存占用降低一个数量级<sup>3</sup>。

### 4.2 计算精度与金融级安全性

在薪资计算中，浮点数运算（IEEE 754）是绝对禁区。0.1 + 0.2 = 0.30000000000000004这类精度丢失可能导致财务报表借贷不平，甚至引发法律风险。

**技术规范**：

- **禁用float64**：在Go代码中，严禁使用float64处理任何金额。

- **十进制定点库（冻结）**：统一使用 `cockroachdb/apd/v3`（`apd.Decimal` + `apd.Context`）。选型理由：Context 可把 precision/rounding/traps 显式化并集中管理，更适合薪资/税务这种“舍入即合同”的场景；舍入/精度/陷阱由 Context 统一承载，禁止在业务代码散落“临时 Round”。

- **舍入合同（必须显式）**：冻结 `rounding_rule` 枚举与“舍入点”（社保基数、各险种金额、合计、实发等在哪一步量化/进位），并用测试覆盖（否则“禁用 float”仍会被隐式舍入漂移破坏）。

- **全链路精度**：数据库 `numeric` → Go `apd.Decimal` → JSON string（对外/内部 API 均以 string 传递金额与费率，避免 `encoding/json` 默认 number 进入 `float64`）。

### 4.3 动态规则引擎与AST缓存（后续里程碑：M2+）

P0 Slice 不引入通用表达式引擎：先用“结构化字段 + 显式代码路径 + 可测试的舍入合同”交付可见闭环，避免把复杂度提前转移到“公式解释器 + 沙箱 + 数值类型”上。

若后续确需“配置化公式”，必须另立子 dev-plan 冻结以下决策后再实现：

- 选型：复用既有依赖还是引入新库（并验证许可证/版本/可维护性）。
- 数值类型：表达式计算必须以 `apd.Decimal` 为主，不得在中间层引入 `float64`。
- 安全与审计：公式输入范围、可用函数集、资源限制（时间/内存）、以及可复现的审计输出（含舍入点）。

### 4.4 管道模式（Pipeline Pattern）

我们将SAP复杂的Schema（如CN00）重构为清晰的Go语言**管道（Pipeline）**：

1.  **Pre-Process**：时间切片（Time Slicing）。利用PG的Range Intersection算法，处理月中调薪、入离职导致的计薪段拆分。

2.  **Gross-Up**：基于考勤和定薪计算应发工资。

3.  **Social Security**：调用社保引擎计算个人扣款。

4.  **Taxable Income**：计算累计应纳税所得额。

5.  **IIT Calculation**：执行累计预扣算法。

6.  **Net Pay**：计算实发工资。

7.  **Post-Process（非目标：本计划不交付）**：生成银行文件数据和财务凭证数据<sup>1</sup>。

## 5. 深度本地化：攻克中国薪酬社保的复杂性

“本项目”的核心竞争力在于对中国特色复杂业务场景的极致适配。

### 5.1 社保政策引擎：应对“碎片化”挑战

中国社保的复杂性在于其地域差异性。

**解决方案**：

- **多维度键值匹配**：引擎根据员工的City_Code（参保地）和Hukou_Type（户口性质）匹配唯一的规则集。

- **基数核定逻辑**：内置通用的基数计算模板 Base = MAX(Floor, MIN(Actual_Salary, Ceiling))。其中Floor和Ceiling通常关联到“社平工资”。系统支持配置社平工资参数，当每年7月社保基数调整时，只需更新参数表，引擎会自动应用新的上下限。

- **变态级舍入控制**：支持中国各地奇特的舍入规则，如“见分进角”（哪怕是0.01元也要进位到0.1元）、“四舍五入到元”等。这些规则通过策略模式（Strategy Pattern）在Go中实现，并在JSON配置中指定引用<sup>1</sup>。

### 5.2 个税引擎：累计预扣法的算法实现

自2019年个税改革以来，**累计预扣法**成为最大的技术挑战。

算法逻辑：

\$\$\text{本期应预扣税额} = (\text{累计收入} - \text{累计免税} - \text{累计减除费用} - \text{累计专项扣除} - \text{累计专项附加扣除}) \times \text{税率} - \text{速算扣除数} - \text{累计已预扣税额}\$\$

**技术难点与对策**：

- **历史依赖**：计算依赖于前N-1个月的数据。利用前述的payroll_balances增量表，将O(N)复杂度的查询降级为O(1)。

- **负数处理**：如果某月新增了巨额专项附加扣除（如补报了房贷利息），导致计算结果为负数，根据税法规定，当月个税为0，多缴纳的税款留抵下月或年度汇算清缴。Go引擎需内置此逻辑，将负值存储在“留抵税额”字段中，并在下月自动抵扣，而非直接退税<sup>16</sup>。

### 5.3 回溯计算（Retroactive Accounting）（后续里程碑：M2+）

说明：P0 Slice 先交付“当前周期算薪 + 定稿只读”的可见闭环，不实现自动回溯重算；本节为后续设计草案，需在进入实现前再单独收敛为子 dev-plan（避免范围膨胀）。

当 HR 在后续日期提交了早于某已定稿 pay period 的定薪/任职变更时，需要生成“重算请求”，并按合规口径处理差额。

**触发与写入口（One Door / No CDC）**

1. 触发点：所有影响算薪输入的变更必须通过 Kernel `submit_*_event` 写入（例如 `staffing.submit_assignment_event` 的 payload 承载薪酬字段）。
2. 在同一事务内，Kernel 负责判断该变更是否命中已定稿 pay period，并写入 `staffing.payroll_recalc_requests`（append-only）或将 `staffing.payroll_runs` 标记为 `needs_recalc`（模式同 “事件 SoT + 同事务投射”）。
3. 禁止通过 CDC/触发器/拦截器绕过 Kernel 直接写队列（避免形成第二写入口；对齐 `AGENTS.md` 的 One Door 与 `docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md` 的单链路原则）。

**重算策略（口径冻结点）**

- **Corrective（修正法）**：重算 YTD 累加器与累计预扣税基数，确保后续月份税额基于修正后总额计算。
- **Forwarding（结转法）**：将差额作为独立 pay item 结转到下一次 pay period 的工资条（补发/扣回），并保留可审计的差额来源（关联原事件/原工资条）<sup>2</sup>。

## 6. 长期激励（LTI）与期权管理架构（非目标：本计划不交付）

随着中国科创企业的兴起，股权激励（ESOP）已成为薪酬包的重要组成部分。2027年个税优惠政策的延续使得这一模块的设计尤为重要。

### 6.1 归属计划（Vesting Schedule）的数据模型

期权归属涉及复杂的时间表管理。

数据库设计：

利用PG的Range Types来存储归属周期。

```sql
CREATE TABLE option_grants (
  grant_id uuid PRIMARY KEY,
  person_uuid uuid NOT NULL,
  total_shares numeric NOT NULL,
  grant_date date NOT NULL,
  vesting_schedule jsonb NOT NULL DEFAULT '{}'::jsonb -- 存储归属规则配置
);

CREATE TABLE vesting_periods (
  period_id uuid PRIMARY KEY,
  grant_id uuid NOT NULL REFERENCES option_grants(grant_id),
  vest_period daterange NOT NULL, -- 归属区间（Valid Time，日粒度）
  shares_vested numeric NOT NULL,
  status text NOT NULL -- LOCKED, VESTED, EXERCISED
);
```

**业务逻辑**：

- **Cliff Vesting**：支持常见的“4年归属，1年Cliff”模式。Go引擎需计算Cliff日期，并在该日期生成的vesting_periods记录。

- **加速归属（Acceleration）**：支持“双触发”（Double Trigger）逻辑，即在IPO或并购发生且员工被离职时，自动更新剩余vesting_periods的状态为VESTED<sup>18</sup>。

### 6.2 税务合规与2027政策适配

根据《关于延续实施上市公司股权激励有关个人所得税政策的公告》，股权激励个税优惠政策已延长至2027年底。这意味着符合条件的期权行权所得不并入当年综合所得，而是全额单独计税<sup>20</sup>。

**系统实现**：

- **独立计税引擎**：在IIT模块中增加“股权激励单独计税”子模块。

- **公式**：\$\text{应纳税额} = \text{股权激励收入} \times \text{适用税率} - \text{速算扣除数}\$。

- **申报表生成**：系统需自动生成《个人所得税减免税事项报告表》，并在金税接口中标记该笔收入为“股权激励所得”，防止被错误合并计税。

## 7. 生态集成：金税四期与银企直连（非目标：本计划不交付）

### 7.1 金税四期（GTS IV）深度集成与风控

金税四期的核心是从“以票管税”转向“以数治税”，通过大数据比对企业的人力成本、社保缴纳和财务报表。

风控驾驶舱（Risk Cockpit）：

“本项目”将内置一个实时风控模块，在发薪前执行三单比对：

1.  **工资表**：系统的实发工资总额。

2.  **社保台账**：系统计算的社保缴费基数总额。

3.  财务报表：GL接口生成的“应付职工薪酬”科目余额。  
    风险指标：如果“社保缴费基数 \< 工资总额”的比例超过设定阈值（如10%），系统将触发高风险报警，提示企业可能面临税务稽查风险，这是金税四期重点监控的异常指标22。

接口规范：

系统支持生成符合 **自然人税收管理系统（ITS）** 标准的 XML/Excel 申报文件，并预留 RESTful API 接口以对接各省市试点的直连申报网关。API 层将统一处理身份认证、报文加签和加密传输<sup>24</sup>。

### 7.2 银企直连与文件工厂

针对中国银行业接口不统一的现状，系统设计了**银行文件工厂（Bank File Factory）**。

策略模式实现：

Go 语言定义统一接口：

```go
type BankFileGenerator interface {
	Generate(batch *PaymentBatch) (byte, error)
}
```

- **招商银行（CMB）**：实现CMBGenerator，通过XML报文对接招行“云直联”接口，支持直接转账和结果查询<sup>25</sup>。

- **工商银行（ICBC）**：实现ICBCGenerator，生成特定格式的加密Excel或定长文本文件。利用Go的excelize库进行流式写入，防止大文件生成时的内存溢出<sup>26</sup>。

- **安全性**：所有生成的银行文件在落盘前即进行AES-256加密，对于直连请求，强制实施mTLS（双向TLS认证）。

## 8. 实施路径与数据导入（No Legacy）

> 说明：本仓库为 Greenfield implementation repo（见 `AGENTS.md`），不在产品运行时提供 legacy 双链路。若存在外部系统/历史数据，仅允许“离线导入 + 离线对账”；失败时走“停写/修复后重试”，不得回退到旧实现（SSOT：`docs/dev-plans/004m1-no-legacy-principle-cleanup-and-gates.md`）。

### 8.1 阶段一：内核与约束先行（Schema/Kernel）

- 核心任务：在 `staffing` schema 内落地 payroll 相关表与 Kernel 写入口（RLS、幂等、Valid Time 日粒度）。
- 数据约束：有效期不重叠（`EXCLUDE USING gist`）、必要时 gapless/last-infinite；并在 Kernel 中对关键外键做 as-of 校验（模式参考 `modules/staffing/infrastructure/persistence/schema/00003_staffing_engine.sql`）。

### 8.2 阶段二：最小可见闭环（UI + 算薪）

- 核心任务：实现 0.5/0.7 定义的 UI 闭环；定稿后只读。
- 验收方式：以 UI 操作为准，辅以 Kernel/SQL 约束可审计（RLS fail-closed、幂等复用、no-overlap）。

### 8.3 阶段三：可控导入与离线对账（不上主链路）

- 数据导入：仅导入通过 no-overlap/有效期边界校验的“历史快照”（例如任职版本与定薪版本）；导入失败应显式报错并阻断上线。
- 对账：在隔离环境运行 diff 工具比较结果（Net Pay/Tax）；对账工具不接入产品路由、不提供回退开关；结果作为上线前证据记录在 `docs/dev-records/`。
- 切换/回滚：上线仅启用新系统写入口；回滚采用“环境级停写/只读/修复后重试”，不引入 legacy 分支。

## 9. 结论

“本项目路线图”所描绘的，不仅是一个薪酬计算工具的升级，更是企业人力资本管理底层逻辑的重构。通过采用**Golang**，我们获得了处理超大规模并发计算的性能红利；通过采用**PostgreSQL**的Range Types和JSONB，我们构建了坚不可摧的数据一致性防线和极具弹性的业务适应能力。

这套对标并超越 SAP/PeopleSoft 的架构，完美契合了中国市场对于 **“合规性（金税四期）”、“灵活性（300+城市社保）”和“时效性（实时算薪）”** 的三重极致需求。它将帮助企业从繁琐的事务性操作和合规风险中解放出来，让 HR 数字化真正成为驱动业务增长的引擎。

#### 引用的著作

1.  HR SaaS 薪酬社保设计框架

2.  Go+PG HR SaaS薪酬设计

3.  Goroutine Worker Pools - Go Optimization Guide, 访问时间为 一月 7, 2026， [<u>https://goperf.dev/01-common-patterns/worker-pool/</u>](https://goperf.dev/01-common-patterns/worker-pool/)

4.  ConcurrencyWorkshop/internal/pattern/workerpool/README.md at main - GitHub, 访问时间为 一月 7, 2026， [<u>https://github.com/romangurevitch/ConcurrencyWorkshop/blob/main/internal/pattern/workerpool/README.md</u>](https://github.com/romangurevitch/ConcurrencyWorkshop/blob/main/internal/pattern/workerpool/README.md)

5.  Mastering the Worker Pool Pattern in Go - Corentin GS's Blog, 访问时间为 一月 7, 2026， [<u>https://corentings.dev/blog/go-pattern-worker/</u>](https://corentings.dev/blog/go-pattern-worker/)

6.  Temporal Constraints in PostgreSQL 18 \| Better Stack Community, 访问时间为 一月 7, 2026， [<u>https://betterstack.com/community/guides/databases/postgres-temporal-constraints/</u>](https://betterstack.com/community/guides/databases/postgres-temporal-constraints/)

7.  Exclusion Constraints in Postgres \| by Java Jedi - Medium, 访问时间为 一月 7, 2026， [<u>https://java-jedi.medium.com/exclusion-constraints-b2cbd62b637a</u>](https://java-jedi.medium.com/exclusion-constraints-b2cbd62b637a)

8.  Preventing Overlapping Data in PostgreSQL - What Goes Into an Exclusion Constraint, 访问时间为 一月 7, 2026， [<u>https://blog.danielclayton.co.uk/posts/overlapping-data-postgres-exclusion-constraints/</u>](https://blog.danielclayton.co.uk/posts/overlapping-data-postgres-exclusion-constraints/)

9.  Effortless Scheduling with PostgreSQL Daterange and Tstzrange \| by Hrishikesh Raverkar, 访问时间为 一月 7, 2026， [<u>https://blog.mahahrishi.com/effortless-scheduling-with-postgresql-daterange-and-tstzrange-7df7c83d561d</u>](https://blog.mahahrishi.com/effortless-scheduling-with-postgresql-daterange-and-tstzrange-7df7c83d561d)

10. Documentation: 18: 8.17. Range Types - PostgreSQL, 访问时间为 一月 7, 2026， [<u>https://www.postgresql.org/docs/current/rangetypes.html</u>](https://www.postgresql.org/docs/current/rangetypes.html)

11. Replacing EAV with JSONB in PostgreSQL, 访问时间为 一月 7, 2026， [<u>https://coussej.github.io/2016/01/14/Replacing-EAV-with-JSONB-in-PostgreSQL/</u>](https://coussej.github.io/2016/01/14/Replacing-EAV-with-JSONB-in-PostgreSQL/)

12. Optimal Scenarios for Using JSON vs JSONB in PostgreSQL - RisingWave, 访问时间为 一月 7, 2026， [<u>https://risingwave.com/blog/optimal-scenarios-for-using-json-vs-jsonb-in-postgresql/</u>](https://risingwave.com/blog/optimal-scenarios-for-using-json-vs-jsonb-in-postgresql/)

13. Write a Go Worker Pool in 15 minutes \| by Joseph Livni - Medium, 访问时间为 一月 7, 2026， [<u>https://medium.com/@j.d.livni/write-a-go-worker-pool-in-15-minutes-c9b42f640923</u>](https://medium.com/@j.d.livni/write-a-go-worker-pool-in-15-minutes-c9b42f640923)

14. Implementing a Rule Engine in Go with Govaluate - Leapcell, 访问时间为 一月 7, 2026， [<u>https://leapcell.io/blog/implementing-rule-engine-go-govaluate</u>](https://leapcell.io/blog/implementing-rule-engine-go-govaluate)

15. A Brief Introduction on The Cumulative Withholding Method For Individual Income Tax Computation in China - Kaizencpa.com, 访问时间为 一月 7, 2026， [<u>https://www.kaizencpa.com/Mobile/Knowledge/info/id/593.html</u>](https://www.kaizencpa.com/Mobile/Knowledge/info/id/593.html)

16. A BRIEF INTRODUCTION ON THE CUMULATIVE WITHHOLDING METHOD FOR INDIVIDUAL INCOME TAX COMPUTATION IN CHINA - Kaizencpa.com, 访问时间为 一月 7, 2026， [<u>https://www.kaizencpa.com/download/china/A%20Brief%20Introduction%20of%20the%20Cumulative%20Withholding%20Method%20For%20IIT%20Computation%20in%20China.pdf</u>](https://www.kaizencpa.com/download/china/A%20Brief%20Introduction%20of%20the%20Cumulative%20Withholding%20Method%20For%20IIT%20Computation%20in%20China.pdf)

17. Vesting Schedules: What is vesting and how does it work - Ledgy, 访问时间为 一月 7, 2026， [<u>https://ledgy.com/blog/vesting-schedules</u>](https://ledgy.com/blog/vesting-schedules)

18. Vesting Explained: Schedules, Cliffs, Acceleration, and Types - Carta, 访问时间为 一月 7, 2026， [<u>https://carta.com/learn/equity/stock-options/vesting/</u>](https://carta.com/learn/equity/stock-options/vesting/)

19. China extends multiple personal tax benefits to 2027 - WTS Global, 访问时间为 一月 7, 2026， [<u>https://wts.com/global/publishing-article/20231002-china-extends-multiple-personal-tax-benefits~publishing-article</u>](https://wts.com/global/publishing-article/20231002-china-extends-multiple-personal-tax-benefits~publishing-article)

20. China strengthens tax management for equity incentives - KPMG International, 访问时间为 一月 7, 2026， [<u>https://kpmg.com/cn/en/home/insights/2024/05/china-tax-alert-04.html</u>](https://kpmg.com/cn/en/home/insights/2024/05/china-tax-alert-04.html)

21. Key Points for Enterprise Self-checking under Golden Tax Project IV in China, 访问时间为 一月 7, 2026， [<u>https://www.kaizencpa.com/Mobile/Knowledge/info/id/1280.html</u>](https://www.kaizencpa.com/Mobile/Knowledge/info/id/1280.html)

22. Audits, compliance in phase four of 'golden tax' reform - Law.asia, 访问时间为 一月 7, 2026， [<u>https://law.asia/golden-tax-reform-phase-four/</u>](https://law.asia/golden-tax-reform-phase-four/)

23. NetSuite Applications Suite - China Golden Tax System Integration API Overview, 访问时间为 一月 7, 2026， [<u>https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_1553240589.html</u>](https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_1553240589.html)

24. Go in Open Banking: Build Secure, Scalable, Compliant APIs - Deployflow, 访问时间为 一月 7, 2026， [<u>https://deployflow.co/blog/role-go-open-banking-secure-scalable-apis/</u>](https://deployflow.co/blog/role-go-open-banking-secure-scalable-apis/)

25. Corporate E-payroll Sheet-Home-ICBC China, 访问时间为 一月 7, 2026， [<u>https://big5.icbc.com.cn/en/column/1438058791030898704.html</u>](https://big5.icbc.com.cn/en/column/1438058791030898704.html)

26. 网络金融频道-电子银行产品栏目 - 中国工商银行, 访问时间为 一月 7, 2026， [<u>https://www.bj.icbc.com.cn/column/1438058471668203660.html</u>](https://www.bj.icbc.com.cn/column/1438058471668203660.html)

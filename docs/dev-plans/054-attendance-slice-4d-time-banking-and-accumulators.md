# DEV-PLAN-054：考勤 Slice 4D——额度与银行闭环（调休/综合工时累加器）

**状态**: 草拟中（2026-01-09 02:17 UTC）

## 1. 背景与上下文

- 本计划把考勤域的“额度与银行”（Quota & Banking）显式化：调休余额、加班桶累计、综合工时周期累加器等，形成权威读模并与日结果同事务一致更新。
- 该能力是后续 Payroll/结算衔接的基础，但本计划不承接 Payroll 的正式结算接口（需与 `DEV-PLAN-039` 系列对齐）。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] **权威余额/累加器读模**：以“租户 + 人员 + 周期（如月/季/年）”为维度，形成可查询的余额/桶累计读模，并记录版本与重算水位线。
- [ ] **同步投射**：余额/累加器必须由 Kernel 在同一事务内维护（与日结果一致），不引入异步权威写。
- [ ] **最小规则**：支持最小集合（可随主数据扩展）：
  - 加班桶累计（工作日/休息日/法定假日）
  - 调休余额（若暂不支持“选择权”，可先以配置默认策略落地）
  - 综合工时周期累计（为后续扩展预留）
- [ ] **UI 可见**：提供余额/累加器查询页面（按人员/周期），并能追溯余额来源（至少关联到日结果）。

### 2.2 非目标（本计划不做）

- 不实现“以休代薪”的审批/选择工作流；如需选择权，需另立 dev-plan。
- 不输出 payroll wage type/工资项映射与结算；仅沉淀可复用的时长/金额前置读模。

## 3. 关键设计决策（草案）

- **一致性边界**：额度/累加器属于派生状态，必须与日结果一起在同事务内更新，避免出现“日结果已变但余额未变”的第二写入口。
- **重算策略**：更正事件触发日结果重算时，累加器必须同步回滚并重算（建议按周期边界做 bounded replay，而非全量重放）。
- **度量单位**：统一用分钟（int）存储时长，必要时在展示层转换为小时；避免浮点误差。

## 4. 数据模型（草案；落地迁移前需手工确认）

> 红线：新增数据库表/迁移落地前必须获得你手工确认。

- 候选表（示意）：
  - `time_bank_accounts`（按人维护余额）
  - `time_bank_cycles`（周期定义：月/季/年）
  - `time_bank_accumulators`（各类桶累计：weekday_ot/restday_ot/holiday_ot/comp_balance 等）
- 备注：字段与约束需与 `DEV-PLAN-053` 的 TimeProfile/Calendar 口径对齐，避免“规则变更=历史漂移”。

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
- 上游切片：`docs/dev-plans/053-attendance-slice-4c-time-profile-holiday-calendar.md`
- 蓝图：`docs/dev-plans/050-hrms-attendance-blueprint.md`

## 6. 实施步骤

1. [ ] 明确“周期”合同（按月/按季/按年）与最小字段，并评审（落迁移前获得手工确认）。
2. [ ] 设计余额/累加器读模：主键、索引、版本字段、重算水位线；落迁移并按 024 闭环执行。
3. [ ] Kernel：在日结果投射中同步维护余额/累加器；更正导致重算时同步回滚与重算（bounded replay）。
4. [ ] sqlc：提供按人员/周期查询余额与来源关联（关联到日结果）。
5. [ ] UI：余额/累加器查询页（可发现、可操作），并能解释余额来源（至少链接到日结果）。
6. [ ] 测试：覆盖“更正导致余额回滚重算”“跨周期边界”的核心用例；包含 RLS/Authz 负例。
7. [ ] 运行门禁并记录结果（必要时补 `docs/dev-records/`）。

## 7. 验收标准

- [ ] 日结果变化会同步反映到余额/累加器（写后读强一致）。
- [ ] 可追溯：余额至少能追溯到贡献的工作日结果（用于对账）。
- [ ] 安全与门禁：RLS/Authz/路由/生成物门禁全绿。

## 8. 开放问题

- [ ] “以休代薪”的选择权如何表达（配置默认 vs. 后续审批/员工选择）。
- [ ] 综合工时制的周期口径与阈值来源（法规/公司制度），以及与 TimeProfile 的绑定方式。


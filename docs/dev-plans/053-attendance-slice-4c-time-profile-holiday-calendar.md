# DEV-PLAN-053：考勤 Slice 4C——主数据与合规闭环（TimeProfile/Shift/HolidayCalendar）

**状态**: 草拟中（2026-01-09 02:17 UTC）

## 1. 背景与上下文

- 本计划承接 `DEV-PLAN-052` 的“标准班次日结果”能力，把可变规则显式化为主数据：TimeProfile/Shift/HolidayCalendar，并通过有效期（`date`）管理以支撑追溯重算与合规解释。
- 本计划是考勤域“合规复杂度”的第一道门：调休覆盖、法定假日识别与加班分桶（1.5/2.0/3.0）需要可验证的数据来源，而不是硬编码常量。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] **主数据可配置**：TimeProfile/Shift/HolidayCalendar 具备最小 CRUD（effective-dated，`date` 粒度），且变更可驱动日结果变化。
- [ ] **合规规则落地（最小集合）**：
  - 调休（逻辑工作日覆盖）
  - 法定假日识别
  - 加班分桶（1.5/2.0/3.0）与基础口径（分钟/小时）统一
- [ ] **参数化策略**：规则扩展优先“有限枚举 + 参数化策略”，不引入通用脚本/规则引擎/表达式解释器（若确需必须另立 dev-plan）。
- [ ] **UI 可见**：至少提供一个“时间档案/假日日历”配置入口（可发现、可操作），并能观察到对日结果的影响。

### 2.2 非目标（本计划不做）

- 不实现审批流（加班申请/补休选择权/申诉流程）与复杂工作流。
- 不覆盖所有行业制度（不定时/综合工时的完整法条实现）；本切片先为“标准工时 + 合规假日”建立可扩展底座，综合工时累加器在 `DEV-PLAN-054`。

## 3. 关键设计决策（草案）

- **有效期（Valid Time）统一为 `date`**：TimeProfile/Shift/HolidayCalendar 的生效期统一使用 `date`；对外展示与筛选也以 `date` 口径，不引入秒级生效期（对齐 `DEV-PLAN-032`）。
- **假日日历作为主数据 SSOT**：调休/法定假日不从 `DayOfWeek` 推断；以显式日历表为 SSOT。
- **规则版本**：日结果读模中的 `ruleset_version` 应可追溯到 TimeProfile/Calendar 的版本（可先用“最后事件 id/版本号”表达，避免过度设计）。

## 4. 数据模型（草案；落地迁移前需手工确认）

> 红线：新增数据库表/迁移落地前必须获得你手工确认。

- 候选表（示意）：
  - `time_profiles` / `time_profile_events` / `time_profile_versions`（同构 029/030：events + versions，effective-dated）。
  - `holiday_calendars` / `holiday_calendar_days`（按 `date` 存储 day_type：workday/restday/legal_holiday，以及 holiday_code 可选）。
  - `shifts`（固定班次定义，开始/结束/跨天窗口/容差等）。

## 5. 工具链与门禁（SSOT 引用）

### 5.1 触发器（勾选本计划命中的项）

- [ ] Go 代码
- [ ] DB 迁移 / Schema（Atlas+Goose）
- [ ] sqlc
- [ ] 路由治理（Routing）
- [ ] Authz（Casbin）
- [ ] `.templ` / Tailwind（UI 资源）
- [ ] 多语言 JSON（如新增 i18n 文案）

### 5.2 SSOT 链接

- 触发器矩阵：`AGENTS.md`
- CI 门禁：`docs/dev-plans/012-ci-quality-gates.md`
- 有效期口径：`docs/dev-plans/032-effective-date-day-granularity.md`
- RLS：`docs/dev-plans/021-pg-rls-for-org-position-job-catalog.md`
- 路由：`docs/dev-plans/017-routing-strategy.md`
- Authz：`docs/dev-plans/022-authz-casbin-toolchain.md`
- 迁移闭环：`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`
- sqlc：`docs/dev-plans/025-sqlc-guidelines.md`
- 上游切片：`docs/dev-plans/052-attendance-slice-4b-daily-results-standard-shift.md`
- 蓝图：`docs/dev-plans/050-hrms-attendance-blueprint.md`

## 6. 实施步骤

1. [ ] 固化“TimeProfile/Shift/HolidayCalendar”的最小字段与有效期规则（`date` 粒度），并评审（落迁移前获得手工确认）。
2. [ ] 落地 migrations（含 RLS/约束/索引），并按 024 闭环执行。
3. [ ] sqlc：新增主数据读写 queries（注意：写路径仍需通过 Kernel `submit_*_event(...)`，避免直接写 versions）。
4. [ ] 更新日结果投射：从“固定标准班次”切换为“根据 TimeProfile/Shift/Calendar”计算；确保 `ruleset_version` 可追溯。
5. [ ] UI：新增最小配置入口（TimeProfile 与 HolidayCalendar 至少各一个），并能驱动某日结果变化作为验收。
6. [ ] 测试：覆盖调休覆盖、法定假日识别、加班分桶的基本用例；包含 RLS/Authz 负例。
7. [ ] 运行门禁并记录结果（必要时补 `docs/dev-records/`）。

## 7. 验收标准

- [ ] 配置可见：能在 UI 中配置 TimeProfile/Calendar，并看到日结果随之变化。
- [ ] 合规可解释：对某个法定假日的加班分桶结果可被读模解释（来源于日历与规则版本）。
- [ ] 安全与门禁：RLS/Authz/路由/生成物门禁全绿。

## 8. 开放问题

- [ ] 假日日历的数据来源与更新策略（手工维护/导入脚本），以及在不引入过度运维前提下的可复现性。
- [ ] TimeProfile 的作用域：按租户全局/按人群（组织/岗位/人员）绑定的最小合同与 UI 入口。


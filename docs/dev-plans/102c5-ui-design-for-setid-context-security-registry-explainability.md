# DEV-PLAN-102C5：102C1-102C3 UI 专项方案（SetID 上下文化安全 + 策略注册表 + 命中解释）

**状态**: 草拟中（2026-02-23 03:00 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的 UI 子计划，专门承接 `102C1/102C2/102C3` 的用户可视化交付。
- 本计划聚焦“可发现、可操作、可验收”的前端方案，不替代后端契约计划。
- `102C4`（流程个性化样板）当前暂缓：项目尚未建设流程模块；本计划不包含流程编排 UI。

## 1. 背景与问题陈述（Context）
- 当前 SetID 相关页面已存在基础治理入口（`/app/org/setid`），但尚未覆盖：
  1. 上下文化授权拒绝原因的可视化（102C1）；
  2. BU 个性化策略注册表的可管理入口（102C2）；
  3. “为何命中该配置”的统一 explain 展示（102C3）。
- 按 `AGENTS.md` 用户可见性原则，新能力若无 UI 入口/操作闭环，视为未交付。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 为 102C1 提供“上下文输入 + deny reason 可见”的交互闭环。
- [ ] 为 102C2 提供“策略注册表列表/编辑/审校状态”页面闭环。
- [ ] 为 102C3 提供 `brief/full` explain 的展示分层与权限边界。
- [ ] 建立跨页面一致的错误展示：`reason_code + trace_id + request_id`。
- [ ] 降低认知负荷：将技术键转为“业务语义 + 技术键次级显示”，并提供下一步动作提示。
- [ ] 建立可操作性目标：关键任务（筛选/登记/解释查看）不超过 3 步完成，首屏可理解。

### 2.2 非目标（边界冻结）
- 不实现流程变体编排与流程引擎页面（102C4 范围，暂缓）。
- 不重写 102C1/102C2/102C3 的后端契约，只消费其输出。
- 不引入 legacy 双链路、隐藏入口或仅后端可见功能。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 实施阶段按触发器矩阵执行：`AGENTS.md` + `docs/dev-plans/012-ci-quality-gates.md`

## 3. 信息架构与入口设计（IA）
### 3.1 导航与路由策略
- 一级导航沿用 `SetID Governance`（避免新增分叉入口）。
- 在 `/app/org/setid` 下新增三块可深链入口（Tab 或二级路由）：
  1. `Security Context`（102C1）
  2. `Strategy Registry`（102C2）
  3. `Explainability`（102C3）
- 页面支持 URL 显式上下文参数（如 `as_of`、筛选条件），满足可回放与分享。

### 3.2 页面层级
- **L1 总览层**：展示当前上下文、能力状态、异常摘要。
- **L2 操作层**：表单/表格操作（新增、筛选、审查、重试）。
- **L3 证据层**：reason code、explain 链路、trace/request 标识与审计跳转。

### 3.3 任务流收敛（降低操作成本）
- **T1（102C1）**：填写上下文 -> 预检 -> 提交；deny 时给“去补上下文/去申请权限”入口。
- **T2（102C2）**：筛选 -> 新增/编辑 -> 审核状态更新；列表支持排序与列筛选。
- **T3（102C3）**：查看 brief ->（如有权限）展开 full -> 复制 trace/request 排障。

## 4. 102C1（上下文化安全）UI 方案
### 4.1 交互目标
- 在涉及 SetID/Scope 管理写操作的 UI 中，强制显式采集并展示上下文字段：
  - `owner_setid`、`scope_code`、`business_unit_id`（主上下文，必填）、`org_unit_id`（可选，仅资源定位）、`as_of/effective_date`。
- 对 deny 场景展示稳定 `reason_code`，并给出用户可执行下一步（补齐上下文/切换权限/联系管理员）。

### 4.2 核心组件
- 上下文条（Context Bar）：固定显示当前操作上下文。
- 拒绝详情卡（Deny Card）：显示 `reason_code`、用户提示、`trace_id/request_id`。
- 权限诊断抽屉（仅管理员）：展示判定输入快照（不暴露敏感 payload）。
- 原因映射卡（Reason Mapping）：同屏展示“用户可读原因 + 技术 reason_code”。

### 4.3 交互不变量
- 缺失上下文时不允许提交（前端先阻断，后端仍 fail-closed）。
- 前端拒绝文案与 102C1 reason code 一一映射，不使用自由文本漂移。
- deny 卡必须包含下一步动作按钮（`补齐上下文` / `申请权限` / `复制追踪信息`）。

## 5. 102C2（策略注册表）UI 方案
### 5.1 页面模型
- `Strategy Registry` 主视图采用 DataGrid，最小列：
  - `capability_key`、`owner_module`、`personalization_mode`、`org_level`、
    `explain_required`、`is_stable`、`change_policy`。
- 支持筛选维度：模块、个性化模式、BU 层级、稳定状态。
- 支持排序、列显隐、分页与关键字检索；禁止以“纯文本块”替代表格主视图。

### 5.2 操作模型
- 新增/编辑弹窗：强约束 `capability_key + personalization_mode + org_level` 必填（其中 `org_level` 仅允许 `tenant/business_unit`）。
- 当 `personalization_mode != tenant_only` 时，前端强制填写 explain/audit 说明。
- 详情面板展示“承接关系”：102C1（授权）/102C3（解释）/071B（scope 约束）链接。

### 5.3 可见性要求
- 注册表必须可从导航直达，不允许仅通过隐藏链接访问。
- 至少 1 条能力可在 UI 完成“登记 -> 审核状态更新 -> 生效可见”的演示闭环。
- 新增/编辑弹窗提供字段级校验提示，避免用户反复提交试错。

## 6. 102C3（命中解释）UI 方案
### 6.1 展示分级
- `brief`：默认展示在业务页上下文区，包含
  `decision`、`reason_code`、`resolved_setid`、`resolved_package_id`。
- `full`：仅管理员可见，放在 Explain Drawer，展示链路阶段与关键上下文键。
- 非管理员点击 `full` 时必须给出明确反馈（无权限原因 + 如何申请）。

### 6.2 接入页面
- 首批接入：
  1. `/app/org/setid`（治理页）
  2. `/app/jobcatalog`（配置消费页）
  3. `/app/staffing/assignments`（业务写入页）
- 三页统一使用 ExplainPanel 组件，避免各页自造展示口径。

### 6.3 错误与审计联动
- deny/失败必须显示 `reason_code` 与 `trace_id`。
- 用户可一键复制 `trace_id/request_id` 供排障；管理员可跳转审计详情。

## 7. 统一设计约束（跨 C1-C3）
- i18n 仅 `en/zh`，新增文案通过统一 key 管理（不写硬编码双语文案）。
- 页面主题与组件遵循 `DEV-PLAN-002` 与现有 MUI X 模式（FilterBar/DataGrid/DetailPanel）。
- URL 参数与请求参数保持显式时间语义（延续 `102B`：禁止隐式 today）。
- 文案分层规则：中文业务文案为主，技术键（如 `owner_setid`）置于次级信息（tooltip/二级行）。
- 渐进披露：首屏只展示“完成任务必需字段”，高级诊断信息默认折叠。
- 可读性基线：正文不小于 12px，关键动作区行高 ≥ 1.4，信息块间距保持稳定节奏。
- 页面必须具备完整状态：加载中、空态、成功、失败、无权限五类反馈。

## 8. 里程碑（文档到实现）
1. [ ] **M1 IA 冻结**：完成入口、路由、页面职责评审（含 102C1/2/3 责任边界）。
2. [ ] **M2 组件冻结**：完成 Context Bar、Deny Card、Strategy Grid、Explain Panel 组件设计。
3. [ ] **M3 页面联调**：完成 `/org/setid` 主入口与三块子能力联动。
4. [ ] **M4 业务接入**：完成 jobcatalog、assignments explain 接入并通过 E2E 样例。
5. [ ] **M5 验收留证**：输出“可发现+可操作+可解释”证据（截图、录屏、测试记录）。

## 9. 验收标准（Acceptance Criteria）
- [ ] 102C1/102C2/102C3 均有可见入口，且非管理员/管理员路径都可验证。
- [ ] 用户可在 UI 直接看到 deny reason（稳定 reason code）并获取 trace/request 标识。
- [ ] 策略注册表可完成最小登记闭环（新增、筛选、查看详情）。
- [ ] Explain `brief/full` 分级生效，且权限控制正确。
- [ ] 与 102C4（暂缓）无依赖阻塞，交付可独立上线。
- [ ] 可操作性达标：3 条关键任务均可在 3 步内完成（由评审走查留证）。
- [ ] 认知负荷达标：评审问卷中“需要反复阅读才能理解”的问题项不高于 20%。

## 10. 风险与缓解
- **R1：入口分散导致学习成本升高**
  - 缓解：统一在 `/org/setid` 内分区承载，避免新增并行导航。
- **R2：reason code 展示与后端不一致**
  - 缓解：建立前端枚举映射表并纳入回归测试。
- **R3：full explain 泄露敏感信息**
  - 缓解：默认 brief；full 仅管理员授权可见并做字段白名单。
- **R4：技术术语过多导致误操作**
  - 缓解：引入“业务语义优先 + 技术键次级展示 + 下一步动作提示”三件套。

## 11. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102c4-bu-process-personalization-pilot.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/002-ui-design-guidelines.md`
- `docs/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- `AGENTS.md`

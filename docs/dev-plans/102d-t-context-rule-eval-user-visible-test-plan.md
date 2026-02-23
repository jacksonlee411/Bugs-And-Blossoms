# DEV-PLAN-102D-T：102D 动态规则引擎测试方案（用户可见性 + 内部评估链路）

**状态**: 规划中（2026-02-22 13:59 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102D` 的专用测试子计划，负责验证 Context + Rule + Eval 机制在“安全、可解释、可操作”三个维度的交付质量。
- 本计划是测试 SSOT，不重写 `102C1/102C2/102C3` 契约细节；仅验证这些契约在 102D 编排中的一致落地。
- 本计划明确覆盖“用户可见性原则”：新增能力必须有 UI 可见入口，并至少可完成 1 条端到端操作链路。

## 1. 测试目标（冻结）
1. [ ] 验证 102D 规则评估链路在显式时间参数下可稳定重放（遵循 102B）。
2. [ ] 验证通用评估入口第一阶段仅内部可用（`/internal/rules/evaluate`），对外不暴露可伪造上下文入口。
3. [ ] 验证上下文可信边界：客户端最小输入，服务端权威回填，冲突即 fail-closed。
4. [ ] 验证至少 1 条用户可见、可操作、可解释（brief explain）的业务链路。
5. [ ] 验证规则冲突决议、拒绝路径、审计留证可预测且可追溯。
6. [ ] 验证三类核心业务场景：跨 BU 误选防范、上下级规则冲突仲裁、全局标准数据按需私有化定制。

## 2. 范围与边界
### 2.1 范围
- 102D 核心执行链路：Context 构建、候选规则粗筛、CEL 执行、冲突决议、explain 输出。
- 用户可见样板链路（至少 1 条）：
  - `allowances` 页面规则过滤，或
  - `comp-plans` 页面命中决策。
- 与 102C 子计划的联动验证：
  - 102C1：拒绝码/上下文授权；
  - 102C2：capability/策略键一致性；
  - 102C3：brief/full explain 分级与最小字段。

### 2.2 非范围
- 不覆盖 070B 的迁移/切流流程测试。
- 不覆盖全部业务模块，只做样板域闭环。
- 不在本计划引入新的业务契约与错误码定义。

## 3. 测试分层
1. **L1 合同测试（API/Service）**
   - 时间参数必填、错误码映射、输入输出合同稳定性。
2. **L2 集成测试（Rule Engine + Repository + Authz/RLS）**
   - 候选规则筛选、CEL 执行、冲突决议、上下文注入与拒绝。
3. **L3 安全测试（内部入口与上下文伪造）**
   - 内外路由隔离、上下文字段伪造、冲突注入、越权访问。
4. **L4 E2E（用户可见性）**
   - 页面可发现入口、可操作链路、结果反馈与 brief explain。
5. **L5 审计与回放**
   - trace/request/reason/explain 证据完整性与跨日期重放一致性。

## 4. 测试数据夹具（最小闭环）
- Tenant：`T1`
- Business Unit：`BU-A`、`BU-B`
- Actor：
  - `u-admin-a`（BU-A 上下文）
  - `u-admin-b`（BU-B 上下文）
- Worker：
  - `w-cn-dev`（CN/Dev）
  - `w-us-sales`（US/Sales）
- 样板 capability：
  - `comp.allowance_eligibility`
  - `comp.plan_selection`
- 样板规则：
  - `R1`：CN 命中，priority=20
  - `R2`：CN+Dev 命中，priority=10
  - `R3`：兜底 true，priority=100

## 5. 用例矩阵（核心）
### 5.1 时间显式与回放一致性（102B 对齐）
- `TC-TIME-001`：缺失 `as_of` 请求，返回 `invalid_as_of`。
- `TC-TIME-002`：同输入+同日期，在不同执行日结果一致（排除审计时间字段）。

### 5.2 内部入口与上下文可信边界
- `TC-SEC-001`：外部路由访问通用评估入口应被拒绝（第一阶段）。
- `TC-SEC-002`：客户端伪造 `business_unit_id/owner_setid` 被服务端覆盖或拒绝。
- `TC-SEC-003`：客户端提交与服务端推导冲突上下文，必须 fail-closed 并留审计。

### 5.3 冲突决议与拒绝路径
- `TC-RULE-001`：同时命中 R1+R2，按 `priority ASC` 命中 R2。
- `TC-RULE-002`：同级冲突按固定 tie-break（如 `effective_date DESC, rule_id ASC`）稳定命中。
- `TC-RULE-003`：无可决规则时返回拒绝，并包含可解释 reason code。

### 5.4 用户可见性 E2E（至少一条）
- `TC-UI-001`（推荐：allowances）：
  - 前置：用户从导航进入页面（可发现入口）。
  - 操作：选择目标员工，触发规则过滤。
  - 期望：只展示符合规则的选项；用户可完成提交（可操作）。
- `TC-UI-002`（拒绝路径）：
  - 操作：构造不满足规则的目标对象。
  - 期望：页面显示 brief explain（不泄露 full explain 敏感字段）。

### 5.5 核心业务场景增补（按用户诉求冻结）
#### 场景一：跨业务单元（BU）的配置误选防范
- `TC-BU-001`（同一 HR 跨 BU 操作）：
  - 前置：同一 HR 同时具备 BU-A 与 BU-B 的管理权限；当前操作对象为 BU-A 员工。
  - 操作：进入业务页面（如下拉选择方案），触发候选配置加载。
  - 期望：仅显示适用于 BU-A 的方案；仅 BU-B 适用方案被自动屏蔽。
- `TC-BU-002`（越界提交防线）：
  - 操作：通过请求篡改方式强行提交 BU-B 专属方案给 BU-A 员工。
  - 期望：服务端 fail-closed 拒绝，返回稳定 reason code，并在日志保留 explain 证据。

#### 场景二：上下级组织规则重叠与冲突仲裁
- `TC-HIER-001`（总部通用 + 下级特例并存）：
  - 前置：BU-A（总部）规则与 BU-B（下级研发中心）规则同时命中同一员工。
  - 操作：执行规则评估。
  - 期望：按冻结冲突规则稳定选中优先项（如 `priority ASC` + tie-break）；结果不随执行日漂移。
- `TC-HIER-002`（冲突可解释）：
  - 操作：请求 `brief explain`（管理员再验证 `full explain`）。
  - 期望：可解释为何选择特例/通用规则（含候选、淘汰原因、最终决策依据）。

#### 场景三：全局标准数据按需私有化定制
- `TC-GLOBAL-001`（全局 + 租户隐藏）：
  - 前置：存在平台全局标准数据集；租户 T1 对其中部分数据配置“隐藏”。
  - 操作：T1 用户查询字典/候选列表。
  - 期望：隐藏项对 T1 不可见，但对其他租户不受影响。
- `TC-GLOBAL-002`（租户私有追加）：
  - 前置：租户 T1 追加仅本租户可见的非标数据（如内部培训机构）。
  - 操作：T1 与 T2 分别查询同一列表。
  - 期望：T1 可见“全局有效项 + 私有追加项”，T2 不可见 T1 私有项。
- `TC-GLOBAL-003`（隔离与回放）：
  - 操作：在不同执行日使用相同显式日期与上下文重复查询。
  - 期望：结果稳定可复现；跨租户无数据污染，审计链完整。

## 6. 门禁与执行命令（SSOT 引用）
- 文档门禁：`make check doc`
- Go 与质量门禁：`go fmt ./... && go vet ./... && make check lint && make test`
- 路由/授权门禁：`make check routing && make authz-pack && make authz-test && make authz-lint`
- E2E 门禁：`make e2e`
- 说明：具体命令口径以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。

## 7. 验收标准
- [ ] 内部评估入口与外部业务入口边界清晰，未出现可伪造上下文的公开接口。
- [ ] 同一规则输入在显式日期相同前提下可稳定重放，结果可预测。
- [ ] 至少 1 条用户可见、可操作链路通过（入口可发现 + 操作成功 + 结果可解释）。
- [ ] 拒绝路径可解释，reason code 与 102C1/102C3 对齐。
- [ ] 关键证据（API、E2E、日志）可追溯并归档到 `dev-records`。
- [ ] 场景一通过：同一 HR 处理 BU-A 员工时，BU-B 专属配置不会出现在可选列表，强行提交会被拒绝。
- [ ] 场景二通过：总部通用规则与下级特例规则冲突时，系统按冻结仲裁机制稳定命中且可解释。
- [ ] 场景三通过：租户可隐藏全局冗余项并追加私有项，且不影响其他租户可见性。

## 8. 里程碑
1. [ ] **M1 用例冻结**：冻结本计划测试矩阵与夹具。
2. [ ] **M2 合同联调**：完成 L1/L2 主链路回归。
3. [ ] **M3 安全验证**：完成 L3 内部入口与上下文伪造测试。
4. [ ] **M4 用户链路验收**：完成 L4 E2E（成功 + 拒绝）证据。
5. [ ] **M5 留证收口**：完成 L5 回放与审计证据归档。

## 9. 风险与缓解
- **R1：只有后端通过，用户入口未接通**
  - 缓解：把 L4 作为发布前强制项，不通过不得标记交付完成。
- **R2：上下文伪造用例覆盖不足**
  - 缓解：L3 增加冲突输入与越权输入双样板，并纳入回归。
- **R3：explain 过度暴露**
  - 缓解：E2E 强校验 `brief` 不泄露 `full` 字段。

## 10. 关联文档
- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/060-business-e2e-test-suite.md`

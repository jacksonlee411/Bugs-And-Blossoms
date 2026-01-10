# DEV-PLAN-062：全链路业务测试子计划 TP-060-02——主数据（组织架构 + SetID + JobCatalog + 职位）

**状态**: 草拟中（2026-01-10 11:40 UTC）

> 上游测试套件（总纲）：`docs/dev-plans/060-business-e2e-test-suite.md`  
> 依赖：建议先完成 `docs/dev-plans/061-test-tp060-01-tenant-login-authz-rls-baseline.md`（可登录 + 隔离基线）。

## 1. 背景

本子计划覆盖 `DEV-PLAN-009` Phase 4 的主数据纵切片：OrgUnit → SetID → JobCatalog → Position（对齐 `docs/dev-plans/026/028/029/030`），并以 UI 闭环保证“可发现/可操作”。

## 2. 目标与非目标

### 2.1 目标（Done 定义）

- [ ] OrgUnit：树/新增/查询闭环可演示（Create → Tree/List 可见）。
- [ ] SetID：SetID/BU/mapping 可配置；缺映射 fail-closed（无默认洞）。
- [ ] JobCatalog：在 UI 入口可看到 `Resolved SetID`，并能创建至少 2 个 Job Family Group（写入→列表可见）。
- [ ] Position：可创建 10 个职位并在列表可见；职位引用 OrgUnit 输入可靠。

### 2.2 非目标

- 不在本子计划强制覆盖 JobCatalog 的 families/levels/profiles（若环境已实现可作为扩展验证；若未实现记录为 `SCOPE_GAP`）。

## 3. 契约引用（SSOT）

- OrgUnit：`docs/dev-plans/026-org-transactional-event-sourcing-synchronous-projection.md`
- SetID：`docs/dev-plans/028-setid-management.md`
- JobCatalog：`docs/dev-plans/029-job-catalog-transactional-event-sourcing-synchronous-projection.md`
- Position：`docs/dev-plans/030-position-transactional-event-sourcing-synchronous-projection.md`
- Valid Time（日粒度）：`docs/dev-plans/032-effective-date-day-granularity.md`
- 路由/UI 可见性：`AGENTS.md`、`docs/dev-plans/018-astro-aha-ui-shell-for-hrms.md`

## 4. 数据准备要求（060-DS1 子集）

- Tenant：`T060`（host：`t-060.localhost`）
- `as_of`：建议 `2026-01-01`
- OrgUnit 树（最小）：Root + 5 个一级部门（HQ/R&D/Sales/Ops/Plant）
- SetID/BU：
  - SetID：`S2601`
  - BU：`BU000`、`BU901`
  - Mapping：`BU000->SHARE`、`BU901->S2601`
- JobCatalog（`setid=S2601`）：Job Family Group 至少 2 条（示例：`JFG-ENG`、`JFG-SALES`）
- Position：10 条（命名即可）

## 5. 测试步骤（执行时勾选）

1. [ ] **OrgUnit：创建与可见**
   - 入口：`/org/nodes?as_of=2026-01-01`
   - 创建 Root（若不存在）与 5 个部门节点；刷新后树上可见。
2. [ ] **SetID：创建与 mapping 保存**
   - 入口：`/org/setid?as_of=2026-01-01`
   - 创建 `S2601`、创建 `BU901`，保存 mapping `BU901->S2601`。
   - 断言：缺失 mapping 时不得隐式回退为 SHARE（用一个不存在映射的 BU 做负例；可记录为“期望 fail-closed”）。
3. [ ] **JobCatalog：解析链路与写入闭环**
   - 入口：`/org/job-catalog?as_of=2026-01-01&business_unit_id=BU901`
   - 断言：页面显示 `Resolved SetID: S2601`。
   - 创建 Job Family Group：至少 2 条（`code/name` 均必填）；刷新后列表可见。
4. [ ] **Position：创建与列表可见**
   - 入口：`/org/positions?as_of=2026-01-01`
   - 创建 10 个职位（至少覆盖多个 OrgUnit）；刷新后列表可见，并记录每条 `position_id`（uuid）。

## 6. 验收证据（最小）

- OrgUnit：树上可见的截图或导出快照（含 node_id）。
- SetID：mapping 保存后的页面证据；负例（缺映射 fail-closed）的证据。
- JobCatalog：`Resolved SetID: S2601` 证据 + 两条 Job Family Group 可见证据。
- Position：10 条职位可见证据（含 position_id）。

## 7. 问题记录（必须写在本子计划中）

| 时间（UTC） | 环境（Host/as_of/模式） | 复现步骤摘要 | 期望（契约引用） | 实际结果 | 严重级别（P0/P1/P2） | 类型（BUG/CONTRACT_DRIFT/CONTRACT_MISSING/ENV_DRIFT） | 处理建议（改实现/先改契约） | 负责人 | 链接（Issue/PR/日志） |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |


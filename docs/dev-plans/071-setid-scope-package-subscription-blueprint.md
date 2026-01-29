# DEV-PLAN-071：SetID Scope Package 订阅蓝图

**状态**: 草拟中（2026-01-29 02:47 UTC）

## 1. 背景与问题
- `DEV-PLAN-070` 将 SetID 作为配置主数据的显式入口，但同一 SetID 只能指向一套“全域配置组合”，难以表达“同一 SetID 在不同配置域选择不同方案”的需求。
- 为避免“新增 SetID 仅用于组合差异”的膨胀，需要引入可复用、可版本化的配置方案层。

## 2. 目标与非目标
### 2.1 目标
- 保留“租户内全局唯一 SetID”的既有口径。
- 引入 `scope_code`（配置域标识）与 `scope_package`（域内配置方案）。
- 支持 `setid -> scope_code -> scope_package` 订阅关系，并以 `as_of_date` 生效。
- 解析链路 fail-closed：缺失订阅即拒绝读取/写入配置。
- 继续满足 One Door、No Tx/No RLS、Valid Time（date）等不变量。

### 2.2 非目标
- 不做跨域运行时合并/覆盖规则。
- 不引入“旧 SetID 直连配置表”的兼容分支。
- 不改变 `DEV-PLAN-070` 的“org_unit 解析 SetID”主流程。
- 不定义共享层包的混合读取规则（如需共享包，另起计划）。

## 3. 术语与边界
- **scope_code**：配置域标识（稳定枚举），例如 `location`、`education_type`、`jobcatalog`。
- **scope_package**：某个配置域的一套完整方案（可复用/可版本化）。
- **订阅关系**：`(tenant_id, setid, scope_code) -> package_id`，具备有效期。

## 4. 架构概览（高阶）
1. `ResolveSetID(tenant_id, org_unit_id, as_of_date)` 仍由 `DEV-PLAN-070` 定义。
2. 新增 `ResolveScopePackage(tenant_id, setid, scope_code, as_of_date)`。
3. 配置数据读取/写入使用 `package_id`（或 `scope_code + package_id`），不再直接用 `setid` 过滤。

## 5. 数据模型（概念蓝图）
> 具体建表需用户确认（对齐 `AGENTS.md` 新增表确认要求）。
- `orgunit.setid_scope_packages`
  - `tenant_id`, `scope_code`, `package_id`, `name`, `status`, `created_at`, `updated_at`
- `orgunit.setid_scope_package_events`
  - 包创建/停用事件（One Door）
- `orgunit.setid_scope_subscriptions`
  - `tenant_id`, `setid`, `scope_code`, `package_id`, `validity`, `last_event_id`
- `orgunit.setid_scope_subscription_events`
  - 订阅变更事件（One Door）

## 6. 核心规则与不变量
- 单一有效订阅：同一 `tenant_id + setid + scope_code` 在同一日期仅允许一个有效 `package_id`。
- 包归属明确：`package_id` 只能绑定一个 `scope_code`，禁止跨域复用。
- 解析 fail-closed：缺少订阅或订阅无效即拒绝。
- 订阅与包均需走事件入口，禁止直接写表。
- 订阅有效期与 `as_of_date` 对齐（Valid Time 语义）。

## 7. 迁移与切换（蓝图）
- 盘点所有 setid-controlled 配置域，形成 `scope_code` 清单。
- 为每个域定义默认包策略（例如每个租户生成 `DEFLT` 对应的包）。
- 迁移窗口内完成：
  1) 生成包与订阅；
  2) 将存量配置数据绑定至对应 `package_id`；
  3) 切换读写到 `package_id`；
  4) 移除旧 `setid` 直连路径（避免 legacy 双链路）。
- 迁移过程遵循停写切换与审计留痕，具体门禁以 `AGENTS.md` 与 `DEV-PLAN-012` 为准。

## 8. API / UI（概要）
- 新增包管理入口：创建/停用 `scope_package`（按 scope_code）。
- 新增订阅入口：为 SetID 在某个 scope 选择包（含有效期）。
- SetID 管理页展示“当前 SetID 的 scope 订阅表”，并允许切换包。

## 9. 实施步骤（草案）
1. [ ] 定义 `scope_code` 清单与治理规则（归属、权限点、命名约束）。
2. [ ] 设计并确认事件表/版本表结构（需用户确认新增表）。
3. [ ] 在解析链路引入 `ResolveScopePackage` 并更新配置读写口径。
4. [ ] 制定并演练迁移切换方案（停写切换、无双链路）。
5. [ ] 补齐 UI、Authz、测试与门禁证据。

## 10. 验收标准（草案）
- [ ] 任一 SetID 可在不同 `scope_code` 订阅不同 `scope_package` 并生效。
- [ ] 缺失订阅时配置读取/写入 fail-closed。
- [ ] 订阅与包均可审计回放（事件 + validity）。
- [ ] 迁移后无旧 `setid` 直连路径残留。

## 11. 关联文档
- `docs/dev-plans/070-setid-orgunit-binding-redesign.md`
- `docs/dev-plans/028-setid-management.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

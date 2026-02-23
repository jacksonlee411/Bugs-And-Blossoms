# DEV-PLAN-102C6：彻底删除 scope_code + package，收敛到 capability_key + setid（Simple First）

**状态**: 草拟中（2026-02-22 16:40 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 系列在“方案 A（硬取消 scope）”上的唯一实施方案。
- 目标是在不引入 legacy 双链路的前提下，彻底删除 `scope_code / scope_package / scope_subscription / package_id` 概念，统一为 `capability_key + setid` 驱动。
- 本计划优先级高于 102C1/102C2/102C3/102D 中与 scope 相关的历史约定；实施前必须先完成契约修订并冻结。
- 若与 `DEV-PLAN-150` 存在口径冲突，以 `DEV-PLAN-150` 作为最终编排与验收基线。

## 1. 背景与问题陈述（Context）
- 当前模型中 `capability_key`、`scope_code`、`package_id` 并存，造成认知负担和接口重复入参。
- 同一业务判定经常出现“能力键 + scope 键 + 包键”多键约束，违背 Simple > Easy 的单一语义原则。
- 本方案核心选择：不做语义降级或外部隐藏，直接删除 scope 与 package 体系，保留 `capability_key + setid` 两个主业务键。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 从数据库、API、Authz、UI、测试、文档中移除 `scope_code` 及派生概念。
- [ ] 从数据库、API、Authz、UI、测试、文档中移除 `package_id` 及派生概念。
- [ ] 将“包解析”改为“SetID 直读”：按 `tenant + capability_key + setid + as_of` 直接命中配置。
- [ ] explain、策略注册表、业务查询统一使用 `capability_key + setid`，不再要求 `scope_code/package_id`。
- [ ] 以一次性切口完成迁移，不保留兼容窗口与双链路。
- [ ] 建立反漂移门禁，阻断新增 `scope_*` 标识。

### 2.2 非目标
- 不改变 102B 的显式时间语义（`as_of/effective_date` 必填）。
- 不扩展新业务域能力，只处理现有 scope 覆盖范围。
- 不引入 feature flag 或 shadow 运行时路径（允许迁移前离线对账，不允许线上双读双写）。

## 3. 设计原则（Simple > Easy）
1. 一个业务能力只允许一个主键：`capability_key`。
2. 一个归属键：`setid`（创建配置数据时必须显式指定归属 SetID）。
3. 一个解析入口：`resolve_capability_config_by_setid(...)`。
4. 一个写入入口：继续遵循 One Door（事件写入 + 同事务投影）。
5. 失败即阻断：迁移映射不完整时 fail-closed，不做默认回退。

### 3.1 capability_key 防退化规则（对标 Workday 思路）
- `capability_key` 仅表达“能力动作”，不表达组织/租户/地域上下文。
- 禁止把 `setid/bu/tenant/region` 编入 key；上下文差异只能通过 `setid + business_unit + as_of` 承担。
- 同一能力跨 BU 差异必须复用同一个 `capability_key`，通过策略数据分流，不新增 key 变体。
- explain 与审计保留 `capability_key` 作为“判定对象锚点”；但隔离与授权决策不得仅依赖 key。

### 3.2 capability_key 命名规范（冻结）
- 格式：`<module>.<capability>[.<action>]`（小写、下划线、点分段）。
- 合法：`staffing.assignment_create`、`comp.allowance_select`、`jobcatalog.profile_defaults`。
- 非法：`staffing.assignment_create.bu_a`、`jobcatalog.setid_s2601`、`comp.allowance.cn`。
- 禁词：`setid`、`bu`、`scope`、`tenant`、国家/地区代码、组织编码。

## 4. 目标模型（删除 scope 后）
### 4.1 语义模型
- 保留：`capability_key`、`setid`、`business_unit_id`。
- 删除：`scope_code`、`scope_package`、`scope_subscription`、`package_id`。
- 新约束：配置数据必须直接归属 `setid`，不再经过“SetID -> package”中间层。

### 4.2 数据库对象（草案）
> 若涉及新建表/迁移，实施前按仓库红线获取用户确认。

- 新函数：
  - `orgunit.resolve_capability_config_by_setid(tenant_uuid, setid, capability_key, as_of)`
  - `orgunit.assert_capability_setid_active_as_of(...)`
- 新表（命名草案）：
  - `orgunit.setid_capability_configs`
  - `orgunit.setid_capability_config_events`
  - `orgunit.setid_capability_config_versions`
- 退役对象：
  - `orgunit.setid_scope_packages*`
  - `orgunit.setid_scope_subscriptions*`
  - `orgunit.global_setid_scope_*`
  - `orgunit.scope_code_registry()`

### 4.3 API 收敛（草案）
- 新接口命名：
  - `GET/POST /org/api/setid-capability-configs`
  - `GET /org/api/setid-capability-configs?setid=...&capability_key=...&as_of=...`
- explain 接口：移除 `scope_code/package_id`，改为 `capability_key + setid` 必填（`setid` 可由 `business_unit_id + as_of` 推导时不要求前端传）。
- 策略注册表：`scope_package` 模式退役，模式收敛为 `tenant_only / setid`。

### 4.4 UI 收敛
- `/app/org/setid` 页面删除 `scope_code/package_id` 输入项，新增“归属 SetID”必填选择。
- JobCatalog / Staffing 等消费页改为 `capability_key + setid` 查询，不再拼 `scope_code` 或传 `package_id`。
- 所有用户可见文案从“scope 包/包订阅”改为“SetID 归属配置”。

## 5. 迁移策略（一次性切口，无 legacy）
### 5.1 Phase M1：契约冻结
- [ ] 修订 070B/102B/102C1/102C2/102C3/102D 中 scope 相关描述。
- [ ] 发布 102C6 字段词汇表（替换矩阵：`scope_code -> capability_key`，`package_id -> setid`）。
- [ ] 冻结错误码迁移表与 API 变更公告。

### 5.2 Phase M2：数据迁移准备
- [ ] 生成现网 `scope_code -> capability_key` 映射清单与 `setid -> package_id` 展开清单（必须 100% 覆盖）。
- [ ] 对无法映射项产出阻断清单；未清零不得进入切换。
- [ ] 完成 DDL/回填脚本评审与演练（含 RLS、索引、唯一约束）。

### 5.3 Phase M3：代码切换
- [ ] 后端：移除 scope/package API 与函数调用，替换为 setid-capability 入口。
- [ ] 前端：删除 scope/package 交互字段与请求参数，新增 SetID 归属选择。
- [ ] Authz：移除 `org.scope_package` / `org.scope_subscription` 对象，替换为 `org.setid_capability_config` 对象。
- [ ] 测试：删除/替换 scope/package 相关用例，新增 setid 归属用例。
- [ ] 映射治理：建立“业务路由/动作 -> capability_key”单点映射，不允许分散硬编码。

### 5.4 Phase M4：数据切换与清理
- [ ] 停写窗口执行最终回填与一致性校验。
- [ ] 切换后立即删除 scope/package 旧对象与旧路由，不保留兼容入口。
- [ ] 执行反漂移扫描：代码与 SQL 中不再出现 `scope_code/scope_package/scope_subscription/package_id`（业务层）。

## 6. 测试与覆盖率
- 覆盖率口径沿用仓库 CI SSOT（`Makefile` + `docs/dev-plans/012-ci-quality-gates.md`）。
- 必测项：
- [ ] 解析一致性：旧样本在 capability 模型下结果与预期一致。
- [ ] SetID 归属性：新增配置数据时未指定 `setid` 必须被拒绝；指定后仅该 `setid` 可见。
- [ ] 同租户跨 BU 差异：基于 capability 仍可稳定命中差异策略。
- [ ] Explain 完整性：`capability_key` 驱动下 reason/trace/request 链路完整。
- [ ] 安全边界：RLS + Authz 不因删 scope 发生越权放行。
- [ ] 迁移回放：同 `(tenant, capability_key, as_of)` 跨天重放稳定。
- [ ] 命名防退化：`capability_key` 不包含上下文禁词，且无动态拼接生成。

## 7. 门禁与执行命令（引用 SSOT）
- 文档：`make check doc`
- Go：`go fmt ./... && go vet ./... && make check lint && make test`
- 路由/授权：`make check routing && make authz-pack && make authz-test && make authz-lint`
- 迁移：按模块执行 Atlas+Goose 闭环（见 `DEV-PLAN-024`）
- 新增反漂移门禁（本计划新增）：阻断 `scope_code|scope_package|scope_subscription|package_id` 新增引用。
- 新增 capability_key 门禁（本计划新增）：
  - 阻断 key 包含上下文禁词；
  - 阻断运行时拼接 key；
  - 阻断未注册 key 直接用于业务判定。

## 8. 风险与缓解
- **R1：一次性切口风险高**
  - 缓解：仅允许“演练通过 + 清单清零”后切换；否则延期，不做灰度双链路。
- **R2：历史契约冲突**
  - 缓解：M1 先修订文档 SSOT，再改代码。
- **R3：跨模块联动回归面大**
  - 缓解：按模块设责任人与回归清单，执行统一冻结窗口。
- **R4：能力语义丢失**
  - 缓解：保留 capability registry 的解释字段和审计字段，避免黑盒化。

## 9. 验收标准（Acceptance Criteria）
- [ ] 仓库主干不再出现 `scope_code/scope_package/scope_subscription/package_id` 运行时代码。
- [ ] 所有相关接口仅接受 `capability_key + setid`（不再要求或返回 scope/package 字段）。
- [ ] 关键业务场景（跨 BU 误选防范、上下级冲突仲裁、全局+私有可见性）在 capability 模型下全部通过。
- [ ] 不引入 legacy 双链路，不保留 scope 兼容入口。
- [ ] 文档、实现、测试三者口径一致，并通过 CI 门禁。

## 10. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102d-context-rule-evaluation-engine-on-top-of-102-foundation.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

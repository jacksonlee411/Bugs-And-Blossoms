# DEV-PLAN-102C1：SetID 上下文化安全模型（承接 102C，避免与 070B 重复）

**状态**: 草拟中（2026-02-23 02:35 UTC）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的子计划，聚焦 **“角色 + BU 上下文 + 条件”** 的授权模型收敛。
- 本计划不处理 070B 的迁移主轴（global_tenant 下线、发布基座、tenant-only 读链路）。
- 本计划的输出是：授权契约、判定矩阵、门禁与验收标准；具体改码按后续实施 PR 执行。

## 1. 背景与问题陈述（Context）
- 当前 SetID/Scope Package 编辑能力仍以“租户角色”作为主要开关，BU 上下文（business_unit/owner_setid）约束不足。
- 现状风险：
  1. 角色粒度足够粗，但“谁可在什么 BU 上下文做什么”不够显式；
  2. 容易出现“同租户管理员对非归属配置可改”的认知歧义；
  3. 审计可记录 who/when，但 why（为何允许/拒绝）解释信息不完整。
- 与 `DEV-PLAN-102C` 的关系：102C 给出“能力差距”，102C1负责先收敛“安全上下文”这条高优先级差距。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 冻结 SetID 相关操作的上下文化授权四元组：`subject/domain/object/action` + `context`（business_unit / owner_setid / scope_code）。
- [ ] 输出“角色权限 × 上下文条件”能力矩阵，明确 allow/deny/fail-closed 规则。
- [ ] 建立统一拒绝码与解释口径（便于前端与审计使用）。
- [ ] 新增防漂移门禁：阻断“仅角色判断、缺上下文校验”的新增路径。
- [ ] 扩展为字段级执行：在上下文化授权后，执行 `required/visible/default` 字段策略判定（承接 102C2）。

### 2.2 非目标（与 070B 明确隔离）
- 不重复定义 070B 的发布、回填、切流、下线任务。
- 不引入新的共享读回退路径（禁止 legacy/双链路）。
- 不扩展到全部业务模块；仅覆盖 SetID/Scope Package 相关对象与调用路径。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 进入实施后按触发器执行：`go fmt ./... && go vet ./... && make check lint && make test`
- [ ] Authz 门禁：`make authz-pack && make authz-test && make authz-lint`
- [ ] Routing 门禁：`make check routing`

## 3. 设计原则（Simple > Easy）
1. **先冻结授权词汇，再改实现**：先定义 object/action/context，不先写补丁判断分支。
2. **Fail-Closed 默认拒绝**：上下文缺失、冲突、不可解释时统一拒绝。
3. **授权与隔离分层**：RLS 继续做租户边界，Casbin/策略做业务能力边界，不互相替代。
4. **可解释优先**：每个拒绝必须对应稳定 reason code，便于 UI 与审计链展示。

## 4. 上下文化授权模型（目标态）
### 4.1 判定输入（标准化）
- 主输入：`subject/domain/object/action`（沿用 022 冻结口径）。
- 上下文输入（新增冻结）：
  - `owner_setid`
  - `scope_code`
  - `business_unit_id`（主上下文，必填）
  - `org_unit_id`（可选，仅用于资源定位，不参与层级策略）
  - `capability_key`
  - `field_key`（字段级策略判定时必填）
  - `actor_scope`（tenant/saas）
  - `as_of/effective_date`（沿用 102B 显式必填）

### 4.2 判定顺序（冻结）
1. 基础授权（Casbin）先判断 object/action 是否可达；
2. 上下文约束再判断 owner/scope/business_unit 是否满足；
3. 字段策略执行：按 BU 上下文判定 `required/visible/default`（承接 102C2 注册表）；
4. 任一环失败即拒绝（fail-closed），并返回稳定 reason code。

### 4.3 拒绝原因码（建议）
- `OWNER_CONTEXT_REQUIRED`：缺失 owner_setid/business_unit 上下文。
- `OWNER_CONTEXT_FORBIDDEN`：存在上下文但不具备该上下文编辑权。
- `SCOPE_CONTEXT_MISMATCH`：scope_code 与目标资源不匹配。
- `ACTOR_SCOPE_FORBIDDEN`：actor_scope 不满足目标操作要求。
- `AUTHZ_CONTEXT_POLICY_MISSING`：策略未覆盖（用于 shadow/审计告警）。
- `FIELD_REQUIRED_IN_CONTEXT`：当前 BU/上下文要求字段必填但缺失。
- `FIELD_HIDDEN_IN_CONTEXT`：字段在当前 BU/上下文不可见，不允许提交。
- `FIELD_DEFAULT_RULE_MISSING`：字段默认值规则缺失或不可解析。
- `FIELD_POLICY_CONFLICT`：字段策略冲突（如 `visible=false` 且 `required=true`）。

## 5. 能力矩阵（草案）
| object | action | 角色 | 额外上下文条件 | 字段策略条件 | 结果 |
| --- | --- | --- | --- | --- | --- |
| `org.scope_package` | `read` | tenant-viewer/admin | `scope_code` 合法 | 无 | allow |
| `org.scope_package` | `admin` | tenant-admin | `owner_setid` 可编辑且 active | `field.required=false` | allow |
| `org.scope_package` | `admin` | tenant-admin | `owner_setid` 合法 | `field.required=true` 且缺失 | deny (`FIELD_REQUIRED_IN_CONTEXT`) |
| `org.scope_package` | `admin` | tenant-admin | `owner_setid` 合法 | `field.visible=false` 但请求提交字段 | deny (`FIELD_HIDDEN_IN_CONTEXT`) |
| `org.scope_subscription` | `admin` | tenant-admin | setid 与 scope_code 关系合法 | 默认值规则缺失 | deny (`FIELD_DEFAULT_RULE_MISSING`) |
| `org.global_scope_package` | `admin` | superadmin/saas actor | `actor_scope=saas` | 无 | allow |
| `org.global_scope_package` | `admin` | tenant-admin | 任意 | 无 | deny (`ACTOR_SCOPE_FORBIDDEN`) |

> 说明：最终 object 命名以 `DEV-PLAN-022` 及代码 registry 为准；本表先冻结业务语义，不在此计划内改 object 命名体系。

## 6. 与现有计划的边界与承接
### 6.1 与 070B 的边界
- 070B 负责“共享改发布、运行时 tenant-only”。
- 102C1 负责“在 tenant-only 前提下，谁可在何上下文改哪些配置”。
- 102C1 不定义发布任务/迁移脚本/切流步骤。

### 6.2 与 102B 的边界
- 102B 已冻结时间口径；102C1 仅复用，不新增并行时间规则。

### 6.3 与 022 的边界
- 022 冻结 authz 基础术语与策略框架；
- 102C1 在其上补“SetID 场景下的上下文约束层”。

## 7. 里程碑（文档到实施）
1. [ ] **M1 契约冻结**：完成上下文输入、判定顺序、拒绝码、能力矩阵评审。
2. [ ] **M2 门禁设计**：定义并落地“缺上下文校验”检测规则（lint/test）。
3. [ ] **M3 字段策略样板**：在同租户双 BU 样板链路实现 `required/visible/default` 字段策略执行。
4. [ ] **M4 回归验收**：补齐接口测试、403 解释信息、字段级审计字段。

## 8. 验收标准（Acceptance Criteria）
- [ ] SetID/Scope Package 相关 admin 操作均具备上下文校验，不再只依赖角色。
- [ ] 上下文缺失/冲突统一 fail-closed，并输出稳定 reason code。
- [ ] 关键拒绝路径有可回归测试（含 role 正确但 context 错误场景）。
- [ ] 与 070B 无任务重复（评审清单通过）。
- [ ] 同租户不同 BU 可稳定复现字段必填差异（A 必填 / B 非必填）。
- [ ] 同租户不同 BU 可稳定复现字段可见性差异（A 可见 / B 不可见）。
- [ ] 同租户不同 BU 可稳定复现字段默认值规则差异（A=`a1` / B=`b2`）。

## 9. 风险与缓解
- **R1：策略复杂度上升**
  - 缓解：先做两条样板链路，再扩展；禁止一次性全域改造。
- **R2：与现有 API 错误码冲突**
  - 缓解：在 M1 冻结错误码映射，保持兼容响应结构。
- **R3：门禁误报/漏报**
  - 缓解：规则先 shadow，再 enforce。

## 10. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/017-routing-strategy.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102c-t-test-plan-for-c1-c3-bu-field-variance.md`

## 11. 外部公开资料（原则级）
- https://www.workday.com/en-ae/why-workday/trust/security.html
- https://www.workday.com/en-us/enterprise-resource-planning.html

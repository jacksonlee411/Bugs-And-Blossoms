# DEV-PLAN-102C4：BU 流程个性化样板（承接 102C，避免与 070B/102C1/102C2/102C3 重复）

**状态**: 草拟中（2026-02-23 00:02 UTC）— 暂缓（项目当前尚未建设流程模块）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的子计划，聚焦“**流程个性化**”最小可行样板（Pilot）。
- 本计划不承担 070B 的共享迁移与切流，不承担 102C1 的授权模型收敛，不承担 102C2 的注册表治理，不承担 102C3 的 explain 合同定义。
- 本计划输出：样板流程选择、变体模型、执行与回放验收标准。

## 1. 背景与问题陈述（Context）
- 现有 SetID/Scope Package 能力主要体现在“数据命中差异”，流程执行路径的 BU 差异化尚未形成稳定样板。
- 当前缺口：
  1. 能解释“命中哪个配置”，但不能稳定演示“同一业务流程在不同 BU 的执行变体”；
  2. 评审中难验证“个性化不是静态配置，而是可执行流程差异”；
  3. 缺少“流程变体 + 审计 + 回放”三者同测的统一验收模板。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 选定 1 条高价值流程做 BU 个性化样板（优先 staffing/assignments 相关流程）。
- [ ] 定义流程变体最小模型（触发条件、步骤差异、失败路径、审计记录）。
- [ ] 建立“同输入+不同 BU -> 不同流程分支”且可回放的验收用例。
- [ ] 输出可复用模板，供后续流程个性化扩展使用。

### 2.2 非目标（避免重叠）
- 不重复 070B 的发布、回填、切流任务。
- 不重做 102C1 的授权判定规则；仅消费其结果。
- 不重做 102C2 的能力注册字段；仅引用 capability_key。
- 不重做 102C3 explain 字段合同；仅把流程执行节点接入 explain 输出。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 进入实施后按触发器执行（Go/DB/Authz/Routing/E2E）

## 3. 样板流程选择（草案）
### 3.1 选择标准
1. 与 BU 差异存在明确业务价值；
2. 可在现有模块中最小改造落地；
3. 能覆盖成功/拒绝/回滚（环境级）路径；
4. 可被 E2E 与回放测试稳定验证。

### 3.2 首选样板（建议）
- `staffing.assignment_create`（创建任职）
  - BU-A：执行“标准校验链”
  - BU-B：执行“扩展审批前置校验链”
- 说明：该选择只定义样板目标，不在本计划内锁死最终业务规则文本。

## 4. 流程个性化模型（冻结草案）
### 4.1 变体最小字段
| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `capability_key` | 对应 102C2 注册键 | `staffing.assignment_create` |
| `variant_key` | 流程变体键 | `std_v1` / `approval_v1` |
| `match_context` | 命中上下文（BU/setid/scope） | `business_unit=BU1001` |
| `step_chain` | 执行步骤链标识 | `validate->approve->commit` |
| `fail_code_map` | 失败码映射 | `APPROVAL_REQUIRED` |
| `audit_level` | 审计级别 | `full` |

### 4.2 执行不变量
- 同一 `(tenant, capability_key, as_of, context)` 在同版本下必须确定性命中单一 `variant_key`。
- 变体差异只影响流程步骤，不得破坏 102B 的时间显式化约束。
- 失败路径必须 fail-closed，并输出稳定 reason code（可被 102C3 explain 捕获）。

## 5. 与现有计划边界（No-Overlap）
| 主题 | 070B | 102C1 | 102C2 | 102C3 | 102C4 |
| --- | --- | --- | --- | --- | --- |
| 共享迁移/切流 | 实施主责 | 不涉及 | 不涉及 | 不涉及 | 不涉及 |
| 上下文化授权 | 不主责 | 实施主责 | 不主责 | 消费结果 | 消费结果 |
| 个性化能力目录 | 不主责 | 不主责 | 实施主责 | 引用 | 引用 |
| 命中解释合同 | 不主责 | 部分关联 | 部分关联 | 实施主责 | 接入并验证 |
| 流程变体样板 | 不主责 | 不主责 | 不主责 | 不主责 | 实施主责 |

## 6. 实施里程碑（文档到样板）
1. [ ] **M1 样板冻结**：确定 pilot 流程、上下文、变体键与验收边界。
2. [ ] **M2 变体引擎样板**：实现最小变体选择与执行链编排（单流程）。
3. [ ] **M3 explain 接入**：将关键步骤与分支结果纳入 102C3 explain 输出。
4. [ ] **M4 E2E 与回放**：同输入在不同 BU 命中不同分支，并可跨天回放稳定。

## 7. 验收标准（Acceptance Criteria）
- [ ] 至少 1 条流程完成 BU 变体样板，并可在 UI/API 可见。
- [ ] 同一流程在不同 BU 命中不同 `variant_key`，行为差异可解释。
- [ ] deny/失败路径有稳定 reason code 与审计记录。
- [ ] 回放测试可复现流程分支（除审计时间戳字段外一致）。
- [ ] 与 070B/102C1/102C2/102C3 无重复实施任务。

## 8. 风险与缓解
- **R1：样板范围过大导致延期**
  - 缓解：单流程、双变体、先闭环再扩展。
- **R2：流程变体与授权边界耦合混乱**
  - 缓解：授权仍由 102C1，流程只消费授权结论。
- **R3：E2E 不稳定**
  - 缓解：固定测试数据集与显式日期参数，纳入回放机制。

## 9. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102c3-setid-configuration-hit-explainability.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/060-business-e2e-test-suite.md`
- `docs/dev-plans/063-test-tp060-03-person-and-assignments.md`

## 10. 外部公开资料（原则级）
- https://www.workday.com/en-us/enterprise-resource-planning.html
- https://www.workday.com/en-ae/why-workday/trust/security.html

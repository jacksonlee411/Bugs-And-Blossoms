# DEV-PLAN-102C3：SetID 配置命中可解释性（Explainability）方案（承接 102C，避免与 070B/102C1/102C2 重复）

**状态**: 准备就绪（2026-02-22 09:44 UTC，已获用户批准进入实施）

## 0. 主计划定位（Plan of Record）
- 本计划是 `DEV-PLAN-102C` 的子计划，聚焦“**为何命中该配置**”的可解释输出与证据链。
- 本计划不承担 070B 的迁移/切流任务，不承担 102C1 的授权判定设计，不承担 102C2 的注册表治理设计。
- 本计划输出：解释字段合同、解释输出接口约定、日志证据模型、验收标准。

## 1. 背景与问题陈述（Context）
- 当前系统已有 SetID / Scope Package / as_of 链路，但“命中原因”主要散落在实现内部与数据库函数，UI 与 API 缺少统一 explain 输出。
- 典型问题：
  1. 故障排查时能看到结果，但难直接回答“为何是这个 package”；
  2. 业务验收时缺少一致的解释字段，跨模块口径不统一；
  3. 审计记录有 who/when，缺少完整 how/why（决策链路）证据。
- 与 102C 的关系：102C 定义差距，102C3 负责补齐“可解释性”高优先级缺口。

## 2. 目标与非目标（Goals & Non-Goals）
### 2.1 核心目标
- [ ] 冻结 SetID 命中解释最小字段集（请求上下文、解析链、命中结果、拒绝原因）。
- [ ] 定义 explain 输出契约（API + 日志）并与错误码体系对齐。
- [ ] 提供“成功命中”和“拒绝失败”两类解释模板。
- [ ] 建立 explain 覆盖率验收口径：关键链路必须可解释。
- [ ] 覆盖字段级差异解释：可解释同租户跨 BU 的 `required/visible/default_rule` 命中结果。

### 2.2 非目标（避免重叠）
- 不设计 070B 的发布任务模型、迁移脚本、切流步骤。
- 不重写 102C1 的授权策略；仅消费其 reason code。
- 不重写 102C2 的注册表字段；仅引用 capability_key 作为 explain 维度。

### 2.3 工具链与门禁（本计划阶段）
- [x] 文档门禁：`make check doc`
- [ ] 进入实施后按触发器执行（Go/DB/Authz/Routing）

## 3. Explain 合同（冻结草案）
### 3.1 Explain 最小字段
| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `trace_id` | 请求追踪 ID（链路级） | `trc-...` |
| `request_id` | 幂等/请求 ID | `req-...` |
| `capability_key` | 业务能力键（引用 102C2） | `jobcatalog.profile_defaults` |
| `tenant_id` | 当前租户 | `...uuid...` |
| `business_unit_id` | BU 上下文 | `BU-A` |
| `as_of` | 查询时点（读） | `2026-03-01` |
| `effective_date` | 生效日（写，若适用） | `2026-03-01` |
| `org_unit_id` | 资源定位上下文（可选，不参与层级策略） | `10000001` |
| `resolved_setid` | 解析出的 SetID | `DEFLT` |
| `scope_code` | 解析 scope | `jobcatalog` |
| `resolved_package_id` | 命中 package | `...uuid...` |
| `package_owner` | 包归属 | `tenant` |
| `decision` | `allow`/`deny` | `allow` |
| `reason_code` | 拒绝/说明码 | `OWNER_CONTEXT_FORBIDDEN` |
| `field_decisions[]` | 字段级判定数组 | `[{field_key,visible,required,default_rule_ref,resolved_default_value,decision,reason_code}]` |

### 3.2 解释链路阶段（固定顺序）
1. 输入上下文归一化（tenant / as_of / business_unit）。
2. SetID 解析（business_unit -> setid；org_unit 仅作可选定位）。
3. Scope Package 解析（setid + scope + as_of -> package）。
4. 授权与上下文约束判定（引用 102C1）。
5. 字段策略判定（引用 102C2，产出 `field_decisions[]`）。
6. 结果落盘（日志）与可选 API 回显（按安全策略）。

### 3.3 输出级别
- `brief`：仅返回关键结论字段（面向 UI 普通展示）。
- `full`：返回完整解释链（面向审计/排障；需权限控制）。
- `brief` 至少包含字段差异摘要：字段是否必填/是否可见/默认值来源。

## 4. 接口与日志契约（草案）
### 4.1 API 回显约定
- 业务 API 默认不返回 full explain。
- 支持通过受控参数 `explain=brief` 获取简版解释（仅含安全可暴露字段）。
- `explain=full` 仅允许审计/管理员场景，且必须走显式授权。

### 4.2 日志约定
- 所有 SetID 关键链路统一记录 full explain（结构化日志）。
- 日志必须包含：`trace_id/request_id/capability_key/decision/reason_code/resolved_setid/resolved_package_id`。
- 禁止在日志中写入敏感原始 payload（仅保留必要键）。

### 4.3 错误码对齐
- 时间参数错误继续沿用 102B：`invalid_as_of` / `invalid_effective_date`。
- 授权上下文错误沿用 102C1：`OWNER_CONTEXT_FORBIDDEN` 等。
- 缺解释覆盖时返回 `EXPLAINABILITY_MISSING`（内部告警码，不对外暴露细节）。
- 字段策略错误沿用 102C1：`FIELD_REQUIRED_IN_CONTEXT` / `FIELD_HIDDEN_IN_CONTEXT` / `FIELD_DEFAULT_RULE_MISSING` / `FIELD_POLICY_CONFLICT`。
- 缺字段级解释覆盖时返回 `FIELD_EXPLAIN_MISSING`（内部告警码）。

## 5. 与现有计划边界（No-Overlap）
| 主题 | 070B | 102C1 | 102C2 | 102C3 |
| --- | --- | --- | --- | --- |
| 共享迁移/切流 | 实施主责 | 不涉及 | 不涉及 | 不涉及 |
| 上下文化授权规则 | 不主责 | 实施主责 | 不主责 | 消费结果 |
| 个性化能力目录 | 不主责 | 不主责 | 实施主责 | 引用 capability_key |
| 命中可解释输出 | 不主责 | 部分关联 | 部分关联 | 实施主责 |

## 6. 里程碑（文档到实施）
1. [ ] **M1 合同冻结**：字段、阶段、输出级别评审通过。
2. [ ] **M2 样板链路**：为 `scope-packages` 与 `jobcatalog` 各落 1 条 explain 样板。
3. [ ] **M3 字段级扩展**：补齐 `field_decisions[]` 字段与三类差异（必填/可见/默认）样板。
4. [ ] **M4 门禁接入**：关键链路缺 explain 字段时测试失败。
5. [ ] **M5 验收留证**：产出 explain 对照样例（success/deny 各至少 3 例）。

## 7. 验收标准（Acceptance Criteria）
- [ ] 关键链路可以回答“为何命中该 package/为何被拒绝”。
- [ ] explain 字段在 API（brief）与日志（full）口径一致。
- [ ] deny 路径均有稳定 reason_code，且与 102C1 对齐。
- [ ] 与 070B/102C1/102C2 无重复实施任务。
- [ ] 可回答“同租户不同 BU 下，该字段为何在 A 必填、在 B 非必填”。
- [ ] 可回答“同租户不同 BU 下，该字段为何在 A 可见、在 B 不可见”。
- [ ] 可回答“同租户不同 BU 下，该字段为何命中 A=`a1`、B=`b2` 默认值规则”。

## 8. 风险与缓解
- **R1：解释信息泄露过多**
  - 缓解：分级输出（brief/full）+ 权限控制。
- **R2：跨模块字段不一致**
  - 缓解：冻结最小字段集，新增字段走变更评审。
- **R3：日志量膨胀**
  - 缓解：结构化采样与关键字段白名单策略。

## 9. 关联文档
- `docs/dev-plans/102c-setid-group-sharing-and-bu-personalization-gap-assessment.md`
- `docs/dev-plans/102c1-setid-contextual-security-model.md`
- `docs/dev-plans/102c2-bu-personalization-strategy-registry.md`
- `docs/dev-plans/102b-070-071-time-context-explicitness-and-replay-determinism.md`
- `docs/dev-plans/070b-no-global-tenant-and-dict-release-to-tenant-plan.md`
- `docs/dev-plans/022-authz-casbin-toolchain.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `docs/dev-plans/102c-t-test-plan-for-c1-c3-bu-field-variance.md`
- `docs/dev-plans/120-org-field-default-values-cel-rule-engine-roadmap.md`

## 10. 外部公开资料（原则级）
- https://www.workday.com/en-ae/why-workday/trust/security.html
- https://www.workday.com/en-us/enterprise-resource-planning.html
- https://blog.workday.com/en-us/2021/how-workday-supports-gdpr-and-data-subject-rights.html

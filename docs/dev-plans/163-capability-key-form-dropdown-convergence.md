# DEV-PLAN-163：Capability Key 表单字段下拉化收敛方案（Strategy Registry）

**状态**: 已完成（2026-02-24 13:26 UTC）

## 1. 背景
- `SetID Governance -> Strategy Registry` 的能力治理表单仍有较多自由输入字段，录入一致性与可操作性不足。
- 按 `AGENTS.md` 的“用户可见性原则”与“契约优先”，本方案先冻结“哪些字段必须下拉化”的口径，再实施页面改造。

## 2. 目标与非目标
### 2.1 目标
- [X] 对 capability_key 表单页字段完成“可下拉化”评估并冻结矩阵。
- [X] 所有可下拉化字段改为下拉交互（包含可搜索下拉与枚举下拉）。
- [X] 保持现有 API 契约不变（仅前端交互收敛）。

### 2.2 非目标
- 不新增后端字段字典 API。
- 不调整 `setid-strategy-registry` 的服务端校验规则。

## 3. 字段评估矩阵（冻结）
| 字段 | 评估结论 | 收敛方式 |
| --- | --- | --- |
| `capability_key` | 可下拉化 | 可搜索下拉（历史值候选） |
| `owner_module` | 可下拉化 | 可搜索下拉（预置模块 + 历史值） |
| `field_key` | 可下拉化 | 可搜索下拉（历史值候选） |
| `personalization_mode` | 可下拉化 | 枚举下拉（tenant_only/setid） |
| `org_applicability` | 可下拉化 | 枚举下拉（tenant/business_unit） |
| `business_unit_id` | 可下拉化 | 可搜索下拉（历史值 + 绑定记录候选） |
| `default_rule_ref` | 可下拉化 | 可搜索下拉（历史值候选） |
| `default_value` | 可下拉化 | 可搜索下拉（历史值候选） |
| `priority` | 可下拉化 | 枚举下拉（预置优先级 + 历史值） |
| `change_policy` | 可下拉化 | 可搜索下拉（预置策略 + 历史值） |
| `required/visible/explain_required/is_stable` | 可下拉化 | 布尔下拉（true/false） |
| `effective_date/end_date` | 不纳入 | 保持日期控件 |
| `request_id` | 不纳入 | 保持文本输入（便于追踪注入） |

## 4. 实施步骤
1. [X] 新建 DEV-PLAN-163 并冻结字段评估矩阵。
2. [X] 改造 Strategy Registry 筛选区：`capability_key/field_key` 改为可搜索下拉。
3. [X] 改造 Upsert 表单：将所有可下拉化字段切换为下拉组件。
4. [X] 执行门禁验证：`make generate && make css && make check doc`（2026-02-24 11:58 UTC，本地通过）。

## 5. 验收标准
- [X] Strategy Registry 页面中，矩阵内“可下拉化字段”全部为下拉交互。
- [X] 表单提交 payload 与改造前字段命名一致，无契约破坏。
- [X] 本地门禁通过并可复现。

## 7. 验证与验收记录（2026-02-24 UTC）
- [X] 环境与服务：执行 `make dev-up`、`make iam migrate up`、`make orgunit migrate up`、`make jobcatalog migrate up`、`make person migrate up`、`make staffing migrate up`；并确认 `:8080`、`:4433`、`:4434`、`:8081` 可用。
- [X] 登录联调（按 `bugs-and-blossoms-dev-login` 技能）：三租户账号 `POST /iam/api/sessions` 均返回 `204` 且写入 `sid` cookie。
- [X] 租户隔离：同租户 `GET /iam/api/dicts?as_of=2026-01-01` 返回 `200`；跨租户复用 sid 返回 `401`。
- [X] 163 方案验收：`/app/org/setid?tab=strategy-registry` 可访问（`200`）；Strategy Registry 筛选区与 Upsert 表单字段已按矩阵下拉化（可搜索下拉 + 枚举下拉 + 布尔下拉）；`onUpsertStrategy` payload 字段名保持不变。

## 6. 关联文档
- `docs/dev-plans/102c5-ui-design-for-setid-context-security-registry-explainability.md`
- `docs/dev-plans/150-capability-key-workday-alignment-gap-closure-plan.md`
- `docs/dev-plans/160-capability-key-m8-m10-ui-delivery-and-evidence-closure.md`
- `AGENTS.md`

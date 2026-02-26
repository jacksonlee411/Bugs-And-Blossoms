# DEV-PLAN-181：OrgUnit Details 三类表单到 Capability Key 映射落地

**状态**: 已完成（2026-02-26 11:15 UTC）

## 1. 背景
DEV-PLAN-180 在迁移附录中冻结了以下映射：
- `FORM + orgunit.details.add_version_dialog` -> `org.orgunit_add_version.field_policy`
- `FORM + orgunit.details.insert_version_dialog` -> `org.orgunit_insert_version.field_policy`
- `FORM + orgunit.details.correct_dialog` -> `org.orgunit_correct.field_policy`

实施前缺口：OrgUnit Details 三类意图（add/insert/correct）未在 UI 写入链路稳定携带 `policy_version`，存在版本漂移风险。

## 2. 目标
- [X] 在 capability 注册表中补齐上述三个 capability_key，并纳入 route-capability 合约。
- [X] 为 `create/add/insert/correct` 建立稳定的 intent->capability_key 映射（后端单点）。
- [X] `GET /org/api/org-units/write-capabilities` 返回 `capability_key + policy_version`，并按 intent 输出对应值。
- [X] `POST /org/api/org-units/write` 对四类 intent 统一执行 `policy_version` 新鲜度校验。
- [X] 前端 OrgUnit Details 写入链路提交 `policy_version`，避免版本漂移写入。

## 3. 非目标
- 不改动 `tenant_field_policies` 历史只读审计窗口语义。
- 不在本计划中新增 scope_key 写入口。
- 不重构 mutation policy 本身（仅补 capability 映射与版本一致性）。

## 4. 设计决策
1. **单点映射函数（后端）**
   - 以 `intent` 为输入输出 `capability_key`；create/add/insert/correct 四类显式枚举。
   - 禁止运行时拼接 capability_key。

2. **写能力接口口径统一**
   - `write-capabilities` 响应新增：
     - `capability_key`
     - `policy_version`
   - policy_version 来源：`policy_activation_runtime.activePolicyVersion(tenant, capability_key)`。

3. **写入接口版本门禁统一**
   - `org-units/write` 对四类 intent 全部要求 `policy_version` 非空且与 active 版本一致。
   - 继续复用既有错误码：`FIELD_POLICY_VERSION_REQUIRED` / `FIELD_POLICY_VERSION_STALE`。

## 5. 实施步骤
1. [X] 文档与契约：新增本计划并补充 AGENTS Doc Map。
2. [X] 后端 capability 注册：补齐 add/insert/correct 三个 key（Go registry + JSON 合约）。
3. [X] 后端 API：
   - `write-capabilities` 输出 capability_key/policy_version；
   - `org-units/write` 统一四类 intent 的 policy_version 校验。
4. [X] 前端：OrgUnit Details 提交写入时携带 `policy_version`。
5. [X] 测试：补齐映射与版本校验用例，验证 contract 同步与核心 go test。

## 6. 验收标准
- [X] 仓库可检索到三条新增 capability_key 的注册定义。
- [X] `write-capabilities?intent=add_version|insert_version|correct` 返回与 intent 对应的 capability_key。
- [X] Details 页 add/insert/correct 提交不再省略 policy_version。
- [X] `org-units/write` 对四类 intent 均执行 policy_version 校验并返回一致错误码。
- [X] `TestCapabilityRouteRegistryContract` 与相关 orgunit API 测试通过。

## 7. 执行记录（2026-02-26 UTC）
- [X] `go test ./internal/server -run "TestCapabilityRouteRegistryContract|TestOrgUnitFieldPolicyCapabilityKeyFor|TestHandleOrgUnitWriteCapabilitiesAPI|TestHandleOrgUnitsWriteAPI|TestHandleOrgUnitFieldPoliciesAPI_ValidationCoverage" -count=1`
- [X] `./scripts/ci/check-capability-route-map.sh`

## 8. 关联
- `docs/dev-plans/180-granularity-hierarchy-governance-and-unification.md`
- `docs/dev-plans/165-field-configs-and-strategy-capability-key-alignment-and-page-positioning.md`
- `docs/dev-plans/156-capability-key-m3-m9-route-capability-mapping-and-gates.md`

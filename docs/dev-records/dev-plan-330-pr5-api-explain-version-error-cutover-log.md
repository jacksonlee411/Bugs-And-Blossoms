# DEV-PLAN-330 PR-5 记录：API / explain / version / 错误码切主链

**状态**: 已完成（2026-04-11 18:20 CST）

## 1. 范围

1. [X] 把双轴 schema 与唯一 PDP 正式接成对外 canonical 主链，而不是只停留在内部实现收口。
2. [X] 统一 explain / internal evaluate / OrgUnit version 契约，明确 `policy_version` 与 `effective_policy_version` 的正式语义。
3. [X] 收敛 Strategy Registry API 主写契约，显式表达 `resolved_setid_scope + resolved_setid`，不再保留“只传 BU 让服务端猜 SetID 轴”的正式主链。
4. [X] 收敛 canonical 错误码，并把历史大写 `FIELD_POLICY_*` 口径降为兼容映射而非正式输出目标。

## 2. 代码交付

1. [X] Strategy Registry API 已切为双轴外部契约：
   [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go)
   [setids.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/setids.ts)
   [SetIDGovernancePage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/SetIDGovernancePage.tsx)
2. [X] explain 与内部评估已统一到 canonical 输出：
   [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go)
   [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go)
   [SetIDExplainPanel.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/components/SetIDExplainPanel.tsx)
3. [X] OrgUnit create-field-decisions / write-capabilities / write submit 已切到双版本主链：
   [orgunit_create_field_decisions_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_create_field_decisions_api.go)
   [orgunit_write_capabilities_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_write_capabilities_api.go)
   [orgunit_write_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_write_api.go)
   [orgUnits.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/orgUnits.ts)
   [OrgUnitsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitsPage.tsx)
   [OrgUnitDetailsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitDetailsPage.tsx)
4. [X] canonical 错误码与错误目录已收口：
   [setid_strategy_pdp.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/fieldpolicy/setid_strategy_pdp.go)
   [orgunit_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_api.go)
   [orgunit_nodes.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_nodes.go)
   [presentApiError.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/errors/presentApiError.ts)
   [catalog.yaml](/home/lee/Projects/Bugs-And-Blossoms/config/errors/catalog.yaml)

## 3. 实施要点

### 3.1 Registry API 已显式表达 SetID 语义轴

1. [X] upsert / disable / list 响应与请求都已显式带出 `resolved_setid_scope`，不再把 exact / wildcard 语义藏在 BU 解析侧。
2. [X] `tenant + exact setid` 与 `tenant + wildcard` 都能通过正式 API 表达；`business_unit` 记录被收敛为 `resolved_setid_scope=exact`。
3. [X] 若 `business_unit_org_code` 解析出的 SetID 与显式 `resolved_setid` 不一致，API 统一返回 `resolved_setid_mismatch` 并 fail-closed。

### 3.2 explain / version 已切成一条正式主链

1. [X] explain 不再返回 `resolved_config_version` 这类过渡字段，改为正式输出：
   - `policy_version`
   - `effective_policy_version`
   - `matched_bucket`
   - `winner_policy_ids`
   - `resolution_trace`
2. [X] `policy_version` 固定表示 intent capability 当前 active version；`effective_policy_version` 固定表示 intent/baseline 组合版本签名。
3. [X] OrgUnit create-field-decisions、write-capabilities 与 write submit 已按同一版本语义对齐，避免“预览一套版本、提交校验另一套版本”。

### 3.3 canonical 错误码已成为正式输出目标

1. [X] PDP 正式错误码切为 lower snake_case：
   - `policy_missing`
   - `policy_conflict_ambiguous`
   - `policy_mode_invalid`
   - `policy_mode_combination_invalid`
2. [X] Registry / OrgUnit 写入相关错误码切为 canonical：
   - `policy_disable_not_allowed`
   - `policy_redundant_override`
   - `policy_version_required`
   - `policy_version_conflict`
3. [X] 前端提示层已补 canonical code 映射；旧 `FIELD_POLICY_*` 仅保留兼容读取，不再作为本批正式输出目标。

## 4. 风险收口

1. [X] 本批没有保留 API 别名窗口或双错误码正式输出窗口；外部契约已直接切向 canonical 主链。
2. [X] 对仍可能出现的历史大写错误码，当前仅在前端提示层做兼容映射，避免页面半切状态影响使用；该兼容不改变后端正式输出目标。
3. [X] `resolved_setid` 已进入 Registry API 正式读写契约，避免继续停留在“schema 双轴、API 单轴”的过渡态。

## 5. 验证

1. [X] `go test ./internal/server/...`
2. [X] `pnpm -C apps/web test -- src/pages/org/OrgUnitDetailsPage.test.tsx src/pages/org/SetIDGovernancePage.test.tsx src/components/SetIDExplainPanel.test.tsx src/errors/presentApiError.test.ts`
3. [X] `go fmt ./...`
4. [X] `go vet ./...`
5. [X] `make check lint`
6. [X] `make test`
7. [X] `make check doc`

## 6. 对 PR-6 的冻结前提

1. [X] `PR-6` 不再需要回头补 API / explain / version / canonical error code 主链切换，可直接承接 route/authz 与旧层退场。
2. [X] 若后续再出现新的 `FIELD_POLICY_*` 正式输出、`resolved_config_version` 回流，或 Registry API 再次回到 BU 单轴输入，应视为对 `PR-5` 的回归。

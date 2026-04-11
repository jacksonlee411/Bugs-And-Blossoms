# DEV-PLAN-330 PR-6 记录：route/authz 收口、旧层退场与门禁封板

**状态**: 已完成（2026-04-11）

## 1. 范围

1. [X] 将 SetID 治理台相关 route-level capability、owner module 与 authz requirement 全部收口到 canonical 主链。
2. [X] 退役旧 `field-policies*` public route、前端 helper、server/module 兼容 store 与半切换残留。
3. [X] 补 `no-legacy` 防回流门禁，并把 `PR-6` 完成状态写回 `DEV-PLAN-330` 主计划与文档地图。

## 2. 代码交付

1. [X] route capability / owner module 已切主：
   [capability_route_registry.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/capability_route_registry.go)
   [route-capability-map.v1.json](/home/lee/Projects/Bugs-And-Blossoms/config/capability/route-capability-map.v1.json)
2. [X] authz 与路由注册已同步收口并退役旧 public route：
   [authz_middleware.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/authz_middleware.go)
   [handler.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/handler.go)
   [allowlist.yaml](/home/lee/Projects/Bugs-And-Blossoms/config/routing/allowlist.yaml)
3. [X] server/module 旧兼容层已删除：
   [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go)
   [orgunit_field_metadata_store.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_store.go)
   [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go)
4. [X] 前端旧 helper 与页面旧 policy dialog 已删除：
   [orgUnits.ts](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/api/orgUnits.ts)
   [OrgUnitFieldConfigsPage.tsx](/home/lee/Projects/Bugs-And-Blossoms/apps/web/src/pages/org/OrgUnitFieldConfigsPage.tsx)
5. [X] retired runtime/public symbol 防回流门禁已补齐：
   [check-no-legacy.sh](/home/lee/Projects/Bugs-And-Blossoms/scripts/ci/check-no-legacy.sh)

## 3. 实施要点

### 3.1 route/authz 已站到 canonical 主链

1. [X] `GET/POST /org/api/setid-strategy-registry`、`POST /org/api/setid-strategy-registry:disable`、`GET /org/api/setid-explain` 与 `POST /internal/rules/evaluate` 的 route bucket 已统一收口到 `org.orgunit_write.field_policy`。
2. [X] 同组路由的 `OwnerModule` 已统一为 `orgunit`，不再继续表达为 staffing capability 所属治理面。
3. [X] capability-route-map、authz requirement 与 allowlist 已同步对齐，不再存在“路由是一套归属、鉴权又是一套归属”的分裂状态。

### 3.2 旧 `tenant_field_policies` public/runtime 兼容层已退场

1. [X] `/org/api/org-units/field-policies`、`/org/api/org-units/field-policies:disable`、`/org/api/org-units/field-policies:resolve-preview` 已从 handler 与 authz requirement 删除。
2. [X] server 侧旧 store/API/test 壳已删除，不再保留“禁用但似乎还能复活”的影子入口。
3. [X] module 侧 `ResolveTenantFieldPolicy` 读 helper 与对应 domain type/test 已删除，避免 module 层继续持有第二套旧策略读语义。
4. [X] 前端 `upsert/disable/resolvePreview` helper、相关类型与页面旧 policy dialog 已删除；字段配置页只保留 canonical 主链的 field-config + Strategy Registry/PDP 动态镜像。
5. [X] 数据库历史表与迁移定义暂不删除，定位收敛为“历史结构 / 迁移事实源”，不再代表 runtime happy path。

### 3.3 门禁已补到“旧路不能回流”

1. [X] `make check no-legacy` 新增 retired symbol 扫描，覆盖：
   - 旧 public route 字符串
   - 前端旧 helper 名称
   - server/module 旧 store API 名称
2. [X] 该扫描仅针对 runtime/public source，显式排除：
   - docs
   - schema 历史定义
   - 测试文件中的负向断言
3. [X] 结果是：既保留了 `404/未映射` 反回归测试，又能阻断真正的 runtime/public 回流。

## 4. 风险收口

1. [X] 本批不保留 preview compatibility、旧 public route 别名或半切换 helper。
2. [X] `tenant_field_policies` 不再以“兼容读壳”身份继续参与正式字段裁决；若后续再出现新的 runtime/public 读写入口，应直接视为 `PR-6` 回归。
3. [X] route/authz 已统一站在 canonical 主链上，避免治理台继续借用 staffing bucket 造成页面归属与审计归属混乱。

## 5. 验证

1. [X] `go test ./internal/server ./modules/orgunit/...`
2. [X] `go fmt ./...`
3. [X] `go vet ./...`
4. [X] `make check routing`
5. [X] `make check capability-route-map`
6. [X] `make authz-pack`
7. [X] `make authz-test`
8. [X] `make authz-lint`
9. [X] `make check no-legacy`
10. [X] `make test`
11. [X] `make check lint`
12. [X] `make check doc`
13. [X] `pnpm --dir e2e exec playwright test tests/tp288b-librechat-live-task-receipt-contract.spec.js --workers=1`
14. [X] `pnpm --dir e2e exec playwright test tests/tp290b-librechat-live-intent-action-chain.spec.js --workers=1`
15. [X] `pnpm --dir e2e exec playwright test tests/tp290b-librechat-live-intent-action-negative.spec.js --workers=1`

### 5.1 PR #494 追绿补充

1. [X] 补齐 OrgUnit `CREATE` / `SET_BUSINESS_UNIT` 提交后、事务提交前的 `setidresolver.EnsureBootstrap(...)`，消除租户初始化后正式写链路仍可能命中 `SETID_BINDING_MISSING` 的时序缺口。
2. [X] superadmin bootstrap registry seed 已显式写入 `resolved_setid`，并将 `ON CONFLICT` canonical key 收敛到 `resolved_setid + business_unit_node_key + effective_date`。
3. [X] formal surface 的 `Confirm` / `Submit` 点击已改为“找最后一个可见按钮”，消除 DOM refresh / hidden button 抖动导致的误点。
4. [X] `tp290b-neg-002` 已按 turn 当前相位与候选态分支断言：
   - 未进入 `await_commit_confirm` 时返回 `conversation_confirmation_required`
   - 多候选待选时返回 `assistant_candidate_not_found`
   - 单候选自动解析时 bad candidate 覆盖被忽略，但不会写入错误 candidate

## 6. 结论

1. [X] `PR-6` 完成后，`DEV-PLAN-330` 的 canonical 主链不再只是“新路已立”，而是“旧路已拆 + route/authz/gates 已压到同一条主链上”。
2. [X] 后续若出现 `field-policies*` public route、旧 helper、旧 store API、或治理台 capability 重新漂回 staffing bucket，应视为对 `PR-6` 的直接回归。

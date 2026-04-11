# DEV-PLAN-330 PR-4 记录：唯一 PDP 与前置测试

**状态**: 已完成（2026-04-11 17:28 CST）

## 1. 范围

1. [X] 抽出共享 PDP，把双轴 bucket 命中、排序、mode 合并、默认值收敛与 explain trace 统一为单一运行时实现。
2. [X] 清掉 `internal/server` 与 `modules/orgunit/infrastructure` 内部并行存在的 happy-path 决策逻辑，避免继续双实现漂移。
3. [X] 为唯一 PDP 补齐确定性测试、mode 矩阵测试与 explain 回放测试，作为 `PR-5` 前置条件。
4. [X] 保持当前 API 契约与错误码主链不在本批次顺手扩散；`PR-5` 前仍不声称 API / explain / version / canonical error code 已全部切主链。

## 2. 代码交付

1. [X] 新增共享 PDP 与独立测试：
   [setid_strategy_pdp.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/fieldpolicy/setid_strategy_pdp.go)
   [setid_strategy_pdp_test.go](/home/lee/Projects/Bugs-And-Blossoms/pkg/fieldpolicy/setid_strategy_pdp_test.go)
2. [X] Registry runtime helper 改为复用共享 PDP，并把 explain trace / 决策结构作为正式输出：
   [setid_strategy_registry_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api.go)
   [setid_strategy_registry_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_strategy_registry_api_test.go)
3. [X] `/internal/rules/evaluate` 改为走共享 PDP 裁决，并对 `FIELD_POLICY_*` 失败场景执行 fail-closed：
   [internal_rules_evaluate_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api.go)
   [internal_rules_evaluate_api_test.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/internal_rules_evaluate_api_test.go)
4. [X] explain、assistant precheck、OrgUnit 字段启用候选与 metadata 预览统一带出 `resolved_setid + business_unit_node_key`：
   [setid_explain_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/setid_explain_api.go)
   [assistant_create_policy_precheck.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/assistant_create_policy_precheck.go)
   [orgunit_create_field_decisions_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_create_field_decisions_api.go)
   [orgunit_field_metadata_api.go](/home/lee/Projects/Bugs-And-Blossoms/internal/server/orgunit_field_metadata_api.go)
5. [X] OrgUnit PG store 改为按双轴读取候选并复用共享 PDP，不再保留 store 私有 happy-path 决策：
   [orgunit_pg_store.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store.go)
   [orgunit_pg_store_policy_test.go](/home/lee/Projects/Bugs-And-Blossoms/modules/orgunit/infrastructure/persistence/orgunit_pg_store_policy_test.go)

## 3. 实施要点

### 3.1 唯一 PDP 已成为正式运行时边界

1. [X] 共享 PDP 的正式输入固定为 `capability_key + field_key + resolved_setid + business_unit_node_key + registry records`。
2. [X] bucket 顺序、候选排序、mode 合并、默认值与 explain trace 不再允许在 handler/store 侧各自重写。
3. [X] wildcard 与 exact 的匹配规则由共享 PDP 单点收敛，避免“schema 双轴、裁决仍多口径”。

### 3.2 `internal/server` 与 persistence 不再各自维护平行 happy path

1. [X] Registry helper 已改为先把 PG/API 记录映射为 `fieldpolicy.Record`，再统一交给 `fieldpolicy.Resolve(...)`。
2. [X] `/internal/rules/evaluate` 不再依赖旧 CEL happy-path 裁决；字段策略缺失/冲突/默认值异常时，统一返回 `deny` 并带出稳定 `reason_code`。
3. [X] OrgUnit store 不再保留独立 bucket/priority 选择逻辑，而是复用共享 PDP 的最终决策与默认值结果。

### 3.3 前置测试已补齐

1. [X] 共享 PDP 已覆盖 bucket 命中顺序与确定性输出。
2. [X] 共享 PDP 已覆盖 mode 矩阵与 fallback default 对齐。
3. [X] explain / internal evaluate / OrgUnit persistence 已补共享 PDP 接入后的回归测试，证明主链不再分叉。

## 4. 验证

1. [X] `go fmt ./...`
2. [X] `go test ./internal/server/...`
3. [X] `go test ./...`
4. [X] `go vet ./...`
5. [X] `make check lint`
6. [X] `make test`

## 5. 对 PR-5 的冻结前提

1. [X] `PR-5` 之前不得把“唯一 PDP 已完成”偷换成“API / explain / version / 错误码主链也已全部完成”；后者仍属于下一批次。
2. [X] `PR-5` 可以直接承接本批共享 PDP 与前置测试，不再需要回头清理平行 happy-path 决策实现。
3. [X] 若后续再出现第二套 bucket/mode/默认值裁决逻辑，应视为对 `PR-4` stopline 的回归，而不是允许的新扩展点。

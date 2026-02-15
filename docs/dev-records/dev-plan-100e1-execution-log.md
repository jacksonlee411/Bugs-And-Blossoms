# DEV-PLAN-100E1 执行日志

**状态**：已实施（2026-02-15）

**关联文档**：
- `docs/dev-plans/100e1-orgunit-mutation-policy-and-ext-corrections-prereq.md`

## 已完成事项

- 2026-02-15：OrgUnit Mutation Policy 单点化（服务端唯一口径）与 capabilities/corrections 对齐（作为 DEV-PLAN-100E 前置）
  - 共享元数据 SSOT：新增 `modules/orgunit/domain/fieldmeta`，server/services 统一引用，避免两套定义漂移。
  - 写路径补齐 enabled ext configs 读取：扩展 `modules/orgunit/domain/ports.OrgUnitWriteStore` + PG 实现（不改 DB schema/迁移）。
  - 策略单点：新增 `modules/orgunit/services/orgunit_mutation_policy.go`（ResolvePolicy/AllowedFields/ValidatePatch），capabilities API 复用该策略（enabled ext 字段集合 E 并入 allowed_fields）。
  - Corrections 支持 `patch.ext`：`POST /org/api/org-units/corrections` 接收 `patch.ext`，fail-closed 校验；严格 JSON 解码（`DisallowUnknownFields`）；显式拒绝客户端提交 `patch.ext_labels_snapshot`，并在服务端生成 DICT canonical `ext_labels_snapshot`。
  - 防回归：补齐 handler/service/store/fieldmeta 与 server 测试；补齐覆盖率分支测试以恢复全仓 100% coverage 门禁。

## 本地验证（门禁对齐）

- 2026-02-15 03:26 UTC：`go fmt ./... && go vet ./... && make check lint && make test && make check doc`（PASS；coverage 门禁通过：总覆盖率 100.00%；doc 门禁通过）

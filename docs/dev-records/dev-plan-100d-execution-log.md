# DEV-PLAN-100D 执行日志

**状态**：已实施（2026-02-14）

**关联文档**：
- `docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`

## 已完成事项

- 2026-02-14：OrgUnit 宽表元数据 Phase 3（服务层 + Internal API）落地（读写可用）
  - 路由 allowlist：新增 Internal API 路由
    - `/org/api/org-units/field-definitions`
    - `/org/api/org-units/field-configs`（GET/POST）
    - `/org/api/org-units/field-configs:disable`
    - `/org/api/org-units/fields:options`
    - `/org/api/org-units/mutation-capabilities`
  - Authz：为新增路由补齐权限门禁（admin/read fail-closed）
  - Internal API（服务端实现）：
    - field-definitions：返回可启用字段定义（含 `data_source_config_options`；DICT/ENTITY 非空）
    - field-configs：
      - list：按 `as_of + status` 过滤（day 粒度半开区间）
      - enable：校验 `data_source_config` 命中 `field-definitions.data_source_config_options`；调用 Kernel 启用函数；幂等重试返回 200
      - disable：调用 Kernel 停用函数；幂等重试返回 200
    - fields:options：DICT 返回 options（支持 `as_of/q/limit`；PLAIN/ENTITY fail-closed）
    - details：扩展 details 响应，返回 `ext_fields[]`（稳定排序；DICT `display_value_source` 覆盖 versions/events/dict_fallback/unresolved）
    - mutation-capabilities：承接 `DEV-PLAN-083`，并补齐扩展字段 `allowed_fields/field_payload_keys(ext.<field_key>)`（MVP：仅 target=CREATE 允许扩展字段写入）
    - list：支持 ext 字段 filter/sort（allowlist + 物理列名正则守卫 + identifier quoting + 值参数化；并限制为 grid/pagination 模式）
  - Store/SQL（PG store）：
    - 读取/管理 `orgunit.tenant_field_configs`（list / enabled-as-of / enable / disable）
    - 读取 versions + labels 快照：`orgunit.org_unit_versions` + `orgunit.org_events_effective.payload.ext_labels_snapshot`
    - 列表分页：扩展 `ListOrgUnitsPage` 支持 ext filter/sort，并在 store 层 fail-closed 拒绝非 allowlist 字段
    - mutation-capabilities 辅助：resolve target event + rescind deny reasons
  - 测试补齐：
    - handler 路由注册存在性（memory store 下 fail-closed）
    - authz middleware 路由权限映射
    - field-configs / options / details ext_fields / mutation-capabilities / list ext query 的 API 契约测试
    - PG store 方法与分支覆盖（含 ext 查询 fail-closed 规则）

## 本地验证（门禁对齐）

- 2026-02-14：已执行并通过
  - Go：`go fmt ./... && go vet ./... && make check lint && make test`
  - Routing：`make check routing`
  - Authz：`make authz-pack && make authz-test && make authz-lint`
  - Doc：`make check doc`
  - E2E（额外）：`make e2e`（Playwright：6 passed）

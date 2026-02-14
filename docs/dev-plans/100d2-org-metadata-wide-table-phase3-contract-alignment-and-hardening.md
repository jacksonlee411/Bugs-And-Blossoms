# DEV-PLAN-100D2：Org 模块宽表元数据落地 Phase 3 修订：契约对齐与 API 实现收口（为 100E/101 做准备）

**状态**: 已完成（2026-02-14 09:25 UTC）

> 目标：在 **不新增 DB schema**、不引入 legacy/双链路 的前提下，把已存在的 Phase 3（`DEV-PLAN-100D`）实现对齐到最新冻结口径（`DEV-PLAN-100A/100D/101/100E`），并补齐必要的测试与门禁证据，为 Phase 4A/4B（`DEV-PLAN-100E/101`）的 UI 联调提供稳定后端。

## 1. 背景

`DEV-PLAN-100D` 已定义 Phase 3 的 SSOT（Internal API + allowlist + fail-closed）。但在 100D 落地之后，我们补齐/澄清了两个会直接影响 UI 与测试稳定性的口径：

1. **停用字段后“隐藏”**：当 `as_of >= disabled_on` 时，该字段不属于 enabled 集合，因此 **details 的 `ext_fields[]` 不返回/不展示**；看历史要切换 `as_of` 或看 Audit（对齐 `DEV-PLAN-100E` 的“ext_fields = enabled 字段全集”）。
2. **启用字段时 DICT/ENTITY 的 data_source_config 可由租户管理员选择**：但必须从 `field-definitions.data_source_config_options` 的枚举候选中选择并提交；启用后映射不可变（`data_source_config` 也不可修改）（对齐 `DEV-PLAN-100A` 的“映射不可变”）。

当前代码尚未完全按以上口径调整时，Phase 4 的典型问题是：

- UI 无法稳定解释“为什么字段消失/不能改”（停用后仍返回但禁用 vs 直接隐藏的口径漂移）。
- enable/disable 的幂等与来源配置校验不一致，导致“可点必败/重试占槽位/配置不可排障”。

因此需要一个 **100D2** 作为“Phase 3 修订与收口”的实施计划，专门描述：差异点、代码落点、测试补齐与门禁证据。

## 2. 范围与非目标

- **范围（In Scope）**
  - 对齐 Phase 3 的 Internal API 实现到最新冻结口径（见 §3）。
  - 补齐/加固服务端校验（尤其是 `data_source_config` 的枚举化校验与 canonical 化）。
  - 补齐 API 契约测试、store/分支测试与门禁证据记录（readiness 级别）。
  - 更新对应执行日志（`docs/dev-records/`）以满足“P0-Ready 证据”口径。

- **非目标（Out of Scope）**
  - 不新增/变更 DB schema、迁移、sqlc（如确需新增表/列必须另起 dev-plan 且先获用户确认）。
  - 不实现 UI（`DEV-PLAN-100E/101` 承接）。
  - 不引入 ENTITY join 的真实实现（若当前字段清单未命中，可保持 fail-closed；若要做，另立计划）。
  - 不改写 One Door：任何写入仍必须走 Kernel 的 `submit_*` 路径（SSOT：`DEV-PLAN-026/100C`）。

## 3. 目标口径（SSOT 引用，100D2 只列“必须对齐的点”）

> 下面每条都必须能在代码 + 测试中证明；细节契约以原 SSOT 为准，避免在本文件复制导致漂移。

1. **field-definitions**
   - 必须返回 `data_source_config_options`（仅 DICT/ENTITY；且为非空数组；固定来源返回单元素数组）。
2. **enable field-config**
   - 请求体允许提交 `data_source_config`（仅 DICT/ENTITY；必须命中 options；禁止任意透传）。
   - `PLAIN` 允许缺省 `data_source_config`，由服务端补齐为 `{}`。
   - 启用后映射不可变（包含 `data_source_config`），冲突/重试遵循 `request_code` 幂等语义（SSOT：`DEV-PLAN-100A/100D`）。
3. **details ext_fields**
   - `ext_fields[]` 必须等于 `as_of` 下 enabled 字段全集（即使值为空也要返回该字段；稳定排序）。
   - 当 `as_of >= disabled_on` 时字段不属于 enabled 集合，因此不得出现在 `ext_fields[]`。
4. **fields:options**
   - PLAIN：不支持 options（fail-closed）。
   - DICT：options 来源由 `data_source_config` 决定（例如 `dict_code`），支持 `as_of/q/limit`；配置缺失/非法必须 fail-closed。
   - ENTITY：若暂未实现，必须明确 fail-closed（避免“看似可用但必失败”）。

## 4. 实施步骤（100D2 执行清单）

> 执行顺序：先做契约对齐的“最小闭环”（API 行为 + 测试），再补齐路由/authz 门禁与证据记录。

1. [x] **差异盘点（以代码为准，不靠记忆）**
   - 对照 `DEV-PLAN-100A/100D/101/100E` 的上述 4 点，列出现状偏差清单（按 endpoint/错误码/排序/可见性）。
   - 标注每个偏差的代码落点（handler/service/store/test）。
   - 盘点结论：实现主体已符合冻结口径；本次补齐点主要集中在“契约测试断言”（options 非空、PLAIN 缺省 `{}`、options 端点错误码 fail-closed）。

2. [x] **对齐 field-definitions：补齐 options 契约**
   - 若缺失：为 DICT/ENTITY 补齐 `data_source_config_options` 的返回与稳定排序。
   - 增加契约测试：DICT/ENTITY 必须有非空 options；PLAIN 不返回该字段或返回空（以 100D 为准）。

3. [x] **对齐 enable：允许在启用时选择并提交 data_source_config**
   - 请求体：支持 `data_source_config` 字段；PLAIN 可缺省。
   - 校验：DICT/ENTITY 必须命中 `field-definitions.data_source_config_options`（canonical JSON 后比较）。
   - 错误码：对齐 `DEV-PLAN-100D`（`ORG_FIELD_CONFIG_INVALID_DATA_SOURCE_CONFIG` 等），并补齐失败路径测试。

4. [x] **对齐 details：停用后隐藏 + enabled 字段全集**
   - 确保 details 的 `ext_fields[]` 来源是 “enabled-as-of 集合”，并且对每个 enabled 字段都返回一条 item（值可以为 null）。
   - 增加测试：`as_of < disabled_on` 可见；`as_of >= disabled_on` 不可见（不在 `ext_fields[]`）。
   - 备注：enabled-as-of 的边界语义由 `ListEnabledTenantFieldConfigsAsOf`（store）与 `orgUnitFieldConfigEnabledAsOf`（单测）共同固化；details 端 `ext_fields[]` 仅消费 enabled-as-of 集合。

5. [x] **对齐 options：按 data_source_config 决定来源，非法配置 fail-closed**
   - DICT：从 `data_source_config` 解析 `dict_code`；缺失/空/非法 -> fail-closed（稳定错误码）。
   - 增加测试：未启用字段/不支持类型/配置非法/keyword+limit 行为。

6. [x] **路由与鉴权门禁（若本次变更触及路由/权限映射）**
   - routing allowlist 更新并通过 `make check routing`。
   - authz 路由权限映射更新并通过 `make authz-pack && make authz-test && make authz-lint`。
   - 本次变更未触及 routing/authz 映射，因此无需更新。

7. [x] **证据记录与收口**
   - 新建/更新执行日志：`docs/dev-records/dev-plan-100d2-execution-log.md`（记录命令、时间戳、结果）。
   - `DEV-PLAN-100D2` 状态在全部完成后更新为 `已完成`，并写入完成时间戳。

## 5. 验收标准（DoD）

- [x] `field-definitions` 满足 DICT/ENTITY options 契约；输出稳定（排序/字段形状）。
- [x] enable 支持 DICT/ENTITY 的 `data_source_config` 选择与校验；PLAIN 可缺省；错误码稳定；幂等重试不重复占槽位。
- [x] details 的 `ext_fields[]` 仅包含 enabled-as-of 字段全集；停用后隐藏（不返回/不展示）。
- [x] DICT options 可用且 fail-closed；不支持类型明确拒绝。
- [x] 门禁证据齐全：至少 Go/doc 门禁与关键契约测试通过（命中项以 `AGENTS.md`/`DEV-PLAN-012` 为准；routing/authz 本次未触及）。

## 6. 关联文档（SSOT）

- Phase 0 冻结契约：`docs/dev-plans/100a-org-metadata-wide-table-phase0-contract-freeze-readiness.md`
- Phase 3 SSOT：`docs/dev-plans/100d-org-metadata-wide-table-phase3-service-and-api-read-write.md`
- Phase 4A（详情页 UI）：`docs/dev-plans/100e-org-metadata-wide-table-phase4a-orgunit-details-capabilities-editing.md`
- Phase 4B（字段配置 UI）：`docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- 路由/门禁/触发器：`AGENTS.md`、`docs/dev-plans/012-ci-quality-gates.md`

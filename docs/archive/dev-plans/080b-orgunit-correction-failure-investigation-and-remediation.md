# [Archived] DEV-PLAN-080B：OrgUnit 生效日更正失败（`orgunit_correct_failed`）专项调查与修复方案

**状态**: 已归档（2026-02-22，错误码提取规范已并入 `DEV-PLAN-111`；本文仅保留专项调查与修复记录）

## 1. 背景
- 触发场景：在页面 `http://localhost:8080/org/nodes?tree_as_of=2026-02-09`，将组织「飞虫与鲜花（org_code=1）」记录生效日期从 `2026-01-01` 更正为 `2025-01-01`，前端报错 `orgunit_correct_failed`。
- 目标：明确失败根因，给出可执行修复方案，并沉淀可复用排障证据。

## 2. 问题描述（用户侧）
- 目标组织信息：
  - 生效日期：`2026-01-01`
  - 状态：`有效`
  - 组织名称：`飞虫与鲜花`
  - 组织编码：`1`
  - 上级组织：`-`
  - 部门负责人：`1 飞虫`
  - 组织长名称：`飞虫与鲜花`
- 实际现象：点击保存后页面显示失败，错误码为 `orgunit_correct_failed`。

## 3. 调查范围与方法
1. 代码链路：UI -> API -> Service -> Store -> DB Kernel。
2. 数据库函数权限状态核查：`pg_proc.prosecdef`（是否 `SECURITY DEFINER`）。
3. 运行态复现实验：
   - 直接调用 `orgunit.submit_org_event_correction(...)`。
   - 在事务内临时切换函数为 `SECURITY DEFINER` 做对照实验（仅验证，最终 `ROLLBACK`）。

## 4. 结论摘要
- 本次失败的**直接根因不是日期越界规则**，而是 **DB Kernel 函数权限语义回归**：
  - `submit_org_event_correction` 当前以 `SECURITY INVOKER` 运行，触发 `org_unit_codes` 写入保护器时，`current_user=app`，被拒绝。
- `orgunit_correct_failed` 是**二次症状**：API 层未抽取 PG stable message，导致真实 DB 错误码未被识别，降级成默认码。

## 5. 证据清单（Evidence）

### E1. 页面写入入口确认为 corrections API
- `internal/server/orgunit_nodes.go:4673`：前端提交到 `POST /org/api/org-units/corrections`。

### E2. API 默认错误码来源
- `internal/server/orgunit_api.go:419`：correction 失败调用 `writeOrgUnitServiceError(..., "orgunit_correct_failed")`。
- `internal/server/orgunit_api.go:641`、`internal/server/orgunit_api.go:642`：错误码取 `err.Error()` 原文。
- `internal/server/orgunit_api.go:656`：未知场景回落为默认码（即 `orgunit_correct_failed`）。

### E3. DB 写保护触发器规则
- `migrations/orgunit/20260202130000_orgunit_org_code_write_gate.sql:50`：`guard_org_unit_codes_write()`。
- `migrations/orgunit/20260202130000_orgunit_org_code_write_gate.sql:55`：仅允许 `current_user=orgunit_kernel` 写 `org_unit_codes`。
- `migrations/orgunit/20260202130000_orgunit_org_code_write_gate.sql:57`：拒绝时报 `ORGUNIT_CODES_WRITE_FORBIDDEN`。

### E4. 运行库函数权限状态（实库查询）
- 实际查询结果显示：
  - `submit_org_event_correction`、`submit_org_status_correction`、`submit_org_event_rescind`、`submit_org_rescind` 的 `prosecdef=false`（即 Security Invoker）。
  - `rebuild_org_unit_versions_for_org` 也是 invoker。
- 这与受保护表写入要求（必须以 `orgunit_kernel` 身份执行）不一致。

### E5. 直接复现同类失败（SQL）
- 在本地 DB 调用 correction（事务内）返回：
  - `ERROR: ORGUNIT_CODES_WRITE_FORBIDDEN`
  - `DETAIL: role=app`
  - `CONTEXT: ... guard_org_unit_codes_write ... rebuild_org_unit_versions_for_org ... submit_org_event_correction`
- 证明失败发生在 DB Kernel 重建链路，而非前端参数校验。

### E6. 对照实验（事务内临时改为 Definer）
- 在同一事务里临时执行：
  - `ALTER FUNCTION orgunit.submit_org_event_correction(...) SECURITY DEFINER;`
  - `ALTER FUNCTION ... SET search_path = pg_catalog, orgunit, public;`
- 再调用同一 correction，返回成功 UUID；随后 `ROLLBACK`。
- 证明：权限语义修复后，原请求可通过。

### E7. 日期边界并非本次阻断点
- `org_events_effective` 对该组织在目标租户下仅有 `2026-01-01` 一条记录，`prev/next` 为空。
- 因此本次未命中 `EFFECTIVE_DATE_OUT_OF_RANGE` / `EVENT_DATE_CONFLICT`。

### E8. UI 无法展示真实 DB 错误码
- `internal/server/orgunit_nodes.go:4688` 的前端映射表未包含 `ORGUNIT_CODES_WRITE_FORBIDDEN`。
- 在 API 已降级默认码时，页面只能展示泛化失败信息。

## 6. 根因分析（Root Cause）
1. **权限语义回归**（主因）
   - 审计链/内核函数重建后，correction/rescind/status-correction 相关函数处于 invoker 模式。
   - 调用链内部会写受保护表 `org_unit_codes`，触发器要求 `current_user=orgunit_kernel`，从而拒绝。
2. **错误映射不完整**（放大器）
   - API 端使用 `err.Error()` 做精确匹配，无法稳定提取 PG message；真实错误码被吞并为 `orgunit_correct_failed`。

## 7. 影响范围评估
- 直接受影响写入口：
  - `submit_org_event_correction`
  - `submit_org_status_correction`
  - `submit_org_event_rescind`
  - `submit_org_rescind`
- 体感现象：用户端表现为通用失败，缺少可操作提示。
- 风险：同类“可定位 DB 错误”会被默认码遮蔽，排障成本上升。

## 8. 解决方案（Remediation Plan）

### 8.1 P0：DB Kernel 权限修复（必须）
新增一条 orgunit migration（建议编号紧随当前迁移序列），统一设置：
1. `ALTER FUNCTION orgunit.submit_org_event_correction(uuid, int, date, jsonb, text, uuid) SECURITY DEFINER;`
2. `ALTER FUNCTION orgunit.submit_org_event_correction(...) SET search_path = pg_catalog, orgunit, public;`
3. 对以下函数同样处理：
   - `submit_org_status_correction(uuid, int, date, text, text, uuid)`
   - `submit_org_event_rescind(uuid, int, date, text, text, uuid)`
   - `submit_org_rescind(uuid, int, text, text, uuid)`
4. 函数 owner 保持 `orgunit_kernel`，与 `org_unit_codes` 写门禁保持一致。

### 8.2 P0：API 稳定错误码提取（必须）
- 将 OrgUnit API 错误映射从 `err.Error()` 调整为优先提取 PG stable message（对齐 `stablePgMessage` 口径）。
- 目标：即使 DB 返回 `P0001` 风格错误，也能稳定映射到业务码。

### 8.3 P1：前端错误文案补齐（建议）
- 在 `internal/server/orgunit_nodes.go` 映射表补充 `ORGUNIT_CODES_WRITE_FORBIDDEN`（管理员可读文案）。

## 9. 验收标准（Acceptance）
1. 页面场景可复现通过：
   - `tree_as_of=2026-02-09`，将目标组织生效日 `2026-01-01 -> 2025-01-01`，保存成功。
2. API 返回码可诊断：
   - 若仍失败，应返回明确稳定码（不再落入 `orgunit_correct_failed`）。
3. 函数权限核查通过：
   - `pg_proc.prosecdef=true`（上述 4 个函数）。
4. 回归范围：
   - correction / status correction / rescind event / rescind org 均可正常执行并通过现有门禁测试。

## 10. 执行清单
1. [X] 完成专项调查与复现实验（代码链路 + DB 对照实验）。
2. [X] 形成证据链并沉淀文档（本文件）。
3. [X] 提交 migration 修复函数权限语义（Definer + search_path）。
4. [X] 补齐 API 错误码提取与测试。
5. [X] 补齐 UI 文案映射与回归验证。

## 10A. 实施结果（2026-02-10）
- 新增迁移：`migrations/orgunit/20260210101000_orgunit_corrections_kernel_privileges.sql`，对 correction/status-correction/rescind-event/rescind-org 四个入口统一回设 `OWNER TO orgunit_kernel + SECURITY DEFINER + search_path`。
- API 错误码提取：`internal/server/orgunit_api.go` 改为优先使用 `stablePgMessage(err)`，避免 `P0001` 场景退化到默认码。
- UI 文案补齐：`internal/server/orgunit_nodes.go` 补充 `ORGUNIT_CODES_WRITE_FORBIDDEN` 映射。
- 回归测试：新增 `internal/server/orgunit_corrections_kernel_privileges_test.go`；并在 `internal/server/orgunit_api_test.go` / `internal/server/orgunit_nodes_test.go` 增补对应断言。


## 11. 关联文档
- `docs/dev-plans/080-orgunit-audit-chain-consolidation.md`
- `docs/dev-records/dev-plan-080-execution-log.md`
- `docs/dev-plans/075b-orgunit-root-backdating-feasibility-and-fix-plan.md`

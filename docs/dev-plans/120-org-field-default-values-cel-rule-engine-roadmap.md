# DEV-PLAN-120：Org 字段默认值（Go+PG+CEL）规则引擎落地路线图

**状态**: 规划中（2026-02-18 21:11 UTC）

## 1. 背景

`docs/dev-records/Go+PG+CEL 规则引擎架构升级.md` 给出了规则引擎蓝图，但当前仓库（尤其 Org 模块）仍缺少“管理员配置规则 -> 创建时自动算默认值 -> 用户可见”的首个落地闭环。

当前现状：

1. 字段配置页（`DEV-PLAN-101`）已支持启用/停用字段，但没有“默认值规则”能力。
2. Org 新建流程仍主要依赖手工填写（例如 `org_code`）。
3. 写入链路遵循 One Door（`submit_org_event(...)`），可作为默认值注入的唯一执行入口。
4. 字段配置列表当前以扩展字段为主，未覆盖 `org_code` 等核心字段，导致无法对核心字段做同口径配置。

本计划目标是把蓝图收敛为可执行路线图，并先完成一个高价值里程碑：
**在字段配置页新增“默认值”列，并支持通过 CEL 规则在新建部门时自动计算默认值（示例：`O+6位最小未占用流水号`）。**

## 2. 目标与非目标

### 2.1 目标（冻结）

1. 引入租户级“字段默认值规则”配置能力（CEL）。
2. 字段配置页新增“默认值”列，支持查看/编辑规则摘要。
3. 创建记录时默认值由后端计算并注入（服务端权威，避免前端绕过）。
4. 首个场景打通：`org_code` 默认值可由规则自动生成（`O000001`、`O000002`...）。
5. 错误反馈确定性：编译错误、运行错误、资源耗尽、冲突错误可区分。
6. 字段配置页新增“可维护”列（是/否）；当“可维护=否”时，该字段在业务表单不可编辑，以系统默认值为准。
7. 字段配置列表统一覆盖“核心字段 + 扩展字段”（包含 `org_code`），避免只有扩展字段可配置。
8. 支持“同一字段在不同表单页面可同可异”的可配置能力，并提供明确优先级规则。

### 2.2 非目标（Stopline）

1. 不在首期实现跨模块（jobcatalog/person/staffing）统一规则引擎接入。
2. 不在首期实现 Tri-State/PENDING 异步补数流程。
3. 不引入 legacy 双链路或灰度开关分叉（遵循 `DEV-PLAN-004M1`）。

## 3. 项目不变量对齐（必须满足）

1. **One Door**：默认值计算发生在写服务进入 `submit_*_event(...)` 前，不新增第二写入口。
2. **No Tx, No RLS**：规则读取与写入均在显式事务 + 租户注入下执行。
3. **No Legacy**：不保留“旧默认逻辑 + 新规则逻辑”长期并存。
4. **request_code 幂等**：默认值计算不破坏同一 `request_code` 的幂等语义。
5. **MUI + en/zh**：字段配置页与创建页文案遵循现有 i18n 约束。

## 4. 方案概览（冻结）

### 4.1 数据与契约

采用“字段目录 + 字段策略 + 现有扩展字段映射”三层模型：

1. **字段目录（catalog）**：统一维护可配置字段清单（CORE/EXT）。
   - CORE 示例：`org_code`、`name`、`status`、`parent_org_code`、`manager_pernr`、`is_business_unit`。
   - EXT 示例：现有 `short_name`、`description`、`d_*`、`x_*` 等。
2. **扩展字段映射（existing）**：保留 `orgunit.tenant_field_configs` 作为 ext 物理槽位映射事实源（`physical_col` 等）。
3. **字段策略（new）**：新增租户策略表（建议：`orgunit.tenant_field_policies`）用于维护行为配置：
   - `maintainable boolean NOT NULL DEFAULT true`
   - `default_mode text NOT NULL DEFAULT 'NONE'`（`NONE|CEL`）
   - `default_rule_expr text NULL`
   - `scope_type text NOT NULL`（`GLOBAL|FORM`）
   - `scope_key text NOT NULL`（如 `orgunit.create_dialog`）
   - `enabled_on/disabled_on`（day 粒度，保持时间语义一致）
   - 审计字段（`created_at/updated_at`）与幂等字段（`request_code`）按现有口径落地

并扩展 API（命名可在 Phase 0 冻结）：

- `GET /org/api/org-units/field-configs`：返回“统一字段列表（CORE+EXT）+ 生效策略（含 scope）”。
- `POST /org/api/org-units/field-policies`：新增/更新字段策略（含 `maintainable/default_mode/default_rule_expr/scope`）。
- `POST /org/api/org-units/field-policies:disable`：停用策略（保留审计链）。

### 4.2 CEL 运行时

新增轻量运行时组件（Go + cel-go）：

1. **保存时编译**：配置规则时 parse/check，失败直接拒绝保存。
2. **执行时求值**：创建请求进入写服务后，针对缺失字段执行默认规则。
3. **安全限制**：白名单变量、白名单函数、成本上限、超时上限。
4. **错误映射**：统一映射为业务可解释错误码（BadRequest/Conflict/Exhausted）。

### 4.3 首个函数能力（为里程碑服务）

首期冻结一个内置函数：

- `next_org_code(prefix, width)`：返回租户内最小未占用编码。
- 示例：`next_org_code("O", 6)` -> `O000001`。

实现要求：

1. 在事务内执行，复用租户写锁语义（避免并发重复分配）。
2. 保持唯一约束兜底（`org_unit_codes` 唯一键仍为最终防线）。
3. 溢出时返回明确错误（如 `ORG_CODE_EXHAUSTED`）。

### 4.4 默认值触发环节（冻结）

1. **保存规则时不计算**：字段配置页提交默认值规则时，仅做 CEL 编译/校验（语法、类型、白名单），不执行实际求值。
2. **仅在写入时计算**：默认值只在写服务处理写请求时执行，且执行位置固定在 `submit_org_event(...)` 之前。
3. **场景白名单**：首期仅 `create_org` 触发默认值计算；`add_version` / `insert_version` / `correct` 不触发。
4. **缺失才补全**：目标字段在请求中缺失或为空时才计算；若用户已显式输入，默认值规则不得覆盖用户输入。
5. **后端权威**：前端可提示“将自动生成”，但最终结果以后端计算为准；不得在前端单独实现一套分配逻辑。
6. **幂等与并发一致性**：同一 `request_code` 重试应返回同一业务结果；并发场景通过事务锁与唯一约束确保不重复分配。

### 4.5 “可维护”开关语义（冻结）

1. 字段配置页新增“可维护”列，管理员可编辑（是/否）。
2. 当 `maintainable=false` 时，该字段在业务写入表单必须禁用编辑控件（只读或隐藏输入）。
3. 后端需 fail-closed：即使客户端强行提交该字段值，也不得按用户输入写入。
4. `maintainable=false` 的字段按系统默认值链路处理：有默认值则按规则计算；无默认值则返回可解释错误（提示缺少系统默认值配置）。
5. 首期范围：先覆盖 `create_org` 场景，后续再扩展到其他 intent。

### 4.6 同字段跨表单可配置（冻结）

1. **作用域模型**：
   - `GLOBAL`：字段全局默认策略（兜底）。
   - `FORM`：特定表单策略（覆盖全局）。
2. **scope_key 冻结为稳定枚举**（首期建议）：
   - `orgunit.create_dialog`
   - `orgunit.details.add_version_dialog`
   - `orgunit.details.insert_version_dialog`
   - `orgunit.details.correct_dialog`
3. **策略解析优先级**（高 -> 低）：
   - 精确命中的 `FORM(scope_key=当前表单)`
   - `GLOBAL`
   - 系统默认（`maintainable=true`、`default_mode=NONE`）
4. **后端权威解析**：当前表单上下文由后端根据路由/intent 推导，不依赖前端自由传值，防止绕过。
5. **同构能力**：若同一字段在多个表单要一致行为，可只配一条 `GLOBAL`；若要差异行为，再追加对应 `FORM` 规则覆盖。

## 5. 分阶段实施路线图

### Phase 0：契约冻结与风险收口

1. [ ] 冻结默认值配置模型（字段、API、错误码、权限口径）。
2. [ ] 冻结 `org_code` 场景边界：仅 `create_org` 生效，其他 intent 暂不生效。
3. [ ] 冻结字段目录（CORE/EXT）与 `scope_type/scope_key` 枚举，明确跨表单优先级。
4. [ ] 在 `DEV-PLAN-120` 之外如需调整 `DEV-PLAN-101/108/109`，先补文档再改代码。

### Phase 1：Schema + Store + API 扩展

1. [ ] 迁移：新增 `tenant_field_policies`（含 `maintainable/default_mode/default_rule_expr/scope`）与事件审计表。
2. [ ] 保持 `tenant_field_configs` 作为 ext 映射事实源，不迁移既有 `physical_col` 语义。
3. [ ] Store 层扩展：支持按 `(field_key, scope)` 读写策略；支持统一列表（CORE+EXT）。
4. [ ] API 扩展：字段配置列表返回统一字段与策略；新增策略 upsert/disable 接口。
5. [ ] 路由与权限门禁对齐（`orgunit.admin`）。

### Phase 2：CEL 引擎最小闭环

1. [ ] 引入 cel-go 依赖，落地编译/求值器与缓存策略。
2. [ ] 实现表达式校验（变量白名单、函数白名单、成本限制）。
3. [ ] 实现 `next_org_code("O", 6)` 并完成并发冲突防护。
4. [ ] 实现“作用域策略解析器”（FORM > GLOBAL > 系统默认）。
5. [ ] 错误映射统一到 API 可消费的结构化错误。

### Phase 3（首个业务目标）：UI + 创建链路打通

1. [ ] 字段配置页新增“默认值”列与“编辑默认值”入口。
2. [ ] 字段配置页新增“可维护”列与开关编辑入口（是/否）。
3. [ ] 字段配置页增加 scope 维度编辑能力（GLOBAL/FORM + scope_key）。
4. [ ] 新建部门弹窗：当 `org_code` 留空时可显示“将按规则自动生成”的提示；当字段 `maintainable=false` 时输入控件禁用。
5. [ ] 写服务：`create_org` 时按 `scope=orgunit.create_dialog` 解析策略并执行；当 `maintainable=false` 时忽略/拒绝用户输入并走系统默认值链路。
6. [ ] 成功后在返回结果中明确回显最终 `org_code`。

### Phase 4：验证与门禁

1. [ ] 单测：规则编译失败、运行失败、并发冲突、溢出场景。
2. [ ] 集成测试：field-config 配置 -> create_org 自动编码 -> 列表/详情可见。
3. [ ] E2E：管理员配置规则后，业务用户新建部门无需手填编码。
4. [ ] 质量门禁：按触发器执行 `make check lint && make test && make check routing && make check doc`。

## 6. 首个里程碑（M1）验收标准

M1 完成判定（全部满足）：

1. 字段配置页列表出现“默认值”列，能显示 `CEL: next_org_code("O", 6)` 摘要。
2. 管理员可为 `org_code` 配置/更新默认规则（仅 `orgunit.admin`）。
3. 新建部门时若 `org_code` 未填，后端自动生成 `O+6位` 最小可用编码。
4. 同并发创建下不出现重复编码；冲突可重试且错误可解释。
5. API/前端均无 legacy 回退逻辑，且测试与门禁通过。
6. 字段配置页可编辑“可维护”开关；当 `org_code` 被设为“可维护=否”时，新建表单不可编辑该字段，提交结果以系统默认值为准。
7. 字段配置页可展示与配置核心字段（包含 `org_code`），不再仅限扩展字段。
8. 同一字段可实现“全局一致”或“按表单差异化”配置，且策略优先级可解释、可验证。

## 7. 风险与应对

1. **并发分配冲突**：通过事务锁 + 唯一索引双保险；必要时失败后一次重试。
2. **规则复杂度失控**：首期仅开放极少函数与变量，限制表达式成本。
3. **语义漂移**：默认值仅用于“缺失字段补全”，不覆盖用户显式输入。
4. **配置误用**：保存时即编译校验，避免运行时才暴露错误。

## 8. 关联文档

- `docs/dev-records/Go+PG+CEL 规则引擎架构升级.md`
- `docs/dev-plans/101-orgunit-field-config-management-ui-ia.md`
- `docs/dev-plans/108-org-crud-ui-actions-consolidation-and-unified-field-mutation-rules.md`
- `docs/dev-plans/109-request-code-unification-and-gate.md`
- `docs/dev-plans/100-org-metadata-wide-table-implementation-roadmap.md`
- `docs/dev-plans/012-ci-quality-gates.md`
- `AGENTS.md`

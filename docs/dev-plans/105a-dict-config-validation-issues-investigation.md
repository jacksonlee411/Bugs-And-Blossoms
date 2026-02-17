# DEV-PLAN-105A：字典配置模块验证问题调查与修复方案（承接 DEV-PLAN-105）

**状态**: 草拟中（2026-02-17 02:07 UTC）

> 本计划用于记录 DEV-PLAN-105 已落地后，在字典模块验证中发现的偏差/缺陷，并冻结调查结论、修复路径与验收口径。  
> 主方案与冻结条款见：`docs/dev-plans/105-dict-config-platform-module.md`；实施证据见：`docs/dev-records/dev-plan-105-execution-log.md`。

## 1. 背景

在本地验证页面 `http://localhost:8080/app/dicts`（字典配置模块 UI）时，发现以下问题：

1. 页面布局未按 DEV-PLAN-105 的 UI 冻结落地（应对齐 Org 模块：左侧字典列表，右侧为值列表 + 详情/变更日志）。
2. 无法增加新的“字典字段”（用户已确认：这里指新增 `dict_code`）。
3. 在 `Values (click a row to select)` 区域点击行会触发运行时错误：
   - `Cannot read properties of undefined (reading 'trim')`

本计划目标是：把上述现象“变成可定位、可复现、可验收”的修复清单，避免继续以“手工记忆/口头描述”推进。

## 2. 术语澄清（避免需求漂移）

- **字典（Dict）**：`dict_code`（例如 `org_type`）。
- **字典值（Value）**：某个 `dict_code` 下的 `(code, label, enabled_on/disabled_on, status)`。
- “新增字典字段”在本计划中已收口为：**新增字典本体（dict_code）**（非新增 value）。

## 3. 问题记录（现象 / 期望 / 偏差）

### 3.1 问题 A：页面布局未对齐 DEV-PLAN-105 冻结 IA

- **现象**：`/app/dicts` 当前为单列纵向堆叠（Context -> Values -> Create/Disable/Correct -> Audit），缺少左侧 Dict List 与右侧 Detail 区域组织。
- **期望**（DEV-PLAN-105 5.2）：左侧字典列表；右侧上半为 Value Grid；右侧下半为 Detail（基本信息 + 生效窗口 + 变更记录）。
- **影响**：可发现性与可操作性下降；后续扩展到多 dict_code 时不可扩展；与“对齐 Org 模块”口径漂移。

### 3.2 问题 B：无法新增“字典字段”（需求/行为需确认）

- **现象**：用户反馈“无法增加新的字典字段”。
- **用户确认（2026-02-17）**：这里指 **新增一个新的 `dict_code`（字典本体）**，并在 UI 左侧可见（即 B2）。
- **期望**：
  - 需要单独冻结“dict_code 生命周期（创建/停用/展示名）+ allowlist/权限/迁移/默认值注入”策略；否则默认不支持（避免运行态任意注入 dict_code，破坏可治理性）。

### 3.3 问题 C：点击 Values 行触发 `trim` 运行时崩溃

- **复现步骤**：
  1. 打开 `http://localhost:8080/app/dicts`
  2. 在 `Values (click a row to select)` 表格点击任意一行
  3. 页面抛出错误：`Cannot read properties of undefined (reading 'trim')`
- **期望**：点击行只应更新“当前选中 value”，并触发 Audit 区域按选中项加载（或显示空态），不应导致页面崩溃。
- **影响**：阻断核心闭环（选择 -> 查看审计 -> disable/correct）。

## 4. 初步定位（基于代码阅读的可验证假设）

> 说明：以下为“可被代码/运行验证”的假设，不等同于最终结论；最终结论需在本计划执行项中以证据收口。

### 4.1 问题 C 的高概率根因：后端 JSON 字段名与前端约定不一致

- 前端期望 values item shape 为 snake_case：`dict_code/code/label/...`（见 `apps/web/src/api/dicts.ts`）。
- 后端响应 `dictValuesResponse` 的顶层字段使用了 json tag（`dict_code/as_of/values`），但 values 数组元素类型 `internal/server.DictValueItem`（以及 dict list 的 `internal/server.DictItem`）未标注 json tag。
- Go 默认 JSON 序列化会输出 `DictCode/Code/Label/...`（驼峰首字母大写），导致前端读取 `v.code` 得到 `undefined`，点击行后 `setSelectedValueCode(v.code)` 将 state 置为 `undefined`，从而触发 `selectedValueCode.trim()` 崩溃（见 `apps/web/src/pages/dicts/DictConfigsPage.tsx`）。

**结论形式（待验证）**：这是一个“契约字段命名不一致导致的前端运行时崩溃”，修复应优先从“后端 JSON tag 对齐”入手，而不是在前端到处加 `?? ''` 兜底（兜底只能作为第二道防线）。

## 5. 修复方案（冻结执行清单）

### 5.1 P0：修复运行时崩溃（问题 C）

1. [ ] 后端：为 `internal/server.DictItem` 与 `internal/server.DictValueItem` 补齐 json tag（snake_case），确保 API 输出字段与前端一致。
2. [ ] 前端：将 `selectedValueCode` 的来源做最小防御性约束（例如仅在 `typeof v.code === 'string' && v.code.trim() !== ''` 时允许选中），并对 audit query 的 `enabled` 条件避免因异常值崩溃。
3. [ ] UI：为 `/app/dicts` 路由补齐 `errorElement` 或全局 ErrorBoundary（对齐 React Router 提示），避免单点异常导致整页白屏。

### 5.2 P1：对齐页面 IA（问题 A）

1. [ ] UI 结构改造为“左侧列表 + 右侧两段”（参考 Org 详情页的 audit 双栏组织方式）：
   - 左：Dict List（Phase 0 仅 `org_type`，但结构必须可扩展）
   - 右上：Value Grid（含 as_of/q/status/limit 语义）
   - 右下：Detail + Audit（选中 value 时显示，否则空态）
2. [ ] 交互收口：点击 value 行后，右下 detail 与 audit 与之联动；disable/correct 操作在 detail 区域完成，并可在 audit 中看到 tx_time 记录。

### 5.3 P1：澄清并收口“新增字典字段”的真实需求（问题 B）

1. [X] 明确用户意图：本问题为 B2（新增 dict_code）。
2. [X] 新增 `DEV-PLAN-105B` 冻结 dict_code 注册与治理策略，避免绕开 allowlist/权限/迁移规则。
3. [ ] 对应补齐验收用例（手测步骤 + 最小自动化覆盖，如 handler/store 层测试或 e2e）。

## 6. 验收标准（DoD）

1. 点击 `/app/dicts` 的任意 value 行不再崩溃；选中项高亮，Audit 区域可加载并展示事件（无事件则空态）。
2. API `GET /iam/api/dicts/values` 返回的 values item 字段名与前端约定一致（snake_case），前端不依赖隐式字段映射。
3. 页面布局对齐 DEV-PLAN-105 的 IA 冻结：左侧 dict 列表；右侧 value grid + detail/audit。
4. “新增字典字段”需求已明确收口为“新增 dict_code（B2）”，并在文档中冻结后续计划入口（`DEV-PLAN-105B`）。

## 7. 门禁与验证（SSOT 引用）

- 命令入口与触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准；本计划预计命中：
  - Go/后端变更：`go fmt ./... && go vet ./... && make check lint && make test`
  - 前端/UI 资源：`make generate && make css`（并确保 `git status --short` 为空）
  - 文档新增：`make check doc`

## 8. 关联文件（便于落点）

- UI：`apps/web/src/pages/dicts/DictConfigsPage.tsx`
- API client：`apps/web/src/api/dicts.ts`
- 后端 API：`internal/server/dicts_api.go`
- 后端 store/model：`internal/server/dicts_store.go`

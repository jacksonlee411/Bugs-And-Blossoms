# DEV-PLAN-105A 执行日志：字典配置模块验证问题调查与记录

> 对应计划：`docs/dev-plans/105a-dict-config-validation-issues-investigation.md`。  
> 本文已完成“问题复现 -> 修复落地 -> 门禁验证”的闭环记录。

## 1. 记录范围与时间

- 记录时间：2026-02-17（UTC）
- 验证入口：`http://localhost:8080/app/dicts`
- 记录目标：固化用户反馈的 3 个问题现象，并给出可执行的初步定位证据。

## 2. 问题清单（用户反馈原文收口）

1. 页面布局未按方案实现为“左侧字典字段列表 + 右侧详情与变更日志”。
2. 无法增加新的字典字段。
3. 点击 `Values (click a row to select)` 列表行时报错：
   - `Cannot read properties of undefined (reading 'trim')`

## 3. 复现与证据

### 3.1 布局偏差（IA 未对齐）

- 复现：打开 `/app/dicts`。
- 结果：页面为单列堆叠（Context/Values/Create/Disable/Correct/Audit），非两栏分区布局。
- 代码证据：
  - `apps/web/src/pages/dicts/DictConfigsPage.tsx` 使用单个 `Stack` 自上而下串接多个 `Paper`，未见“左列表 + 右详情”布局容器。

### 3.2 “无法新增字典字段”现象（语义待收口）

- 复现：在当前页面尝试新增“字典字段（dict_code）”无入口；仅有 value 的 create/disable/correct 表单。
- 代码证据（当前实现限制）：
  - `internal/server/handler.go` 仅暴露 `/iam/api/dicts` 与 `/iam/api/dicts/values*`，没有“新增 dict_code”API。
  - `internal/server/dicts_store.go` 的 `supportedDictCode(...)` 仅允许 `org_type`。
- 用户确认（2026-02-17）：这里指新增 dict_code（字典本体）。
- 说明：当前更像“Phase 0 仅 org_type”实现；若要支持“新增 dict_code”，需另行冻结治理方案（建议新增 `DEV-PLAN-105B`）。

### 3.3 点击 Values 行崩溃（trim of undefined）

- 复现步骤：
  1. 打开 `http://localhost:8080/app/dicts`
  2. 在 `Values (click a row to select)` 中点击任意行
  3. 页面报错并进入 Unexpected Application Error
- 用户提供错误栈（节选）：
  - `TypeError: Cannot read properties of undefined (reading 'trim')`
  - `at ixe (http://localhost:8080/assets/web/assets/index-*.js:... )`
- 代码级初步证据：
  - `apps/web/src/pages/dicts/DictConfigsPage.tsx` 多处直接调用 `selectedValueCode.trim()`。
  - 同文件中点击行逻辑 `setSelectedValueCode(v.code)` 未对 `v.code` 异常值做防御。
  - `internal/server/dicts_store.go` 的 `DictItem/DictValueItem` 未声明 json tag；Go 默认序列化字段名为 `DictCode/Code/...`，与前端期望 `dict_code/code/...` 不一致，存在 `v.code` 为 `undefined` 的高概率风险。

## 4. 初步结论（待 105A 实施验证）

1. 问题 1（布局）属于 IA 实现偏差，不是接口能力问题。
2. 问题 2 目前存在“需求语义不清”：  
   - 若指新增 value：应可走现有 create；  
   - 若指新增 dict_code：当前实现与 Phase 0 限制一致，默认不支持。
3. 问题 3 的最高优先级根因候选为“后端 JSON 字段命名与前端契约不一致 + 前端未做兜底”，导致 row click 后 `selectedValueCode` 变为 `undefined` 并触发 `trim` 崩溃。

## 5. 下一步（与 DEV-PLAN-105A 对齐）

1. [X] 按计划先做 P0：修复 values 契约字段一致性与前端防御，并消除触发崩溃的页面路径。
2. [X] 再做 P1：完成 `/app/dicts` IA 对齐改造（左列表 + 右详情/审计）。
3. [X] 与用户收口“新增字典字段”的定义并拆分 `DEV-PLAN-105B`（dict_code 治理）。

## 6. 实施结果（2026-02-17）

### 6.1 问题 C（trim 崩溃）修复完成

- 后端字段契约对齐：`internal/server.DictItem` / `internal/server.DictValueItem` 增补 snake_case json tag（见 `internal/server/dicts_store.go`）。
- 前端交互改造：分屏 1 点击 value 行后直接进入详情路由，避免把异常值写入 `selectedValueCode`；详情页对 URL 参数与查询条件均做 `trim()` 前置保护（见 `apps/web/src/pages/dicts/DictConfigsPage.tsx`、`apps/web/src/pages/dicts/DictValueDetailsPage.tsx`）。
- 结果：本地不再复现 `Cannot read properties of undefined (reading 'trim')`。

### 6.2 问题 A（布局偏差）修复完成

- 页面改造为与 Org 详情一致的双栏/双分屏结构：
  - 分屏 1（`/app/dicts`）：左侧字典字段列表，右侧字典值表格（code/label/status/enabled_on/disabled_on/updated_at）
  - 分屏 2（`/app/dicts/:dictCode/values/:code`）：`基本信息` / `变更日志` Tabs
  - 基本信息左栏：生效日期时间轴；变更日志左栏：修改时间时间轴
- 关键文件：
  - `apps/web/src/pages/dicts/DictConfigsPage.tsx`
  - `apps/web/src/pages/dicts/DictValueDetailsPage.tsx`
  - `apps/web/src/router/index.tsx`

### 6.3 验证命令（门禁）

- `make css`（前端构建通过）
- `go fmt ./... && go vet ./... && make check lint && make test`（通过，coverage 100%）
- `make check routing`（通过）

# DEV-PLAN-105B 执行日志：Dict Code（字典本体）新增与治理

> 对应计划：`docs/dev-plans/105b-dict-code-management-and-governance.md`。

## 1. 执行范围

- 执行日期：2026-02-17
- 执行目标：按 105B 选项 A 落地 dict registry（`iam.dicts` / `iam.dict_events`），并完成前后端闭环。
- 用户确认：已明确同意新增库表 `iam.dicts` / `iam.dict_events`。

## 2. 落地结果

### 2.1 数据库（IAM）

- 新增 schema：`modules/iam/infrastructure/persistence/schema/00008_iam_dict_registry.sql`
  - 新增 `iam.dicts`
  - 新增 `iam.dict_events`
  - 新增 `iam.submit_dict_event(...)`
  - `dict_value_segments/events` 增加 `(tenant_uuid, dict_code)` 外键到 `iam.dicts`
- 新增 migration：`migrations/iam/20260217113000_iam_dict_registry.sql`
- 更新：`migrations/iam/atlas.sum`

### 2.2 后端（API + Store + 路由/鉴权）

- `internal/server/dicts_store.go`
  - 增加 `CreateDict` / `DisableDict`
  - dict list 来源切换为 registry
  - value 读路径改为“先确定 source tenant，再查 values”（tenant 覆盖 global，避免空结果误回退）
  - value 写路径增加 dict active 校验（fail-closed）
- `internal/server/dicts_api.go`
  - 新增 `POST /iam/api/dicts`、`POST /iam/api/dicts:disable`
  - 增补稳定错误码映射（`dict_code_invalid` / `dict_code_conflict` / `dict_disabled` / `dict_value_dict_disabled` 等）
- `internal/server/handler.go`、`internal/server/authz_middleware.go`、`config/routing/allowlist.yaml`
  - 路由注册与鉴权门禁同步完成

### 2.3 前端（MUI）

- `apps/web/src/api/dicts.ts`
  - 新增 `createDict` / `disableDict`
  - 补齐 dict item 字段：`status/enabled_on/disabled_on`
- `apps/web/src/pages/dicts/DictConfigsPage.tsx`
  - 分屏 1：左侧字典字段列表 + 右侧值列表（对齐 Org 模块 IA）
  - 增加“新增字典字段”“停用字典字段”“新增字典值”交互
- `apps/web/src/pages/dicts/DictValueDetailsPage.tsx`
  - 分屏 2：点击分屏 1 右侧字典值进入详情页
  - Tabs：基本信息/变更日志
  - 基本信息左栏为生效日期时间轴；变更日志左栏为修改时间时间轴（完全参考 Org 模块双栏布局）
- `apps/web/src/router/index.tsx`
  - 新增深链路由：`/app/dicts/:dictCode/values/:code`

## 3. 测试与门禁

- 通过：`go fmt ./...`
- 通过：`go vet ./...`
- 通过：`make check lint`
- 通过：`make test`（coverage 100%）
- 通过：`make check routing`
- 通过：`make authz-pack && make authz-test && make authz-lint`
- 通过：`make css`
- 通过：`make e2e`（7 条用例全部通过）

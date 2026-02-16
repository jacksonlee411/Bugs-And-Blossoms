# DEV-PLAN-105 执行日志：全模块字典配置模块（DICT 值配置 + 生效日期 + 变更记录）

> 本记录用于固化 DEV-PLAN-105 的实施证据与口径收敛点；计划本体见：`docs/dev-plans/105-dict-config-platform-module.md`。

## 1. 冻结口径（用户确认）

- 不存在旧值自动映射需求：不做“旧 code -> 新 code”自动映射；发现异常值走人工修复清单（fail-closed）。
- `as_of` 必填：所有字典读取接口必须显式 `as_of=YYYY-MM-DD`，避免隐式“今天”。
- 字典值具备“生效日期（Valid Time/day）+ 审计/变更记录（Tx Time）”。
- 预配置字段可配置字典关联：字段元数据里的 `data_source_config` 以 `dict_code` 绑定字典（并受 allowlist 约束）。

## 2. 实施完成内容（落地）

### 2.1 数据库（IAM）

- 新增字典值投射表：`iam.dict_value_segments`（Valid Time：`enabled_on/disabled_on`，半开区间 `[enabled_on, disabled_on)`；并通过 exclusion constraint 阻止窗口重叠）。
- 新增字典值事件表：`iam.dict_value_events`（Audit/Tx Time：`tx_time`，含 before/after 快照）。
- 新增 One Door 写入口：`iam.submit_dict_value_event(...)`（同 request_code 幂等；不同 payload 复用 request_code 冲突拒绝）。
- 默认样板 dict：`org_type`，默认值 `10/20`（部门/单位），写入 global tenant + local tenant 1。

对应文件：
- `migrations/iam/20260216165000_iam_dict_config.sql`
- `modules/iam/infrastructure/persistence/schema/00007_iam_dict_config.sql`
- `internal/sqlc/schema.sql`（同步）

### 2.2 后端（Internal API + Store）

新增/接入接口（均要求租户上下文，fail-closed）：
- `GET /iam/api/dicts?as_of=...`
- `GET /iam/api/dicts/values?dict_code=...&as_of=...&q=...&limit=...&status=...`
- `POST /iam/api/dicts/values`
- `POST /iam/api/dicts/values:disable`
- `POST /iam/api/dicts/values:correct`
- `GET /iam/api/dicts/values/audit?dict_code=...&code=...&limit=...`

实现文件（核心）：
- `internal/server/dicts_api.go`
- `internal/server/dicts_store.go`
- `pkg/dict/dict.go`（跨模块门面：Resolver 注册 + Resolve/List）
- `internal/server/handler.go`（路由注册 + resolver 注入）

### 2.3 Org 接入（去静态 registry，改为 dict 模块 SSOT）

- 字段定义（预配置）使用 `DataSourceType=DICT` + `DataSourceConfig={"dict_code":"org_type"}` 绑定字典：
  - `modules/orgunit/domain/fieldmeta/fieldmeta.go`
- 字段配置 enable 时对 `data_source_config` 做 allowlist 校验（避免任意 dict_code 注入）：
  - `internal/server/orgunit_field_metadata_api.go`
- `fields:options` 的 DICT 分支改为调用 `pkg/dict.ListOptions(...)`：
  - `internal/server/orgunit_field_metadata_api.go`
- Org 写入侧 DICT fail-closed 校验并写 `ext_labels_snapshot`：
  - `modules/orgunit/services/orgunit_write_service.go`

### 2.4 前端（MUI SPA）

- 新增页面：`/app/dicts`（导航可见、可操作）。
- 页面能力：左侧上下文（as_of/dict_code/q）+ 值列表 + create/disable/correct + audit 列表。

对应文件：
- `apps/web/src/pages/dicts/DictConfigsPage.tsx`
- `apps/web/src/api/dicts.ts`
- `apps/web/src/router/index.tsx`
- `apps/web/src/navigation/config.tsx`
- `apps/web/src/i18n/messages.ts`

### 2.5 路由与权限（治理 + 门禁）

- 路由 allowlist：
  - UI：`/app/dicts`
  - Internal API：`/iam/api/dicts*`
  - `config/routing/allowlist.yaml`
- Casbin：
  - object：`iam.dicts`
  - actions：`read|admin`
  - `config/access/policy.csv` + `config/access/policies/00-bootstrap.csv`
  - `pkg/authz/registry.go`（集中注册 object 常量）
- UI 权限：
  - 页面访问使用 `permissionKey='dict.admin'`（`apps/web/src/router/index.tsx`）

## 3. 验收与门禁证据（本地）

- `make test`：通过（总覆盖率门禁 100% OK）。
- `make authz-pack && make authz-test && make authz-lint`：通过。
- `make check routing`：通过。
- `make generate && make css`：通过（本仓库 generate 为 placeholder；css 会触发 web build 并同步 assets）。
- `make e2e`：通过（Playwright 7 条用例全部通过）。
- `make preflight`：通过（对齐 CI 一键门禁）。


# DEV-PLAN-487：EHR 角色定义持久化与保存 API 方案

**状态**: 已实施后端闭环、角色定义 UI 保存接入，并完成 480A 组合运行时与 A/B E2E 验收（2026-05-02 CST）

## 0. 适用范围与评审分级

- **评审分级**：`T2`
- **范围一句话**：冻结角色定义的在线保存 API、持久化模型、服务端校验与运行时能力授权生效路径；承接 `DEV-PLAN-481` 留出的角色定义后端闭环，不扩展到角色复制、组织范围、字段策略或用户授权分配。
- **关联模块/目录**：`pkg/authz/**`、`modules/iam/domain/**`、`modules/iam/services/**`、`modules/iam/infrastructure/**`、`modules/iam/infrastructure/persistence/schema/**`、`internal/server/**`、`config/access/**`、`scripts/authz/**`、`apps/web/src/**`
- **关联计划/标准**：`AGENTS.md`、`DEV-PLAN-000`、`DEV-PLAN-001`、`DEV-PLAN-012`、`DEV-PLAN-017`、`DEV-PLAN-019`、`DEV-PLAN-022`、`DEV-PLAN-024`、`DEV-PLAN-025`、`DEV-PLAN-032`、`DEV-PLAN-300`、`DEV-PLAN-304`、`DEV-PLAN-480`、`DEV-PLAN-480A`、`DEV-PLAN-481`、`DEV-PLAN-482`、`DEV-PLAN-483`、`DEV-PLAN-484`、`DEV-PLAN-489`、`DEV-PLAN-489A`
- **用户入口/触点**：`系统管理 > 角色管理 > 新建/编辑角色`、角色列表、角色详情、角色定义保存按钮、角色 authz capability 候选项 options API

### 0.1 Simple > Easy 三问

1. **边界**：487 只拥有角色定义后端闭环。角色定义回答“这个角色能做什么”；用户授权、组织范围、字段安全、有效期、冲突检测、复制角色和审计解释 UI 不进入本计划。
2. **不变量**：角色定义保存 payload 只能提交基础信息、`role_slug`、`revision` 与 canonical `authz_capability_keys`；服务端必须用 482/483/484 契约校验 authz capability key，保存成功后运行时授权必须能按该角色能力集合生效。
3. **可解释**：管理员在 481 页面保存角色后，DB 中有唯一角色定义和能力集合；后续请求由 `DEV-PLAN-489A` 从 principal 已分配角色集合读取这些角色定义并做 capability union。同一 tenant 角色不得同时依赖 DB 与 policy CSV 两条授权来源做 OR 判定；普通 tenant role cutover 是硬切换，不是兼容层。

### 0.2 当前缺口

`DEV-PLAN-481` 已冻结角色定义 UI 和边界，但明确把保存语义留给后续实现计划。当前仓库仍存在以下缺口：

1. `iam.principals.role_slug` 只是文本字段，没有对应在线角色定义 SoT。
2. `config/access/policies/**` 与 `config/access/policy.csv` 能表达静态 bootstrap policy，但不能承接租户管理员在线保存角色。
3. `pkg/authz.Authorizer` 当前从文件 policy 加载 Casbin 策略；角色定义保存成功后不会天然进入运行时授权判断。
4. 482/483/484 已分别冻结 authz capability registry、canonical key 与覆盖门禁，但不拥有角色基础信息、保存 API 或角色能力集合持久化。
5. `DEV-PLAN-489` 拥有用户授权组织范围 SoT，不拥有角色定义本身的保存模型；487 与 489 必须通过 `role_slug` 和 authz capability `scope_dimension` 对接，不能互相吞并职责。

## 1. 背景与上下文

EHR 角色管理首期需要可操作闭环：管理员能在角色管理页创建或编辑角色，选择功能权限并保存。保存不是前端本地状态，也不是修改 `config/access/policy.csv`；它必须进入租户内的角色定义事实源，并在后续 API 授权判断中生效。

因此需要把三件事分开：

| 事项 | Owner | 说明 |
| --- | --- | --- |
| 角色定义 UI | `DEV-PLAN-481` | 表单、功能权限矩阵、取消/保存、新建/编辑边界 |
| capability 候选和 key 校验基础 | `DEV-PLAN-482/483/484` | registry、canonical `object:action`、覆盖门禁 |
| 角色定义保存与运行时生效 | `DEV-PLAN-487` | DB SoT、保存 API、服务端校验、授权运行时读取 |
| 用户授权与组织范围 | `DEV-PLAN-489` | 把角色授予 principal，并保存组织范围与 orgunit 运行时裁剪 |

## 2. 目标与非目标

### 2.1 核心目标

1. [X] 冻结角色定义持久化模型，使每个 tenant 内 `role_slug` 唯一、能力集合可追溯、更新具备修订号。
2. [X] 冻结角色定义保存 API：列表、详情、新建、编辑；保存即生效，不提供“保存并发布”。
3. [X] 冻结服务端校验：基础信息、slug、修订号、authz capability key、重复 key、未知 key、禁用/不可分配/无覆盖 key、复制语义字段。
4. [X] 冻结运行时生效路径：普通 tenant role 的能力授权以角色定义 DB SoT 为准；不得通过 DB 与 policy CSV 双链路 OR 放行。
5. [X] 定义迁移、seed、sqlc、authz lint、测试与 E2E 验收切片；本轮未命中 sqlc，E2E 已随 481/489 闭环补齐。

### 2.2 非目标

1. 不实现角色复制、克隆、从已有角色新建，也不接受 `clone_from`、`source_role_slug`、`copy_permissions_from` 等 payload 字段。
2. 不在角色定义里保存组织范围、`scope_required`、字段策略、有效期、冲突检测或策略预览。
3. 不实现用户授权/角色分配表、principal 多角色 union 运行时、团队/岗位授权或组织范围 SoT；这些分别属于 `DEV-PLAN-489A` 与 `DEV-PLAN-489`。
4. 不引入 Casbin `g/g2`、role 继承、角色组或第二套 policy engine。
5. 不把 authz capability registry DB 化；registry 与 options API 仍由 `DEV-PLAN-482` 承接。
6. 不新增角色删除、禁用、导入、导出、审计解释页面；首期只冻结新建/编辑保存。
7. 不恢复 legacy、SetID、scope/package、org_level/scope_type/scope_key 或旧权限 key 兼容。

### 2.3 用户可见性交付

- 管理员打开 `系统管理 > 角色管理`，能看到当前 tenant 的角色列表。
- 管理员新建 `flower-hr`，填写名称、标识、描述，选择 `orgunit.orgunits:read`、`orgunit.orgunits:admin`、`cubebox.conversations:use` 等 authz capability，并保存。
- 服务端返回保存后的 `role_slug`、`revision` 与 authz capability 集合；页面可继续编辑。
- 该角色被 `DEV-PLAN-489` 用户授权链路分配给 principal 后，运行时按 `DEV-PLAN-489A` 对 principal 的角色集合做 capability union。角色定义不包含组织范围；若后续用户授权缺少组织范围，仍由 489 保存链路拒绝。

## 2.4 工具链与门禁

- **命中触发器**：
  - [X] Go 代码
  - [X] DB Schema / Migration（实施前已获得用户手工确认）
  - [ ] sqlc（本轮未新增 sqlc query/config）
  - [X] Routing / allowlist / responder
  - [X] Authz（registry、route requirement、policy/bootstrap、lint）
  - [X] `apps/web/**` / presentation assets / 生成物（错误映射与 capability 常量命中）
  - [X] i18n（错误提示映射命中）
  - [X] E2E
  - [X] 文档 / readiness / 证据记录（本次新建 dev-plan 命中）

实施阶段命令入口以 `AGENTS.md`、`Makefile` 与 CI 为准。若进入迁移实现，必须按模块执行 Atlas/Goose、sqlc 与 schema verify 相关门禁。

## 2.5 测试设计与分层

| 层级 | 本计划承接内容 | 说明 |
| --- | --- | --- |
| `pkg/authz` | role slug 规范化、authz capability key 解析、角色能力校验 | 黑盒表驱动测试，覆盖未知 key、旧格式 key、重复 key、不可分配 key |
| `modules/iam/services` | 角色定义保存用例、修订号冲突、system-managed 保护 | 纯服务测试优先，避免把业务规则塞进 handler |
| `modules/iam/infrastructure` | role repository、schema/sqlc 查询 | 事务、tenant 隔离、唯一约束、FK/seed 行为 |
| `internal/server` | HTTP API、authz requirement、错误映射 | 400/403/404/409 响应契约与路由保护 |
| `apps/web/src/**` | 481 页面保存 API 消费 | UI 只测状态消费、校验错误展示和取消/保存交互 |
| `E2E` | 新建/编辑角色最小闭环 | 至少覆盖一条角色保存后重新打开可见 |

新增测试文件不得用 `*_coverage_test.go`、`*_gap_test.go`、`*_more_test.go`、`*_extra_test.go` 等补洞式命名。

## 3. 概念模型

### 3.1 Role Definition

角色定义是 tenant 内可复用的功能权限集合：

| 字段 | 含义 |
| --- | --- |
| `tenant_uuid` | 角色归属租户；superadmin/global 角色不进入本表首期编辑面 |
| `role_slug` | tenant 内稳定标识，不带 `role:` 前缀 |
| `name` | 管理员可见名称 |
| `description` | 可选说明 |
| `authz_capability_keys` | canonical `object:action` 授权项标识集合 |
| `system_managed` | 系统内置角色标记；首期在线 API 只读 |
| `revision` | 乐观并发修订号 |

角色定义不包含组织范围。组织范围由 489 的用户授权/角色分配事实表达。

### 3.2 Built-in Roles

首期内置 tenant 角色：

| role_slug | 用途 | 在线 API 行为 |
| --- | --- | --- |
| `tenant-admin` | 租户管理员 bootstrap 角色 | `system_managed=true`，首期只读 |
| `tenant-viewer` | 租户只读 bootstrap 角色 | `system_managed=true`，首期只读 |

`anonymous` 与 `superadmin` 不作为 tenant 角色定义在线编辑项。匿名与控制面授权继续由 bootstrap/static policy 或控制面专用机制承接。

## 4. 持久化模型

### 4.1 Schema 提案

实施迁移前必须再次获得用户手工确认。当前冻结的目标模型如下：

```sql
CREATE TABLE iam.role_definitions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_uuid uuid NOT NULL REFERENCES iam.tenants(id) ON DELETE CASCADE,
  role_slug text NOT NULL,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  system_managed boolean NOT NULL DEFAULT false,
  revision bigint NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_uuid, role_slug)
);

CREATE TABLE iam.role_authz_capabilities (
  role_id uuid NOT NULL REFERENCES iam.role_definitions(id) ON DELETE CASCADE,
  authz_capability_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (role_id, authz_capability_key)
);
```

约束要求：

1. `role_slug` 必须 trim、lowercase，且不得以 `role:` 开头。
2. `name` trim 后不能为空。
3. `revision >= 1`。
4. `authz_capability_key` 必须 trim、lowercase，且由服务层用 482 registry 校验存在性；不对静态 registry 建 DB FK。
5. `iam.principals(tenant_uuid, role_slug)` 应在 seed 完成后收敛为引用 `iam.role_definitions(tenant_uuid, role_slug)` 的关系，避免 dangling role slug。若实施时存在迁移顺序风险，必须在同一计划切片中说明 stopline，不得长期靠应用层补洞。

### 4.2 Seed 与迁移

迁移切片必须完成：

1. 为现有每个 tenant seed `tenant-admin` 与 `tenant-viewer`。
2. 将当前静态 policy 中普通 tenant role 的能力集合转入 `iam.role_authz_capabilities`。
3. 新 tenant 创建路径必须在同一事务内 seed 内置角色，避免 principal 创建时引用不存在 role。
4. 普通 tenant role 完成 DB SoT 切换后，`config/access/policies/**` 不再作为普通 tenant role 的运行时授权来源；保留的 static policy 只能覆盖匿名、控制面、bootstrap 或明确 system surface。

## 5. API 契约

### 5.1 Endpoint

| 方法 | 路径 | 用途 | Requirement |
| --- | --- | --- | --- |
| `GET` | `/iam/api/authz/roles` | 角色列表 | `iam.authz:read` |
| `GET` | `/iam/api/authz/roles/{role_slug}` | 角色详情 | `iam.authz:read` |
| `POST` | `/iam/api/authz/roles` | 新建角色 | `iam.authz:admin` |
| `PUT` | `/iam/api/authz/roles/{role_slug}` | 编辑角色 | `iam.authz:admin` |

这些 route 必须进入 484 覆盖门禁：route requirement、registry、policy/bootstrap 或 DB role seed、前端消费必须一致。

### 5.2 保存请求

新建：

```json
{
  "role_slug": "flower-hr",
  "name": "鲜花公司 HR",
  "description": "维护鲜花公司组织与基础配置",
  "authz_capability_keys": [
    "orgunit.orgunits:read",
    "orgunit.orgunits:admin",
    "cubebox.conversations:use"
  ]
}
```

编辑：

```json
{
  "revision": 3,
  "name": "鲜花公司 HR",
  "description": "维护鲜花公司组织与基础配置",
  "authz_capability_keys": [
    "orgunit.orgunits:read",
    "orgunit.orgunits:admin",
    "cubebox.conversations:use",
    "iam.dicts:read"
  ]
}
```

规则：

1. `role_slug` 只在新建时提交；编辑时以 path 为准，不支持 rename。
2. `revision` 编辑必填；不匹配返回 `409 stale_revision`。
3. 保存 payload 不接受未知字段；复制语义字段必须明确拒绝。
4. `authz_capability_keys` 必须非空；空角色不作为保存即生效角色交付。

### 5.3 响应

```json
{
  "role": {
    "role_slug": "flower-hr",
    "name": "鲜花公司 HR",
    "description": "维护鲜花公司组织与基础配置",
    "system_managed": false,
    "revision": 4,
    "authz_capability_keys": [
      "orgunit.orgunits:read",
      "orgunit.orgunits:admin",
      "cubebox.conversations:use",
      "iam.dicts:read"
    ]
  }
}
```

列表接口可返回同一 shape 的摘要版，但不得省略 `revision` 与 `system_managed`。

## 6. 服务端校验

### 6.1 基础信息

1. `role_slug` 规范：`^[a-z][a-z0-9-]{1,62}$`。
2. `role_slug` 不得为 `anonymous`、`superadmin`，不得带 `role:` 前缀。
3. `tenant-admin`、`tenant-viewer` 为首期 system-managed slug，不允许通过在线 API 修改。
4. `name` trim 后不能为空；`description` 可为空但必须归一化为字符串。

### 6.2 Capability 集合

角色保存必须复用 482/483/484 的校验口径：

1. key 格式必须是 canonical `object:action`。
2. key 必须存在于 registry。
3. entry 必须是 `status=enabled`、`assignable=true`、`surface=tenant_api`。
4. entry 必须具备当前 tenant API 覆盖证据。
5. 同一请求内重复 key 返回明确错误，不静默去重。
6. `scope_dimension=organization` 只影响 489 用户授权保存时的组织范围必填校验；角色定义保存不接收组织范围字段。

### 6.3 错误映射

| 场景 | HTTP | code |
| --- | --- | --- |
| payload 格式错误、未知字段、复制语义字段 | `400` | `invalid_role_payload` |
| slug/name/authz capability 校验失败 | `400` | `invalid_role_definition` |
| 未授权访问角色管理 API | `403` | `forbidden` |
| 角色不存在 | `404` | `role_not_found` |
| `role_slug` 已存在 | `409` | `role_slug_conflict` |
| `revision` 过期 | `409` | `stale_revision` |
| system-managed 角色被写入 | `409` | `system_role_readonly` |

错误提示必须走项目标准错误类型和 `make check error-message` 口径，禁止泛化失败文案直出。

## 7. 运行时授权生效

### 7.1 授权来源

普通 tenant role 的运行时能力授权以 DB role definition 为 SoT：

```text
principal assigned_role_slugs[] (DEV-PLAN-489A)
  -> iam.role_definitions(tenant_uuid, role_slug)
  -> iam.role_authz_capabilities(authz_capability_key)
  -> DISTINCT UNION(authz_capability_key)
  -> object/action decision
```

约束：

1. 不把角色保存 API 写入 `config/access/policy.csv`。
2. 不让普通 tenant role 同时从 DB 与 CSV policy 两处 OR 放行。
3. `config/access/policies/**` 可继续承接匿名、控制面、bootstrap 或 system surface，但普通 tenant role cutover 后不得作为在线角色定义的 fallback。
4. request-scope 或短 TTL 进程内缓存可以作为性能优化；缓存 miss 必须回源 PostgreSQL，不引入 Redis 等外部缓存。

硬切换要求：

1. 实施 PR 不得出现“先查 DB，不命中再查 policy CSV”的普通 tenant role OR 判定。
2. 若 cutover 前需要迁移验证，只能做离线对账或测试断言；合入后的 active runtime 必须只有一个普通 tenant role 授权来源。
3. `make authz-lint` 必须能区分 bootstrap/static/system surface policy 与普通 tenant role DB authz capability；普通 tenant role 在 CSV 中残留可放行 capability 时应失败。
4. 489 用户授权链路只能引用 487 的角色定义摘要和 `role_slug`；489A 只能通过 487 服务读取角色 capability 集合做 union，不得另建角色定义表或角色 authz capability 表。
5. P3 HTTP 写入 API 与 P4 运行时 cutover 必须形成同一可用闭环；不得合入“角色可保存到 DB，但普通请求仍按 CSV tenant policy 判定”的可调用管理入口。若工程上拆 PR，前置 PR 只能提交不可路由的 service/repository 或离线迁移对账，不能暴露在线保存入口。
6. 487 不得单独宣布“480 系列运行时授权完成”。运行时闭环必须与 489 的 `principal_role_assignments`、scope provider / orgunit 裁剪，以及 489A 的多角色 union 一起验收；否则只能声明角色定义 DB SoT 子能力完成。

### 7.2 与 Casbin 的关系

487 不替换 022 的 `subject/domain/object/action` 形态，也不引入 `g/g2`。489A 已把普通 tenant EHR 运行时从单 subject 修订为 subject set；实施可选择以下内部形态之一，但对外契约必须一致：

1. 在 `pkg/authz` 中增加 DB-backed tenant role authz capability reader，由统一 `Authorize(ctx, Request)` 门面使用。
2. 在请求生命周期内把 DB authz capability set 转换为等价的 `role:{slug}[]、tenant、object、action` 判定输入，由统一授权门面做 union。

无论内部实现如何，handler 和业务模块只能调用统一授权门面，不得各自查询 `iam.role_authz_capabilities`。

## 8. 实施切片

### 8.1 P0：契约冻结

1. [X] 新建 487 文档并加入 AGENTS Doc Map。
2. [X] 480/481/482/483/484/022 引用 487，明确角色定义后端闭环 owner。
3. [X] 明确新增 DB 表、迁移和 FK 前必须获得用户手工确认。

### 8.2 P1：Schema 与 Seed

1. [X] 获得用户对新增 `iam.role_definitions`、`iam.role_authz_capabilities` 的手工确认。
2. [X] 新增 IAM schema / migration，并补 grants、RLS/tenant 访问口径。
3. [X] seed 现有 tenant 的 `tenant-admin`、`tenant-viewer` 与当前能力集合。
4. [X] 新 tenant 创建路径同步 seed 内置角色。
5. [ ] 评估并落地 `iam.principals(tenant_uuid, role_slug)` 到 role definition 的引用约束。

### 8.3 P2：Domain / Service / Repository

1. [X] 在 IAM runtime store 中新增 role definition 持久化与服务端访问边界；后续可按 DDD 分层继续下沉到 IAM module service/repository。
2. [X] 实现 role slug、payload、revision、system-managed、authz capability key 校验。
3. [X] 实现保存事务：upsert role 基础信息、替换 authz capability 集合、递增 revision。
4. [X] 补 handler/runtime store 相关测试；专用 IAM service/repository 测试待后续分层下沉时补齐。

### 8.4 P3：HTTP API 与 Authz 覆盖

1. [X] 新增角色列表、详情、新建、编辑 API。
2. [X] route requirement 使用 `iam.authz:read` / `iam.authz:admin`。
3. [X] registry 登记 `iam.authz:admin`，并补齐 484 覆盖证据。
4. [X] handler 测试覆盖成功、无权限、重复 slug、未知 authz capability、stale revision、system-managed 写入。
5. [X] P3 route 挂载为可调用 API 前已同步完成 P4，未交付“保存成功但运行时不生效”的在线入口。

### 8.5 P4：运行时 Cutover

1. [X] 普通 tenant role 授权切到 DB role authz capability SoT。
2. [X] 删除或停止使用普通 tenant role 在 policy CSV 中的运行时授权来源。
3. [X] 保留匿名、控制面、bootstrap/system surface 的明确来源，并由 authz lint 区分。
4. [X] 覆盖“DB 有权限则允许、DB 无权限则拒绝、CSV 旧 tenant role 不得兜底放行”的测试。

### 8.6 P5：481 页面接入

1. [X] 481 角色定义页调用 487 API 保存。
2. [X] 角色详情重新打开后展示服务端返回的 revision 和 authz capability 集合。
3. [X] UI 消费服务端保存错误，不把组织范围、字段策略或复制语义字段写入 payload。

### 8.7 P6：门禁与验收

1. [X] `make check doc`
2. [X] `go fmt ./... && go vet ./... && make check lint && make test`
3. [X] IAM schema / migration 相关门禁；本轮未命中 sqlc
4. [X] `make authz-pack && make authz-test && make authz-lint`
5. [X] `make check routing`
6. [X] `make check error-message`
7. [X] 前端 typecheck / Vitest / `make generate && make css`；`make e2e` 已随 `make preflight` 通过

## 9. ADR 摘要

- **决策 1：角色定义 DB SoT，不写 policy CSV**
  - policy CSV 适合 bootstrap/static policy，不适合管理员在线保存租户角色。
  - 保存 API 写 CSV 会引入运行时文件持久化、并发写、审计回滚和部署漂移问题。

- **决策 2：首期 system-managed 角色只读**
  - 拒绝首期让管理员直接编辑 `tenant-admin`，避免自锁和 bootstrap 权限不可恢复。
  - 若后续需要编辑系统角色，必须另起计划冻结保护规则。

- **决策 3：角色 slug 不支持 rename**
  - rename 会影响 principal/assignment 引用和审计追溯。
  - 首期通过新建角色和后续用户授权调整完成业务变更。

- **决策 4：487 不拥有多角色 union**
  - 当前角色定义仍以单个 `role_slug` 为标识。
  - principal 多角色 union、团队/岗位授权与组织范围绑定不在 487 内扩展；多角色运行时语义由 `DEV-PLAN-489A` 承接，用户授权与组织范围 SoT 由 `DEV-PLAN-489` 承接。

## 10. 风险与停止线

| 风险 | 表现 | 停止线 |
| --- | --- | --- |
| 双链路授权 | DB 没有 authz capability 但 CSV 旧 policy 仍放行 | 普通 tenant role cutover 后不得 CSV fallback |
| 489 重复建角色 SoT | 用户授权表组里再次定义角色主表或角色 authz capability 主表 | 角色定义和能力集合只归 487；489 只保存 principal assignment 与 org scope |
| 单角色回流 | 489A 后仍按 `session role_slug` 或 `roles[0]` 做普通 tenant 授权 | 489A 统一 principal role set union；487 只提供单角色定义与 capability 读取 |
| 自锁 | 管理员改掉唯一管理角色的 `iam.authz:admin` | 首期 system-managed 角色只读 |
| authz capability 漂移 | 保存了 registry 外 key 或无覆盖 key | 482/483/484 校验必须在服务端执行 |
| 角色与范围混淆 | 角色定义 payload 接收组织范围 | 400 拒绝，范围只属于 489 用户授权 |
| schema 未确认 | 未获确认就创建表 | 停止实施，先获得用户手工确认 |
| UI 保存但运行时不生效 | 角色保存成功，API 仍按旧 policy 判断 | P4 cutover 未完成前不得宣称角色保存闭环交付 |
| 子计划单独宣布运行时完成 | 487 DB SoT 可保存，但 principal assignments、scope provider 或 489A union 尚未接入 | 只能声明 487 子能力完成；480 系列运行时闭环必须按 480A 的 487/489/489A 组合验收 |

## 11. 验收标准

1. [X] 角色定义保存 API 使用 DB SoT，保存后重新读取结果一致。
2. [X] 未知、禁用、不可分配、无覆盖、旧格式、重复 authz capability key 均被服务端拒绝。
3. [X] system-managed 角色不能通过在线 API 修改。
4. [X] revision 冲突返回 `409 stale_revision`。
5. [X] 普通 tenant role 的运行时授权来自 DB role authz capability，CSV 旧 tenant policy 不会兜底放行。
6. [X] 481 页面只提交基础信息、revision 与 `authz_capability_keys`，不提交组织范围、字段策略或复制语义字段。
7. [X] 487 作为 480 系列运行时授权交付的一部分，已与 489/489A 以及 481 UI 保存交互同步满足 480A 的首批用户可见闭环口径；不得仅凭角色保存 API 和 DB SoT 单点宣称完成。
8. [X] 相关文档引用 487，`make check doc` 通过。

## 12. 验证记录

- 2026-05-01 10:23 CST：创建方案文档，待本轮文档同步后运行 `make check doc`。
- 2026-05-02 CST：后端闭环已实施。新增 `iam.role_definitions`、`iam.role_authz_capabilities`，内置 role seed，角色定义读写 API，普通 tenant runtime 改为 DB role capability SoT；`config/access/policy.csv` 不再保留普通 tenant role grants。已验证：`go test ./...`、`go vet ./...`、`make check lint`、`make authz-pack && make authz-test && make authz-lint`、`make check routing`、`make check error-message`、`make iam plan && make iam lint`、`pnpm -C apps/web typecheck && pnpm -C apps/web test`、`make generate && make css`、`make check root-surface && make check no-legacy && make check doc`。
- 2026-05-02 CST：随 489/489A 完成 480A 后端运行时组合验收，并新增 `make check authz-role-union` 专用反回流门禁；补充验证通过 `go test ./internal/server ./internal/routing ./pkg/authz`、`make test`、`make check authz-role-union`、`make check no-legacy && make check chat-surface-clean && make check no-scope-package && make check granularity && make check request-code`。
- 2026-05-02 CST：481 角色定义 UI 已接入 487 API；相关前端测试和完整 `make preflight` 通过。A/B E2E 随 489 用户授权闭环验证通过。

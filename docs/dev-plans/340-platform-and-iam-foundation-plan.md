# DEV-PLAN-340：平台与 IAM 基座子计划（Tenancy / AuthN / AuthZ / Shell）

**状态**: 规划中（2026-03-17 03:16 CST）

## 1. 背景与上下文

本计划承接 [DEV-PLAN-300](/home/lee/Projects/Bugs-And-Blossoms/docs/dev-plans/300-greenfield-csharp-hr-platform-functional-blueprint.md) 中“从零重做、功能优先”的蓝图，负责在工程、数据与安全基线之上落地平台基座部分。

如果把整个平台比作一栋楼，`340` 负责的是：

- 楼的地基
- 门禁系统
- 公共大厅与导航
- 租户边界
- 用户身份
- 角色与权限
- 审计、任务、通知的最小平台能力

没有这一层，后续 `350/360/370/380/390` 会被迫各自重复实现基础能力，导致系统从第一天开始分叉。

## 2. 目标与非目标

### 2.1 核心目标

- [ ] 建立多租户平台基线：tenant、domain、session、platform settings。
- [ ] 建立统一认证与会话模型，支持本地登录与 OIDC 扩展位。
- [ ] 建立角色与权限主模型，支撑 HR 平台的后台管理权限控制。
- [ ] 建立前后端统一的应用壳：导航、租户上下文、用户上下文、错误页、空状态、列表与详情页基础组件。
- [ ] 建立统一审计日志、通知、后台任务基座，供后续业务复用。
- [ ] 建立 superadmin 控制面最小闭环：租户创建、启停、域名绑定、租户管理员初始化。

### 2.2 非目标

- [ ] 本计划不实现 Org / JobCatalog / Staffing / Person 的具体业务规则。
- [ ] 本计划不实现完整审批流引擎与复杂流程编排。
- [ ] 本计划不实现 Assistant 的复杂语义编排，只保留平台接入点。
- [ ] 本计划不引入数据库级 RLS 或事件溯源。

## 3. 范围

### 3.1 后端模块

- `Platform.IAM`
- `Platform.Tenancy`
- `Platform.Configuration`
- `Platform.Audit`
- `Platform.Notifications`
- `Platform.Jobs`

### 3.2 前端模块

- 应用壳与路由
- 登录页
- 租户切换/上下文展示
- 用户菜单
- 统一列表页与详情页基座
- Superadmin 控制台入口

## 4. 关键设计决策

### 4.1 认证模式（选定）

- Phase 1 采用本地会话模型。
- 登录入口支持：
  - 本地用户名密码
  - 预留 OIDC 登录入口
- Session 保持服务器端可撤销。

### 4.2 授权模式（选定）

- 采用 `role + permission` 模型。
- 角色分为：
  - `superadmin`
  - `tenant_admin`
  - `tenant_hr`
  - `tenant_viewer`
- 权限粒度以模块资源和动作表示，不把路由路径直接当作权限模型。

### 4.3 多租户模式（选定）

- 默认单库多租户。
- 所有业务数据显式带 `tenant_id`。
- 平台层统一注入 tenant context，不允许业务模块自行解析租户。

### 4.4 平台基座边界（选定）

- 审计、通知、任务调度属于平台能力，不归属某个 HR 业务域。
- 后续业务模块只能复用平台基座，不允许重新实现第二套任务系统、第二套审计日志、第二套路由壳。

## 5. 功能拆分

### 5.1 M1：租户与认证

- [ ] Tenant 模型
- [ ] Tenant Domain 模型
- [ ] User / Principal 模型
- [ ] Session 模型
- [ ] 登录、登出、续期
- [ ] 租户管理员初始化

### 5.2 M2：授权与应用壳

- [ ] 角色与权限模型
- [ ] API 授权策略
- [ ] 前端路由守卫
- [ ] 导航、Topbar、用户菜单
- [ ] 403 / 404 / 500 页面

### 5.3 M3：平台公共能力

- [ ] 审计日志
- [ ] 通知模型
- [ ] 后台任务调度
- [ ] 系统配置
- [ ] 字典配置基础框架

### 5.4 M4：Superadmin 控制台

- [ ] 租户列表
- [ ] 创建租户
- [ ] 启停租户
- [ ] 域名绑定
- [ ] 初始化租户管理员

## 6. 数据模型方向

建议至少包括以下主表：

- `tenants`
- `tenant_domains`
- `users`
- `sessions`
- `roles`
- `permissions`
- `role_permissions`
- `user_roles`
- `audit_logs`
- `notifications`
- `background_jobs`
- `app_settings`
- `dictionary_definitions`
- `dictionary_values`

## 7. API 与前端交付面

### 7.1 API

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/me`
- `GET /api/tenants/current`
- `GET /api/admin/tenants`
- `POST /api/admin/tenants`
- `POST /api/admin/tenants/{id}/enable`
- `POST /api/admin/tenants/{id}/disable`
- `GET /api/admin/audit-logs`

### 7.2 UI

- `/login`
- `/app`
- `/admin/tenants`
- `/admin/tenants/:id`
- `/admin/audit`

## 8. 依赖关系

- `340` 默认依赖 `310/320/330` 提供的工程、数据与安全基线。
- `340` 是 `350/360/370/380/390` 的平台前置计划。
- `360` 中所有核心 HR 模块默认依赖本计划提供的 tenancy、auth、audit、dictionary、jobs。
- `370/380/390` 依赖本计划提供的 task、notification、auth 与应用壳基座。

## 9. 验收标准

- [ ] 用户能完成“登录 → 进入应用壳 → 获取当前租户与权限上下文”。
- [ ] Superadmin 能完成“创建租户 → 绑定域名/基础信息 → 初始化租户管理员”。
- [ ] 后端 API 已具备统一认证、授权、审计、错误处理基线。
- [ ] 前端已有统一应用壳，不再需要业务模块各自拼装导航与会话逻辑。
- [ ] 字典、通知、任务与审计能力已可被后续业务模块复用。

## 10. 后续拆分建议

1. [ ] `341`：Tenancy / AuthN 详细设计
2. [ ] `342`：AuthZ 与平台权限矩阵设计
3. [ ] `343`：Superadmin 控制台与租户生命周期设计
4. [ ] `344`：Audit / Notification / Background Jobs 基座设计

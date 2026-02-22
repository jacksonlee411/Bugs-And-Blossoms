# DEV-PLAN-105B：Dict Code（字典本体）新增与治理方案（承接 DEV-PLAN-105/105A）

**状态**: 已完成（2026-02-17）

> 本计划承接 `DEV-PLAN-105`（字典值配置模块）与 `DEV-PLAN-105A`（验证问题调查）。  
> 目标：补齐“新增 dict_code（字典本体）”的能力与治理口径，避免运行态任意注入 dict_code 导致不可控扩散与漂移。

## 0. TL;DR（冻结结论）

- 本计划选择 **选项 A：引入 dict registry（字典本体表）作为 SSOT**。
- 这更符合 `DEV-PLAN-003` 的 “Simple > Easy”：把“dict_code 是什么/是否存在/是否可用”变成一个**显式、可审计、可治理**的边界，而不是靠“创建第一条 value”隐式生成。

### 0.1 后续收敛注记（2026-02-22）

- 按 `DEV-PLAN-070B`，字典运行时读取口径已收敛为 **tenant-only**，本计划中涉及 `tenant/global fallback` 的描述均视为历史阶段口径。
- 按 `STD-002` / `DEV-PLAN-102B`，`as_of` 缺失/非法统一 `400 invalid_as_of`（message：`as_of required`），禁止 default today。

## 1. 背景

用户在 `http://localhost:8080/app/dicts` 验证字典模块时，明确提出并确认需求为：**新增新的 `dict_code`（字典本体）**，并在 UI 左侧列表可见（见 `DEV-PLAN-105A` 问题 B2）。

当前实现偏向 Phase 0：仅支持 `org_type`（代码层 `supportedDictCode(...)` 限制），且后端不存在“创建 dict_code”的 API；UI 也没有对应入口（仅 value 的 create/disable/correct）。

> 关键问题：`DEV-PLAN-105` 的落地把 **dict value（字典值）** 做成可配置，但 **dict code（字典本体）** 仍然是“硬编码/隐式出现”（来自 segments distinct + supportedDictCode hardcode），导致：
> - UI 左侧 Dict List 无法扩展（不能新增、不能停用、不能治理展示名）。
> - dict_code 的“存在性/可用性”不再是单一事实源（容易 drift）。

## 2. 目标与非目标

### 2.1 目标（本计划冻结）

1. 支持在 UI `/app/dicts` 中**新增 dict_code**（字典本体），新增后出现在左侧 Dict List。
2. dict_code 的新增遵循可治理原则：
   - 命名规则与校验（与 DB check 约束一致，且在 API 层有稳定错误码）。
   - 必须有展示名（display_name 或 i18n key），避免 UI 出现“只有 code 没有可读名”。
   - 具备启用/停用（至少停用），避免历史遗留“僵尸 dict”无法收口。
3. 保持 One Door / No Tx, No RLS / fail-closed / No Legacy。

### 2.2 非目标（Stopline）

1. 不在本计划引入“业务数据多语言”（仍遵循 `DEV-PLAN-020`）；dict 本体展示名可以使用 i18n key，但不扩展为值级多语言。
2. 不在本计划要求“跨模块自动发现全部 dict_code”；新增 dict_code 仍需显式创建/登记（治理优先）。
3. 不强制一次性迁移所有模块 DICT 字段接入；接入仍按模块/字段逐步推进。
4. MVP 不引入 dict_code 的“多段有效期窗口”（不做 re-enable/多段版本）；若未来需要，另起子计划冻结状态机与不变量。

## 3. 方案选择（冻结）

### 3.1 选项 A（本计划采纳）：引入 dict registry（字典本体表）作为 SSOT

本计划冻结以下结论：

1. **新增 dict registry 表**（建议命名：`iam.dicts`）作为 dict_code 的唯一事实源（SSOT）。
2. `GET /iam/api/dicts?as_of=...` 的 dict list 来源切换为 registry（允许返回“尚无 values 的空 dict”）。
3. dict value 仍存于 `iam.dict_value_segments`/`iam.dict_value_events`（`DEV-PLAN-105` 已落地，不回退）。
4. dict registry 的写入同样遵循 One Door：新增 `iam.submit_dict_event(...)`（建议），与 `iam.submit_dict_value_event(...)` 平行；应用层不允许直写表。

为什么更符合 `DEV-PLAN-003`（Simple > Easy）：
- **边界清晰**：dict_code 的存在性从“隐式出现（segments distinct）+ hardcode”变成“registry 显式事实”。
- **不变量可强制**：dict_code 命名、唯一性、停用语义可由 DB + Kernel 强制，而不是散落在代码 if/else。
- **可解释**：先建 dict（本体）-> 再配 values（值），主流程与失败路径更可叙述。

代价/风险（接受）：
- 需要新增表/迁移（触发仓库红线：**新增表前必须用户手工确认**）。
- 需要把现有 `/iam/api/dicts` 的实现从“读 segments distinct”迁移为“读 registry（tenant-only 运行态）”，并补齐测试与 UI 交互。

### 3.2 选项 B（本计划拒绝）：通过“创建第一个 value”隐式生成 dict_code

拒绝理由（对齐 003）：
- 这是“省迁移的容易（Easy）”，但把治理复杂度埋进隐式约定：dict_code 何时存在、何时可见、如何停用/审计、空 dict 如何表达，都变得不清晰。
- 很容易形成“到处补洞”的漂移：既要放开 `supportedDictCode(...)`，又要补 UI 兜底、补 allowlist、补规则解释，整体不再 Simple。

## 4. 领域模型与不变量（选项 A，冻结）

### 4.1 核心实体

- `dict`（字典本体，SSOT）：
  - `dict_code`：稳定键（lower_snake）
  - `name`：展示名（MVP 允许直接存可读文本；也可在后续收敛为 i18n key）
  - `enabled_on/disabled_on`：day 粒度 Valid Time 窗口（与 dict_value_segments 对齐；半开区间 `[enabled_on, disabled_on)`）
  - `status`：**派生字段**（不作为存储事实），按 `as_of` 计算：`enabled_on <= as_of < disabled_on(or +inf)`
  - `created_at/updated_at`：审计辅助（tx time）
- `dict_event`（字典本体事件，审计 SoT）：
  - `event_type`：`DICT_CREATED|DICT_DISABLED|DICT_RENAMED|DICT_RESCINDED`（最小集合；MVP 可先 `CREATED/DISABLED`）
  - `request_code/initiator_uuid/tx_time/before_snapshot/after_snapshot`

### 4.2 不变量（必须强制，fail-closed）

1. `dict_code` 命名约束必须与 `DEV-PLAN-105` 对 `dict_value_segments.dict_code` 的 check 保持一致（lower + btrim + regex）。
2. 同租户下 `dict_code` 唯一（幂等与冲突口径稳定）。
3. 停用后的 dict_code：
   - 不能再被用于创建/启用新的 dict value（写入 fail-closed）。
   - UI 仍可查看历史 values/audit（读不回退，不走 legacy）。
4. One Door：应用层不得直写 `iam.dicts`，必须通过 Kernel（`submit_dict_event`）写入，并在同事务内落审计事件。
5. No Tx, No RLS：任何访问 dict registry/dict values 的 store 必须显式事务 + `app.current_tenant` 注入；缺失即拒绝。
6. **消除隐式 dict_code**：`iam.dict_value_segments` 与 `iam.dict_value_events` 必须通过 DB 外键引用 `iam.dicts(tenant_uuid, dict_code)`；不存在/不可用的 dict_code 不能写入 values（DB+Kernel 双保险，fail-closed）。

### 4.3 `as_of` 语义（与 DEV-PLAN-105 对齐）

`DEV-PLAN-105` 已冻结：`GET /iam/api/dicts` 的 `as_of` 必填。  
本计划冻结：在选项 A 下仍保持 `as_of` 必填，并采用以下最小口径（避免“今天”隐式时区问题）：

- `GET /iam/api/dicts?as_of=...`：返回 `as_of` 下可用的 dict_code 列表。
- 可用性判定（冻结）：`enabled_on <= as_of < disabled_on(or +inf)` 且 `status=active`。

补充（接入方口径冻结）：
- 当业务模块需要在“启用字段配置（enable field-config）”阶段选择/校验 `dict_code` 时，统一以该字段的 `enabled_on`（date）作为 `as_of` 调用 `GET /iam/api/dicts?as_of=...`（fail-closed；对齐 `DEV-PLAN-106`）。
- 说明（对齐 `DEV-PLAN-106A`）：业务模块可将 dict_code 推导为 `field_key=d_<dict_code>`。由于部分模块的 `field_key` DB check 可能更严格（例如长度上限），接入方必须对“可推导”做显式校验：不可推导时不得返回为候选，且启用/写入阶段必须 fail-closed 并提供可排障信息（避免隐性产品边界漂移）。

## 5. 数据库契约（选项 A，冻结并已落地）

> 用户已于 2026-02-17 确认按 105B 落库新增 `iam.dicts` / `iam.dict_events`。

### 5.1 表：`iam.dicts`（建议）

- 主键：`(tenant_uuid, dict_code)`
- 字段（建议最小集）：
  - `tenant_uuid uuid not null`
  - `dict_code text not null`
  - `name text not null`（btrim non-empty）
  - `enabled_on date not null`
  - `disabled_on date null`
  - `created_at/updated_at timestamptz not null default now()`
- 约束：
  - `dict_code` 的 check 与 `iam.dict_value_segments` 对齐（lower + btrim + regex）
  - `name` 非空且 `btrim(name)=name`
  - 若存在 `disabled_on`：`enabled_on < disabled_on`（半开区间 `[enabled_on, disabled_on)`）
- RLS：
  - `ENABLE/FORCE RLS`
  - policy：运行态业务读写仅允许 `current_tenant`；`globalTenant` 仅作为发布链路的控制面数据来源，不进入业务 API 读取路径。

### 5.1A 外键：values -> dicts（必须）

- `iam.dict_value_segments(tenant_uuid, dict_code)` -> `iam.dicts(tenant_uuid, dict_code)`
- `iam.dict_value_events(tenant_uuid, dict_code)` -> `iam.dicts(tenant_uuid, dict_code)`

> 目的：从 DB 层阻断“先写 value 就能隐式生成 dict_code”的路径，使 dict_code 的存在性只有 registry 一种权威表达（对齐 `DEV-PLAN-003` 的 Simple）。

### 5.2 表：`iam.dict_events`（建议）

- 用于审计 dict_code 的生命周期事件（同 `request_code` 幂等）。
- RLS 同上；应用层不得直写，只允许 Kernel 写入。

### 5.3 Kernel：`iam.submit_dict_event(...)`（建议）

- 输入（冻结）：`tenant_uuid, dict_code, event_type, effective_day(date), payload(jsonb), request_code, initiator_uuid`
- 幂等：同 `(tenant_uuid, request_code)` 幂等；不同 payload 复用 request_code 冲突拒绝（稳定错误码）。
- 同事务写：
  - `iam.dicts`（投射/当前态）
  - `iam.dict_events`（审计 SoT）

## 6. API 合约（选项 A，冻结）

### 6.1 读接口（保持并调整语义）

- `GET /iam/api/dicts?as_of=YYYY-MM-DD`
  - 来源：`iam.dicts`（tenant-only）
  - Response（冻结）：`{as_of, dicts:[{dict_code,name,enabled_on,disabled_on,status}]}`（`status` 为派生字段；字段名 snake_case）
- `GET /iam/api/dicts/values?dict_code=...&as_of=...`（延续 `DEV-PLAN-105`）

### 6.2 写接口（新增）

- `POST /iam/api/dicts`
  - Body：`{dict_code,name,enabled_on,request_code}`
  - 权限：`dict.admin`
  - 行为：创建 dict_code（幂等 + 冲突稳定）
- `POST /iam/api/dicts:disable`
  - Body：`{dict_code,disabled_on,request_code}`
  - 权限：`dict.admin`
  - 行为：停用 dict_code（停用后禁止新增 values；历史可读）

## 6A. tenant-only 读取与发布迁移口径（冻结）

> 按 `DEV-PLAN-070B`，共享改发布：平台基线通过发布任务写入租户本地；业务运行时不再跨租户读取。

1. `GET /iam/api/dicts?as_of=...`（dict list）：
   - 仅返回当前租户在 `as_of` 下可见 dict。
   - 不再合并 global 视图；租户无数据即按 fail-closed 处理。
2. `GET /iam/api/dicts/values?dict_code=...&as_of=...`（values list）与 `ResolveValueLabel(...)`：
   - 只在当前租户范围查询；结果为空/未命中时不回退 global。
   - 租户未完成基线导入时返回 `dict_baseline_not_ready`；其余未命中返回 `dict_not_found` 或 `dict_value_not_found_as_of`。

### 6.3 稳定错误码（最小集合）

- `invalid_as_of`（缺失/非法统一 `400`，message：`as_of required`）
- `dict_code_required`
- `dict_code_invalid`
- `dict_name_required`
- `dict_enabled_on_required`
- `dict_enabled_on_invalid`
- `dict_disabled_on_required`
- `dict_disabled_on_invalid`
- `dict_code_conflict`
- `dict_not_found`
- `dict_baseline_not_ready`
- `dict_disabled`
- `dict_value_dict_disabled`（当对停用 dict_code 写入 value 时）
- `dict_request_code_required`
- `forbidden`

## 7. UI/IA（对齐 105/105A）

> 说明：页面整体两栏布局与“点击 value 行崩溃”的修复仍由 `DEV-PLAN-105A` 承接；本计划只补齐“新增 dict_code”的 UI 能力与其治理口径。

- 左侧 Dict List：
   - 新增「Create Dict」入口（Dialog）
   - Dict 行展示：`name + dict_code + status`
   - 支持停用（分屏 1 左侧提供停用入口；不引入额外 Dict detail 子页面）
- 右侧（选中 dict 时）：
   - Value Grid（现有）
   - 点击 value 行进入分屏 2（值详情页），展示基本信息与变更日志（参考 Org 模块双栏布局）

## 8. 实施步骤（Checklist）

1. [X] （红线）新增表/迁移前：用户手工确认本计划引入 `iam.dicts/iam.dict_events`（或等价命名）。
2. [X] DB：新增迁移与 schema（Atlas+Goose 闭环），并补齐 RLS 与 Kernel（One Door）。
3. [X] DB：为 `dict_value_segments/events` 增加 `(tenant_uuid, dict_code)` -> `iam.dicts` 外键，并在 `submit_dict_value_event(...)` 中补齐 dict 可用性校验（DB+Kernel 双保险）。
4. [X] 后端：调整 `/iam/api/dicts` 读路径（registry SSOT + tenant-only 运行态）；新增 dict 写接口（create/disable）。
5. [X] 后端：移除/替换 `supportedDictCode(...)` 的硬编码限制：改为“dict_code 必须在 registry 存在且在 as_of 下可用”（fail-closed）。
6. [X] 后端：将 `status` 统一改为派生字段（由 `enabled_on/disabled_on + as_of` 计算），不再引入第二套存储事实。
7. [X] 前端：在 `/app/dicts` 增加 dict_code 创建/停用交互（对齐 `DEV-PLAN-105A` 的两栏 IA）。
8. [X] 测试：补齐 store/handler 覆盖，至少包含：
   - tenant-only 读取路径（无 global fallback）；
   - tenant dict 存在但 values 为空时返回租户内空结果/未命中，不跨租户回退；
   - disabled dict_code 的 value 写入 fail-closed。

## 9. 验收标准（DoD）

1. 管理员可在 UI 新增 dict_code；创建成功后在左侧立刻可见，并可选中查看（即使 values 为空也有明确空态）。
2. 停用 dict_code 后：
   - UI 仍能查看其历史 values/audit；
   - 写接口 `POST /iam/api/dicts/values` 对该 dict_code 一律拒绝（稳定错误码，fail-closed）。
3. `/iam/api/dicts` 的 dict list 来源单一且可解释（registry SSOT），不再依赖“segments distinct + supportedDictCode hardcode”。
4. `status` 对外可见但仅为派生值；系统内部仅以 `enabled_on/disabled_on` 作为状态事实源。
5. 运行时读取满足 tenant-only 冻结规则：无 `global_tenant` fallback，缺基线时 `dict_baseline_not_ready` fail-closed。

## 10. 门禁与验证（SSOT 引用）

- 命令入口与触发器以 `AGENTS.md` 与 `docs/dev-plans/012-ci-quality-gates.md` 为准。
- 本计划已冻结选项 A，迁移落地按 Atlas+Goose 模块闭环指引执行（`docs/dev-plans/024-atlas-goose-closed-loop-guide.md`）。

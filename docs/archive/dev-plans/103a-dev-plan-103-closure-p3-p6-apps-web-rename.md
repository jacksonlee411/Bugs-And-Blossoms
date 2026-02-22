# DEV-PLAN-103A：DEV-PLAN-103 收尾（P3 业务页闭环 + P6 工程改名：去技术后缀）

**状态**: 已完成（2026-02-15 20:52 UTC）

> 本计划是 `DEV-PLAN-103` 的“收尾与清理”补充计划：把 P3/P6 彻底闭环，并将残留的旧 UI/命名技术后缀一次性收口到可验收状态。  
> 门禁与命令入口以 `AGENTS.md` / `Makefile` / CI workflow 为 SSOT；本文只冻结“要做什么、做到什么程度、如何验收与留证据”。

## 0. 关联文档（SSOT 引用）

- 主计划（目标与背景）：`docs/archive/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`
- 时间语义与路由时间上下文矩阵：`docs/archive/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- CI/门禁结构（SSOT）：`docs/dev-plans/012-ci-quality-gates.md`
- 本地必跑入口（SSOT）：`AGENTS.md`
- 执行证据（本计划）：`docs/dev-records/dev-plan-103a-execution-log.md`

## 1. 背景

`DEV-PLAN-103` 已完成“移除 Astro/旧局部渲染链路的运行路径与构建链路（P4/P5）”，但仍存在两类未闭环事项：

1) P3：业务页面迁移到 MUI 的收尾未完成（缺少《旧 UI → MUI 映射表》；Person 页面仍暴露“ignored as-of”输入，时间上下文口径不干净）。  
2) P6：工程命名去技术后缀已执行（`apps/web` 为唯一前端工程目录），仍需补齐“引用收口 + 证据登记”以形成长期 SSOT。

本计划要把上述两类事项收口为“可验证、可审计、可长期演进”的最终状态，并将清理动作拆成可 review 的 PR 序列。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [x] P3 完整收尾：输出并维护《旧 UI → MUI 映射表》，且能用事实源与门禁验证“旧 UI 不可达/不可构建/不可误用”，业务入口在 MUI 中可发现、可操作（至少覆盖当前已实现能力）。
- [x] 映射表覆盖 `DEV-PLAN-103` 里已识别的旧 UI 入口家族（至少：`/login`、`/ui/*`、`/lang/*`、`/org/nodes*`、`/org/snapshot`、`/org/setid`、`/org/job-catalog`、`/org/positions`、`/org/assignments`、`/person/persons`），并逐条标注证据类型（allowlist/handler/测试）。
- [x] Person 页面去除“ignored as-of”输入，使时间上下文口径与 `DEV-PLAN-102` 一致（不制造“伪需求参数”与歧义 UI）。
- [x] P6 完整收尾：完成前端工程目录改名（去技术后缀）+ 全仓引用更新；CI 触发器、构建脚本、文档 SSOT 同步更新；本地能跑通 UI 构建与全门禁。
- [x] 清理残留旧 UI 死代码/测试（不改变对外契约）：覆盖并收口 **三类残留面**，避免后续误引入“兼容别名窗口/legacy 回退”：
  - 旧 HTML/旧局部渲染链路 handler 家族（`internal/server/**` 下的旧页面渲染、旧表单 action、旧 redirect URL 等）。
  - 中间件放行口径（例如对 `/login` 的特殊放行/绕过条件，避免潜在 backdoor）。
  - Authz 路由判定残留（旧 UI 路由对应的授权分支与其测试，避免“路由虽不可达但代码仍保活”）。

### 2.2 非目标（本计划不做）

- 不引入新的 UI 功能、不做大规模 UI 重构（仅收尾：入口/口径/命名/可达性/证据）。
- 不更改 API 契约/鉴权模型（若确需调整，必须先更新对应 dev-plan，遵循 Contract First）。
- 不更改 localStorage key、package name 等“运行态标识”（除非发现明确冲突/门禁失败；若要改，必须单独开子任务并提供迁移策略，避免暗改造成用户侧状态丢失）。
- 不批量改写历史执行记录文档（`docs/dev-records/**`）；历史文档允许保留旧路径引用，但“可执行文档/脚本/门禁入口”必须收敛到新路径。

## 3. 范围与约束（不变量）

- **No Legacy**：不引入 `/login` 兼容窗口、旧路由 alias、read=legacy 等回退通道（见 `DEV-PLAN-004M1`）。
- **以 allowlist/route_class 为准**：UI vs API 的未登录行为以 `route_class` 判定；内部 API 必须 JSON 401，不得 302（见 `DEV-PLAN-103` 与 `DEV-PLAN-102`）。
- **SuperAdmin 控制面不在本计划收尾范围**：`/superadmin/*` 属于独立 entrypoint，其登录页 `/superadmin/login` 的存在不等价于“legacy 回退通道”；本计划的 No Legacy 约束聚焦于租户应用侧的旧 UI 入口（尤其 `/login` HTML）。
- **机械改名不夹带功能改动**：P6 的目录改名必须是“机械改名 + 引用更新”，不混入功能变更；功能变更必须在独立 PR、独立勾选项里执行。

## 4. 交付物

- `docs/dev-records/dev-plan-103a-execution-log.md`：
  - 《旧 UI → MUI 映射表》（表格化，含状态/证据）。
  - 本计划关键命令执行记录（时间戳、结果、环境要点）。
- 代码与配置收口：
  - `apps/web/` 作为唯一前端工程目录。
  - 触发器/脚本/文档 SSOT 不再引用旧目录名（技术后缀）。
- `DEV-PLAN-103` 更新：
  - P3/P6 对应条目可被勾选为完成（并在验收标准处体现证据入口）。

## 5. 实施步骤（建议按 PR 拆分）

### PR-103A-0：收尾准备与证据骨架

1. [x] 建立本计划执行日志：`docs/dev-records/dev-plan-103a-execution-log.md`。
2. [x] 在执行日志中生成《旧 UI → MUI 映射表》的初版（以事实源为准），并标注迁移/删除状态。
   - 事实源（建议顺序，可按实际补齐）：
     - `config/routing/allowlist.yaml`（route_class / 可达性门禁）
     - `internal/server/**`（旧 HTML/旧局部渲染链路 handler 与 redirect/表单 action 的实际残留面；避免仅盯 `handler.go`）
     - `internal/server/authz_middleware.go`（路由 → 权限判定；用于识别“路由已删但授权分支仍保活”）
     - `apps/*/src/router/index.tsx`（MUI SPA 路由）
     - `apps/*/src/navigation/config.tsx`（导航入口 + permissionKey）
     - `e2e/tests/**`（外部可见链路证据）
   - 至少覆盖：`/login`、`/ui/nav`、`/ui/topbar`、`/ui/flash`、`/lang/en`、`/lang/zh`、`/org/nodes*`、`/org/snapshot`、`/org/setid`、`/org/job-catalog`、`/org/positions`、`/org/assignments`、`/person/persons`。

> 映射表建议字段（可按实际调整）：旧路径/模式、旧 route_class、旧时间上下文(A/B/C)、旧状态(已移除/不可达/仍有死代码)、新 MUI path（`/app` 内路由）、新 API（如有）、permissionKey、备注（证据与清理点）。

### PR-103A-1：P3 收尾（Person 页面时间口径清理 + 映射表补齐）

3. [x] Person 页面去除“ignored as-of”输入与相关状态变量：  
   - 原则：Person 页面若不需要时间上下文，应按 `DEV-PLAN-102` 归类为 C 类（无时间上下文），不暴露日期输入以避免歧义。
4. [x] 在映射表中补齐 Person/JobCatalog/Staffing/SetID 的“入口→页面→API→权限→时间口径”信息，并标注当前实现状态（已迁移/占位/未实现）。

### PR-103A-2：旧 UI 残留清理（死代码/测试/绕过口径）

5. [x] 以“覆盖残留面”为原则清理旧 UI（死代码/测试/绕过口径），避免后续误用或被再次挂回路由形成 backdoor：  
   - **中间件口径收口**：移除对 `/login` 的特殊放行/绕过条件（当前代码中如存在该口径，应视为必须清理项），并用测试锁定“tenant app 不提供 `/login` HTML”。
   - **旧渲染辅助函数收口**：移除或隔离旧 UI 的渲染/壳层/旧局部渲染链路协商函数（Nav/Topbar/Flash、最小 Shell、`HX-Request` 分支、旧 login form、`/lang/*` 链接等）及其测试，避免形成“看似死代码但可复活”的隐性入口。
   - **旧 HTML handler 家族清理**：以 `internal/server/**` 为范围，清理仍存活的旧页面渲染、表单 action、redirect URL 与其测试（例如 Org Nodes/Snapshot、JobCatalog、Staffing、Person、SetID 中残留的旧 HTML 交互链路）。
   - **Authz 路由判定清理**：同步清理旧 UI 路由相关的 authz requirement 分支与单测，避免“路由删了但授权映射仍保活”。
   - **不改变对外契约边界**：仅移除旧 UI（HTML/旧局部渲染链路）相关分支；JSON API 与 `/app/**`（SPA）入口保持既有契约。
   - **Stopline（必须可证明）**：在本 PR 内给出可复现证据，证明 `/login`（HTML）不存在、且旧 HTML/旧局部渲染链路页面入口不会通过“中间件放行/重定向/隐性链接”被再次触达（证据可来自：allowlist、Go 单测、E2E、或映射表的“不可达/已移除”标注）。
6. [x] 更新 `DEV-PLAN-103` 的验收/风险说明：把“残留清理”与“证据表”链接到本计划执行日志，形成可追溯收口点。

### PR-103A-3：P6 工程改名（去技术后缀，机械改名）

7. [x] 执行目录改名：`apps/web` 为唯一前端工程目录（仅机械改名 + 引用更新，不夹带功能改动）。
8. [x] 全仓机械更新引用（并以门禁为准收口）：  
   - 构建脚本（如 `scripts/ui/*`）  
   - CI 触发器（如 `scripts/ci/paths-filter.sh`）  
   - 文档 SSOT 与可执行文档（至少：`AGENTS.md`、`docs/dev-plans/011-tech-stack-and-toolchain-versions.md`、`docs/dev-plans/012-ci-quality-gates.md`、`docs/archive/dev-plans/103-remove-astro-legacy-ui-and-converge-to-mui-x-only.md`、以及 UI 规范类文档如 `docs/dev-plans/002-ui-design-guidelines.md`）  
   - 历史文档按“可读不执行”处理：允许保留旧路径，仅在必要处补充“历史说明/已被 103A 收口”注记，避免篡改证据语义
   - E2E 触发条件与可能的路径硬编码引用（如存在）。
9. [x] Stopline（必须可证明）：除 `docs/dev-records/**` 与明确标注“历史”的文档外，仓库内不应再出现旧目录名路径引用；同时保持运行态标识不暗改（例如 localStorage key、package name 等，除非门禁失败迫使变更并给出迁移策略）。

### PR-103A-4：验收与门禁对齐（本地可复现）

10. [x] 跑 UI 构建门禁并提交生成物（如有）：`make css`，然后 `git status --short` 必须为空。
11. [x] 跑全门禁对齐 CI：`make preflight`。
12. [x] 将关键命令与结果登记到 `docs/dev-records/dev-plan-103a-execution-log.md`（时间戳、结果；不在 dev-plan 内复制命令矩阵）。
13. [x] 回写 `DEV-PLAN-103`：将 P3/P6 条目更新为 `[x]`，并在验收标准中引用本计划的执行日志作为证据入口。

## 6. 验收标准

- [x] 《旧 UI → MUI 映射表》存在且可追溯（明确事实源、覆盖本计划约定的旧入口清单，并对每条路由给出状态/证据类型）。
- [x] 前端工程目录为 `apps/web`；**代码/脚本/CI/可执行文档（SSOT）** 不再引用旧目录名。  
  `docs/dev-records/**` 与明确标注“历史”的文档允许保留旧路径文本，不作为阻塞项。
- [x] Person 页面不再出现 “As-of (ignored)” 或等价输入；时间上下文口径与 `DEV-PLAN-102` 一致。
- [x] 不存在 `/login` HTML 页面或兼容跳转窗口；不在中间件层保留对 `/login` 的特殊放行/绕过逻辑形成潜在 backdoor。
- [x] `internal/server/**` 中不再残留旧 UI 交互链路（旧 HTML 页面渲染、旧表单 action/redirect 指向旧路由、以及旧局部渲染链路/hx-* 标记相关输出），且对应旧 UI 单测已清理或迁移到与 MUI-only 方向一致的断言。
- [x] 本地 `make css` 与 `make preflight` 可通过，且证据记录已落盘。

## 7. 风险与缓解

- 风险：机械改名牵涉面大，遗漏引用导致 CI gate/E2E 不触发或构建失败。  
  缓解：先用 `rg` 全仓扫描旧目录名路径引用，再以 `make preflight` 与 CI 路径触发器双重验证。
- 风险：把“死代码清理”与“改名”混在同一 PR，review/回滚困难。  
  缓解：按 PR 拆分，保持每个 PR 只有一个主轴。

## 8. 交付物清单（最终应出现的文件/变化）

- `docs/archive/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`（本文件）
- `docs/dev-records/dev-plan-103a-execution-log.md`（执行证据）
- `apps/web/`

# DEV-PLAN-103A：DEV-PLAN-103 收尾（P3 业务页闭环 + P6 工程改名 apps/web-mui → apps/web）

**状态**: 草拟中（2026-02-15 13:46 UTC）

> 本计划是 `DEV-PLAN-103` 的“收尾与清理”补充计划：把 P3/P6 彻底闭环，并将残留的旧 UI/命名技术后缀一次性收口到可验收状态。  
> 门禁与命令入口以 `AGENTS.md` / `Makefile` / CI workflow 为 SSOT；本文只冻结“要做什么、做到什么程度、如何验收与留证据”。

## 0. 关联文档（SSOT 引用）

- 主计划（目标与背景）：`docs/dev-plans/103-remove-astro-htmx-and-converge-to-mui-x-only.md`
- 时间语义与路由时间上下文矩阵：`docs/dev-plans/102-as-of-time-context-convergence-and-critique.md`
- CI/门禁结构（SSOT）：`docs/dev-plans/012-ci-quality-gates.md`
- 本地必跑入口（SSOT）：`AGENTS.md`
- 执行证据（本计划）：`docs/dev-records/dev-plan-103a-execution-log.md`

## 1. 背景

`DEV-PLAN-103` 已完成“移除 Astro/HTMX 的运行路径与构建链路（P4/P5）”，但仍存在两类未闭环事项：

1) P3：业务页面迁移到 MUI 的收尾未完成（缺少《旧 UI → MUI 映射表》；Person 页面仍暴露“ignored as-of”输入，时间上下文口径不干净）。  
2) P6：工程命名去技术后缀未执行（`apps/web-mui` 尚未改名为 `apps/web`），导致引用、触发器、脚本、文档仍携带技术后缀。

本计划要把上述两类事项收口为“可验证、可审计、可长期演进”的最终状态，并将清理动作拆成可 review 的 PR 序列。

## 2. 目标与非目标

### 2.1 目标（Done 的定义）

- [ ] P3 完整收尾：输出并维护《旧 UI → MUI 映射表》，且能用事实源与门禁验证“旧 UI 不可达/不可构建/不可误用”，业务入口在 MUI 中可发现、可操作（至少覆盖当前已实现能力）。
- [ ] Person 页面去除“ignored as-of”输入，使时间上下文口径与 `DEV-PLAN-102` 一致（不制造“伪需求参数”与歧义 UI）。
- [ ] P6 完整收尾：执行 `apps/web-mui` → `apps/web` 机械改名 + 全仓引用更新；CI 触发器、构建脚本、文档 SSOT 同步更新；本地能跑通 UI 构建与全门禁。
- [ ] 清理残留旧 UI 死代码/测试（不改变对外契约）：移除或隔离不可达的 HTMX/Nav/Topbar/旧登录表单渲染逻辑与其测试，避免后续误引入“兼容别名窗口/legacy 回退”。

### 2.2 非目标（本计划不做）

- 不引入新的 UI 功能、不做大规模 UI 重构（仅收尾：入口/口径/命名/可达性/证据）。
- 不更改 API 契约/鉴权模型（若确需调整，必须先更新对应 dev-plan，遵循 Contract First）。
- 不更改 localStorage key、package name 等“运行态标识”（除非发现明确冲突/门禁失败；若要改，必须单独开子任务并提供迁移策略，避免暗改造成用户侧状态丢失）。

## 3. 范围与约束（不变量）

- **No Legacy**：不引入 `/login` 兼容窗口、旧路由 alias、read=legacy 等回退通道（见 `DEV-PLAN-004M1`）。
- **以 allowlist/route_class 为准**：UI vs API 的未登录行为以 `route_class` 判定；内部 API 必须 JSON 401，不得 302（见 `DEV-PLAN-103` 与 `DEV-PLAN-102`）。
- **机械改名不夹带功能改动**：P6 的目录改名必须是“机械改名 + 引用更新”，不混入功能变更；功能变更必须在独立 PR、独立勾选项里执行。

## 4. 交付物

- `docs/dev-records/dev-plan-103a-execution-log.md`：
  - 《旧 UI → MUI 映射表》（表格化，含状态/证据）。
  - 本计划关键命令执行记录（时间戳、结果、环境要点）。
- 代码与配置收口：
  - `apps/web/`（替代 `apps/web-mui/`）作为唯一前端工程目录。
  - 触发器/脚本/文档 SSOT 不再引用 `apps/web-mui`。
- `DEV-PLAN-103` 更新：
  - P3/P6 对应条目可被勾选为完成（并在验收标准处体现证据入口）。

## 5. 实施步骤（建议按 PR 拆分）

### PR-103A-0：收尾准备与证据骨架

1. [ ] 建立本计划执行日志：`docs/dev-records/dev-plan-103a-execution-log.md`（落盘后再开始收尾实施）。
2. [ ] 在执行日志中生成《旧 UI → MUI 映射表》的初版（以 `internal/server/handler.go` + `config/routing/allowlist.yaml` + `apps/*/src/router/index.tsx` 为事实源），并标注迁移/删除状态。

> 映射表建议字段（可按实际调整）：旧路径/模式、旧 route_class、旧时间上下文(A/B/C)、旧状态(已移除/不可达/仍有死代码)、新 MUI path（`/app` 内路由）、新 API（如有）、permissionKey、备注（证据与清理点）。

### PR-103A-1：P3 收尾（Person 页面时间口径清理 + 映射表补齐）

3. [ ] Person 页面去除“ignored as-of”输入与相关状态变量：  
   - 原则：Person 页面若不需要时间上下文，应按 `DEV-PLAN-102` 归类为 C 类（无时间上下文），不暴露日期输入以避免歧义。
4. [ ] 在映射表中补齐 Person/JobCatalog/Staffing/SetID 的“入口→页面→API→权限→时间口径”信息，并标注当前实现状态（已迁移/占位/未实现）。

### PR-103A-2：旧 UI 残留清理（死代码/测试/绕过口径）

5. [ ] 清理不可达的旧 UI 渲染辅助函数与测试（例如 HTMX Nav/Topbar/旧登录表单渲染），避免后续误用：  
   - 重点检查：是否仍存在“绕过/放行 `/login`”之类的兼容口径；若存在则移除，并用测试锁定“不提供兼容窗口”。
6. [ ] 更新 `DEV-PLAN-103` 的验收/风险说明：把“残留清理”与“证据表”链接到本计划执行日志，形成可追溯收口点。

### PR-103A-3：P6 工程改名（apps/web-mui → apps/web，机械改名）

7. [ ] 执行目录改名：`apps/web-mui` → `apps/web`（仅机械改名 + 引用更新，不夹带功能改动）。
8. [ ] 全仓机械更新引用（并以门禁为准收口）：  
   - 构建脚本（如 `scripts/ui/*`）  
   - CI 触发器（如 `scripts/ci/paths-filter.sh`）  
   - 文档 SSOT（如 `DEV-PLAN-011`、`DEV-PLAN-010`、`DEV-PLAN-103`、`AGENTS.md` 等）  
   - E2E 触发条件与可能的路径硬编码引用（如存在）。

### PR-103A-4：验收与门禁对齐（本地可复现）

9. [ ] 跑 UI 构建门禁并提交生成物（如有）：`make css`，然后 `git status --short` 必须为空。
10. [ ] 跑全门禁对齐 CI：`make preflight`。
11. [ ] 将关键命令与结果登记到 `docs/dev-records/dev-plan-103a-execution-log.md`（时间戳、结果；不在 dev-plan 内复制命令矩阵）。
12. [ ] 回写 `DEV-PLAN-103`：将 P3/P6 条目更新为 `[x]`，并在验收标准中引用本计划的执行日志作为证据入口。

## 6. 验收标准

- [ ] 《旧 UI → MUI 映射表》存在且可追溯（明确事实源与每条路由的状态/证据）。
- [ ] `apps/web-mui` 不再存在；前端工程目录为 `apps/web`；全仓无 `apps/web-mui` 路径引用（允许“历史记录”文档明确标注除外，但原则上应收敛）。
- [ ] Person 页面不再出现 “As-of (ignored)” 或等价输入；时间上下文口径与 `DEV-PLAN-102` 一致。
- [ ] 不存在 `/login` HTML 页面或兼容跳转窗口；不在中间件层放行旧路径形成潜在 backdoor。
- [ ] 本地 `make css` 与 `make preflight` 可通过，且证据记录已落盘。

## 7. 风险与缓解

- 风险：机械改名牵涉面大，遗漏引用导致 CI gate/E2E 不触发或构建失败。  
  缓解：先用 `rg` 全仓扫描 `apps/web-mui`，再以 `make preflight` 与 CI 路径触发器双重验证。
- 风险：把“死代码清理”与“改名”混在同一 PR，review/回滚困难。  
  缓解：按 PR 拆分，保持每个 PR 只有一个主轴。

## 8. 交付物清单（最终应出现的文件/变化）

- `docs/dev-plans/103a-dev-plan-103-closure-p3-p6-apps-web-rename.md`（本文件）
- `docs/dev-records/dev-plan-103a-execution-log.md`（执行证据）
- `apps/web/`（替代 `apps/web-mui/`）


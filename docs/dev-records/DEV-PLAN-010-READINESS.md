# DEV-PLAN-010 Readiness（证据记录）

> 目的：把 “P0-Ready” 的关键结论固化为可审计证据（时间戳/环境/命令/结果）。
> 本文件为模板；每次完成一个里程碑，在对应小节补齐证据。

## 1. 基本信息

- repo: Bugs-And-Blossoms
- 分支保护：main 禁止直推/禁止 force-push/必须 PR（GitHub 侧配置）

## 2. Required Checks（不出现 skipped）

- `Code Quality & Formatting`：`make check fmt` / `make check lint`
- `Unit & Integration Tests`：`make test`
- `Routing Gates`：`make check routing`
- `E2E Tests`：`make e2e`

证据（贴运行时间与结论链接/截图均可）：
- 日期：
- 运行环境（本机/CI）：
- 结论：

## 3. UI 壳（用户可见性）

占位：完成 UI 壳后记录“能打开壳、能看到 4 模块入口与占位页”的证据。

## 4. 最小登录链路

占位：完成 tenant 解析（fail-closed）与最小登录闭环后记录证据。

## 5. Routing Gates

占位：记录 allowlist SSOT 存在、entrypoint key 冻结、门禁阻断示例（缺失/漂移时失败）。

## 6. DB/迁移闭环（至少 iam）

占位：记录 `make iam plan` / `make iam migrate up` 的 smoke 证据与 RLS fail-closed 证据链接。


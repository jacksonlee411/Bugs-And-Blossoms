# DEV-PLAN-170 执行日志

## 执行记录

| 时间（UTC） | 命令 | 结果 |
| --- | --- | --- |
| 2026-02-24 10:57 UTC | `pnpm -C apps/web lint` | 通过 |
| 2026-02-24 10:57 UTC | `pnpm -C apps/web typecheck` | 通过 |
| 2026-02-24 10:57 UTC | `pnpm -C apps/web test` | 17 files / 61 tests 通过 |
| 2026-02-24 10:57 UTC | `pnpm -C apps/web build` | 通过 |
| 2026-02-24 10:57 UTC | `make generate && make css` | 通过（产物已更新） |
| 2026-02-24 10:57 UTC | `make check doc` | `[doc] OK` |
| 2026-02-24 10:57 UTC | `make check lint && make test` | 通过 |
| 2026-02-24 10:57 UTC | `make check routing && make check capability-route-map && make check error-message` | 通过 |
| 2026-02-24 10:57 UTC | `pnpm -C e2e exec playwright test tests/tp060-04-orgunit-details-two-pane.spec.js tests/tp060-02-orgunit-record-wizard.spec.js` | 失败（环境未启动 `http://localhost:4434`，`ECONNREFUSED`） |
| 2026-02-24 11:04 UTC | `make e2e` | 通过（8/8） |
| 2026-02-24 11:04 UTC | `make preflight` | 通过（含全部门禁 + E2E） |
| 2026-02-24 13:45 UTC | `pnpm -C apps/web lint` | 通过 |
| 2026-02-24 13:45 UTC | `pnpm -C apps/web typecheck` | 通过 |
| 2026-02-24 13:46 UTC | `pnpm -C apps/web test` | 17 files / 61 tests 通过 |
| 2026-02-24 13:46 UTC | `pnpm -C e2e exec playwright test --list tests/tp060-04-orgunit-details-two-pane.spec.js` | 通过（用例可被 Playwright 正常发现） |
| 2026-02-24 13:46 UTC | `make check doc` | `[doc] OK` |
| 2026-02-24 13:49 UTC | `pnpm -C apps/web lint && pnpm -C apps/web typecheck && pnpm -C apps/web test` | 通过（评审收口修订后复检） |
| 2026-02-24 13:49 UTC | `make check doc` | `[doc] OK` |
| 2026-02-24 13:53 UTC | `git diff --check` | 通过（无 trailing whitespace） |
| 2026-02-24 13:53 UTC | `make check doc` | `[doc] OK` |
| 2026-02-24 13:57 UTC | `make preflight` | 失败（`tp060-04` 严格模式匹配到 2 个 `Event UUID` 节点，触发 Playwright strict mode） |
| 2026-02-24 13:59 UTC | `make e2e` | 通过（8/8） |
| 2026-02-24 14:00 UTC | `make preflight` | 通过（含全部门禁 + E2E） |
| 2026-02-24 14:06 UTC | `make dev-up && make iam/orgunit/jobcatalog/person/staffing migrate up` | 通过（本地依赖与模块迁移就绪） |
| 2026-02-24 14:11 UTC | `make dev-kratos-stub + TRUST_PROXY=1 make dev-server + make dev-superadmin` | 通过（4433/4434/8080/8081 健康检查通过） |
| 2026-02-24 14:11 UTC | `seed_kratosstub_identity.sh`（3 租户账号） | 通过（admin0/admin/admin2 全部写入） |
| 2026-02-24 14:12 UTC | 多租户登录与隔离 curl 验证 | 通过（3×`POST /iam/api/sessions`=204；同租户 200；跨租户 401） |
| 2026-02-24 14:12 UTC | `pnpm -C e2e exec playwright test tests/tp060-04-orgunit-details-two-pane.spec.js` | 通过（1/1） |
| 2026-02-24 23:59 UTC | `pnpm -C apps/web lint && pnpm -C apps/web typecheck && pnpm -C apps/web test` | 通过（17 files / 61 tests） |
| 2026-02-25 00:00 UTC | `pnpm -C e2e exec playwright test tests/tp060-04-orgunit-details-two-pane.spec.js` | 失败（未启动 superadmin，`http://localhost:8081` 拒绝连接） |
| 2026-02-25 00:01 UTC | `make e2e` | 失败（本机已有 kratosstub 占用 4434 端口） |
| 2026-02-25 00:05 UTC | `make generate && make css` | 通过（Web 产物更新；清除旧上下文摘要前端缓存） |
| 2026-02-25 00:06 UTC | `make e2e` | 通过（8/8） |
| 2026-02-25 00:07 UTC | `make preflight` | 通过（含全部门禁 + E2E） |

## 结论

- DEV-PLAN-170 的“仅详情页壳层改造、弹窗不改”已落地。
- 首次单独执行弹窗回归 E2E 因本地依赖环境未启动失败；复检环境并执行 `make e2e` 与 `make preflight` 后，E2E 与全部门禁均已通过。
- DEV-PLAN-170A 评审收口完成：审计上下文改为绑定 `selectedAuditEvent`，修复 `as_of` 硬编码，新增 `org-context-summary` 断言锚点并同步 E2E 用例断言。
- 170A 实施收口阶段修复了 E2E 选择器歧义（`Event UUID` 在上下文区与详情区重复出现导致 strict mode 失败），已改为行级精确匹配并复检通过。
- 按 `bugs-and-blossoms-dev-login` 流程完成“全服务启动 + 账号 seed + 多租户登录/隔离 + 170A 指定 E2E”验收，当前可在 `localhost/saas.localhost/tenant2.localhost:8080` 直接联调。
- DEV-PLAN-170B 已完成：移除详情页顶部上下文区，URL 恢复定位切换为“左栏选中 + 右栏事实锚点”，并完成“版本详情/事件详情”标题边界与字段排版收敛。

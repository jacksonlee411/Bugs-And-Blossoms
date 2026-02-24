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

## 结论

- DEV-PLAN-170 的“仅详情页壳层改造、弹窗不改”已落地。
- 首次单独执行弹窗回归 E2E 因本地依赖环境未启动失败；复检环境并执行 `make e2e` 与 `make preflight` 后，E2E 与全部门禁均已通过。

# DEV-PLAN-101I 执行日志

> 记录维度：按 readiness/门禁证据固化（命令 + 结果 + 时间）。

## 2026-02-17

### 实施内容

- MUI OrgUnit 详情页新增“新增版本 / 插入版本”显式入口（Record Wizard）。
- 前端实现生效日期规则：add/insert 默认值、区间提示、无槽位阻断（提交前校验 + fail-closed）。
- append-capabilities 由“选中版本 effective_date”改为“弹窗内 effective_date + 变更类型”联动（避免错误能力缓存）。
- i18n：新增 add/insert/日期限制与校验相关文案（en/zh）。
- 单测：补齐日期规则纯函数测试。

### 门禁证据

- `pnpm --dir apps/web lint`：PASS
- `pnpm --dir apps/web typecheck`：PASS
- `pnpm --dir apps/web test`：PASS
- `make generate && make css`：PASS（`internal/server/assets/web` 已更新）
- `make check doc`：PASS


# DEV-PLAN-291 风险清单

- 生成时间：2026-03-09 01:54:16 CST

## 高风险
- `R9` 未通过：`docs/dev-records/assets/dev-plan-290/tp290-real-case-evidence-index.json` 仍记录 Case 2/3/4 未通过，原因是 pending placeholder bubble，直接阻断 `291 -> 285` 交接。
- `291` 当前不能替代 `288/290` 的业务与消息树证据；若强行封板会违反 `DEV-PLAN-271` 与 `DEV-PLAN-280` 的硬门槛。

## 中风险
- `292` compat alias 当前仍保留；虽然本轮复核确认其未形成第二正式 API 面，但后续若继续扩语义，需要重新执行 `291`。
- 任一影响 vendored UI、formal entry、compat API、routing、no-legacy 的合入都会使本轮 `291` 证据失效。
- `tp288` 已刷新为完成态，但只要 `290A` 或 `240C/240D/240E` 发生影响性合入，`R9` 仍需重新核对 `288/290` 两份索引的新鲜度。

## 低风险
- `R4` 当前依赖 compose config 与 lock 文件静态比对；若后续需要更强运行态版本确认，可补专用脚本，但不影响本轮结论。

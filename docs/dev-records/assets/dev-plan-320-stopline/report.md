# DEV-PLAN-320 Stopline Explain Report

- captured_at: `2026-04-10T01:24:38Z`
- as_of_date: `2026-01-01`

## Samples

- heavy tenant: `00000000-0000-0000-0000-000000000001` (`Local Tenant`, `localhost`)
- heavy root: `1` / `AAAAAAAB`
- heavy details target: `2` / `AAAAAAAC`
- heavy subtree filter: `3` / `AAAAAAAB.AAAAAAAD`
- heavy move: `3` -> `2` at `2026-01-02`
- chain tenant: `00d01cf4-7633-4ac6-93c7-748b2a6d6678` (`TP060-02 Tenant 1775779599776`, `t-tp060-02-1775779599776.localhost`)
- chain business unit: `RND9776` setid=`S2601`

## source-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.573 | 2.379 | 11 | 0 | 0 | 0 | `source-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.654 | 2.265 | 19 | 0 | 0 | 0 | `source-real-org-children.explain.json` |
| `org-details` | 0.877 | 2.776 | 12 | 0 | 0 | 0 | `source-real-org-details.explain.json` |
| `org-full-name-rebuild` | 0.586 | 1.672 | 1 | 0 | 0 | 0 | `source-real-org-full-name-rebuild.explain.json` |
| `org-move` | 5.489 | 9.075 | 285 | 0 | 0 | 0 | `source-real-org-move.explain.json` |
| `org-roots` | 0.705 | 2.319 | 17 | 0 | 0 | 0 | `source-real-org-roots.explain.json` |
| `org-search` | 0.512 | 1.917 | 22 | 0 | 0 | 0 | `source-real-org-search.explain.json` |
| `org-subtree-filter` | 0.150 | 1.240 | 1 | 0 | 0 | 0 | `source-real-org-subtree-filter.explain.json` |
| `setid-resolve` | 0.374 | 1.501 | 14 | 0 | 0 | 0 | `source-real-setid-resolve.explain.json` |
| `staffing-by-org` | 0.239 | 0.550 | 8 | 0 | 0 | 0 | `source-real-staffing-by-org.explain.json` |

## target-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.529 | 1.999 | 9 | 0 | 0 | 0 | `target-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.564 | 1.455 | 33 | 0 | 0 | 0 | `target-real-org-children.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-details` | 0.401 | 1.885 | 12 | 0 | 0 | 0 | `target-real-org-details.explain.json` |

说明：manager/person 关联未纳入当前 target bootstrap

| `org-full-name-rebuild` | 4.181 | 1.080 | 152 | 0 | 6 | 0 | `target-real-org-full-name-rebuild.explain.json` |
| `org-move` | 0.582 | 3.596 | 1 | 0 | 0 | 0 | `target-real-org-move.explain.json` |
| `org-roots` | 0.383 | 1.153 | 17 | 0 | 0 | 0 | `target-real-org-roots.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-search` | 0.256 | 1.137 | 18 | 0 | 0 | 0 | `target-real-org-search.explain.json` |
| `org-subtree-filter` | 0.247 | 0.927 | 17 | 0 | 1 | 0 | `target-real-org-subtree-filter.explain.json` |

## target-shadow

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `setid-resolve` | 0.900 | 1.735 | 34 | 0 | 0 | 0 | `target-shadow-setid-resolve.explain.json` |

说明：consumer runtime 的 SetID schema 尚未切到 org_node_key；此处使用 stopline shadow 表按 org_code -> org_node_key 导入当前态样本，仅用于 explain 对比

| `staffing-by-org` | 0.272 | 0.642 | 23 | 0 | 0 | 0 | `target-shadow-staffing-by-org.explain.json` |

说明：consumer runtime 的 Staffing schema 尚未切到 org_node_key；此处使用 stopline shadow 表按 org_code -> org_node_key 导入当前态样本，仅用于 explain 对比


## Notes

1. `target-real` 当前只覆盖 `orgunit` 新 schema；SetID / Staffing / Person 相关 post-cutover explain 尚未纳入 dedicated target bootstrap。
2. `target-shadow` 使用 stopline shadow 表承载 SetID / Staffing 当前态样本，并通过 `org_code -> org_node_key` 映射导入 dedicated target；该证据仅用于 stopline 对比，不等同于 consumer runtime 已完成 cutover。
3. `org-move` 与 `org-full-name-rebuild` 均在事务内执行 `EXPLAIN (ANALYZE, BUFFERS)`，采集后由调用侧回滚。
4. 原始 explain JSON 见同目录 `*.explain.json`。

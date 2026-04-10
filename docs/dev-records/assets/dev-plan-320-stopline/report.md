# DEV-PLAN-320 Stopline Explain Report

- captured_at: `2026-04-10T23:00:08Z`
- as_of_date: `2026-04-11`

## Samples

- heavy tenant: `00000000-0000-0000-0000-000000000001` (`Local Tenant`, `localhost`)
- heavy root: `1` / `AAAAAAAB`
- heavy details target: `2` / `AAAAAAAC`
- heavy subtree filter: `3` / `AAAAAAAB.AAAAAAAD`
- heavy move: `3` -> `2` at `2026-04-12`
- chain tenant: `00d01cf4-7633-4ac6-93c7-748b2a6d6678` (`TP060-02 Tenant 1775779599776`, `t-tp060-02-1775779599776.localhost`)
- chain business unit: `RND9776` setid=`S2601`

## source-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.206 | 1.140 | 11 | 0 | 0 | 0 | `source-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.361 | 0.834 | 17 | 0 | 0 | 0 | `source-real-org-children.explain.json` |
| `org-details` | 0.351 | 0.811 | 12 | 0 | 0 | 0 | `source-real-org-details.explain.json` |
| `org-full-name-rebuild` | 0.591 | 0.468 | 1 | 0 | 0 | 0 | `source-real-org-full-name-rebuild.explain.json` |
| `org-move` | 4.067 | 5.400 | 276 | 10 | 22 | 0 | `source-real-org-move.explain.json` |
| `org-roots` | 0.329 | 1.103 | 15 | 0 | 0 | 0 | `source-real-org-roots.explain.json` |
| `org-search` | 0.166 | 0.592 | 21 | 0 | 0 | 0 | `source-real-org-search.explain.json` |
| `org-subtree-filter` | 0.069 | 0.404 | 1 | 0 | 0 | 0 | `source-real-org-subtree-filter.explain.json` |
| `setid-resolve` | 0.253 | 0.897 | 14 | 0 | 0 | 0 | `source-real-setid-resolve.explain.json` |
| `staffing-by-org` | 0.131 | 0.287 | 8 | 0 | 0 | 0 | `source-real-staffing-by-org.explain.json` |

## target-real

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `org-ancestor-chain` | 0.182 | 0.608 | 6 | 0 | 0 | 0 | `target-real-org-ancestor-chain.explain.json` |
| `org-children` | 0.128 | 0.295 | 33 | 0 | 0 | 0 | `target-real-org-children.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-details` | 0.126 | 0.577 | 12 | 0 | 0 | 0 | `target-real-org-details.explain.json` |

说明：manager/person 关联未纳入当前 target bootstrap

| `org-full-name-rebuild` | 1.080 | 0.221 | 127 | 0 | 1 | 1 | `target-real-org-full-name-rebuild.explain.json` |
| `org-move` | 0.145 | 0.826 | 1 | 0 | 0 | 0 | `target-real-org-move.explain.json` |
| `org-roots` | 0.201 | 0.380 | 17 | 0 | 0 | 0 | `target-real-org-roots.explain.json` |

说明：target bootstrap 当前仅覆盖 org 纯 schema；无 person/setid/staffing 关联表

| `org-search` | 0.079 | 0.370 | 18 | 0 | 0 | 0 | `target-real-org-search.explain.json` |
| `org-subtree-filter` | 0.091 | 0.308 | 15 | 0 | 0 | 0 | `target-real-org-subtree-filter.explain.json` |
| `staffing-by-org` | 0.091 | 0.275 | 17 | 0 | 0 | 0 | `target-real-staffing-by-org.explain.json` |

说明：使用 committed staffing target schema（`staffing.position_versions`）采集 explain；当前态样本通过 org_code -> org_node_key 映射导入 dedicated target，但不再走 shadow 表


## target-shadow

| key | execution_ms | planning_ms | shared_hit | shared_read | shared_dirtied | shared_written | explain |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| `setid-resolve` | 0.426 | 0.725 | 31 | 0 | 0 | 0 | `target-shadow-setid-resolve.explain.json` |

说明：consumer runtime 的 SetID schema 尚未切到 org_node_key；此处使用 stopline shadow 表按 org_code -> org_node_key 导入当前态样本，仅用于 explain 对比


## Notes

1. `target-real` 当前覆盖 `orgunit` 新 schema与 committed `staffing.position_versions`；其中 Staffing 当前态样本通过 `org_code -> org_node_key` 映射导入 dedicated target。
2. `target-shadow` 目前仅保留 SetID binding explain，对应 consumer runtime 尚未完成 target-real schema cutover 的链路。
3. `org-move` 与 `org-full-name-rebuild` 均在事务内执行 `EXPLAIN (ANALYZE, BUFFERS)`，采集后由调用侧回滚。
4. 原始 explain JSON 见同目录 `*.explain.json`。

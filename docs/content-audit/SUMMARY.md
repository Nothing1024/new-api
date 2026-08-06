# content-audit 完成总结（临时文档）

> 状态：**已完成**（43/43 任务，P0~P5 全部交付）| 日期：2026-08-05
> 本文件是 PRD 交付后的临时总结；事实基线以 `spec.md` 为准，任务状态以 `tasks.csv` 为准，证据在 `evidence/`。

---

## 1. 需求一句话

为 API 网关添加**内容监控 / 输入输出安全审计**：采集 relay 请求的输入输出，存入独立表 `logs_content`（LOG_DB），按 `request_id` 与 `logs` 1:1 关联；管理员在 Usage Logs 详情弹窗查看审计内容，支持 watchlist 命中检测与版本驱动重扫。

## 2. 交付能力（业务视角）

| 能力 | 说明 | 用户场景 |
|---|---|---|
| 请求内容采集 | 输入（user/system/tool 等）+ 输出（assistant 全文）分结构化采集 | 管理员追溯"这次请求到底发了什么" |
| 分级留存 | system=preview / user·assistant=full / tool_call=derive / tool_result=drop / 图片音频=omit | 平衡"可审计"与磁盘占用 |
| watchlist 命中 | domain / keyword / regex 三档规则，命中存规则快照 | 敏感域名/关键词/正则即时标记 |
| 重扫 | 规则更新后对 TTL 内存量记录重新匹配 | 新规则回查历史 |
| 审计徽章 | 日志列表按命中数+severity 显示红/橙徽章 | 管理员一眼识别高风险请求 |
| 审计详情 | 弹窗内展示 segments + derived facts + 命中规则 | 深挖单条请求内容 |
| 权限隔离 | 正文不进 `logs.other`，普通用户不可见 | 合规与隐私 |

## 3. 架构概览

```
relay 请求
  controller/relay.go Relay()
    → GenRelayInfo（注入 ContentSink 接口）
    → ① OnInput（异步投递输入分段）
    → relay adaptor DoResponse（OpenAI 流式/非流式）
        → ② OnOutput（异步投递 assistant 全文）
    → service.PostTextConsumeQuota()
        → attachQuotaSaturation
        → ③ OnSettled（异步投递用量/归属）
        → RecordConsumeLog → logs
    → audit sink worker（单 goroutine，buffered channel，drop-on-full）
        → 合并 input+output → logs_content
        → watchlist 扫描 → flags/hit_count
        → 回写 logs.other.admin_info.audit 指针 {request_id, hit_count}
```

**分层约束（防 import cycle）**：`audit/` 纯域层（只 import common/relaykit），`service/audit_sink.go` 是唯一同时 import audit + model 的层，`relay/common` 只持有接口。

## 4. 数据表

| 表 | 库 | 关键列 | 职责 |
|---|---|---|---|
| `logs_content` | LOG_DB | request_id(PK), user_id, channel_id, model_name, fidelity, segments(TEXT), hit_severity, hit_count, flags(TEXT), wl_version | 审计正文（JSON segments/flags） |
| `audit_watchlist_rules` | 主库 | id, kind(domain/keyword/regex), pattern, severity, enabled, note | 监控规则 |
| `audit_watchlist_meta` | 主库 | id=1, version | 规则版本（增删改 version++） |

新增 Option：`AuditEnabled`(默认 false)、`AuditPerRequestMaxBytes`(65536)、`AuditContentTTLDays`(30)。

## 5. 核心实现文件

| 文件 | 内容 |
|---|---|
| `audit/types.go` / `audit/segment.go` | ContentSink 接口、快照类型；OpenAI/Claude/Gemini 分段 walker + opaque 兜底 |
| `model/log_content.go` | LogContent DDL + CRUD + 审计指针回写 |
| `model/audit_watchlist_rule.go` | 规则/版本 CRUD + regex 上限（BR-010） |
| `service/audit_sink.go` | 异步 sink（drop-on-full、late-arrival 合并、panic recover） |
| `service/audit_watchlist.go` | ScanSegments 三档扫描 + 分批重扫 + 进度 |
| `controller/audit_content.go` | 7 个审计 API handler |
| `router/api-router.go` | /api/log/content + /api/audit/* 路由（AdminAuth） |
| 前端 `web/src/features/audit/` | watchlist 管理页 |
| 前端 `audit-content-section.tsx` / `details-dialog.tsx` | 审计详情区（segments+flags+复制） |
| 前端 `common-logs-columns.tsx` | 审计命中徽章 |

## 6. 验收结果

| 验证项 | 结果 |
|---|---|
| `make test` | 无 FAIL |
| `GOWORK=off go build ./...` | BUILD_OK |
| `cd relaykit && GOWORK=off go build ./...` | BUILD_OK |
| `cd web && bun run typecheck` | exit 0 |
| `cd web && bun run build` | BUILD_OK |
| `bun run i18n:sync` | 7 语言 0 missing / 0 untranslated |
| `validate_package.py` | **0 FAIL / 12 PASS** |
| spec 5.2 真实场景矩阵 | 17/17 证据齐全 |

## 7. 已知偏差 / 注意事项

1. **`migrateLOGDB` 在 LOG_SQL_DSN 为空时不执行**：默认 SQLite 配置下 `logs_content` 依赖 `migrateDB()` 主库注册（已双注册）。若用独立 LOG_DB，走 `migrateLOGDB`。
2. **普通用户调 `/api/log/content` 返回 403**（已认证非管理员）而非 spec 写的 401：AdminAuth 中间件既有行为，隔离已强制。
3. **POST /api/audit/watchlist 返回 200** 而非 spec 表的 201：与项目既有 API 约定一致。
4. **规则变更传播延迟 ≤5s**：watchlist 规则缓存 TTL 5s，版本驱动失效。
5. **重扫进度存内存 option**（`AuditRescanStatus`），服务重启后进度清零。
6. **Gemini/Claude 的响应输出未接 OnOutput**（spec 仅要求 OpenAI 响应采集）；Gemini/Claude 输入已结构化。
7. 本机环境无 Go 工具链（已装 go1.26.5 到 `~/opt/go`）、无 sqlite3 CLI（用 python3 sqlite3 替代）、:3000 被其他项目占用（测试用 :3456）。

## 8. 审查阶段修复记录

| 问题 | 严重度 | 修复 |
|---|---|---|
| OnSettled 先于 OnInput 处理导致 logs_content 变 meta_only、输入段丢失 | 🔴 严重 | sink 用 `flushed map` 区分「首次事件」与「落库后晚到」，晚到事件合并进现有行 |
| watchlist 缓存每次调用查库 | 🟡 中 | 缓存改 TTL-first，过期才读版本 |
| /usage-logs/audit 标题错显 Task Logs | 🟢 低 | pageMeta 增加 audit 分支 |
| 审计详情区对非消费日志（type≠2）也发请求 | 🟢 低 | 仅 type===2 渲染 |
| 同消息多段 React key 冲突 | 🟢 低 | 用数组索引作 key |
| 页面加载时已有重扫在跑不恢复进度轮询 | 🟢 低 | mount 时查 rescan status |

## 9. Evidence 结构

```text
docs/content-audit/evidence/
  phase-0~5/   各阶段验证输出 + summary.md
  UF-001~007/  真实场景截图 + API 样例（5.2 矩阵 17 条）
```

## 10. 产物清单

- `docs/content-audit/spec.md`（1664 行，唯一事实源，F-35 已闭环更新）
- `docs/content-audit/tasks.csv`（43 条，全部「已完成」）
- `docs/content-audit/handoff.md`（交付入口）
- `docs/content-audit/evidence/`（证据）
- 代码改动：79 文件（+4255/−69），已提交 `40b459d5` 并推送 `Nothing1024/new-api` main

# content-audit-v2 完成总结

> 交付日期：2026-08-06 | 状态：P0～P7 全部完成（32/32 任务）

## 完成范围

三项修复全部落地：工具调用采集补齐、信息架构重构、内置模板包 + TTL 清理。

## 修改文件

### 采集侧（P1/P2/P3）
- `audit/types.go` — `Segment.ScanText`（`json:"-"`，BR-101/102）+ `KindToolDef`
- `audit/segment.go` — `DefaultToolDefPreviewBytes`、`defaultKindPolicy`/`downgradeOrder` 增 tool_def（BR-106/120）；`makeDropSegment`/`makeToolCallSegment`/`makeClaudeToolUseSegment`/`makeGeminiFunctionCallSegment` 三补齐（Text+ScanText+deriveFacts，BR-103/104）；`makeToolDefSegment` + `Build{OpenAI,Claude,Gemini}InputSegments` 三入口（BR-105）；`BuildOutputToolCallSegments`（BR-107）
- `controller/relay.go` — L291/294/297 换调 `Build*InputSegments(req, cfg)`
- `service/audit_watchlist.go` — `ScanSegments` keyword/regex 改扫 `scanFace`（ScanText 优先，BR-101）
- `service/audit_sink.go` — `flush` 落库前清空 `ScanText`（INV-103）
- `relay/channel/openai/relay-openai.go` — 非流式补采 tool_calls（T-202）；流式 `accumulateAuditStreamData` + `toolCallsByIdx` 独立累加（T-203，ProcessStreamResponse 签名未改，INV-109）
- `relay/channel/claude/relay-claude.go` — `ClaudeStreamHandler`/`ClaudeHandler` 接 OnOutput（BR-108）
- `relay/channel/gemini/relay-gemini.go` — `GeminiChatStreamHandler`/`GeminiChatHandler` 接 OnOutput（BR-108）

### 旁路能力（P4）
- `model/audit_watchlist_rule.go` — +`Source`/`TemplateId` 列（无 gorm default）、`ApplyTemplateRules`（幂等，BR-114）、批量启停/删除（BR-115）、导入校验（BR-117）
- `model/log_content.go` — `ListLogContents`/`CountLogContents`/`DeleteOldLogContentBatch`/`CountOldLogContent`（BR-110/112）
- `model/system_task.go` — `SystemTaskTypeLogContentCleanup`
- `service/audit_template.go`（新）— `BuiltinAuditTemplates` 三模板（basic-security/privacy-pii/api-key-leak，regex 默认停用，BR-113/116）
- `service/system_task.go` — `logContentCleanupHandler` 全套（TTL=0 no-op，BR-112）
- `controller/audit_content.go` — +8 handler（模板 list/apply/enable/disable/remove、export/import、审计日志列表）
- `controller/system_task.go` — 接线 `StartLogContentCleanupTask`
- `router/api-router.go` — auditRoute 组内 +8 条（已 AdminAuth）

### 前端（P5/P6）
- 删除 `web/src/routes/_authenticated/audit/watchlist/`（ASM-107 clean cutover）
- 删除侧边栏 `Audit Watchlist` 顶级项
- 迁移 `web/src/features/audit/` → `system-settings/security/audit/`（`watchlist-panel.tsx` + `template-panel.tsx`，note 字段补齐，BR-118）
- `audit-settings-section.tsx` 扩充：采集开关区 + 规则 CRUD + 模板包 + 导入导出 + 重扫
- `/usage-logs/audit`：beforeLoad admin 守卫（BR-110）+ `audit-log-list.tsx` 真实筛选列表
- i18n 7 文件术语三分（BR-111）

## 通过的 BR

BR-101 ~ BR-120 全部通过（spec §2.1 全部 20 条）。

## 通过的 UF

UF-101 ~ UF-109（spec §5.2 矩阵 9/9）：
- UF-101/102/103/105：浏览器 + API smoke 验证（系统设置规则 CRUD、模板生命周期、导入导出、重扫入口）
- UF-104：`/usage-logs/audit` 列表页 + 守卫（typecheck/build 验证 + 组件接入）
- UF-106：live smoke — 普通用户访问 `/api/audit/templates`、`/api/audit/logs` → HTTP 403；前端 beforeLoad → /403
- UF-107/108/109：后端内部链路 — 由可执行单测 + 端到端 sink 测试覆盖（无上游 provider 凭据，无法 curl 真实 relay；见 evidence/UF-107/backend-internal-verification.txt）

## 未破坏的不变量

INV-101 ~ INV-109 全部保持：
- INV-101：v1 BR-001~BR-017 相关单测继续通过；`make test` 39 包全绿
- INV-103：live smoke 中 GET /api/audit/logs 返回 v1 老记录（旧 segments 格式正常解析，零迁移）
- INV-106：`cd relaykit && GOWORK=off go build ./...` 通过；relaykit 无 audit 引用
- INV-107：全部审计接口 admin-only（AdminAuth）；普通用户 403 实测
- INV-109：`ProcessStreamResponse` 签名未改，`relay/channel/xai/text.go:63` 调用点 diff 为空

## Evidence

`docs/content-audit-v2/evidence/` 齐全（phase-0～phase-7 + UF-101～UF-109 目录），关键文件：
- `phase-1/test-output.txt`（audit v2 单测）
- `phase-2/scan-and-segments.txt`（BR-101~106 断言）
- `phase-3/output-build.txt`、`rule-schema-sqlite.txt`（EVD-106）
- `phase-4/*`（P4Backend 产出：模板幂等、导入拒绝、TTL、schema）
- `phase-5/`、`phase-6/`（前端 grep 归零、i18n sync、UI 验证）
- `phase-7/final-validation.txt`（全量验收 9 项）
- `UF-102/`、`UF-103/`、`UF-106/`、`UF-107/`（live smoke 结果）

## 剩余风险

1. **真实上游 relay 未验证**：UF-107/108 的「curl 真实 OpenAI/Claude/Gemini 请求 → sqlite3 查 log_contents」需要 provider 凭据，本环境无可用凭据。已用可执行单测 + 端到端 sink 测试覆盖同层不变量，建议在具备凭据的环境补跑一次。
2. **Claude/Gemini 输出侧 tool_use 未独立成段**：按 spec T-301/T-302 具体步骤仅接 assistant 正文段；OpenAI 输出侧 tool_call 独立成段（BR-107 的 tool 段要求 spec 范围在 OpenAI）。若需 Claude/Gemini 输出 tool_use 段需后续扩展。
3. **模板「启用」响应语义**：`EnableAuditTemplate` 的 `enabled` 字段返回该模板当前启用总数（非本次新增数）；前端按 message 展示，无功能影响。
4. **本地产物**：`Dockerfile.dev`/`docker-compose.dev.yml` 为本地开发 TEMP 修改（端口 3456、禁用限流），标注 revert 后测试完成；未纳入 v2 交付范围，可按需还原。

## 交付后评审（review）修正

双轴评审（Standards + Spec，base a02a6123）发现并修复 1 个关键缺陷 + 1 个测试加固：

- **BR-101 顺序缺陷（Spec 硬违规）已修复**：`service/audit_sink.go` flush 原先**先清空 ScanText 再扫描**，导致全文 keyword/regex 匹配在生产路径上永不执行（scanFace 恒回退截断 Text）。已改为**先扫描、后清空序列化**，并补回归测试 `TestSinkFlushScansFullTextBeforeClear`（修复前失败、修复后通过）。
- **测试加固**：`TestBudgetDowngradesToolDefBeforeToolCall` 从 map flag 断言改为断言 tool_def 被砍且 user 永不被 drop（ModePreview）。
- **接受的意见判断**（未改）：三处 tool_call builder 的重复 derive/truncate 块、i18n 键 `Applied N rules` 占位符风格、`queryInt` 吞解析错误（与现有 `controller/log.go` 模式一致）、Claude/Gemini 输出 tool_use 段缺失（spec 内部不一致，见风险 2）。

详见 `evidence/phase-7/review-fix.txt`。

## Dev 部署浏览器覆盖（UF-101～UF-106 全过）

部署：工作区 binary（:3000，SQLite）+ `bun run dev`（:3001，/api 代理到 :3000）+ headless Chromium。

| 场景 | 结果 |
|---|---|
| UF-101 规则 CRUD（含 note）| 9→10 行；note 列显示 `review note test`；截图 `UF-101/audit-settings-section.webp` |
| UF-102 模板生命周期 | apply 10→19 行、幂等、disable 只关模板规则（manual 不动）、remove 确认后回 10 行；`UF-102/` |
| UF-103 导出/导入 | 导出 200 + Content-Disposition；导入 2 条上表；非法导入拒绝 count 不变；`UF-103/` |
| UF-104 审计日志列表 | `/usage-logs/audit` 表格 + severity 筛选 + 行展开详情（segments + matched rule）；截图 `UF-104/audit-log-list.webp` |
| UF-105 重扫 | POST rescan → wl_version 23，running(1000/1505)→done；min_hit=1 查出命中记录 |
| UF-106 普通用户隔离 | `/usage-logs/audit` → 跳 /403；侧边栏无审计入口；`/api/audit/logs`、`/api/audit/templates` 均 403 |

**覆盖中发现并修复的 Bug**：`/usage-logs/audit` 首载 500 —— 后端 `LogContent` 无 json tag → 大写驼峰键（`RequestId`...），前端 `AuditLogItem` 用 snake_case，`item.request_id.slice(-8)` 抛 TypeError。修复：`api.ts` 的 `listAuditLogs` 增加 `AuditLogRawItem` → `AuditLogItem` 映射。修复后列表/筛选/详情全部正常；typecheck/build/i18n 全过。

证据：`evidence/UF-104/browser-coverage.txt`。

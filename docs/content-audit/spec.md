# content-audit Spec

> Version: 0.1.0 | Date: 2026-08-01 | Status: Skeleton 骨架
>
> 本文件是本需求的**唯一事实源**：事实基线、业务合同、技术方案、任务计划、验收协议全部在此。
> 其他文件（handoff.md、tasks.csv）只引用本文件，不复制内容。
>
> 三态规则：表格单元格只允许——1. 验证过的事实（注明来源命令）；2. 显式假设 `ASM-xxx`；3. `待勘察`。
> 禁止编造命令、symbol、文件名。

---

## 1. 事实基线与假设

### 1.1 需求与运行模式

| 项 | 结论 |
|---|---|
| 原始需求 | 为 API 网关添加内容监控 / API 输入输出安全审计能力：采集 relay 请求的输入和输出，存入独立表 `logs_content`（LOG_DB），通过 `request_id` 与 `logs` 1:1 关联，管理员在现有 Usage Logs 详情弹窗查看审计内容，支持 watchlist 命中检测和版本驱动重扫 |
| 输入类型 | 空输入，上下文回退（前序对话中需求已完整讨论并设计） |
| Mode | oneclick |
| 置信度 | 高（前序对话已走完完整设计讨论，核心决策已拍板，并经历过一次完整实现和回滚，积累了真实验证过的硬约束） |
| 输出目录 | `docs/content-audit/` |

### 1.2 任务类型路由

| 维度 | 结论 |
|---|---|
| 任务类型 | backend（采集、存储、API）、frontend（审计弹窗、徽章、设置页）、data（新表 DDL、迁移）、security（watchlist 规则、重扫） |
| 主要风险 | relay 主链路性能影响；import cycle（audit→model 成环，实测确认）；AutoMigrate 在运行时未生效的疑点；ClickHouse LOG_DB 兼容性；三数据库 DDL 差异 |
| 行号引用策略 | 业务/API 优先 symbol+anchor，行号只作 hint；relaykit 文件需注意块注释造成的 grep 漂移 |
| 必需验收方式 | backend: curl 请求完整路径 + server log；frontend: 浏览器 MCP 点击 + 截图；data: AutoMigrate 建表确认；security: watchlist CRUD + 重扫命中验证 |
| 必须覆盖用户场景 | 管理员查看审计徽章/详情；普通用户隔离（负向）；管理员配置审计策略；watchlist CRUD；重扫进度 |

### 1.3 勘察事实清单

| 事实 ID | 事实 | 来源命令 | 输出摘要 |
|---|---|---|---|
| F-01 | `logs.Other string` 无 Omit 标记，0次 Omit 调用 | `grep -c "Omit(" model/log.go` | **0** |
| F-02 | `GetAllLogs`(L468) / `GetUserLogs`(L564) 直接 `Find(&logs)`，不过滤 Other 列 | `grep -n "func GetAllLogs\|func GetUserLogs" model/log.go` | L468, L564 |
| F-03 | `formatUserLogs` 已有 `delete(otherMap,"admin_info")`，普通用户自动过滤 admin 字段 | `grep -n "delete(otherMap" model/log.go` | L123, L125 |
| F-04 | `GenRelayInfo` 在 `controller/relay.go:123`；`attachQuotaSaturation` 在 `service/text_quota.go:524`；挂 OnSettled 点在 524 之后 | `grep -n "GenRelayInfo" controller/relay.go; grep -n "attachQuotaSaturation" service/text_quota.go` | relay.go:123, text_quota.go:524 |
| F-05 | `PostTextConsumeQuota` 在 `service/text_quota.go:397`，内部顺序：calculateSummary → SettleBilling → GenerateOtherInfo → attachQuotaSaturation → RecordConsumeLog | `grep -n "func PostTextConsumeQuota" service/text_quota.go` | L397 |
| F-06 | `RelayInfo.RequestId string` 在 `relay/common/relay_info.go:140`，赋值在 L477 | `grep -n "RequestId" relay/common/relay_info.go` | L140 (字段), L477 (赋值) |
| F-07 | `migrateLOGDB()` 原始体 = `LOG_DB.AutoMigrate(&Log{})`，ClickHouse 分支走 `migrateClickHouseLogDB()` | `sed -n "399,412p" model/main.go` | L399 函数体确认 |
| F-08 | `newGormConfig` 在 `model/gorm_logger.go:25`，`LogLevel: logger.Warn`(L42)，`PrepareStmt: prepareStmt`(L27)；普通 SQL（CREATE TABLE）不打印日志 | `grep -n "LogLevel\|func newGormConfig\|PrepareStmt" model/gorm_logger.go` | L25, L27, L42 |
| F-09 | `InitLogDB()` 在 `model/main.go:212`；`IsMasterNode` 检查在 L241；日志迁移在 L244 `migrateLOGDB()` | `grep -n "func InitLogDB\|IsMasterNode\|migrateLOGDB" model/main.go` | L212, L241, L244 |
| F-10 | `attachQuotaSaturationToOther` 在 `service/log_info_generate.go:25`；`attachQuotaSaturation` 在 L40 | `grep -rn "func attachQuotaSaturation" service/` | L25 (inner), L40 (public) |
| F-11 | `common.RelayCtxGo(ctx, f)` 在 `common/gopool.go:23`（底层 bytedance/gopkg gopool） | `grep -n "func RelayCtxGo" common/gopool.go` | L23 |
| F-12 | common/gin.go API helpers：`ApiError`(L199), `ApiErrorMsg`(L206), `ApiSuccess`(L213), `ApiErrorI18n`(L223), `ApiSuccessI18n`(L232)；**无 `ApiErrorStr`** | `grep -n "^func Api" common/gin.go` | L199-L232 |
| F-13 | `common/json.go` JSON 包装器：`Unmarshal`(L9), `UnmarshalJsonStr`(L13), `DecodeJson`(L17), `Marshal`(L21), `GetJsonType`(L25), `JsonRawMessageToString`(L48) | `grep -n "^func " common/json.go` | L9-L48 |
| F-14 | `model/option.go:50` `LogConsumeEnabled` 默认值；L330 sync switch `case "LogConsumeEnabled":` | `grep -n "LogConsumeEnabled" model/option.go` | L50, L330 |
| F-15 | `details-dialog.tsx:472` `DetailsDialogProps`；L483 `parseLogOther(props.log.other)`；L784 quota_saturation 展示块（审计徽章/tab 的实现先例） | `grep -n "interface DetailsDialogProps\|parseLogOther\|quota_saturation" .../details-dialog.tsx` | L472, L483, L784 |
| F-16 | `section-registry.tsx:24` `USAGE_LOGS_SECTIONS` 仅含 common/drawing/task 三项；加 audit 只需数组追加 | `grep -n "USAGE_LOGS_SECTIONS" -A18 .../section-registry.tsx` | L24-L40 |
| F-17 | `common-logs-columns.tsx:108`：`if (isAdmin && other?.admin_info?.quota_saturation)` 徽章先例，审计徽章照抄此模式 | `grep -n "quota_saturation" .../common-logs-columns.tsx` | L108 |
| F-18 | i18n locale 文件：`web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`（7个文件，平铺JSON） | `ls web/src/i18n/locales/` | 7 files |
| F-19 | `web/node_modules` 已存在；bun 1.3.14；go1.26.5 | `test -d web/node_modules; bun --version; go version` | present, 1.3.14, go1.26.5 |
| F-20 | `make test` = `GOWORK=off go test $root_packages` + `cd relaykit && GOWORK=off go test ./...`；运行结果全绿，无 FAIL | 本次 `make test` | 全 ok，无 FAIL |
| F-21 | `relay/channel/openai/relay-openai.go:104` `OaiStreamHandler`；L222 `OpenaiHandler`（Phase 2 response 采集挂点） | `grep -n "func OaiStreamHandler\|func OpenaiHandler" relay/channel/openai/relay-openai.go` | L104, L222 |
| F-22 | `relaykit/dto/openai_request.go:543` `(m *Message) ParseContent()` (真实定义，L733 在块注释 L677-L848 内)；`type Message struct`(L303)；`type MediaContent struct`(L316) | `grep -n "ParseContent" relaykit/dto/openai_request.go; awk ".../blockcomment"` | L543, 块注释 L677-L848 |
| F-23 | `relaykit/dto/openai_request.go:119` `GeneralOpenAIRequest.GetTokenCountMeta()`；L202 `CombineText = strings.Join(...)`（角色信息在此丢失，审计必须在 ParseContent 前捕获）| `grep -n "GetTokenCountMeta\|CombineText" relaykit/dto/openai_request.go` | L119, L202 |
| F-24 | `relaykit/dto/claude.go:168` `(c *ClaudeMessage) ParseContent()`；L243 `ClaudeRequest.GetTokenCountMeta()`（handoff记录L355已漂移，实际L243）| `grep -n "ParseContent\|GetTokenCountMeta" relaykit/dto/claude.go` | L168, L243 |
| F-25 | `relaykit/types/request_meta.go:22` `type TokenCountMeta`，含 `CombineText string` | `grep -n "type TokenCountMeta\|CombineText" relaykit/types/request_meta.go` | L22 |
| F-26 | `service/str.go:75` `acKey`；`L98` `getOrBuildAC`（AC自动重建，词表变则哈希变）；`L132` `AcSearch` | `grep -n "func acKey\|func getOrBuildAC\|func AcSearch" service/str.go` | L75, L98, L132 |
| F-27 | `service/sensitive.go:40` `SensitiveWordContains(text)` | `grep -n "func SensitiveWordContains" service/sensitive.go` | L40 |
| F-28 | `logger/logger.go:80` `func LogWarn(ctx context.Context, msg string)` | `grep -rn "func LogWarn" logger/` | L80 |
| F-29 | `router/api-router.go:271` `logRoute` 组；L272 `GetAllLogs`；L277 `GetUserLogs`（新审计 API 插此段） | `grep -n "logRoute" router/api-router.go` | L271-L278 |
| F-30 | `model/locking.go:20` `func lockForUpdate(tx *gorm.DB)` | `grep -rn "func lockForUpdate" model/` | L20 |
| F-31 | `web/src/features/usage-logs/data/schema.ts:46-47` 已有 `request_id`, `upstream_request_id` zod 字段 | `grep -n "request_id\|upstream_request_id" .../data/schema.ts` | L46-L47 |
| F-32 | `web/src/features/usage-logs/components/dialogs/prompt-dialog.tsx` 含 `ScrollArea` + `useCopyToClipboard`（复制骨架先例）| `grep -n "useCopyToClipboard\|ScrollArea" .../prompt-dialog.tsx` | L25-L26 |
| F-33 | `git status --porcelain` 干净（完整回滚已验证） | `git status --porcelain` | 空输出 |
| F-34 | 实现期确认：`audit → model` 直接 import 导致编译报 import cycle（`relay/common/relay_info.go` 内 `import audit` → audit import model → model import relay/common → relay/common import audit`）| 实现期真实报错 | import cycle not allowed |
| F-35 | `model/main.go:399` `migrateLOGDB` 原始版：仅 AutoMigrate `&Log{}`；服务运行时 SQLite `logs_content` 表不存在（F-35a）；隔离测试 newGormConfig(true) 调 AutoMigrate 成功建表（F-35b） | `sqlite3 one-api.db "SELECT name..."` = 空；隔离测试 PASS | **疑点：待 P0 校准** |
| F-36 | `common/constants.go:93` `var LogConsumeEnabled = true`；`common/init.go:89` `IsMasterNode = os.Getenv("NODE_TYPE") != "slave"` | `grep -n "LogConsumeEnabled\|IsMasterNode" common/constants.go common/init.go` | L93, L89 |

### 1.4 假设清单

| 假设 ID | 内容 | 推荐值 | 风险 | 确认方式 |
|---|---|---|---|---|
| ASM-001 | sink 实例挂载位置：`RelayInfo` 结构体字段 `ContentSink audit.ContentSink` vs gin `context.Set` key | **推荐 RelayInfo**（跟随请求生命周期，无反射/string key 风险，已有 QuotaClamp 先例） | 若选 context，不同 goroutine context 传递需额外同步 | 进入 Phase 1 前拍板，影响 relay_info.go 修改 |
| ASM-002 | Phase 2 response 采集时机：DoResponse 结束立即写 vs 推迟到 OnSettled 合并写 | **推荐推迟到 OnSettled 前** `（OnOutput→暂存 RelayInfo.ResponseText，OnSettled 时合并写库）` | 若流中断，OnSettled 可能不被调用，则丢失 response；不过 OnSettled 本就是非必保路径 | Phase 2 设计时拍板 |
| ASM-003 | 留存分级方案：full 30d / preview+derive 90d / drop+omitted 不写 | **推荐分级（方案Z）**：节省磁盘，opaque 按 preview 算 | 若选均等留存，磁盘增速=日志表×约 2-3 倍 | Phase 1 milestone review 前拍板，影响 migrateLOGDB TTL 字段 |
| ASM-004 | input 阶段是否强制生成 TokenCountMeta（走 ParseContent 拿 segments）| **推荐不强制**：走 request.GetTokenCountMeta() 已有的结果；image/audio 走 opaque fidelity；需 segments 时再调 ParseContent | 若强制，所有请求额外一次 ParseContent，增加 CPU | Phase 1 设计时拍板 |
| ASM-005 | 新包路径：`audit/` 放 `github.com/QuantumNous/new-api/audit`（根模块） | 根模块 audit 包 | 若放 relaykit，relaykit 需 import model 相关 → 破坏独立性 | Phase 1 实施前确认 |

---
## 2. 业务合同

> 本章是 BR/UF/INV/EVD 的唯一定义处。任务、handoff、review 只引用 ID，不复制表格。

### 2.1 BR 业务规则

| 规则 ID | 规则 | 正例 | 反例 | 影响范围 | 验证方式 |
|---|---|---|---|---|---|
| BR-001 | `logs_content` 记录通过 `request_id` 与 `logs` 1:1 关联，每个 request_id 至多一条 `logs_content` | `logs.request_id='req-abc'` → `logs_content.request_id='req-abc'` 各一条 | 同一 request_id 两条 logs_content；或 request_id 为空 | model/log_content.go, service/audit_sink.go | `SELECT count(*) FROM logs_content WHERE request_id NOT IN (SELECT request_id FROM logs)` → 0 |
| BR-002 | `logs.other.admin_info.audit` 只存 request_id 指针（约 40 字节），正文不进 `logs.other` | `{"admin_info":{"audit":{"request_id":"req-abc","hit_count":2}}}` ≈ 64B | `logs.other` 含 segments JSON blob，每页查询传输全量正文 | service/audit_sink.go OnSettled | `jq length` on `other.admin_info.audit` < 200 bytes |
| BR-003 | 普通用户 GET /api/log/self 响应不含 `audit` 字段（现有 `delete(otherMap,"admin_info")` 自动覆盖） | 普通用户响应 `logs[].other` 不含 audit key | 普通用户看到 `logs[].other.admin_info.audit` | model/log.go formatUserLogs（F-03） | curl 普通用户 token → jq `.other` 无 admin_info |
| BR-004 | `audit/` 包只能 import `common`、`relaykit/dto`、`relaykit/types`；禁止 import `model`、`relay/common`（会成环，F-34） | `import "github.com/QuantumNous/new-api/common"` OK | `import "github.com/QuantumNous/new-api/model"` → 编译报 import cycle | audit/（新包） | `GOWORK=off go build ./audit/...` → BUILD_OK |
| BR-005 | 审计总开关关闭时 `RelayInfo.ContentSink == nil`，所有 sink 调用点做 nil 检查，零开销 | 审计关闭 → if sink == nil { return }，无 goroutine 泄漏 | 开关关闭仍有 goroutine 轮询 watchlist | relay/common/relay_info.go, controller/relay.go | audit disabled → pprof goroutines 无 audit 相关 |
| BR-006 | sink channel 满时立即 drop 事件并递增丢弃计数器，绝不阻塞 Relay goroutine | 高并发下 relay P99 不变 | select 用阻塞 send → relay P99 抬升 | audit/sink.go | ab/wrk 并发 100 请求 + server log 确认 drop 计数存在 |
| BR-007 | 任何将要 drop/omit 的 segment，必须先提取 derived facts（urls/domains/tools/args_keys），再丢弃原文 | tool_result（mode=drop）→ derived.domains 已填 | mode=drop 但 derived 为空，watchlist domain 规则无法命中 | audit/segment.go | 含 URL 的 tool_result → logs_content.segments[kind=tool_result].derived.domains 非空 |
| BR-008 | 各 kind 默认 mode：system=preview/512B, user=full/16KB, assistant=full/16KB, tool_call=derive/1KB, tool_result=drop, image=omitted, audio=omitted | user 16KB → segments[kind=user,mode=full,text=全文] | user 16KB 被截为 512B | audit/segment.go | curl 含 16KB user message → logs_content.segments[kind=user].mode == "full" |
| BR-009 | 超 per_request_max_bytes 时降级顺序：tool_result → tool_call → system → assistant → user（user 最后砍） | 超限 → tool_result 先变 drop，user prompt 保持 full | 直接截断 user message 而保留 tool_result | audit/segment.go | 构造超限请求 → segments user.mode=full，tool_result.mode=drop |
| BR-010 | watchlist regex 类型规则默认关闭；启用时系统上限 ≤ 8 条 enabled=true regex 规则 | 无配置 → regex count=0 | 默认开启 regex → 高并发每条消息额外 ~50µs/KB | model/audit_watchlist_rule.go（新）| 初始化后 `SELECT count(*) FROM audit_watchlist_rules WHERE kind='regex' AND enabled=1` → 0 |
| BR-011 | watchlist 规则存主库 `audit_watchlist_rules` 独立表，表外有 `audit_watchlist_meta` 元数据行含 `version`；增删改均 version++ | 更新规则 → version 3→4 | 规则写 option → 多节点 SyncOptions 延迟，版本不同步 | model/audit_watchlist_rule.go（新）| 插入规则 → `SELECT version FROM audit_watchlist_meta` 递增 |
| BR-012 | `logs_content.flags` 每条命中记录存 rule_id + pattern 快照；规则被改/删后历史记录含义不变 | 删除 rule #5 → flags 仍含 `{rule_id:5,pattern:"xxx"}` | flags 只存 rule_id → 规则删除后无法解读历史 | model/log_content.go（新）| 创建规则触发命中 → 删除规则 → 查 logs_content.flags 仍含 pattern |
| BR-013 | 重扫只处理 `logs_content.created_at > NOW() - content_ttl_days`，超 TTL 行跳过 | 30d TTL → 31d 前记录不参与重扫 | 扫全表 → 百万行时超时/OOM | service/audit_watchlist.go（新）| 插入超龄记录 → 重扫后该记录 flags/hit_count 不变 |
| BR-014 | `UsingLogDatabase(ClickHouse)` 为 true 时，`migrateLOGDB` 不建 `logs_content`，审计功能静默关闭 | ClickHouse 配置 → 启动无 panic，审计 API 返回 disabled | ClickHouse 下建 TEXT 列报 DDL 错误 | model/main.go migrateLOGDB（F-07）| 单测 ClickHouse 分支 → migrateLOGDB 不调 AutoMigrate |
| BR-015 | audit sink goroutine 内任何 error 必须用 `logger.LogWarn(ctx, msg)` 显式打点；不依赖 GORM logger（F-08 确认：LogLevel=Warn 但 CREATE TABLE 不打印）| 故意传错 db → server log 含 `[WARN] audit:...` | 依赖 GORM warn，异常静默消失 | service/audit_sink.go | 错误注入 → server log 确认 audit warn 条目 |
| BR-016 | `logs_content` DDL + CRUD 必须在 SQLite / MySQL ≥5.7.8 / PostgreSQL ≥9.6 均通过；禁用 MySQL-only / PG-only 语法 | GORM AutoMigrate TEXT 列在三库均成功 | `JSON_EXTRACT()` → PG 报错 | model/log_content.go（新）| make test（sqlite）+ PG 容器 AutoMigrate |
| BR-017 | `relaykit/` 模块不引入 audit 依赖；`cd relaykit && GOWORK=off go build ./...` 始终通过 | relaykit 无 audit import → 独立构建 OK | audit/types.go 放入 relaykit → relaykit 依赖根模块 | relaykit/ 模块边界 | `cd relaykit && GOWORK=off go build ./...` → BUILD_OK |

### 2.2 UF 用户验收场景（索引）

| 场景 ID | Given | When | Then | 角色 | 验证方式 | Evidence |
|---|---|---|---|---|---|---|
| UF-001 | 管理员已登录，Usage Logs 列表已加载，审计功能已开启，该请求有 watchlist 命中 | 管理员查看日志列表行 | 该行显示审计命中徽章（命中数 / severity 颜色） | admin | browser MCP 截图 | EVD-005 |
| UF-002 | 管理员在日志列表，某行有审计内容（有或无命中均可） | 点击该行打开详情弹窗，切换到"审计"Tab | 弹窗展示该请求的 segments（fidelity/kind/mode/text/derived）+ flags（命中规则列表） | admin | browser MCP 截图 | EVD-005 |
| UF-003 | 管理员在详情弹窗审计 Tab，某 segment 有正文 | 点击"复制"按钮 | 该 segment.text 被复制到剪贴板，按钮短暂变化反馈 | admin | browser MCP 交互 | EVD-005 |
| UF-004 | 普通用户已登录，发起正常 API 请求，请求被审计并命中规则 | 普通用户调 GET /api/log/self | 响应中 `logs[].other` 不含 admin_info / audit 字段 | user | curl + jq | EVD-006 |
| UF-005 | 管理员已登录，进入系统设置 → 审计配置页 | 修改审计总开关、per_request_max_bytes、content_ttl_days 等参数并保存 | 设置保存成功，下次请求按新策略采集 | admin | browser MCP 截图 | EVD-010 |
| UF-006 | 管理员已登录，进入审计管理页面 watchlist 列表 | 管理员增加 / 编辑 / 删除 / 启用禁用 watchlist 规则 | 规则变更立即生效（version++），后续请求按新规则命中 | admin | browser MCP + curl | EVD-004 |
| UF-007 | 管理员已登录，watchlist 规则已更新，存在历史 logs_content 记录 | 管理员点击"重扫"按钮，确认范围（TTL 内记录）| 顶部进度条显示重扫进度，完成后命中数更新，server log 有重扫完成条目 | admin | browser MCP + server log | EVD-009 |
| UF-008 | relay 请求完整路径（controller → DoResponse → PostTextConsumeQuota）| 任意 OpenAI 格式请求成功 | OnInput + OnSettled 均被调用，logs_content 写入一条 | 内部（2.3 豁免：纯后台流程，无用户交互界面）| curl + sqlite3 | EVD-002 |
| UF-009 | audit sink goroutine 内发生 panic / db error | relay 主链路照常完成 | sink 错误不中断 relay 响应，server log 含 warn 条目 | 内部（2.3 豁免：纯后台容错路径，无用户交互界面）| 错误注入 + server log | EVD-007 |

> UF-008、UF-009 为内部流程，无用户可见交互，2.3 节豁免流程脚本（2.3 节覆盖 UF-001 ～ UF-007）。

### 2.3 核心业务流程（步骤级交互脚本）

#### UF-001: 管理员在日志列表看到审计命中徽章

**前置状态**：管理员已登录；审计功能 enabled=true；当前列表页有 ≥1 条已命中 watchlist 的请求日志。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 进入 `/usage-logs`（Common 标签）| 列表 loading spinner | GET /api/log?... | — |
| 2 | — | — | 响应 `logs[].other.admin_info.audit` 含命中数据 | 该行在 status/model 列区域旁显示橙色/红色审计徽章，标注命中数 |
| 3 | 悬停徽章 | tooltip 展示 top severity 和命中规则数 | — | 用户确认是 watchlist 命中 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 审计未命中 | 该请求无 watchlist 命中（hit_count=0）| 该行无审计徽章（正常，无需提示） | — | N/A |
| 审计功能关闭 | `AuditEnabled=false` | 所有行均无审计徽章 | ContentSink==nil，OnInput 不调用 | 管理员去设置页开启 |
| 非管理员账号 | 普通用户 token 登录 | 无审计徽章（admin_info 被 formatUserLogs 过滤） | server 不返回 admin_info | N/A（预期行为） |

**界面状态机**：

```
loading → loaded（无命中：行无徽章）
               ↓（有命中）
         loaded（行有徽章）→ tooltip hover → tooltip visible
```

**入口接线清单**：
- 路由 `/usage-logs` → `common-logs-columns.tsx` renderCell 函数 → 审计徽章渲染逻辑

---

#### UF-002: 管理员在详情弹窗查看完整审计内容

**前置状态**：管理员已登录；详情弹窗对应的请求有 logs_content 记录（有或无命中）。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点击日志行"详情"按钮 | 弹窗打开，默认 Tab | GET /api/log/content?request_id=xxx（新 API）| — |
| 2 | 点击"审计"Tab | Tab 激活，content loading | — | — |
| 3 | — | — | 响应 logs_content 记录（segments + flags）| 展示 segments 列表：每条含 kind 标签、mode badge、bytes、text（或 truncated 提示）、derived facts；flags 区展示命中规则快照 |
| 4 | 展开某 segment | segment 展开显示完整 text | — | 可阅读全文 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 无审计记录 | 该请求发生在审计开启前，或 fidelity=meta_only | "暂无审计内容" 空态提示 | GET /api/log/content 返回 404 或空 | N/A（数据不存在为正常） |
| 内容已过期 | logs_content 超 TTL 被清理 | "审计内容已超保留期限" | — | N/A |
| 网络错误 | API 请求失败 | Tab 内 error state + 重试按钮 | — | 点重试 |

**界面状态机**：

```
弹窗打开 → 审计Tab未激活
              ↓ 点击审计Tab
           loading → 成功：segments渲染
                  ↓ 失败
               error（重试按钮）
                  ↓ 数据为空
               empty（空态提示）
```

**入口接线清单**：
- 日志行"详情"按钮 → `details-dialog.tsx` 打开 → 审计 Tab 注册到现有 Tab 组件
- 审计 Tab onActive → GET /api/log/content API 调用

---

#### UF-003: 管理员复制审计正文片段

**前置状态**：管理员在 UF-002 详情弹窗的审计 Tab，某 segment.mode ∈ {full, preview}（有 text）。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点击 segment 旁"复制"图标按钮 | 按钮变为对勾图标（约 2s）| `navigator.clipboard.writeText(segment.text)` | 剪贴板含 segment.text 完整内容 |
| 2 | 2s 后 | 按钮恢复复制图标 | — | — |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| segment.mode=drop/omit | 无 text 字段 | 复制按钮不显示，显示 mode badge | — | N/A（预期行为） |
| 剪贴板权限被拒 | 浏览器 permission denied | toast "复制失败，请手动选择" | — | 用户手动选中文本复制 |

**界面状态机**：

```
复制图标 → 点击 → 对勾图标（2s）→ 复制图标
                        ↓ 失败
                   复制图标 + error toast
```

**入口接线清单**：
- 审计 Tab segment 渲染 → 复制按钮 onClick → `useCopyToClipboard` hook（复用 prompt-dialog.tsx 先例）

---

#### UF-004: 普通用户无法看到审计字段（负向隔离）

**前置状态**：普通用户持有有效 token；当前有已审计的请求日志。

**成功主路径**（验证隔离正确）：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | GET /api/log/self | — | `formatUserLogs` 调用 `delete(otherMap,"admin_info")` | 响应 logs[].other 无 admin_info 字段 |
| 2 | GET /api/log/content?request_id=xxx（若接口存在）| — | middleware.AdminAuth() 拦截 → 401 | 用户无法访问审计内容 API |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 权限绕过（应禁止）| 普通用户直接调 /api/log/content | — | 401 Unauthorized | N/A（应拒绝） |

**界面状态机**：N/A（纯 API 验证场景，无 UI 状态）

**入口接线清单**：
- GET /api/log/content → `middleware.AdminAuth()` 前置（路由注册时加）

---

#### UF-005: 管理员配置审计总开关和采集策略

**前置状态**：管理员已登录；进入系统设置 → 审计配置。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 切换"审计总开关"到开启 | 开关视觉变化 | — | — |
| 2 | 修改 `per_request_max_bytes`（默认 65536）| 输入框更新 | — | — |
| 3 | 修改 `content_ttl_days`（默认 30）| 输入框更新 | — | — |
| 4 | 点击"保存" | 按钮 loading、禁止重复点击 | PUT /api/option → 写 OptionMap，广播到所有 relay goroutine | toast "设置已保存" |
| 5 | — | — | — | 下次请求按新策略采集 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 参数越界 | per_request_max_bytes < 0 或 > 10MB | 输入框错误提示，保存按钮禁用 | 前端 validation | 用户修正输入 |
| 保存失败 | 网络或服务端错误 | toast "保存失败"，设置保留 | 服务端 4xx/5xx | 用户重试 |

**界面状态机**：

```
idle → 编辑中 → 保存loading → 保存成功（toast）
                        ↓ 失败
                   error toast（设置可重试）
```

**入口接线清单**：
- 路由 `/settings` → 审计配置 section → 保存按钮 onClick → PUT /api/option
- option sync switch 新增 `AuditEnabled` / `AuditPerRequestMaxBytes` / `AuditContentTTLDays` case

---

#### UF-006: 管理员 CRUD watchlist 规则

**前置状态**：管理员已登录；进入审计管理页面 watchlist 规则列表。

**成功主路径（新增规则）**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点击"新增规则" | 弹出新增表单 | — | — |
| 2 | 填写 kind=keyword, pattern="敏感词", severity=high | 实时校验 pattern 不为空 | — | — |
| 3 | 点击"保存" | 按钮 loading | POST /api/audit/watchlist → INSERT + version++ | 列表刷新，新规则出现，version 徽章更新 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| pattern 为空 | 提交时 pattern='' | 表单校验错误，不调 API | 前端 validation | 用户填写 pattern |
| regex 语法错误 | kind=regex，pattern=`[invalid` | toast "正则表达式无效" | 服务端校验 → 400 | 用户修正 |
| regex 超限 | 已有 8 条 enabled regex | toast "regex 规则已达上限 8 条" | 服务端校验 → 400（BR-010）| 先禁用旧规则 |
| 删除规则 | 点击删除，确认弹窗 | 确认后列表动画移除 | DELETE /api/audit/watchlist/{id} + version++ | 规则消失，version 更新 |

**界面状态机**：

```
列表idle → 点新增 → 表单modal → 保存loading → 列表刷新
                        ↓ 失败
                   form error（可修改重提交）
```

**入口接线清单**：
- 路由 `/audit/watchlist`（新页面）→ 列表组件 → 新增/编辑弹窗 → 各按钮 onClick → /api/audit/watchlist/* REST API
- 路由需注册到 admin 路由组

---

#### UF-007: 管理员发起重扫并查看进度

**前置状态**：管理员已登录；watchlist 规则版本已更新（version N）；logs_content 内有 wl_version < N 的记录。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点击"重扫"按钮（审计管理页）| 确认弹窗："将对 TTL 内所有记录重新匹配当前规则" | — | — |
| 2 | 点击确认 | 按钮变"重扫中"禁用 | POST /api/audit/rescan → 启动后台 goroutine | 顶部进度条出现，显示"已处理 0/N 条" |
| 3 | — | 进度条每 2s 轮询更新 | goroutine 分批处理（500/批），写进度到 option | 进度条更新 |
| 4 | — | 进度条完成动画（100%）| goroutine 完成，option 写入完成状态 | toast "重扫完成，更新了 M 条记录"；重扫按钮恢复 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 无可重扫记录 | TTL 内无 wl_version < N 的记录 | toast "无需重扫：所有记录已是最新版本" | API 返回 no_op | N/A |
| 重扫中再次点击 | 已有重扫任务运行 | 按钮禁用，toast "重扫进行中" | 服务端检查进度 option → 409 | 等当前完成 |
| 重扫中途失败 | db error | 进度条停止，toast "重扫异常中断"，server log | goroutine 写 error 状态到 option | 管理员可重新发起 |

**界面状态机**：

```
idle（重扫按钮enabled）→ 确认 → loading（按钮disabled）→ 进度条显示 → 100%完成 → idle
                                                              ↓ 失败
                                                         error state（可重新发起）
```

**入口接线清单**：
- 审计管理页"重扫"按钮 onClick → POST /api/audit/rescan
- 进度轮询 → GET /api/audit/rescan/status（或读 option）
- 顶部进度条组件接线（复用或新建）

### 2.4 INV 不变量

| 不变量 ID | 内容 | 关联 BR/UF | 验证方式 |
|---|---|---|---|
| INV-001 | relay 主链路 P99 延迟和成功率不受审计 sink 影响（sink 全异步 goroutine，BR-006）| BR-005, BR-006 | ab/wrk 对比开关 on/off，P99 差异 < 5ms |
| INV-002 | `LogConsumeEnabled=false` 时 `RecordConsumeLog` 立即 return，不写任何审计内容（F-36, model/log.go:343）| BR-005 | 设置 LogConsumeEnabled=false → sqlite3 logs_content count 不增 |
| INV-003 | 现有 86 根模块包 + relaykit 全部测试通过（基线：本次 make test 全绿）| BR-004, BR-017 | `make test` → 无 FAIL |
| INV-004 | `cd relaykit && GOWORK=off go build ./...` 始终通过（BR-017）| BR-017 | 每次提交后验证 |
| INV-005 | 普通用户 GET /api/log/self 响应体不含 `admin_info` / `audit` 字段（F-03 formatUserLogs 已实现）| BR-003, UF-004 | curl 普通 token → jq '.data[].other | fromjson | has("admin_info")' → false |
| INV-006 | 受保护标识 `new-api` / `QuantumNous` 的引用、元数据、归属信息不得修改或删除 | — | git diff 无受保护标识改动 |

### 2.5 EVD 证据清单

| 证据 ID | 类型 | 期望证据 | 保存位置 |
|---|---|---|---|
| EVD-001 | log/sql | `sqlite3 one-api.db ".tables"` 含 `logs_content`；AutoMigrate 成功日志 | `evidence/phase-1/automigrate.txt` |
| EVD-002 | api+log | curl 发送一条 OpenAI 请求 → server log 确认 OnInput+OnSettled 均调用 → sqlite3 logs_content 有对应记录 | `evidence/phase-1/sink-invoke.json` + `evidence/phase-1/server.log` |
| EVD-003 | api+log | curl 发送流式请求 → logs_content.segments 有 assistant kind 条目，text 非空 | `evidence/phase-2/response-capture.json` |
| EVD-004 | api | watchlist CRUD 响应样例（POST/GET/PUT/DELETE /api/audit/watchlist）+ version 递增截图 | `evidence/phase-4/watchlist-crud.json` |
| EVD-005 | screenshot | 管理员日志列表徽章截图 + 详情弹窗审计 Tab 截图（含 segments + flags）| `evidence/UF-001/badge.png`, `evidence/UF-002/detail-tab.png` |
| EVD-006 | api | 普通用户 curl GET /api/log/self → jq 输出确认无 admin_info 字段 | `evidence/UF-004/user-response.json` |
| EVD-007 | log+test | make test → 全绿输出；relaykit build OK | `evidence/phase-1/test-baseline.txt` |
| EVD-008 | build | `GOWORK=off go build ./...` + `cd relaykit && GOWORK=off go build ./...` 输出 BUILD_OK | `evidence/phase-1/build-check.txt` |
| EVD-009 | log+screenshot | 重扫触发后 option 进度更新截图 + 完成后 server log 条目 | `evidence/UF-007/rescan-progress.png` + `evidence/UF-007/server.log` |
| EVD-010 | screenshot | 管理员设置页审计配置区域截图 + 保存 toast 截图 | `evidence/UF-005/settings.png` |

### 2.6 角色与权限矩阵

| 角色 | 可见 | 可操作 | 禁止 | 失败提示 | 验证场景 |
|---|---|---|---|---|---|
| admin | 日志列表审计徽章；详情弹窗审计 Tab；watchlist 管理页；审计设置；重扫 | 全部审计 API（读/写）| — | — | UF-001～UF-007 |
| user（普通）| 自己的日志（无 admin_info）| 无审计相关操作 | GET /api/log/content；POST /api/audit/watchlist 等 | 401 Unauthorized | UF-004 |

### 2.7 负向 / 破坏性场景

| 场景 | Given | When | Then | Evidence |
|---|---|---|---|---|
| 权限不足 | 普通用户 token | GET /api/log/content?request_id=xxx | 401；other 中无 audit 字段 | EVD-006 |
| 空数据 | 审计功能刚开启，无 logs_content 记录 | 管理员查看详情弹窗审计 Tab | 空态提示"暂无审计内容" | EVD-005（空态截图）|
| sink panic | sink goroutine 内 panic | relay 主链路正常返回 | goroutine recover，server log WARN | EVD-007（错误注入）|
| 超限请求 | per_request_max_bytes = 1KB，请求 body 10KB | OnInput 处理 | segments 按 BR-009 降级顺序压缩 | EVD-002（含降级验证）|
| 旧数据兼容 | logs_content 为 wl_version=0（无规则版本）| watchlist 规则更新后发起重扫 | wl_version=0 行被重扫并更新 | EVD-009 |
| ClickHouse LOG_DB | UsingLogDatabase=ClickHouse | 服务启动 migrateLOGDB | 不建 logs_content，审计功能 disabled | BR-014 单测 |

### 2.8 非目标

- 不做外挂审计服务（第二进程）；不做 webhook 外发
- 不支持 ClickHouse 下的审计内容存储
- 不修改 relaykit 模块（audit 代码全在根模块）
- 不实现实时告警推送（邮件/webhook）
- logs_content 无分表；暂不支持分库写入
- 不在此 PRD 内实现 Gemini / Responses 格式 walker（属 Phase 3，单独任务）

---
## 3. 技术方案

> **Stage 2 补全**：3.1 架构 Before/After、3.2 模块改造、3.4 API/数据/权限/路由影响 将在 Stage 2 写入。
> 本 Stage 只交付 3.3 三段式定位清单作为实现依据。

### 3.3 三段式定位清单

> 行号只是 hint；漂移时以 symbol + rg anchor 为准。`待勘察` = 未验证不许猜；`ASM-xxx` = 已登记假设；「待创建」= 新文件，内容待 Stage 2 设计。

| 文件 | 稳定定位 | 搜索定位（rg anchor） | 行号 hint | 备注 |
|---|---|---|---|---|
| `model/log.go` | `type Log struct` | `rg "type Log struct" model/log.go` | L61 | 含 RequestId(L78), UpstreamRequestId(L79), Other(L80) |
| `model/log.go` | `func createLog` | `rg "func createLog" model/log.go` | L101 | LOG_DB.Create |
| `model/log.go` | `func formatUserLogs` | `rg "func formatUserLogs" model/log.go` | L116 | delete(otherMap,"admin_info") L123 |
| `model/log.go` | `func GetAllLogs` | `rg "func GetAllLogs" model/log.go` | L468 | admin 查询入口 |
| `model/log.go` | `func GetUserLogs` | `rg "func GetUserLogs" model/log.go` | L564 | 普通用户查询入口 |
| `model/log.go` | `func RecordConsumeLog` | `rg "func RecordConsumeLog" model/log.go` | L343 | LogConsumeEnabled 守卫 L344 |
| `model/log.go` | `type RecordConsumeLogParams` | `rg "type RecordConsumeLogParams" model/log.go` | L328 | — |
| `model/main.go` | `func InitLogDB` | `rg "func InitLogDB" model/main.go` | L212 | IsMasterNode 守卫 L241 |
| `model/main.go` | `func migrateLOGDB` | `rg "func migrateLOGDB" model/main.go` | L399 | 需新增 `&LogContent{}` AutoMigrate（新结构体） |
| `model/main.go` | `func migrateClickHouseLogDB` | `rg "func migrateClickHouseLogDB" model/main.go` | L406 | ClickHouse 分支，不建 logs_content |
| `model/gorm_logger.go` | `func newGormConfig` | `rg "func newGormConfig" model/gorm_logger.go` | L25 | LogLevel=Warn(L42)，CREATE TABLE 不打印 —— **不可依赖此日志确认建表** |
| `model/option.go` | `OptionMap["LogConsumeEnabled"]` | `rg "LogConsumeEnabled" model/option.go` | L50, L330 | 新 audit option 照此模式注册 |
| `model/locking.go` | `func lockForUpdate` | `rg "func lockForUpdate" model/locking.go` | L20 | watchlist 写操作加行锁 |
| `model/log_content.go` | `type LogContent struct` | `rg "type LogContent struct" model/log_content.go` | 待创建 | ASM-005；含 request_id PK, fidelity, segments TEXT, flags TEXT, wl_version |
| `model/audit_watchlist_rule.go` | `type AuditWatchlistRule struct` | `rg "type AuditWatchlistRule struct" model/` | 待创建 | ASM-005；含 id/kind/pattern/severity/enabled/note/created_at/updated_at |
| `relay/common/relay_info.go` | `type RelayInfo struct` | `rg "type RelayInfo struct" relay/common/relay_info.go` | L83 | 需新增 `ContentSink` 字段（ASM-001 推荐此处） |
| `relay/common/relay_info.go` | `func GenRelayInfo` | `rg "func GenRelayInfo" relay/common/relay_info.go` | L548 | 审计 sink 注入点 |
| `controller/relay.go` | `func Relay` | `rg "func Relay" controller/relay.go` | L71 | OnInput 调用点：L112 GetAndValidateRequest 之后 |
| `controller/relay.go` | `func fastTokenCountMetaForPricing` | `rg "func fastTokenCountMetaForPricing" controller/relay.go` | L267 | input 阶段分支判断参考 |
| `service/text_quota.go` | `func PostTextConsumeQuota` | `rg "func PostTextConsumeQuota" service/text_quota.go` | L397 | OnSettled 挂点：L524 attachQuotaSaturation 之后、RecordConsumeLog 之前 |
| `service/log_info_generate.go` | `func attachQuotaSaturation` | `rg "func attachQuotaSaturation" service/log_info_generate.go` | L40 | OnSettled 插入此处之后 |
| `service/sensitive.go` | `func SensitiveWordContains` | `rg "func SensitiveWordContains" service/sensitive.go` | L40 | keyword 扫描可复用 getOrBuildAC |
| `service/str.go` | `func getOrBuildAC` | `rg "func getOrBuildAC" service/str.go` | L98 | AC 自动重建，watchlist keyword 扫描复用 |
| `service/audit_sink.go` | `type LogContentSink struct` | `rg "type LogContentSink struct" service/audit_sink.go` | 待创建 | ASM-001；实现 audit.ContentSink 接口 |
| `audit/types.go` | `type ContentSink interface` | `rg "type ContentSink interface" audit/types.go` | 待创建 | ASM-005；OnInput/OnOutput/OnSettled 三方法 |
| `audit/segment.go` | `func BuildSegments` | `rg "func BuildSegments" audit/segment.go` | 待创建 | ASM-005；ParseContent → []Segment，derive + mode 逻辑 |
| `controller/audit.go` | `func GetLogContent` | `rg "func GetLogContent" controller/audit.go` | 待创建 | ASM-005；GET /api/log/content?request_id |
| `controller/audit.go` | `func ListWatchlistRules` | `rg "func ListWatchlistRules" controller/audit.go` | 待创建 | ASM-005 |
| `router/api-router.go` | `logRoute := apiRouter.Group` | `rg "logRoute := apiRouter" router/api-router.go` | L271 | 新审计路由接此段注册 |
| `relay/channel/openai/relay-openai.go` | `func OaiStreamHandler` | `rg "func OaiStreamHandler" relay/channel/openai/relay-openai.go` | L104 | Phase 2 stream response 采集挂点 |
| `relay/channel/openai/relay-openai.go` | `func OpenaiHandler` | `rg "func OpenaiHandler" relay/channel/openai/relay-openai.go` | L222 | Phase 2 non-stream response 采集挂点 |
| `relaykit/dto/openai_request.go` | `func (m *Message) ParseContent` | `rg "func.*Message.*ParseContent" relaykit/dto/openai_request.go` | L543（真实；L677-L848 为块注释内副本）| 块注释漂移风险，以 L543 为准 |
| `relaykit/dto/openai_request.go` | `func (r *GeneralOpenAIRequest) GetTokenCountMeta` | `rg "GetTokenCountMeta" relaykit/dto/openai_request.go` | L119 | CombineText 赋值 L202，审计需在此前捕获角色 |
| `relaykit/dto/claude.go` | `func (c *ClaudeMessage) ParseContent` | `rg "ClaudeMessage.*ParseContent" relaykit/dto/claude.go` | L168 | Phase 3 Claude walker |
| `relaykit/dto/claude.go` | `func (c *ClaudeRequest) GetTokenCountMeta` | `rg "ClaudeRequest.*GetTokenCountMeta" relaykit/dto/claude.go` | L243（handoff 记录 L355 已漂移，以 L243 为准）| — |
| `relaykit/types/request_meta.go` | `type TokenCountMeta` | `rg "type TokenCountMeta" relaykit/types/request_meta.go` | L22 | CombineText string |
| `logger/logger.go` | `func LogWarn` | `rg "func LogWarn" logger/logger.go` | L80 | 审计异常打点唯一入口 |
| `common/gopool.go` | `func RelayCtxGo` | `rg "func RelayCtxGo" common/gopool.go` | L23 | sink 异步投递 |
| `common/json.go` | `func Marshal` / `func Unmarshal` | `rg "^func (Marshal\|Unmarshal)" common/json.go` | L9, L21 | 所有 JSON 序列化必须走此包 |
| `common/gin.go` | `func ApiSuccess` / `func ApiError` | `rg "func Api(Success\|Error)" common/gin.go` | L199, L213 | audit controller 响应入口（无 ApiErrorStr）|
| `web/src/features/usage-logs/section-registry.tsx` | `USAGE_LOGS_SECTIONS` | `rg "USAGE_LOGS_SECTIONS" .../section-registry.tsx` | L24 | 加 audit section 是数组追加 |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | `interface DetailsDialogProps` | `rg "interface DetailsDialogProps" .../details-dialog.tsx` | L472 | 审计 Tab 在此组件内注册 |
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | `parseLogOther(props.log.other)` | `rg "parseLogOther" .../details-dialog.tsx` | L483 | other.admin_info.audit 指针从此读取 |
| `web/src/features/usage-logs/components/columns/common-logs-columns.tsx` | `quota_saturation badge` | `rg "quota_saturation" .../common-logs-columns.tsx` | L108 | 审计徽章照此模式实现 |
| `web/src/features/usage-logs/components/dialogs/prompt-dialog.tsx` | `useCopyToClipboard` | `rg "useCopyToClipboard" .../prompt-dialog.tsx` | L26 | 复制 segment.text 复用此 hook |
| `web/src/features/usage-logs/types.ts` | `interface LogOtherData` | `rg "interface LogOtherData" .../types.ts` | L115 | 需新增 `admin_info.audit` 字段类型 |
| `web/src/features/usage-logs/data/schema.ts` | `request_id: z.string` | `rg "request_id" .../data/schema.ts` | L46 | 审计详情 API 请求用此字段 |
| `web/src/i18n/locales/en.json` | i18n 平铺 JSON | `rg "audit" web/src/i18n/locales/en.json` | 待添加 | 7 语言均需新增 audit 相关 key |

**定位清单 ASM/待勘察 占比**：8 「待创建」条目 / 47 总条目 ≈ **17%**，低于 30% 阈值 ✓

---

## 4. Phase 计划与任务详情

> **Stage 2 补全**：每条任务详情（操作步骤、验证命令、evidence 要求）将在 Stage 2 写入。
> 本 Stage 交付 Phase 依赖链 + 每 Phase 一句话目标。

### Phase 依赖链

```
Phase 0（P0）
  勘察校准
  ├── 验证 AutoMigrate 运行时建表疑点（F-35）
  └── 验证 audit/service/relay 分层不成环
        │
Phase 1（P1）
  OnInput + OnSettled 采集骨架
  ├── audit/ 包：ContentSink 接口 + types + segment builder（OpenAI + opaque fidelity）
  ├── model/log_content.go：logs_content 表 DDL
  ├── service/audit_sink.go：LogContentSink 实现
  ├── relay/common/relay_info.go：ContentSink 字段注入
  ├── controller/relay.go：OnInput 钩子
  └── service/text_quota.go：OnSettled 钩子
        │
Phase 2（P2）
  Response 采集
  ├── relay/channel/openai：OnOutput 钩子（stream + non-stream）
  └── service/audit_sink.go：OnOutput 实现
        │
Phase 3（P3）
  Claude / Gemini / Responses 多格式 walker
  ├── audit/segment.go Claude walker
  └── audit/segment.go Gemini walker（relayconvert 已有转换逻辑）
        │
Phase 4（P4）
  Watchlist + 重扫
  ├── model/audit_watchlist_rule.go + audit_watchlist_meta
  ├── service/audit_watchlist.go：keyword/domain/regex 扫描 + 重扫逻辑
  └── controller/audit.go + router：watchlist CRUD API + rescan API
        │
Phase 5（P5）
  前端可视化 + 全套测试
  ├── 日志列表审计徽章（common-logs-columns.tsx）
  ├── 详情弹窗审计 Tab（details-dialog.tsx）
  ├── watchlist 管理页 + 设置页审计配置
  ├── i18n（7 语言）
  └── 执行 spec 5.2 真实场景全套测试
```

### Phase 0: 勘察校准（P0）

**你在哪里**：代码回滚干净，存在两个未闭环疑点（F-35 AutoMigrate 运行时未生效；分层不成环待正式验证）。
**做完之后**：两个疑点有明确结论和 evidence，写入 spec 事实基线；开发可无障碍开始 Phase 1。

> 任务详情见 Stage 2。包含：T-01 AutoMigrate 疑点校准；T-02 import cycle 边界测试；T-03 Phase 0 回归验证。
### 内嵌状态表（Stage 1 骨架；Stage 2 将替换为 tasks.csv）

> 任务数预计 ≥ 8，Stage 2 补全任务详情后生成 tasks.csv 并删除本表。当前仅列出已确定的 Phase 0 任务作为占位。

| 序号 | 任务 | 前置 | 验证命令 | 状态 |
|---|---|---|---|---|
| 1 | 校准 AutoMigrate 运行时建表疑点（F-35）| 无 | `sqlite3 one-api.db ".tables" \| grep log_content` → 非空 | 待开始 |
| 2 | 验证 audit/service/relay 分层不成环 | 无 | `GOWORK=off go build ./audit/... && echo BUILD_OK` | 待开始 |
| 3 | 执行 Phase 0 回归验证 | 1;2 | `make test` → 无 FAIL；EVD-007/EVD-008 归档 | 待开始 |
| … | Phase 1～Phase 4 任务（Stage 2 展开）| — | — | 待开始 |
| N-1 | 执行 spec 5.2 真实场景全套测试 | N-2 | 2.3 节全部 UF 主路径+失败分支通过；evidence/ 齐全 | 待开始 |
| N | 执行 Phase 5 回归验证 | N-1 | `make test` → 无 FAIL；`cd web && bun run typecheck` → 0 | 待开始 |



### Phase 1: OnInput + OnSettled 采集骨架（P1）

**你在哪里**：P0 疑点已闭环，分层验证通过。
**做完之后**：发一条 OpenAI curl 请求 → logs_content 表有记录（fidelity=structured 或 opaque），OnInput + OnSettled 均被调用，分层不成环，build + test 全绿。

> 任务详情见 Stage 2。包含：T-04～T-11（audit 包骨架、model DDL、sink 实现、relay_info 注入、controller 钩子、text_quota 钩子、option 开关、Phase 1 回归验证）。

### Phase 2: Response 采集（P2）

**你在哪里**：P1 完成，logs_content 有输入记录。
**做完之后**：流式和非流式 OpenAI 响应均采集到 assistant segments，fidelity=structured。

> 任务详情见 Stage 2。包含：T-12～T-15。

### Phase 3: Claude / Gemini / Responses 多格式 Walker（P3）

**你在哪里**：P2 完成，OpenAI 格式全链路通。
**做完之后**：发 Claude 和 Gemini 格式请求，segments 有正确的 kind 和 text，非 OpenAI 格式不再退化为 opaque。

> 任务详情见 Stage 2。包含：T-16～T-19。

### Phase 4: Watchlist + 重扫（P4）

**你在哪里**：P3 完成，三格式采集均正确。
**做完之后**：管理员可 CRUD watchlist 规则；新请求实时命中；重扫可触发并显示进度；flags 含 rule_id + pattern 快照；BR-010 regex 上限有效。

> 任务详情见 Stage 2。包含：T-20～T-28。

### Phase 5: 前端可视化 + 全套测试（P5）

**你在哪里**：P4 完成，全部后端能力就绪。
**做完之后**：管理员可在 UI 看到审计徽章、在弹窗查看并复制审计内容、在设置页配置参数、在管理页 CRUD watchlist、发起并跟踪重扫；普通用户隔离通过；spec 5.2 全套测试通过；evidence 齐全。

> 任务详情见 Stage 2。包含：T-29～T-4x（前端各 UF 组件 + 接线 + i18n + 真实场景全套测试 + Phase 5 回归验证）。

#### Task 98: 执行 spec 5.2 真实场景全套测试

- **关联**：UF-001 ～ UF-007（全部用户可见 UF）
- **前置任务**：Phase 5 前端实现任务全部完成
- **验证**：按 5.2 执行矩阵逐行回放，全部通过；evidence/ 齐全
- **Evidence**：`evidence/UF-001/` ～ `evidence/UF-007/`

#### Task 99: 执行 Phase 5 回归验证

- **关联**：BR-001～BR-017 / INV-001～INV-006 全部
- **前置任务**：98
- **验证**：`make test` → 无 FAIL；`cd web && bun run typecheck` → exit 0；`cd relaykit && GOWORK=off go build ./...` → BUILD_OK
- **Evidence**：`evidence/phase-5/`

---

## 5. 验收与 Review 协议

> **验收铁律：命令级验证（5.1）通过只是入场券，不是完成。** 用户可见需求必须通过 5.2 真实场景全套测试才算完成。

### 5.1 命令级验证（入场券）

> **Stage 2 补全**：完整验证矩阵在 Stage 2 写入。已知必要项如下。

| 验证项 | 命令 | 期望 | Evidence |
|---|---|---|---|
| 根模块构建 | `GOWORK=off go build ./...` | BUILD_OK | EVD-008 |
| relaykit 独立构建 | `cd relaykit && GOWORK=off go build ./...` | BUILD_OK | EVD-008 |
| 全量测试 | `make test` | 无 FAIL | EVD-007 |
| audit 包不成环 | `GOWORK=off go build ./audit/...` | BUILD_OK | EVD-008 |
| 前端类型检查 | `cd web && bun run typecheck` | exit 0 | EVD-010 |

### 5.2 真实场景全套测试（Real-Run，完成的唯一标准）

> 在真实运行的应用上，把第 2.3 节每条流程脚本从头到尾走一遍。禁止用"跑了单测"替代本节。

**环境准备**：

| 项 | 值 |
|---|---|
| 后端启动命令 | `cd /Users/nothing/workspace/new-api-better/new-api && go run main.go` |
| 前端启动命令 | `make dev-web`（:5173，代理 /api → :3000）|
| 访问入口 | http://localhost:5173 |
| 测试账号/数据 | 管理员账号（root，初次启动设置密码）+ 普通用户账号 + 至少一个 Channel + Token |
| 干净状态定义 | `sqlite3 one-api.db "DELETE FROM logs_content"` + 重启服务 |
| 可用测试工具 | 浏览器自动化 MCP（chrome-devtools-proxy）已配置；curl；sqlite3 |
| 后端日志查看 | `go run main.go` stdout；或 `journalctl` 若以 systemd 运行 |

**执行矩阵**（Stage 2 补全每行操作细节；结构已定）：

| UF | 执行方式 | 操作来源 | 必须核对的点 | Evidence |
|---|---|---|---|---|
| UF-001 主路径 | browser MCP | 2.3 UF-001 成功主路径 | 命中行出现审计徽章；颜色/数字正确；console 无 error | `evidence/UF-001/badge.png` |
| UF-001 失败分支：无命中 | browser MCP | 2.3 UF-001 失败分支 | 无命中行不显示徽章 | `evidence/UF-001/no-hit.png` |
| UF-002 主路径 | browser MCP | 2.3 UF-002 成功主路径 | 审计 Tab 可点击；segments/flags 正确展示 | `evidence/UF-002/detail-tab.png` |
| UF-002 失败分支：无记录 | browser MCP | 2.3 UF-002 空态分支 | 显示"暂无审计内容" | `evidence/UF-002/empty.png` |
| UF-003 主路径 | browser MCP | 2.3 UF-003 成功主路径 | 复制按钮状态变化；剪贴板有内容 | `evidence/UF-003/copy.png` |
| UF-004 隔离验证 | curl | 2.3 UF-004 成功主路径 | 普通用户响应无 admin_info | `evidence/UF-004/user-response.json` |
| UF-005 主路径 | browser MCP | 2.3 UF-005 成功主路径 | 设置保存 toast；再次打开值已更新 | `evidence/UF-005/settings.png` |
| UF-006 新增规则 | browser MCP | 2.3 UF-006 成功主路径 | 规则出现在列表；version 徽章递增 | `evidence/UF-006/crud.png` |
| UF-006 regex 超限 | browser MCP + curl | 2.3 UF-006 regex 超限分支 | 第 9 条 regex 被拒绝，toast "已达上限" | `evidence/UF-006/regex-limit.png` |
| UF-007 主路径 | browser MCP | 2.3 UF-007 成功主路径 | 进度条更新；完成 toast；server log 有完成条目 | `evidence/UF-007/rescan-progress.png` |
| UF-007 失败分支：无记录 | browser MCP | 2.3 UF-007 无记录分支 | toast "无需重扫" | `evidence/UF-007/no-op.png` |

**通过标准**：执行矩阵全部行通过且 evidence 齐全。任何一行失败 = 本需求未完成，回对应任务修复后重跑。

### 5.4 Review 专项检查清单（预览）

- [ ] `audit/` 包无 `model`、`relay/common` import（BR-004）
- [ ] `cd relaykit && GOWORK=off go build ./...` 通过（BR-017）
- [ ] `logs.other.admin_info.audit` 字节数 < 200B（BR-002）
- [ ] GET /api/log/self 响应无 admin_info（INV-005）
- [ ] relay P99 在 ab/wrk 下与 audit 开关关闭时差异 < 5ms（INV-001）
- [ ] logs_content 在 SQLite / PG 双库 AutoMigrate 无错误（BR-016）
- [ ] 5.2 执行矩阵全部通过，evidence 齐全（完成的唯一标准）
- [ ] 2.3 节每条流程的入口接线清单已实现，从真实入口可达

---

## 质量记录

| 项 | 结果 |
|---|---|
| Stage | Stage 1（Skeleton 骨架） |
| 事实基线（F-xx 条目数）| 36 |
| 假设（ASM-xxx）| 5（ASM-001～ASM-005） |
| 业务规则（BR）| 17（BR-001～BR-017） |
| 用户场景（UF）| 9（UF-001～UF-009；7 个用户可见有流程脚本，2 个内部已豁免）|
| 不变量（INV）| 6（INV-001～INV-006）|
| 证据清单（EVD）| 10（EVD-001～EVD-010）|
| 定位清单 ASM/待勘察 占比 | 8/47 ≈ 17%（< 30% ✓）|
| 待 P0 校准疑点 | 1（F-35 AutoMigrate 运行时建表）|
| validate_package.py | 待 Stage 2 完成后运行 |
| 基线测试 | make test 全绿（2026-08-01）|

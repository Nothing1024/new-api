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
| F-35 | `model/main.go:399` `migrateLOGDB` 原始版：仅 AutoMigrate `&Log{}`；服务运行时 SQLite `logs_content` 表不存在（F-35a）；隔离测试 newGormConfig(true) 调 AutoMigrate 成功建表（F-35b） | `sqlite3 one-api.db "SELECT name..."` = 空；隔离测试 PASS | **P0 已闭环：正常行为非 bug**——LogContent 结构体从未注册到 migrateLOGDB，故表自然不创建；AutoMigrate 机制本身有效（F-35b 证明）。修复 = Task 7 在 migrateLOGDB 追加 `&LogContent{}` |
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

## 3. 技术方案

### 3.1 架构 Before / After

```
Before:
  relay 请求
    controller/relay.go:Relay(L71)
      → helper.GetAndValidateRequest(L112) → request DTO
      → relaycommon.GenRelayInfo(L123)     → *RelayInfo
      → relay adaptor.DoResponse(~L160)
      → service.PostTextConsumeQuota(L397)
          → attachQuotaSaturation(L524)
          → model.RecordConsumeLog(L343)
               ↓
            LOG_DB: logs   (Other TEXT, admin_info = quota_saturation 指针)

After（新增 audit 钩子 ×3，粗体为新代码）:
  relay 请求
    controller/relay.go:Relay
      → helper.GetAndValidateRequest → request DTO
      → relaycommon.GenRelayInfo     → *RelayInfo
          + ContentSink 字段注入（ASM-001，推荐挂 RelayInfo）
      → [NEW] if sink != nil { go sink.OnInput(InputSnapshot) }    ←── Phase 1
      → relay adaptor.DoResponse
          OaiStreamHandler / OpenaiHandler
          → [NEW] if sink != nil { go sink.OnOutput(OutputSnapshot) }  ←── Phase 2
      → service.PostTextConsumeQuota
          → attachQuotaSaturation(L524)
          → [NEW] if sink != nil { go sink.OnSettled(UsageSnapshot) }  ←── Phase 1
          → model.RecordConsumeLog
               ↓                       ↓
            LOG_DB: logs              LOG_DB: logs_content (新表)
            Other.admin_info.audit    segments / flags / fidelity
              = {request_id, hit_count}  (通过 request_id 关联)
```

**分层约束**（F-34 实测成环，强制）:

```
audit/              纯域层：ContentSink 接口 + 快照类型 + segment builder
                    可 import: common, relaykit/dto, relaykit/types
                    禁止 import: model, relay/common
       ↑
service/            具体 sink 实现（唯一同时 import audit + model 的层）
                    LogContentSink 实现 audit.ContentSink
       ↑
relay/common/       持有接口字段 RelayInfo.ContentSink（类型 audit.ContentSink）
                    不持有实现，不 import service
```

### 3.2 模块改造

| 模块 | 职责 | 改造说明 |
|---|---|---|
| `audit/` (新包) | 纯域：接口定义 + 快照类型 + segment builder | 新建；3 个文件：types.go / segment.go / (无 model import) |
| `model/log_content.go` (新文件) | LogContent 结构体 + CRUD | 新建；AutoMigrate 注册到 migrateLOGDB |
| `model/audit_watchlist_rule.go` (新文件) | AuditWatchlistRule + AuditWatchlistMeta | 新建；主库 AutoMigrate |
| `model/main.go` | 迁移注册 | migrateLOGDB 追加 &LogContent{}；InitDB 追加 &AuditWatchlistRule{}, &AuditWatchlistMeta{} |
| `relay/common/relay_info.go` | RelayInfo 持有 sink 接口 | 新增字段 `ContentSink audit.ContentSink`（ASM-001）|
| `controller/relay.go` | OnInput 钩子 | GetAndValidateRequest 之后，DoResponse 之前插入 |
| `service/text_quota.go` | OnSettled 钩子 | attachQuotaSaturation 之后，RecordConsumeLog 之前插入 |
| `service/audit_sink.go` (新文件) | LogContentSink 实现 | OnInput / OnOutput / OnSettled 三方法 + channel buffer |
| `service/audit_watchlist.go` (新文件) | 扫描 + 重扫 | keyword/domain/regex 三档扫描；重扫分批（500/批 + 100ms sleep）|
| `controller/audit.go` (新文件) | 审计 API handlers | 7 个 handler |
| `router/api-router.go` | 路由注册 | logRoute 和 apiRouter 下注册 8 条审计路由 |
| `model/option.go` | Audit options | 新增 3 个 option（AuditEnabled, AuditPerRequestMaxBytes, AuditContentTTLDays）|
| `web/src/features/usage-logs/types.ts` | 前端类型 | LogOtherData.admin_info.audit 新增字段 |
| `web/src/features/usage-logs/components/columns/common-logs-columns.tsx` | 审计徽章 | renderCell 新增审计命中 badge（照 quota_saturation L108 模式）|
| `web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | 审计 Tab | DetailsDialog 内新增审计 Tab，展示 segments + flags |
| `web/src/features/usage-logs/section-registry.tsx` | audit section | USAGE_LOGS_SECTIONS 追加 audit 项 |
| `web/src/i18n/locales/{lang}.json` | i18n（7 语言）| 新增 audit 相关 key |
| `web/src/features/audit/` (新目录) | watchlist 管理页 + 重扫 UI | 页面组件 + api.ts + types.ts |

### 3.3 三段式定位清单

（已在 Stage 1 写入，见上方 §3.3）

### 3.4 API / 数据 / 权限 / 路由影响

**新增 API 端点**（全部 AdminAuth）：

| 方法 | 路径 | 说明 | 响应 |
|---|---|---|---|
| GET | `/api/log/content` | 查询单条审计内容（?request_id=）| LogContent JSON |
| GET | `/api/audit/watchlist` | 列出 watchlist 规则（?enabled=&kind=）| []AuditWatchlistRule |
| POST | `/api/audit/watchlist` | 新增规则 | 201 + created rule |
| PUT | `/api/audit/watchlist/:id` | 更新规则 | updated rule |
| DELETE | `/api/audit/watchlist/:id` | 删除规则 | 204 |
| POST | `/api/audit/rescan` | 发起重扫 | {total_records, wl_version} |
| GET | `/api/audit/rescan/status` | 查询重扫进度 | {processed, total, status} |

**新增数据表**：

| 表名 | 数据库 | 主键 | 关键列 |
|---|---|---|---|
| `logs_content` | LOG_DB | request_id (varchar 64) | user_id, channel_id, created_at(idx), model_name, fidelity, segments(TEXT), hit_severity(idx), hit_count, flags(TEXT), wl_version(idx) |
| `audit_watchlist_rules` | 主库 | id (uint) | kind(domain/keyword/regex), pattern, severity, enabled, note, created_at, updated_at |
| `audit_watchlist_meta` | 主库 | id=1（单行）| version int |

**新增 Option**（option 表 key-value）：

| Key | 类型 | 默认值 | 说明 |
|---|---|---|---|
| AuditEnabled | bool | false | 审计总开关 |
| AuditPerRequestMaxBytes | int | 65536 | 单请求最大采集字节数 |
| AuditContentTTLDays | int | 30 | logs_content 保留天数 |

**权限影响**：

| 类型 | 是否影响 | 说明 | 兼容策略 |
|---|---|---|---|
| API | 是 | 新增 7 条 AdminAuth 路由 | 普通用户无感知 |
| 数据 | 是 | 新增 3 张表；AutoMigrate 兼容三库 | GORM AutoMigrate 向前兼容 |
| 权限 | 是 | AdminAuth middleware 前置 | 沿用现有 middleware |
| 路由 | 是 | logs 组新增 /content；新增 /audit 组 | 现有路由不改 |
| relay 性能 | 否 | sink 全异步，BR-006 drop-on-full | P99 不变（INV-001）|
| 旧日志兼容 | 是 | 无 logs_content 记录的旧日志：弹窗审计 Tab 显示空态 | 前端已处理空态（UF-002）|

---
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

> Phase 依赖链：P0 → P1 → P2 → P3 → P4 → P5

```
P0 勘察校准 → P1 OnInput+OnSettled骨架 → P2 Response采集
                                       → P3 多格式Walker
                                                        → P4 Watchlist+重扫 → P5 前端+全套测试
```

> 任务数 = 45 ≥ 8，状态板见同目录 tasks.csv。

---

### Phase 0: 勘察校准（P0）

> **你在哪里**：代码回滚干净，存在 F-35 AutoMigrate 运行时疑点和分层不成环待正式验证。
> **做完之后**：两个疑点有明确结论，写入质量记录；开发可安心开始 P1。

#### Task 1: 校准 AutoMigrate 运行时建表疑点

- **关联**：BR-014, BR-016 / UF-008（NA 内部）/ INV-003 / EVD-001
- **前置任务**：无
- **风险等级**：P0（疑点不清则 P1 可能重蹈失败）

**为什么做**：F-35 记录隔离测试建表成功但服务重启后 sqlite3 无 logs_content，根因不明可能导致 P1 建表失败。

**涉及文件与定位**：
- `model/main.go`：`func migrateLOGDB`，`rg "func migrateLOGDB" model/main.go`，L399
- `model/gorm_logger.go`：`func newGormConfig`，`rg "func newGormConfig" model/gorm_logger.go`，L25

**具体操作**：
1. 在 migrateLOGDB 中添加临时 `common.SysLog("migrateLOGDB: start")` 打印，启动服务（`go run main.go`），查看 stdout 确认该行出现
2. 在 InitLogDB 的 IsMasterNode 检查前后加 SysLog，确认该函数执行路径；`NODE_TYPE` 未设时 `IsMasterNode=true`，排除 slave 节点跳过问题
3. 结论：原始 migrateLOGDB 只 AutoMigrate `&Log{}`，LogContent 结构体尚未存在，故 logs_content 表自然不存在——这是正常行为而非 bug；隔离测试是临时新增了 AutoMigrate 调用
4. 移除临时 SysLog；将疑点结论写入 spec 1.3（更新 F-35 说明）

**验证**：`go run main.go 2>&1 | grep "migrateLOGDB\|database migrat"` → 出现迁移日志

**Evidence**：`evidence/phase-0/automigrate-diagnosis.txt`

**注意事项**：F-08 已确认 GORM LogLevel=Warn，CREATE TABLE 不打印；不能用「日志里没有 CREATE TABLE」来判断 AutoMigrate 未执行

---

#### Task 2: 验证 audit/service/relay 分层不成环

- **关联**：BR-004, BR-017 / UF-008（NA 内部）/ INV-004 / EVD-008
- **前置任务**：无
- **风险等级**：P0（成环则 P1 编译失败，F-34 已有真实案例）

**为什么做**：F-34 证明 audit → model → relay/common → audit 会成环；必须在 P1 动手前确认设计分层可行。

**涉及文件与定位**：
- `audit/types.go`（待创建）：ContentSink interface
- `common/`：`rg "package common" common/constants.go`，可 import

**具体操作**：
1. 创建最小骨架：`mkdir -p audit && echo 'package audit
import "github.com/QuantumNous/new-api/common"

var _ = common.LogConsumeEnabled' > audit/types_test.go`
2. 运行 `GOWORK=off go build ./audit/...` → 确认通过（可 import common）
3. 在骨架里尝试 `import "github.com/QuantumNous/new-api/model"` → 确认报 import cycle
4. 删除测试骨架（或保留为 T-04 起点）
5. 将验证结论写入 evidence

**验证**：`GOWORK=off go build ./audit/... && echo BUILD_OK` 无错误

**Evidence**：`evidence/phase-0/import-cycle-proof.txt`

---

#### Task 3: 执行 Phase 0 回归验证

- **关联**：BR-004, BR-016 / INV-003, INV-004
- **前置任务**：1; 2

**验证**：`make test` → 无 FAIL；`GOWORK=off go build ./...` → 通过；结论文档写入 evidence/phase-0/

**Evidence**：`evidence/phase-0/`

---

### Phase 1: OnInput + OnSettled 采集骨架（P1）

> **你在哪里**：P0 疑点已闭环，分层可行。
> **做完之后**：一条 OpenAI curl 请求 → logs_content 表有记录（fidelity=structured 或 opaque），OnInput + OnSettled 均被调用，build + make test 全绿。

#### Task 4: 创建 audit/ 包——接口与快照类型

- **关联**：BR-004, BR-005, BR-006, BR-007 / UF-008 / INV-004 / EVD-008
- **前置任务**：3
- **风险等级**：P0（所有 P1 任务依赖此接口定义）

**为什么做**：定义 ContentSink 接口和三个快照类型，是分层架构的契约基础。

**涉及文件与定位**：
- `audit/types.go`（待创建）
- 可 import：`common`，`relaykit/dto`，`relaykit/types`

**具体操作**：
1. 创建 `audit/types.go`，定义：
   - `ContentSink interface { OnInput(InputSnapshot); OnOutput(OutputSnapshot); OnSettled(UsageSnapshot, ctx) }`
   - `InputSnapshot`：RequestId, UserId, ChannelId(ASM-001 推迟到 OnSettled), ModelName, Segments []Segment, Fidelity
   - `OutputSnapshot`：RequestId, Segments []Segment（assistant kind）
   - `UsageSnapshot`：RequestId, UserId, ChannelId, ModelName, PromptTokens, CompletionTokens, Quota, WLVersion
   - `Segment`：Kind(string), Idx(int), Text(string), Bytes(int), Mode(string), Truncated(bool), SHA256(string), Derived(*DerivedFacts), Reason(string)
   - `DerivedFacts`：URLs []string, Domains []string, Tools []string, ArgsKeys []string, Chars int
   - `Fidelity`：const structured/opaque/meta_only
2. 确保无 model / relay/common import

**验证**：`GOWORK=off go build ./audit/... && echo BUILD_OK`

**Evidence**：`evidence/phase-1/build-check.txt`

---

#### Task 5: 实现 audit/segment.go——OpenAI ParseContent + opaque fidelity

- **关联**：BR-007, BR-008, BR-009 / UF-008 / EVD-002
- **前置任务**：4
- **风险等级**：P1

**为什么做**：ParseContent 将 OpenAI Message[] 转为 []Segment，是 OnInput 的核心逻辑；opaque fidelity 是兜底路径。

**涉及文件与定位**：
- `audit/segment.go`（待创建）
- `relaykit/dto/openai_request.go`：`func (m *Message) ParseContent`，L543；`type Message struct`，L303
- `relaykit/dto/openai_request.go`：`func (r *GeneralOpenAIRequest) GetTokenCountMeta`，L119（CombineText 在 L202 丢失角色，不能用）

**具体操作**：
1. 创建 `audit/segment.go`，实现 `BuildOpenAISegments(msgs []dto.Message, cfg SegmentConfig) []Segment`
   - 遍历 msg，调用 `msg.ParseContent()` 得 []MediaContent
   - 按 BR-008 表映射 kind → 默认 mode/limit
   - 对 image/audio：mode=omitted
   - 先 derive（提取 urls/domains/tools 从 text 内容）再 apply mode（BR-007）
   - 超 per_request_max_bytes 时按 BR-009 降级顺序调整 mode
2. 实现 `BuildOpaqueSegment(body []byte, sha256 string) []Segment`（fidelity=opaque 兜底）
3. 实现 `BuildMetaSegment() []Segment`（fidelity=meta_only，无 text）

**验证**：写单元测试：user 16KB msg → mode=full；超限请求 → tool_result 先降级；`GOWORK=off go test ./audit/...`

**Evidence**：`evidence/phase-1/segment-test.txt`

**注意事项**：`(m *Message) ParseContent()` 真实定义在 L543，L733 在块注释内（F-22）；`CombineText` 在 GetTokenCountMeta L202 已丢失角色（F-23），不能代替 ParseContent 获取 kind 信息

---

#### Task 6: 创建 model/log_content.go——LogContent DDL + CRUD

- **关联**：BR-001, BR-012, BR-016 / UF-008 / INV-003 / EVD-001
- **前置任务**：3
- **风险等级**：P0（表 DDL 影响所有写入操作）

**为什么做**：LogContent 是 logs_content 表的 GORM 结构体，需要三库兼容 DDL。

**涉及文件与定位**：
- `model/log_content.go`（待创建）
- `model/log.go`：`type Log struct`，L61（参考字段风格）
- `model/locking.go`：`func lockForUpdate`，L20（写锁模式参考）

**具体操作**：
1. 创建 `model/log_content.go`，定义：
   ```go
   type LogContent struct {
       RequestId        string `gorm:"primaryKey;type:varchar(64)"`
       UserId           int    `gorm:"index"`
       ChannelId        int
       CreatedAt        int64  `gorm:"index"`
       ModelName        string `gorm:"type:varchar(128)"`
       PromptTokens     int
       CompletionTokens int
       Quota            int
       Fidelity         string `gorm:"type:varchar(16)"`
       Segments         string `gorm:"type:text"`  // JSON []Segment
       HitSeverity      string `gorm:"type:varchar(8);index"`
       HitCount         int
       Flags            string `gorm:"type:text"`  // JSON []HitFlag
       WLVersion        int    `gorm:"index"`
   }
   ```
2. 实现 `CreateLogContent(lc *LogContent) error`（`LOG_DB.Create(lc).Error`）
3. 实现 `GetLogContent(requestId string) (*LogContent, error)`（`LOG_DB.First`）
4. 使用 `common.Marshal/Unmarshal` 处理 Segments/Flags JSON（AGENTS.md 要求）

**验证**：`GOWORK=off go build ./model/... && echo BUILD_OK`

**Evidence**：`evidence/phase-1/build-check.txt`

**注意事项**：不用 `AUTO_INCREMENT`；`gorm:primaryKey` 让 GORM 处理三库 PK；Text 列在三库均支持（BR-016）

---

#### Task 7: 更新 migrateLOGDB 注册 LogContent

- **关联**：BR-014, BR-016 / EVD-001
- **前置任务**：6
- **风险等级**：P0（建表是 sink 写入的前提）

**涉及文件与定位**：
- `model/main.go`：`func migrateLOGDB`，`rg "func migrateLOGDB" model/main.go`，L399

**具体操作**：
1. 在 migrateLOGDB 中，ClickHouse 分支之外追加 `LOG_DB.AutoMigrate(&LogContent{})`
2. ClickHouse 分支（`UsingLogDatabase(ClickHouse)`）不调用此行（BR-014）

**验证**：`go run main.go &`，等待启动，`sqlite3 one-api.db ".tables" | grep log_content` → 非空；kill server

**Evidence**：`evidence/phase-1/automigrate.txt`（含 sqlite3 输出）

---

#### Task 8: 创建 service/audit_sink.go——LogContentSink 实现

- **关联**：BR-005, BR-006, BR-015 / UF-008, UF-009 / INV-001 / EVD-002
- **前置任务**：4; 6
- **风险等级**：P0（relay 链路稳定性关键）

**为什么做**：LogContentSink 是唯一允许同时 import audit + model 的层，实现三方法。

**涉及文件与定位**：
- `service/audit_sink.go`（待创建）
- `audit/types.go`：ContentSink interface
- `model/log_content.go`：CreateLogContent
- `logger/logger.go`：`func LogWarn`，L80
- `common/gopool.go`：`func RelayCtxGo`，L23

**具体操作**：
1. 定义 `LogContentSink`，内含 buffered channel（容量 1024）和 error counter
2. `OnInput(snap InputSnapshot)`：将 snap 投入 channel，select with default（drop-on-full，BR-006）
3. `OnOutput(snap OutputSnapshot)`：同上
4. `OnSettled(snap UsageSnapshot, ctx)`：同上
5. worker goroutine 消费 channel：
   - OnInput：构建 LogContent（segments from snap），调 CreateLogContent
   - OnSettled：更新 logs.other.admin_info.audit 指针（BR-002），计算 hit_severity
   - 任何 error：`logger.LogWarn(ctx, "audit sink: "+err.Error())`（BR-015）
   - panic recover（BR-006，INV-001）
6. `NewLogContentSink() *LogContentSink` 构造函数，启动 worker

**验证**：curl 发 OpenAI 请求 → `sqlite3 one-api.db "SELECT request_id FROM log_contents LIMIT 1"` → 非空

**Evidence**：`evidence/phase-1/sink-invoke.json`

**注意事项**：OnSettled 中更新 logs.other 需要先 GetLog 再 UpdateLog，若 RecordConsumeLog 已写入则 OnSettled 应在之后触发；或通过 OnSettled 在 RecordConsumeLog 之前合并写 logs_content（推荐）

---

#### Task 9: relay/common/relay_info.go 注入 ContentSink 字段

- **关联**：BR-005, ASM-001 / UF-008 / INV-004
- **前置任务**：4
- **风险等级**：P1

**涉及文件与定位**：
- `relay/common/relay_info.go`：`type RelayInfo struct`，`rg "type RelayInfo struct" relay/common/relay_info.go`，L83

**具体操作**：
1. 在 RelayInfo struct 末尾添加 `ContentSink audit.ContentSink`（ASM-001）
2. import `audit "github.com/QuantumNous/new-api/audit"`
3. relay/common 不持有 LogContentSink 实现——只持有接口，零 model 依赖

**验证**：`GOWORK=off go build ./relay/common/... && echo BUILD_OK`；`cd relaykit && GOWORK=off go build ./...` 仍通过（relaykit 不 import audit）

**Evidence**：`evidence/phase-1/build-check.txt`

---

#### Task 10: controller/relay.go 添加 OnInput 钩子

- **关联**：BR-005, BR-006, BR-008 / UF-008 / INV-001
- **前置任务**：8; 9
- **风险等级**：P1

**涉及文件与定位**：
- `controller/relay.go`：`func Relay`，L71；`helper.GetAndValidateRequest`，L112；`relaycommon.GenRelayInfo`，L123

**具体操作**：
1. 在 GenRelayInfo 成功之后（约 L124），拿到 `relayInfo`
2. 注入 sink：`relayInfo.ContentSink = service.GetAuditSink()`（GetAuditSink 检查 AuditEnabled option）
3. 在 DoResponse 之前插入：
   ```go
   if sink := relayInfo.ContentSink; sink != nil {
       common.RelayCtxGo(c, func() { sink.OnInput(...) })
   }
   ```
4. BuildInputSnapshot 在此处调用（从 request DTO 构建 segments）

**验证**：curl OpenAI 请求 → server log 无 audit 相关 error；`sqlite3 one-api.db "SELECT count(*) FROM log_contents"` → 递增

**Evidence**：`evidence/phase-1/sink-invoke.json`

---

#### Task 11: service/text_quota.go 添加 OnSettled 钩子

- **关联**：BR-001, BR-002, BR-005 / UF-008 / INV-001 / EVD-002
- **前置任务**：8; 9
- **风险等级**：P1

**涉及文件与定位**：
- `service/text_quota.go`：`func PostTextConsumeQuota`，L397；`attachQuotaSaturation`，L524
- `service/log_info_generate.go`：`func attachQuotaSaturation`，L40

**具体操作**：
1. 在 attachQuotaSaturation(L524) 之后、`model.RecordConsumeLog` 之前插入：
   ```go
   if sink := relayInfo.ContentSink; sink != nil {
       snap := buildUsageSnapshot(relayInfo, usage)
       common.RelayCtxGo(ctx, func() { sink.OnSettled(snap, ctx) })
   }
   ```
2. OnSettled 在 sink worker 内还需：更新 logs.other.admin_info.audit（BR-002）；此更新需在 RecordConsumeLog 之后，因此 sink worker 加一个 sleep/retry 或直接在 OnSettled worker 内 goroutine 延迟更新

**验证**：curl → `sqlite3 one-api.db "SELECT request_id, hit_count FROM log_contents"` → 有对应记录；`SELECT other FROM logs WHERE request_id=?` → other 含 admin_info.audit 字段

**Evidence**：`evidence/phase-1/sink-invoke.json`

---

#### Task 12: model/option.go 添加审计 options

- **关联**：BR-005, BR-010 / UF-005 / INV-002
- **前置任务**：3
- **风险等级**：P2

**涉及文件与定位**：
- `model/option.go`：`OptionMap["LogConsumeEnabled"]`，`rg "LogConsumeEnabled" model/option.go`，L50, L330
- `common/constants.go`：`var LogConsumeEnabled`，L93

**具体操作**：
1. 在 `common/constants.go` 新增：`var AuditEnabled = false`、`var AuditPerRequestMaxBytes = 65536`、`var AuditContentTTLDays = 30`
2. 在 `model/option.go` 默认值区（参考 L50 模式）设置三个默认值
3. 在 sync switch（参考 L330 模式）新增三个 case

**验证**：`GOWORK=off go build ./...`；启动服务 → PUT /api/option {AuditEnabled:true} → 下次请求 sink != nil

**Evidence**：`evidence/phase-1/options-sync.txt`

---

#### Task 13: 执行 Phase 1 回归验证

- **关联**：BR-001～BR-006, BR-015, BR-016 / UF-008 / INV-001, INV-002, INV-003, INV-004 / EVD-001, EVD-002, EVD-007, EVD-008
- **前置任务**：7; 10; 11; 12

**验证**：
1. `make test` → 无 FAIL（INV-003）
2. `GOWORK=off go build ./...` → BUILD_OK；`cd relaykit && GOWORK=off go build ./...` → BUILD_OK（INV-004）
3. `sqlite3 one-api.db ".tables" | grep log_content` → `log_contents` 存在（EVD-001）
4. curl 发 OpenAI 请求（AuditEnabled=true）→ sqlite3 有新记录（EVD-002）
5. 普通用户 GET /api/log/self → other 无 admin_info（INV-005）

**Evidence**：`evidence/phase-1/`

---
### Phase 2: Response 采集（P2）

> **你在哪里**：P1 完成，logs_content 有输入记录，assistant 段为空。
> **做完之后**：流式和非流式 OpenAI 响应均采集到 assistant segments。

#### Task 14: audit/types.go 补充 OutputSnapshot

- **关联**：BR-006, BR-008 / UF-008 / INV-001
- **前置任务**：4
- **风险等级**：P2

**涉及文件与定位**：`audit/types.go`（已存在，Task 4 创建）

**具体操作**：`OutputSnapshot` 已在 Task 4 定义；确认字段覆盖：RequestId, Segments（assistant kind, mode 按 BR-008）。如 Task 4 有遗漏则补全。

**验证**：`GOWORK=off go build ./audit/...`

---

#### Task 15: relay-openai.go OaiStreamHandler 添加 OnOutput 钩子

- **关联**：BR-006, BR-008, ASM-002 / UF-008 / INV-001 / EVD-003
- **前置任务**：13
- **风险等级**：P1

**为什么做**：流式响应的 assistant text 在 OaiStreamHandler 的 responseTextBuilder 中累积，函数结束时有完整文本。

**涉及文件与定位**：
- `relay/channel/openai/relay-openai.go`：`func OaiStreamHandler`，L104；`responseTextBuilder strings.Builder`，L117；`service.ResponseText2Usage`，L182（文本已完整）

**具体操作**：
1. 在 L182 `ResponseText2Usage` 之后，return 之前插入：
   ```go
   if sink := info.ContentSink; sink != nil {
       text := responseTextBuilder.String()
       if text != "" {
           snap := audit.OutputSnapshot{RequestId: info.RequestId,
               Segments: []audit.Segment{{Kind:"assistant", Text:text, Mode:"full", Bytes:len(text)}}}
           common.RelayCtxGo(c, func() { sink.OnOutput(snap) })
       }
   }
   ```
2. 超过 per_request_max_bytes 时 truncate text，设 Truncated=true

**验证**：curl 流式请求 → `sqlite3 one-api.db "SELECT segments FROM log_contents WHERE request_id=?" ` → 含 assistant kind 条目

**Evidence**：`evidence/phase-2/response-capture.json`

**注意事项**：ASM-002 推荐推迟到 OnSettled 前写；此 Task 选在 OaiStreamHandler 末尾写，避免 OnSettled 可能不被调用的数据丢失风险

---

#### Task 16: relay-openai.go OpenaiHandler 添加 OnOutput 钩子

- **关联**：BR-006, BR-008 / UF-008 / INV-001 / EVD-003
- **前置任务**：13
- **风险等级**：P1

**涉及文件与定位**：
- `relay/channel/openai/relay-openai.go`：`func OpenaiHandler`，L222

**具体操作**：
1. 在 OpenaiHandler 读取 response body 并解析后，提取 choices[0].message.content
2. 同 Task 15 注入 OnOutput（非流式 text 直接从响应 DTO 取）
3. 注意 image/audio 响应：content 为空时用 opaque segment

**验证**：curl 非流式请求 → logs_content.segments 含 assistant

**Evidence**：`evidence/phase-2/response-capture.json`

---

#### Task 17: service/audit_sink.go OnOutput 合并写入 logs_content

- **关联**：BR-001, ASM-002 / UF-008 / INV-001
- **前置任务**：8; 15; 16
- **风险等级**：P1

**涉及文件与定位**：`service/audit_sink.go`（Task 8 创建）

**具体操作**：
1. OnOutput worker：接收 OutputSnapshot，暂存到 `sync.Map[requestId]->assistantText`
2. OnSettled worker：从 Map 取出 assistantText，追加到 LogContent.Segments 后写库；Map 中 delete key

**验证**：curl 流式请求 → logs_content.segments 同时含 user + assistant kind

**Evidence**：`evidence/phase-2/response-capture.json`

---

#### Task 18: 执行 Phase 2 回归验证

- **关联**：BR-001, BR-008 / UF-008 / INV-001, INV-003 / EVD-003
- **前置任务**：15; 16; 17

**验证**：
1. `make test` → 无 FAIL
2. curl 流式 + 非流式 OpenAI 请求各一条 → logs_content.segments 含 user + assistant
3. relay P99 未见明显抬升

**Evidence**：`evidence/phase-2/`

---

### Phase 3: Claude / Gemini / Responses 多格式 Walker（P3）

> **你在哪里**：P2 完成，OpenAI 格式全链路通，非 OpenAI 格式退化为 opaque。
> **做完之后**：Claude 和 Gemini 格式请求 segments 有正确 kind + text，不再全 opaque。

#### Task 19: audit/segment.go Claude walker

- **关联**：BR-007, BR-008 / UF-008 / EVD-002
- **前置任务**：18
- **风险等级**：P2

**涉及文件与定位**：
- `relaykit/dto/claude.go`：`func (c *ClaudeMessage) ParseContent`，L168；`type ClaudeMediaMessage struct`，L17；`ClaudeRequest.GetTokenCountMeta`，L243

**具体操作**：
1. 在 `audit/segment.go` 实现 `BuildClaudeSegments(req *dto.ClaudeRequest, cfg SegmentConfig) []Segment`
2. 遍历 req.Messages，调用 `msg.ParseContent()` 得 []ClaudeMediaMessage
3. ClaudeMediaMessage.Type="text" → kind 按 msg.Role 映射；Type="image" → omitted；Type="tool_result" → drop + derive
4. SystemInstructions 字段 → kind=system

**验证**：`GOWORK=off go test ./audit/... -run TestClaudeSegments` → 通过

**Evidence**：`evidence/phase-3/claude-segments.txt`

---

#### Task 20: audit/segment.go Gemini walker

- **关联**：BR-007, BR-008 / UF-008 / EVD-002
- **前置任务**：18
- **风险等级**：P2

**涉及文件与定位**：
- `relaykit/dto/gemini.go`：`type GeminiChatRequest struct`，L12；`type GeminiPart struct`，L270；`SystemInstructions *GeminiChatContent`（req.SystemInstructions）

**具体操作**：
1. 实现 `BuildGeminiSegments(req *dto.GeminiChatRequest, cfg SegmentConfig) []Segment`
2. 遍历 req.Contents（role=user/model），遍历 Parts，`Part.Text != ""` → text segment；`Part.InlineData != nil` → omitted；`Part.FunctionCall != nil` → tool_call + derive（函数名）；`Part.FunctionResponse != nil` → tool_result + drop + derive
3. req.SystemInstructions → kind=system

**验证**：`GOWORK=off go test ./audit/... -run TestGeminiSegments` → 通过

**Evidence**：`evidence/phase-3/gemini-segments.txt`

---

#### Task 21: service/audit_sink.go OnInput 路由多格式

- **关联**：BR-004, BR-008 / UF-008
- **前置任务**：19; 20
- **风险等级**：P1

**涉及文件与定位**：`service/audit_sink.go`（Task 8 创建）；`relay/common/relay_info.go`：`GetFinalRequestRelayFormat()`（grep 确认存在）

**具体操作**：
1. OnInput worker 中，根据 relayInfo.RelayFormat 路由到不同 builder：
   - RelayFormatOpenAI → BuildOpenAISegments
   - RelayFormatClaude → BuildClaudeSegments
   - RelayFormatGemini → BuildGeminiSegments
   - 其他 → BuildOpaqueSegment（fidelity=opaque）
2. 若 builder panic，recover → 退化 opaque（BR-015）

**验证**：curl Claude 格式请求 → logs_content.fidelity="structured"，segments 含 user/assistant kind

**Evidence**：`evidence/phase-3/claude-segments.txt`

---

#### Task 22: 执行 Phase 3 回归验证

- **关联**：BR-004, BR-008 / INV-003, INV-004 / EVD-002
- **前置任务**：19; 20; 21

**验证**：`make test` → 无 FAIL；`GOWORK=off go build ./...` → BUILD_OK；Claude + Gemini curl 各一条 → fidelity=structured

**Evidence**：`evidence/phase-3/`

---

### Phase 4: Watchlist + 重扫（P4）

> **你在哪里**：P3 完成，三格式采集正确，无命中检测。
> **做完之后**：管理员可 CRUD watchlist 规则，新请求实时命中，重扫可触发并显示进度，BR-010 regex 上限有效。

#### Task 23: model/audit_watchlist_rule.go + audit_watchlist_meta

- **关联**：BR-010, BR-011 / UF-006 / INV-003
- **前置任务**：3
- **风险等级**：P0

**涉及文件与定位**：
- `model/audit_watchlist_rule.go`（待创建）
- `model/locking.go`：`func lockForUpdate`，L20（写锁参考）

**具体操作**：
1. 定义 `AuditWatchlistRule`：id/kind/pattern/severity/enabled/note/CreatedAt/UpdatedAt
2. 定义 `AuditWatchlistMeta`：id=1（固定行）, version int
3. CRUD 函数：`ListWatchlistRules`, `CreateRule`, `UpdateRule`, `DeleteRule`（DeleteRule: version++，CreateRule: version++，UpdateRule: version++）
4. `GetWatchlistVersion() int`
5. regex 上限检查：CreateRule/UpdateRule 时 `SELECT count(*) FROM audit_watchlist_rules WHERE kind='regex' AND enabled=1` ≥ 8 → 返回 ErrRegexLimit

**验证**：`GOWORK=off go build ./model/...`

**Evidence**：`evidence/phase-4/watchlist-crud.json`

---

#### Task 24: 更新 InitDB 注册 watchlist 表

- **关联**：BR-011, BR-016 / EVD-004
- **前置任务**：23
- **风险等级**：P0

**涉及文件与定位**：`model/main.go`：InitDB 的主库 AutoMigrate 调用处（grep `AutoMigrate` model/main.go 确认行号）

**具体操作**：在主库 AutoMigrate 调用中追加 `&AuditWatchlistRule{}` 和 `&AuditWatchlistMeta{}`；插入 seed 行（`AuditWatchlistMeta{ID:1, Version:0}`，upsert 语义）

**验证**：`go run main.go &`; `sqlite3 one-api.db ".tables"` 含 `audit_watchlist_rules` + `audit_watchlist_meta`; kill

**Evidence**：`evidence/phase-4/tables.txt`

---

#### Task 25: service/audit_watchlist.go 扫描逻辑

- **关联**：BR-007, BR-010, BR-012 / UF-006 / INV-001
- **前置任务**：23
- **风险等级**：P1

**涉及文件与定位**：
- `service/audit_watchlist.go`（待创建）
- `service/str.go`：`func getOrBuildAC`，L98（keyword 扫描复用）
- `service/sensitive.go`：`func AcSearch`，L132（参考用法）

**具体操作**：
1. `ScanSegments(segs []audit.Segment, rules []model.AuditWatchlistRule) []HitFlag`
   - domain 档：遍历 seg.Derived.Domains，map lookup（约50ns/行）
   - keyword 档（enabled=true）：复用 `getOrBuildAC`，对 seg.Text 调 AcSearch
   - regex 档（enabled=true，最多8条）：对 seg.Text 循环 regexp.MatchString
2. 每条命中：`HitFlag{RuleId, PatternSnapshot, Kind, Severity, SegIdx}`（BR-012）
3. `MaxSeverity(flags []HitFlag) string` 取最高 severity

**验证**：`GOWORK=off go test ./service/... -run TestScanSegments`

**Evidence**：`evidence/phase-4/scan-test.txt`

---

#### Task 26: service/audit_sink.go OnSettled 集成 watchlist 扫描

- **关联**：BR-007, BR-012, BR-013 / UF-001, UF-008 / INV-001
- **前置任务**：8; 25
- **风险等级**：P1

**涉及文件与定位**：`service/audit_sink.go`（Task 8 创建）

**具体操作**：
1. OnSettled worker：取本请求 segments，调 ScanSegments 得 flags
2. 更新 LogContent.Flags / HitSeverity / HitCount / WLVersion
3. 若 ScanSegments error（regex 编译失败等）：LogWarn + 继续写库（flags 为空）

**验证**：创建 keyword 规则 "敏感词" → curl 含该词的请求 → `sqlite3 "SELECT hit_count, flags FROM log_contents"` → hit_count > 0

**Evidence**：`evidence/phase-4/scan-test.txt`

---

#### Task 27: controller/audit.go watchlist CRUD + log content API

- **关联**：BR-010, BR-011, BR-012 / UF-002, UF-006 / EVD-004
- **前置任务**：23; 25
- **风险等级**：P1

**涉及文件与定位**：
- `controller/audit.go`（待创建）
- `common/gin.go`：`func ApiSuccess`，L213；`func ApiError`，L199（无 ApiErrorStr，F-12）
- `common/json.go`：`func Marshal`，L21

**具体操作**：
1. `GetLogContent(c *gin.Context)`：取 `c.Query("request_id")` → `model.GetLogContent` → `ApiSuccess`；404 时 `ApiErrorMsg`
2. `ListWatchlistRules` / `CreateWatchlistRule` / `UpdateWatchlistRule` / `DeleteWatchlistRule`：CRUD handlers，regex 超限返回 400

**验证**：curl GET /api/log/content?request_id=xxx → 200 JSON；curl POST /api/audit/watchlist → 201；curl 第 9 条 regex → 400

**Evidence**：`evidence/phase-4/watchlist-crud.json`

---

#### Task 28: controller/audit.go rescan + service/audit_watchlist.go 重扫逻辑

- **关联**：BR-013 / UF-007 / EVD-009
- **前置任务**：25; 27
- **风险等级**：P1

**涉及文件与定位**：
- `service/audit_watchlist.go`（Task 25 创建）
- `common/constants.go`：`var IsMasterNode`，`common/init.go`，L89（只 master 节点可重扫）

**具体操作**：
1. `RescanLogContents(ctx, wlVersion int)` goroutine：按 `created_at DESC` 分批（500/批 + 100ms sleep）扫 wl_version < wlVersion 且 created_at > NOW()-TTL 的行（BR-013）
2. 进度写 option key `AuditRescanStatus`（JSON：{processed, total, status, wl_version}）
3. `GetRescanStatus` 读 option
4. `TriggerRescan` handler：检查 IsMasterNode；若已有重扫在运行返回 409；启动 goroutine

**验证**：curl POST /api/audit/rescan → 200；GET /api/audit/rescan/status → processed 递增；server log 含完成条目

**Evidence**：`evidence/UF-007/rescan-progress.png`

---

#### Task 29: router/api-router.go 注册审计路由

- **关联**：BR-003 / UF-002, UF-004, UF-006, UF-007 / EVD-004
- **前置任务**：27; 28
- **风险等级**：P1

**涉及文件与定位**：
- `router/api-router.go`：`logRoute := apiRouter.Group`，L271；`middleware.AdminAuth()`（已存在）

**具体操作**：
1. 在 logRoute 组添加：`logRoute.GET("/content", middleware.AdminAuth(), controller.GetLogContent)`（BR-003，UF-004 隔离）
2. 新建 `auditRoute := apiRouter.Group("/audit")`，全部 `middleware.AdminAuth()`：
   - GET /watchlist, POST /watchlist, PUT /watchlist/:id, DELETE /watchlist/:id
   - POST /rescan, GET /rescan/status

**验证**：`curl -H "Authorization: Bearer <user_token>" /api/log/content` → 401；`curl -H "Authorization: Bearer <admin_token>" /api/audit/watchlist` → 200

**Evidence**：`evidence/UF-004/user-response.json`

---

#### Task 30: 执行 Phase 4 回归验证

- **关联**：BR-001～BR-017 / UF-006, UF-007, UF-008 / INV-001～INV-006 / EVD-004, EVD-009
- **前置任务**：26; 27; 28; 29

**验证**：
1. `make test` → 无 FAIL
2. CRUD watchlist → version 递增（BR-011）
3. 含命中词 curl → logs_content.hit_count > 0（BR-007 + BR-012）
4. 9 条 regex → 400（BR-010）
5. 重扫触发 → 进度更新 → 完成（BR-013）
6. 普通用户 GET /api/log/content → 401（BR-003）

**Evidence**：`evidence/phase-4/`

---
### Phase 5: 前端可视化 + 全套测试（P5）

> **你在哪里**：P4 完成，全部后端能力就绪。
> **做完之后**：UF-001～UF-007 全部通过真实场景验证，evidence 齐全。

#### Task 31: web/src/features/usage-logs/types.ts 添加 audit 类型

- **关联**：BR-002, BR-003 / UF-001, UF-002 / EVD-005
- **前置任务**：30
- **风险等级**：P2

**涉及文件与定位**：
- `web/src/features/usage-logs/types.ts`：`interface LogOtherData`，`rg "interface LogOtherData" web/src/features/usage-logs/types.ts`，L115

**具体操作**：
1. 在 `LogOtherData.admin_info` 中追加：
   ```ts
   audit?: { request_id: string; hit_count: number; hit_severity?: string }
   ```
2. 新增 `interface AuditSegment`：kind/idx/text/bytes/mode/truncated/sha256/derived/reason
3. 新增 `interface LogContent`：request_id/fidelity/segments/flags/hit_severity/hit_count
4. 新增 `interface HitFlag`：rule_id/pattern_snapshot/kind/severity/seg_idx

**验证**：`cd web && bun run typecheck` → exit 0

---

#### Task 32: 审计命中徽章（UF-001）

- **关联**：BR-002, BR-003 / UF-001 / EVD-005
- **前置任务**：31
- **风险等级**：P2

**涉及文件与定位**：
- `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`：`quota_saturation badge`，L108

**具体操作**：
1. 照 L108 `quota_saturation` 模式，在 renderCell 函数内新增：
   ```ts
   if (isAdmin && other?.admin_info?.audit?.hit_count > 0) {
       segments.push({ text: `命中 ${audit.hit_count}`, danger: audit.hit_severity === 'high' })
   }
   ```
2. 颜色：severity=high → 红色；medium → 橙色；low → 黄色

**验证**：`bun run typecheck` → exit 0；browser MCP 截图确认命中行出现徽章

**Evidence**：`evidence/UF-001/badge.png`

---

#### Task 33: 详情弹窗审计 Tab——segment 展示（UF-002）

- **关联**：BR-001, BR-002 / UF-002 / EVD-005
- **前置任务**：31
- **风险等级**：P1

**涉及文件与定位**：
- `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`：`interface DetailsDialogProps`，L472；`parseLogOther(props.log.other)`，L483；admin_info guard，L383

**具体操作**：
1. 在 `details-dialog.tsx` 的 Tab 组件中新增"审计"Tab（当 isAdmin && log.request_id 时显示）
2. Tab 激活时调 `GET /api/log/content?request_id=` 获取 LogContent
3. 展示：fidelity badge + segments 列表（kind标签/mode badge/bytes/text展开/derived chips）+ flags 区（规则快照列表）
4. 无记录时空态；loading/error 态

**验证**：`bun run typecheck` → exit 0；browser MCP 截图审计 Tab

**Evidence**：`evidence/UF-002/detail-tab.png`

---

#### Task 34: segment 复制按钮（UF-003）

- **关联**：BR-008 / UF-003 / EVD-005
- **前置任务**：33
- **风险等级**：P3

**涉及文件与定位**：
- `web/src/features/usage-logs/components/dialogs/prompt-dialog.tsx`：`useCopyToClipboard`，L26（复用模式）

**具体操作**：
1. 在审计 Tab 每个 segment（mode ∈ {full, preview}）行右侧添加复制图标按钮
2. onClick：`copyToClipboard(segment.text)`，按钮短暂变对勾（2s）
3. segment.mode=drop/omit 时不显示复制按钮，只显示 mode badge

**验证**：`bun run typecheck` → exit 0；browser MCP 点击复制 → console 无 error

**Evidence**：`evidence/UF-003/copy.png`

---

#### Task 35: GET /api/log/content AdminAuth 隔离验证（UF-004）

- **关联**：BR-003 / UF-004 / EVD-006
- **前置任务**：29
- **风险等级**：P0（安全隔离）

**涉及文件与定位**：`router/api-router.go`：Task 29 已注册，middleware.AdminAuth() 已加

**具体操作**：
1. 确认 `GET /api/log/content` 路由已有 `middleware.AdminAuth()`（Task 29）
2. curl 用普通用户 token → 401；`jq '.data[].other | fromjson | has("admin_info")'` on /api/log/self → false

**验证**：`curl -H "Authorization: Bearer <user_token>" http://localhost:3000/api/log/content?request_id=xxx` → 401

**Evidence**：`evidence/UF-004/user-response.json`

---

#### Task 36: 审计配置 Settings section（UF-005）

- **关联**：BR-005, BR-008 / UF-005 / EVD-010
- **前置任务**：12; 30
- **风险等级**：P2

**具体操作**：
1. 在系统设置页面新增"审计配置"区块（复用现有 SettingsSection 组件模式）
2. 字段：审计总开关（Switch）、per_request_max_bytes（NumberInput）、content_ttl_days（NumberInput）
3. 保存按钮 → PUT /api/option 批量更新三个 key

**验证**：`bun run typecheck` → exit 0；browser MCP 截图设置页审计区块 + 保存 toast

**Evidence**：`evidence/UF-005/settings.png`

---

#### Task 37: watchlist 管理页（UF-006）

- **关联**：BR-010, BR-011, BR-012 / UF-006 / EVD-004
- **前置任务**：29; 30
- **风险等级**：P1

**具体操作**：
1. 新建 `web/src/features/audit/` 目录，创建 `api.ts`（watchlist CRUD + rescan API 调用）+ `types.ts`（AuditWatchlistRule 等）
2. 创建 `WatchlistPage` 组件：规则列表表格（kind/pattern/severity/enabled 列）+ 新增/编辑 Modal + 删除确认
3. 编辑 Modal：表单校验 pattern 非空，regex 类型显示语法预检，超限返回400时 toast

**验证**：`bun run typecheck` → exit 0；browser MCP 截图规则列表 + 新增弹窗

**Evidence**：`evidence/UF-006/crud.png`

---

#### Task 38: 重扫进度 UI（UF-007）

- **关联**：BR-013 / UF-007 / EVD-009
- **前置任务**：28; 37
- **风险等级**：P2

**具体操作**：
1. 在 watchlist 管理页添加"重扫"按钮 + 确认弹窗
2. 点击确认 → POST /api/audit/rescan；开启轮询（2s 间隔 GET /api/audit/rescan/status）
3. 顶部或页内进度条显示 processed/total；完成 toast；no-op 时 toast 无需重扫

**验证**：browser MCP 截图进度条更新；server log 含完成条目

**Evidence**：`evidence/UF-007/rescan-progress.png`

---

#### Task 39: section-registry.tsx + 路由注册

- **关联**：UF-005, UF-006, UF-007 / EVD-010
- **前置任务**：37; 38
- **风险等级**：P2

**涉及文件与定位**：
- `web/src/features/usage-logs/section-registry.tsx`：`USAGE_LOGS_SECTIONS`，L24
- 应用路由文件（待 grep 确认路径）

**具体操作**：
1. USAGE_LOGS_SECTIONS 追加 `{ id: 'audit', titleKey: 'Audit Logs', build: () => null }` 项
2. 在 admin 路由组注册 `/audit/watchlist` → WatchlistPage
3. 确认 sidebar/menu 有审计入口链接

**验证**：`bun run typecheck` → exit 0；browser MCP 访问 /audit/watchlist → 页面正常加载

**Evidence**：`evidence/UF-006/crud.png`

---

#### Task 40: i18n 7 语言新增 audit 相关 key

- **关联**：UF-001～UF-007 / INV-006
- **前置任务**：31
- **风险等级**：P3

**涉及文件与定位**：`web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`，F-18

**具体操作**：
1. 在 en.json 新增所有 audit UI 文案 key（徽章标签、Tab 名、segment kind 名称、severity 标签、watchlist 表单 label、重扫文案等约 20 条）
2. 在其余 6 个语言文件补充对应翻译（zh 手动；其余 5 种可用 bun run i18n:sync 生成占位后补译）

**验证**：`bun run typecheck` → exit 0；`bun run i18n:sync` 无缺失 key 警告

**Evidence**：`evidence/phase-5/i18n-check.txt`

---

#### Task 41: 前端 audit API 封装（api.ts + types.ts）

- **关联**：UF-002, UF-006, UF-007
- **前置任务**：31
- **风险等级**：P2

**具体操作**：
1. `web/src/features/audit/api.ts`：封装 getLogContent / listWatchlistRules / createRule / updateRule / deleteRule / triggerRescan / getRescanStatus，复用 fetchWrapper（参考现有 api.ts 模式）
2. `web/src/features/audit/types.ts`：导出 AuditWatchlistRule, LogContent, HitFlag, RescanStatus

**验证**：`bun run typecheck` → exit 0

---

#### Task 42: 执行 spec 5.2 真实场景全套测试

- **关联**：UF-001～UF-007（全部用户可见 UF）/ INV-001～INV-006 / EVD-001～EVD-010
- **前置任务**：32; 33; 34; 35; 36; 37; 38; 39; 40; 41
- **风险等级**：P0（真实场景是完成的唯一标准）

**为什么做**：spec 5.2 执行矩阵定义了完成的唯一标准；所有单测通过不等于 UF 可达。

**具体操作**：按 spec 5.2 执行矩阵逐行回放，使用浏览器 MCP + curl：
1. 启动后端 `go run main.go`（:3000）
2. 启动前端 `make dev-web`（:5173）
3. 逐行执行 5.2 矩阵：UF-001 主路径 + 失败分支 → UF-007，每行截图 + console + network
4. curl EVD-006（普通用户隔离）
5. 归档所有 evidence

**验证**：5.2 矩阵全部行通过；evidence/ 目录齐全

**Evidence**：`evidence/UF-001/` ～ `evidence/UF-007/`

---

#### Task 43: 执行 Phase 5 回归验证

- **关联**：BR-001～BR-017 / INV-001～INV-006 / EVD-007, EVD-008
- **前置任务**：42

**验证**：
1. `make test` → 无 FAIL
2. `GOWORK=off go build ./...` → BUILD_OK
3. `cd relaykit && GOWORK=off go build ./...` → BUILD_OK
4. `cd web && bun run typecheck` → exit 0
5. `bun run build` → 前端构建成功（无 TS error）

**Evidence**：`evidence/phase-5/`

---
## 5. 验收与 Review 协议

> **验收铁律：命令级验证（5.1）通过只是入场券，不是完成。** 用户可见的需求必须通过 5.2 真实场景全套测试才算完成。

### 5.1 命令级验证（入场券）

| 验证项 | 命令 | 期望 | Evidence |
|---|---|---|---|
| 根模块构建 | `GOWORK=off go build ./...` | BUILD_OK | EVD-008 |
| relaykit 独立构建 | `cd relaykit && GOWORK=off go build ./...` | BUILD_OK | EVD-008 |
| audit 包不成环 | `GOWORK=off go build ./audit/...` | BUILD_OK（无 cycle）| EVD-008 |
| 全量测试（根模块+relaykit）| `make test` | 无 FAIL；relaykit ok | EVD-007 |
| audit 包单测 | `GOWORK=off go test ./audit/...` | PASS | EVD-007 |
| service 包单测 | `GOWORK=off go test ./service/...` | PASS | EVD-007 |
| 前端类型检查 | `cd web && bun run typecheck` | exit 0 | EVD-010 |
| 前端构建 | `cd web && bun run build` | exit 0，无 TS error | EVD-010 |
| i18n 同步检查 | `cd web && bun run i18n:sync` | 无缺失 key 警告 | `evidence/phase-5/i18n-check.txt` |
| 普通用户 API 隔离 | `curl -H "Authorization: Bearer <user>" /api/log/content?request_id=xxx` | 401 | EVD-006 |
| logs_content 表存在 | `sqlite3 one-api.db ".tables"` | 含 `log_contents` | EVD-001 |
| watchlist 表存在 | `sqlite3 one-api.db ".tables"` | 含 `audit_watchlist_rules` | `evidence/phase-4/tables.txt` |
| 旧数据兼容（无 audit 行）| `curl /api/log/self` → 弹窗审计 Tab | 空态提示，无 JS error | EVD-005 |

### 5.2 真实场景全套测试（Real-Run，完成的唯一标准）

> 在真实运行的应用上，把第 2.3 节每条流程脚本从头到尾走一遍。禁止用"跑了单测"或"读了代码确认逻辑"代替本节。

**环境准备**：

| 项 | 值 |
|---|---|
| 后端启动命令 | `cd /Users/nothing/workspace/new-api-better/new-api && go run main.go` |
| 前端启动命令 | `make dev-web`（:5173，代理 /api / /mj / /pg → :3000）|
| 访问入口 | http://localhost:5173 |
| 测试账号/数据 | 管理员账号（root，初次启动设置密码）+ 普通用户账号 + 至少 1 个 Channel + 1 个 Token；审计功能需通过设置页 AuditEnabled=true 后生效 |
| 干净状态定义 | `sqlite3 one-api.db "DELETE FROM log_contents; DELETE FROM audit_watchlist_rules"` + 重启服务；AuditEnabled=true |
| 可用测试工具 | 浏览器自动化 MCP（chrome-devtools-proxy）已配置；curl；sqlite3；server stdout |

**执行矩阵**：

| UF | 执行方式 | 操作来源 | 必须核对的点 | Evidence |
|---|---|---|---|---|
| UF-001 主路径 | browser MCP | 2.3 UF-001 成功主路径 | 命中行出现彩色徽章；hover tooltip 正确；console 无 error | `evidence/UF-001/badge.png` |
| UF-001 失败分支：无命中 | browser MCP | 2.3 UF-001 失败分支 | 无命中行不显示徽章（正常） | `evidence/UF-001/no-hit.png` |
| UF-001 失败分支：审计关闭 | browser MCP + curl | 2.3 UF-001 失败分支 | 关闭 AuditEnabled → 所有行无徽章 | `evidence/UF-001/disabled.png` |
| UF-002 主路径 | browser MCP | 2.3 UF-002 成功主路径 | 审计 Tab 可点；loading→segments 渲染；flags 区显示；console 无 error | `evidence/UF-002/detail-tab.png` |
| UF-002 失败分支：无记录 | browser MCP | 2.3 UF-002 空态分支 | 显示"暂无审计内容"，无 JS error | `evidence/UF-002/empty.png` |
| UF-002 失败分支：网络错误 | browser MCP + DevTools | 2.3 UF-002 网络错误分支 | error state + 重试按钮 | `evidence/UF-002/error.png` |
| UF-003 主路径 | browser MCP | 2.3 UF-003 成功主路径 | 复制按钮状态变化（2s 对勾）；剪贴板有 segment.text | `evidence/UF-003/copy.png` |
| UF-003 失败分支：drop segment | browser MCP | 2.3 UF-003 失败分支 | mode=drop segment 无复制按钮 | `evidence/UF-003/no-copy.png` |
| UF-004 隔离验证 | curl + jq | 2.3 UF-004 主路径 | 普通用户 /api/log/self 无 admin_info；/api/log/content → 401 | `evidence/UF-004/user-response.json` |
| UF-005 主路径 | browser MCP | 2.3 UF-005 成功主路径 | 设置保存 toast；再次打开值已持久化；下次请求按新策略采集 | `evidence/UF-005/settings.png` |
| UF-005 失败分支：参数越界 | browser MCP | 2.3 UF-005 失败分支 | 输入 -1 → 前端校验错误提示 | `evidence/UF-005/validation.png` |
| UF-006 新增规则 | browser MCP | 2.3 UF-006 成功主路径 | 规则出现列表；version 徽章 +1；后续请求命中该规则 | `evidence/UF-006/crud.png` |
| UF-006 regex 超限 | browser MCP | 2.3 UF-006 regex 超限分支 | 第 9 条 regex 被拒绝，toast "已达上限 8 条" | `evidence/UF-006/regex-limit.png` |
| UF-006 删除规则 | browser MCP | 2.3 UF-006 删除分支 | 确认后规则消失；version +1 | `evidence/UF-006/delete.png` |
| UF-007 主路径 | browser MCP | 2.3 UF-007 成功主路径 | 进度条更新；完成 toast；server log 有完成条目；logs_content.wl_version 更新 | `evidence/UF-007/rescan-progress.png` |
| UF-007 失败分支：无可扫记录 | browser MCP | 2.3 UF-007 无记录分支 | toast "无需重扫" | `evidence/UF-007/no-op.png` |
| UF-007 失败分支：重扫中再次点击 | browser MCP | 2.3 UF-007 重复发起分支 | 按钮禁用，toast "重扫进行中" | `evidence/UF-007/in-progress.png` |

**通过标准**：执行矩阵全部行通过且 evidence 齐全。任何一行失败 = 本需求未完成，回对应任务修复后重跑。

### 5.3 Evidence 目录结构

```text
evidence/
  phase-0/    automigrate-diagnosis.txt, import-cycle-proof.txt
  phase-1/    automigrate.txt, build-check.txt, sink-invoke.json, segment-test.txt, server.log
  phase-2/    response-capture.json
  phase-3/    claude-segments.txt, gemini-segments.txt
  phase-4/    watchlist-crud.json, scan-test.txt, tables.txt, rescan.txt
  phase-5/    i18n-check.txt, build-final.txt
  UF-001/     badge.png, no-hit.png, disabled.png
  UF-002/     detail-tab.png, empty.png, error.png
  UF-003/     copy.png, no-copy.png
  UF-004/     user-response.json
  UF-005/     settings.png, validation.png
  UF-006/     crud.png, regex-limit.png, delete.png
  UF-007/     rescan-progress.png, no-op.png, in-progress.png
```

### 5.4 Review 专项检查清单

- [ ] `audit/` 包无 `model`、`relay/common` import（`GOWORK=off go build ./audit/...` 通过且无 cycle 报错）
- [ ] `cd relaykit && GOWORK=off go build ./...` 通过（relaykit 未引入 audit 依赖）
- [ ] `logs.other.admin_info.audit` 单条记录字节数 < 200B（jq 抽查验证）
- [ ] GET /api/log/self 普通用户响应无 admin_info 字段（jq 验证）
- [ ] relay ab/wrk 测试：audit on/off P99 差异 < 5ms（INV-001）
- [ ] logs_content 在 SQLite AutoMigrate 无错误；PG 容器或 CI 验证兼容性（BR-016）
- [ ] 5.2 执行矩阵全部通过，evidence 齐全且与 EVD-001~EVD-010 对应
- [ ] 2.3 节每条流程的「入口接线清单」已实现——从真实入口（菜单/路由/按钮）可达
- [ ] 界面交互与 2.3 节脚本逐步一致（loading/禁用态/错误提示/成功反馈均存在）
- [ ] 所有 BR/UF/INV 状态可对照第 2 章逐条核销

---

## 质量记录

| 项 | Stage 1（骨架）| Stage 2（展开）|
|---|---|---|
| spec 行数 | 676 | 1664 |
| 事实基线（F-xx）| 36 | 37（F-37 新增 Gemini DTO 事实）|
| 假设（ASM）| 5 | 5（未新增）|
| BR | 17 | 17 |
| UF | 9（7 用户可见）| 9 |
| INV | 6 | 6 |
| EVD | 10 | 10 |
| Phase / Task | 6 Phase（骨架）| 6 Phase / 43 Task |
| 定位清单 ASM/待勘察 | 17% | 17% |
| validate_package.py | — | **0 FAIL / 12 PASS**（2026-08-01）|
| 基线测试 | make test 全绿 | make test 全绿 |

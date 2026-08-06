# content-audit-v2 Spec

> Version: 0.2.0 | Date: 2026-08-06 | Status: Stage 2 展开
>
> 本文件是本需求的**唯一事实源**：事实基线、业务合同、技术方案、任务计划、验收协议全部在此。
> 其他文件（handoff.md、tasks.csv）只引用本文件，不复制内容。
>
> 三态规则：表格单元格只允许——1. 验证过的事实（注明来源命令）；2. 显式假设 `ASM-1xx`；3. `待勘察`。
> 禁止编造命令、symbol、文件名。
>
> **与 `docs/content-audit/spec.md`（v1）的关系**：v1 的 BR-001~BR-017 / INV / EVD 继续有效，本文件只定义**增量**规则，编号从 1xx 起，避免与 v1 冲突。凡本文件未提及的 v1 规则一律不得破坏（见 2.4 节 INV-101）。

---

## 1. 事实基线与假设

### 1.1 需求与运行模式

| 项 | 结论 |
|---|---|
| 原始需求 | 基于 v1 内容审计做三项修复：(1) tool 调用捕获不齐全；(2) 审计信息架构错乱——规则配置页命名像日志列表且挂在侧边栏顶级，配置应归入系统设置，`/usage-logs/audit` 是空占位；(3) 缺少常见模板规则，需支持快速切换 |
| 输入类型 | description（用户口述 3 条修复需求）+ 前序对话完成的完整调研 |
| Mode | oneclick |
| 置信度 | 高（本会话已完成后端/前端双向勘察，三项决策已由用户拍板） |
| 输出目录 | `docs/content-audit-v2/` |
| 用户已拍板决策 | ①**暂不拦截**，审计保持 record-only，后期再考虑；②留存策略**默认保守**，后期按需开放配置；③模板包**内置 + 支持导入导出** |

### 1.2 任务类型路由

| 维度 | 结论 |
|---|---|
| 任务类型 | backend（segment builder 修复、多格式 OnOutput 接线、模板包 API、审计日志列表 API）、frontend（IA 重构、模板包 UI、审计日志表）、data（rule 表加列迁移）、security（规则模板内容、权限守卫）、refactor（路由/术语收敛） |
| 主要风险 | ①改 `audit.Segment` 结构影响已落库 JSON 的兼容性；②`ProcessStreamResponse` 被 xai 复用，改签名有外溢；③删除 `/audit/watchlist` 路由需迁移全部入口引用；④rule 表加列需三库 AutoMigrate 兼容；⑤`MaxEnabledRegexRules = 8` 与模板包冲突 |
| 行号引用策略 | 业务/API 优先 symbol + rg anchor，行号仅 hint；`audit/segment.go` 与 `relay/channel/openai/` 已知会随实现漂移 |
| 必需验收方式 | backend: `make test` + curl 真实 relay 请求 + `sqlite3` 查 `log_contents`；frontend: 浏览器实际点击 + 截图 + console；data: AutoMigrate 建列确认；security: 普通用户越权 403 验证 |
| 必须覆盖用户场景 | 规则 CRUD（系统设置内）、模板包应用/切换、模板导入导出、审计日志列表筛选、重扫、普通用户越权隔离 |

### 1.3 勘察事实清单

> 每条事实来自本会话实际执行的命令。

| 事实 ID | 事实 | 来源命令 | 输出摘要 |
|---|---|---|---|
| F-101 | `defaultKindPolicy` 现值：system=preview/512B、user=full/16KB、assistant=full/16KB、tool_call=derive/1KB、tool_result=drop/0、image/audio=omitted | `read audit/segment.go:31-39` | L31-L39 确认 |
| F-102 | `makeToolCallSegment`（OpenAI，L161-189）设 `Derived={Tools,ArgsKeys}`，**从不调用 `deriveFacts`**，故 `Derived.Domains` 恒空 | `read audit/segment.go:161-189` | L173 直接构造 `&DerivedFacts{Tools,ArgsKeys}` |
| F-103 | `makeClaudeToolUseSegment`（L309-330）与 `makeGeminiFunctionCallSegment`（L379-400）均**不设 `Text`、不调 `deriveFacts`**，返回体只有 `Kind/Idx/Mode/Derived` | `read audit/segment.go:309-400` | 两函数 return 语句确认无 Text 字段 |
| F-104 | `ScanSegments` 三档匹配面：domain 只读 `seg.Derived.Domains`；keyword 与 regex 均要求 `seg.Text != ""` | `grep -n "func ScanSegments" -A60 service/audit_watchlist.go` | keyword/regex 分支条件含 `seg.Text != ""` |
| F-105 | `makeDropSegment`（L144-159）会调 `deriveFacts` 但清空 `Text`，故 tool_result 只能命中 domain 档 | `read audit/segment.go:144-159` | 无 `seg.Text` 赋值 |
| F-106 | `BuildOpenAISegments` / `BuildClaudeSegments` / `BuildGeminiSegments` 均只遍历消息，**未采集请求的 tools 定义**（`req.Tools`） | `read audit/segment.go:54-64,254-307,333-377` | 三函数体内无 `req.Tools` 引用 |
| F-107 | DTO 中 tools 定义字段：OpenAI `GeneralOpenAIRequest.Tools []ToolCallRequest`（openai_request.go:55）；Claude `ClaudeRequest.Tools any`（claude.go:221）+ `GetTools()`（L402）+ `ProcessTools()`（L425）+ `type Tool`（L172）；Gemini `GeminiChatRequest.Tools json.RawMessage`（gemini.go:17） | `grep -n "Tools" relaykit/dto/{openai_request,claude,gemini}.go` | 三处字段类型不同，需分别解析 |
| F-108 | 非流式 OpenAI OnOutput（relay-openai.go:349-360）只累加 `choice.Message.StringContent()`，**tool_calls 丢失** | `grep -n "ContentSink" -A12 relay/channel/openai/relay-openai.go` | L352-355 循环体只写 StringContent |
| F-109 | 流式 OpenAI 的 `ProcessStreamResponse`（helper.go:93-107）把 `tool.Function.Name` + `Arguments` 直接写进 `responseTextBuilder`，与正文/reasoning 混流；该 builder 最终成为单个 `KindAssistant` 段 | `read relay/channel/openai/helper.go:93-107` + `relay-openai.go:195-204` | L102-105 WriteString tool 字段 |
| F-110 | `ProcessStreamResponse` 有外部调用者 `relay/channel/xai/text.go:63`，改签名会外溢 | `grep -rn "ProcessStreamResponse" --include=*.go .` | 3 处：helper.go 定义、helper.go:117 自调、xai/text.go:63 |
| F-111 | Claude / Gemini handler **无任何 `ContentSink` 引用**，OnOutput 未接线 | `grep -rn "ContentSink" --include=*.go .` | 命中仅在 openai/relay-openai.go（L197、L350） |
| F-112 | Claude 流式累加器为 `ClaudeResponseInfo.ResponseText strings.Builder`（`relaykit/relayconvert/internal/claude_messages/to_oai_chat_resp.go:19`），别名在 `relay/channel/claude/relay-claude.go:43` | `grep -rn "ClaudeResponseInfo struct" -A14 --include=*.go .` | L15-L22 |
| F-113 | Gemini 流式累加器为局部 `responseText strings.Builder`（gemini handler L151，写入点 L173） | `grep -n "responseText" relay/channel/gemini/relay-gemini.go` | L151 声明、L173 WriteString |
| F-114 | `AuditWatchlistRule` 现有列：Id/Kind/Pattern(varchar512)/Severity/Enabled/Note(varchar512)/CreatedAt/UpdatedAt；**无来源或分组列** | `read model/audit_watchlist_rule.go:24-34` | L25-L34 |
| F-115 | `MaxEnabledRegexRules = 8`，由 `countEnabledRegexRules` 强制，超限返回 `ErrRegexLimit` | `read model/audit_watchlist_rule.go:12-22,151-159` | L18 常量、L22 error |
| F-116 | 审计路由现状：`/api/audit/watchlist` GET/POST、`/watchlist/:id` PUT/DELETE、`/rescan` POST、`/rescan/status` GET，整组 `middleware.AdminAuth()` | `grep -n "auditRoute" -A8 router/api-router.go` | L283-L292 |
| F-117 | `/api/log/content?request_id=` 为唯一审计内容读接口，**无列表接口**；`GetAllLogs` 过滤参数不含审计命中字段 | `grep -n "func GetAllLogs" -A40 model/log.go`；`grep -n "logRoute" -A12 router/api-router.go` | GetAllLogs 签名 12 参数，无 hit 相关 |
| F-118 | 前端 `/audit/watchlist` 独立路由（`web/src/routes/_authenticated/audit/watchlist/index.tsx`），有 `ROLE.ADMIN` guard，component=`WatchlistPage` | `read web/src/routes/_authenticated/audit/watchlist/index.tsx` | beforeLoad 重定向 /403 |
| F-119 | 侧边栏 `Audit Watchlist` 为 Admin 组**顶级项**，url=`/audit/watchlist`，icon=ShieldCheck | `grep -n "Audit Watchlist" -B4 -A4 web/src/hooks/use-sidebar-data.ts` | L162 |
| F-120 | 安全设置注册顺序：rate-limit → audit(`Audit Content`) → sensitive-words → ssrf → token-limits | `grep -n "id: '" -A2 web/src/features/system-settings/security/section-registry.tsx` | L29-L99 |
| F-121 | `/usage-logs/audit` 渲染 `AuditSectionPlaceholder`（L62-L74），内容为一张卡片 + 跳 `/audit/watchlist` 的按钮；`section-registry.tsx` 中 audit 项 `build: () => null` | `grep -n "AuditSectionPlaceholder\|audit" web/src/features/usage-logs/index.tsx` | L57-L74、L179-L180 |
| F-122 | `/usage-logs/$section` 路由 `beforeLoad` 只校验 section id 合法性，**无 admin guard**；search schema 无审计相关字段 | `read web/src/routes/_authenticated/usage-logs/$section.tsx` | usageLogsSearchSchema 13 字段，无 hit |
| F-123 | 混淆命名实证：zh.json L482 `Audit Content Monitoring`=审计内容监控（系统设置标题）、L486 `Audit Watchlist`=审计监控列表（规则页标题）、L2595 `Manage watchlist`=管理监控规则 | `grep -n "Audit Content Monitoring\|Audit Watchlist\|Manage watchlist" web/src/i18n/locales/zh.json` | 三条并存 |
| F-124 | i18n locale 共 7 个文件：en/zh/zh-TW/fr/ru/ja/vi，`Audit Content Monitoring` 键在全部 7 个文件的 L482 | `grep -rn "Audit Content Monitoring" web/src/i18n/locales/` | 7 命中 |
| F-125 | `AuditContentTTLDays` 唯一读点是 `runRescan` 的截止时间（service/audit_watchlist.go:183）；`log_contents` **无清理任务**（仅 `logs` 有 `DeleteOldLogBatch` + `SystemTaskTypeLogCleanup`） | `grep -rn "AuditContentTTLDays" --include=*.go .`；`grep -rn "func DeleteOldLog" model/` | TTL 无清理消费者；`DeleteOldLogBatch` 仅 model/log.go:703 |
| F-126 | `RuleEditorDialog`（web/src/features/audit/index.tsx L311-429）不渲染 `note` 输入框，但 `AuditWatchlistRuleInput` 含 note 字段 | `read web/src/features/audit/index.tsx` | 类型有 note，表单无对应 FormField |
| F-127 | `audit.Segment` 现有 9 字段，`Text` 带 `json:"text,omitempty"`；`DerivedFacts` 含 URLs/Domains/Tools/ArgsKeys/Chars | `read audit/types.go:73-92` | L73-L92 |
| F-128 | 现有 audit 包单测文件 2 个：`audit/segment_test.go`、`audit/segment_walkers_test.go` | `ls audit/` | segment.go, segment_test.go, segment_walkers_test.go, types.go |
| F-129 | 选项注册模式：`model/option.go` seed（`common.OptionMap["AuditEnabled"]=...`）+ sync switch case（`case "AuditPerRequestMaxBytes":` 带 `>0` 守卫） | `grep -n "Audit" -B2 -A6 model/option.go` | seed 段 + sync case 段 |
| F-130 | `LogContent` 表已有索引：UserId、CreatedAt、HitSeverity、WLVersion；`RequestId` 为主键 | `read model/log_content.go:10-26` | gorm tag 确认 |

### 1.4 假设清单

| 假设 ID | 内容 | 推荐值 | 风险 | 确认方式 |
|---|---|---|---|---|
| ASM-101 | 「扫描面」承载方式：给 `audit.Segment` 增加不落库字段 `ScanText string \`json:"-"\`` vs 另建并行结构体传递全文 | **推荐 `ScanText` + `json:"-"`**：builder 填全文，sink flush 前清空，落库 JSON 结构完全不变（F-127 兼容），零迁移 | 若忘记 flush 前清空，全文会随 `Segments` 落库，违反保守留存 | Phase 1 落地时以单测钉死「落库 JSON 不含 scan 文本」（EVD-102） |
| ASM-102 | 请求 tools 定义采集：新增 `KindToolDef` segment kind vs 塞进 system 段 | **推荐新增 `KindToolDef`**（默认 `preview/1KB`），语义独立、可单独调策略 | 新 kind 需同步 `downgradeOrder`、前端 kind 文案、旧数据无此 kind（前端需容错未知 kind） | Phase 1 定；前端按「未知 kind 兜底展示」实现 |
| ASM-103 | 流式 OpenAI tool_calls 独立成段的实现方式：改 `ProcessStreamResponse` 签名 vs 在 `OaiStreamHandler` 内独立累加 | **推荐在 `OaiStreamHandler` 内独立累加**（新增局部 tool 累加器），不动 `ProcessStreamResponse` 签名，避免外溢到 xai（F-110） | handler 内多一份累加逻辑；需确认 stream chunk 的 tool index 拼接正确 | Phase 2 实施；以真实流式 tool_call 请求验证 |
| ASM-104 | 模板包与规则的关联方式：rule 表加 `source`+`template_id` 两列 vs 独立关联表 | **推荐加两列**（`source varchar(16)`：manual/template；`template_id varchar(64)`），无 join、迁移简单（F-114） | 同一模板重复应用需去重逻辑（按 template_id + kind + pattern 唯一） | Phase 3 定；AutoMigrate 三库验证（EVD-106） |
| ASM-105 | 模板包 regex 与 `MaxEnabledRegexRules=8`（F-115）冲突处理：模板内 regex 规则默认 `enabled=false` vs 提高上限 | **推荐模板内 regex 默认 enabled=false**，上限保持 8 不动；应用模板时若 regex 会超限，返回明确错误并说明「已应用 N 条，regex 需手动启用」 | 用户可能误以为模板未生效 | Phase 3 定；UF-102 失败分支覆盖 |
| ASM-106 | 审计日志列表数据源：新增 `GET /api/audit/logs`（直查 `log_contents`）vs 扩展 `GetAllLogs` 加审计筛选 | **推荐新增 `GET /api/audit/logs` 直查 `log_contents`**：`HitSeverity` 已有索引（F-130），不改动 `GetAllLogs` 12 参数签名（F-117），不影响普通用户日志路径 | 与「日常日志」两个列表并存，需 UI 文案区分职责 | Phase 5 定；以 curl + 索引命中确认 |
| ASM-107 | `/audit/watchlist` 路由处置：直接删除 vs 保留并 redirect 到系统设置 | **推荐删除路由 + 删侧边栏项**（clean cutover，仓库规则要求迁移全部调用方不留 shim）；`/usage-logs/audit` 占位卡片内的跳转按钮同步改指向系统设置 | 已收藏该 URL 的管理员会 404 | Phase 4 实施；全仓 grep `audit/watchlist` 归零作为验收 |
| ASM-108 | `AuditContentTTLDays` 无清理任务（F-125）是否纳入本次范围 | **纳入**：本需求重建审计配置区，展示一个不生效的「保留天数」是明确缺陷；复用 `SystemTaskTypeLogCleanup` 同款系统任务模式 | 增加 Phase 3 工作量 | 见 BR-112；不做则须从设置 UI 移除该项 |

---

## 2. 业务合同

> 本章是本需求 BR/UF/INV/EVD 的唯一定义处。任务、handoff、review 只引用 ID，不复制表格。
> v1 规则（`docs/content-audit/spec.md` BR-001~BR-017）在本需求中作为不可破坏的既有约束，见 INV-101。

### 2.1 BR 业务规则

| 规则 ID | 规则 | 正例 | 反例 | 影响范围 | 验证方式 |
|---|---|---|---|---|---|
| BR-101 | **匹配面与留存面分离**：watchlist 扫描必须针对「截断/丢弃前的全文」执行；落库正文仍按各 kind 留存策略。承载字段 `Segment.ScanText` 标 `json:"-"`，`flush` 写库前必须清空 | 16KB tool_result 正文含关键词 → `flags` 命中；`log_contents.segments[].text` 仍为空 | 扫描只看截断后 `Text`，导致 1KB 之后的关键词漏检 | `audit/types.go`、`audit/segment.go`、`service/audit_sink.go`、`service/audit_watchlist.go` | 单测：构造超限含关键词段 → 有 HitFlag 且序列化 JSON 无该文本 |
| BR-102 | **落库 JSON 结构不变**：本需求新增的扫描字段不得出现在 `log_contents.segments` 序列化结果中，v1 已落库记录无需迁移即可继续解析 | v1 老记录经新代码 `GetLogContent` 正常返回 | 新增字段无 `json:"-"`，老前端解析出未知键 | `audit/types.go` | 单测：`common.Marshal(Segment{ScanText:"x"})` 输出不含 `x` |
| BR-103 | **tool_call 参数三档可匹配**：OpenAI/Claude/Gemini 三种格式的 tool_call segment 必须均填充 `Text`（按 tool_call 留存策略截断）、`ScanText`（全文）、`Derived`（Tools + ArgsKeys + 由参数全文 derive 出的 URLs/Domains） | Claude `tool_use` 参数含 `https://evil.com` → domain 规则命中 | Claude/Gemini tool_call 仅有 Tools/ArgsKeys，domain/keyword/regex 全部漏检（F-103） | `audit/segment.go`：`makeToolCallSegment`、`makeClaudeToolUseSegment`、`makeGeminiFunctionCallSegment` | 单测：三格式各构造含 URL + 关键词的 tool_call → 各命中 domain 与 keyword |
| BR-104 | **tool_result 正文可被 keyword/regex 匹配**：`ModeDrop` 段落库 `Text` 仍为空，但必须填 `ScanText` 全文 | 联网搜索结果回灌命中 keyword 规则，`segments[kind=tool_result].text` 为空 | tool_result 只能命中 domain 档（F-105） | `audit/segment.go`：`makeDropSegment` | 单测：drop 段含关键词 → 有 HitFlag，落库 text 为空 |
| BR-105 | **请求 tools 定义纳入采集**：三格式请求的工具定义（name/description/parameters 或 input_schema）生成 `kind=tool_def` segment，默认 `preview/1KB`；无 tools 时不产生该段 | 请求带 3 个 tool 定义 → 3 条 `kind=tool_def` 段 | tools 定义完全不采集（F-106），工具描述注入不可见 | `audit/segment.go` 三个 Build\*Segments 入口 | curl 带 tools 的请求 → `segments[]` 含 `kind=tool_def` |
| BR-106 | **`tool_def` 参与降级顺序**：`downgradeOrder` 中 `tool_def` 优先级位于 `tool_result` 之后、`tool_call` 之前（即第二个被砍），user 仍最后砍 | 超预算 → tool_result 先降级，再 tool_def，user 保持 full | tool_def 排在 user 之后导致 user prompt 先被砍 | `audit/segment.go`：`downgradeOrder` | 单测：构造超预算多 kind 请求 → 降级次序断言 |
| BR-107 | **输出侧 tool_call 独立成段**：OpenAI 流式与非流式的响应 tool_calls 必须生成 `kind=tool_call` 输出段，不得混入 `kind=assistant` 正文段；同一请求开/关 stream 的审计覆盖面必须一致 | 非流式响应含 tool_calls → 输出 segments 有独立 tool_call 段 | 非流式丢 tool_calls（F-108）；流式把 tool 参数拼进 assistant 全文（F-109） | `relay/channel/openai/relay-openai.go`、`relay/channel/openai/helper.go` | curl 同一 tools 请求各发 stream=true/false → 两次 `segments[]` 均含 tool_call 段 |
| BR-108 | **Claude / Gemini 输出侧接线**：两格式的流式与非流式 handler 均须调用 `sink.OnOutput`，行为与 OpenAI 一致（assistant 正文段 + tool_call 段） | Claude 非流式响应 → `log_contents` 有输出段 | 仅 OpenAI 有 OnOutput（F-111），Claude/Gemini 只有输入侧 | `relay/channel/claude/relay-claude.go`、`relay/channel/gemini/relay-gemini.go` | curl Claude/Gemini 各一次 → `segments[]` 含 assistant 输出段 |
| BR-109 | **审计规则配置归入系统设置**：规则 CRUD / 模板包 / 重扫入口全部位于 `系统设置 > 安全 > 内容审计`；`/audit/watchlist` 路由与侧边栏顶级项删除，全仓无残留引用 | 侧边栏无 `Audit Watchlist` 项，配置项均在系统设置内 | 配置页仍挂侧边栏顶级，与日志列表命名混淆（F-119、F-123） | `web/src/features/system-settings/`、`web/src/hooks/use-sidebar-data.ts`、`web/src/routes/` | `grep -r "audit/watchlist" web/src` → 0 命中 |
| BR-110 | **`/usage-logs/audit` 必须是可筛选的真实日志列表**：至少支持按 severity、最小命中数、时间范围、用户、模型筛选，行可展开进详情；且必须有 admin 守卫 | 普通用户访问 `/usage-logs/audit` → 跳 /403 | 渲染占位卡片（F-121）；无 admin guard（F-122） | `web/src/features/usage-logs/`、`web/src/routes/_authenticated/usage-logs/$section.tsx` | 浏览器：admin 见列表、普通用户 403 |
| BR-111 | **审计术语三分**：配置区=「内容审计」、日志区=「内容审计日志」、规则=「审计规则」；与既有「操作审计」（`middleware/audit.go`）文案显式区分。7 个 locale 文件同步 | zh.json 无「审计监控列表」，配置与日志标题不再同义 | 「审计内容监控」与「审计监控列表」并存（F-123） | `web/src/i18n/locales/*.json`（7 文件，F-124） | `grep -c` 旧键在 7 文件均为 0；`bun run build` 通过 |
| BR-112 | **`AuditContentTTLDays` 必须真实清理 `log_contents`**：超 TTL 记录由系统任务批量删除；TTL=0 表示不清理 | TTL=30 → 31 天前 `log_contents` 行被删除 | 设置项存在但只作重扫截止时间，表无限增长（F-125） | `model/log_content.go`、`service/`（系统任务）、`model/main.go` | 插入超龄行 → 任务执行后该行不存在；TTL=0 时行保留 |
| BR-113 | **内置模板包为只读代码资产**：内置模板定义在 Go 侧，含 `id`/`name`/`description`/`rules[]`；用户不可编辑内置定义本体，只能应用/移除，或应用后编辑生成的规则 | 应用模板后修改某条规则 → 该规则 `source` 仍为 template，内置定义不变 | 模板定义写库后被用户改坏，升级无法收敛 | `service/`（模板数据）、`controller/audit_content.go` | `GET /api/audit/templates` 返回内置列表；无写接口 |
| BR-114 | **模板应用幂等**：同一模板重复应用不产生重复规则，判重键为 (`template_id`, `kind`, `pattern`)；应用与移除均 bump watchlist version | 连续应用同模板两次 → 规则数不变 | 重复应用导致规则翻倍、命中计数虚高 | `model/audit_watchlist_rule.go` | 连续两次 POST apply → `SELECT count(*)` 不变；version 递增 |
| BR-115 | **模板包可整包快速切换**：提供按 `template_id` 批量启用/停用/移除；`source=manual` 的手工规则不受批量操作影响 | 停用模板 A → A 的规则 enabled=false，手工规则不变 | 批量操作误伤手工规则 | `model/audit_watchlist_rule.go`、`controller/audit_content.go` | 混合规则场景下批量停用 → 手工规则 enabled 不变 |
| BR-116 | **模板内 regex 规则默认 `enabled=false`**（ASM-105）；应用模板时若启用后会超出 `MaxEnabledRegexRules`（F-115），非 regex 规则照常应用，接口返回已应用条数 + regex 未启用的明确原因，不整体失败 | 模板含 2 条 regex → 应用成功，2 条 regex enabled=false，提示需手动启用 | 因 regex 超限导致整个模板应用失败且无说明 | `model/audit_watchlist_rule.go`、`controller/audit_content.go` | 构造已有 8 条 enabled regex → 应用含 regex 模板，返回 success + 说明 |
| BR-117 | **模板导入导出为 JSON**：导出产出 `{version, template_id, name, description, rules[]}`；导入校验 kind/severity/pattern 合法性与条数上限，非法条目逐条报错且不部分写入 | 导入 5 条合法规则 → 全部创建；含 1 条非法 kind → 整批拒绝并指出行号 | 部分写入导致规则集处于半应用状态 | `controller/audit_content.go`、`model/audit_watchlist_rule.go` | 导入非法 JSON → 400 且 `SELECT count(*)` 不变 |
| BR-118 | **规则 note 字段可编辑可见**：规则编辑表单含 note 输入，列表展示 note | 填写 note 后保存 → 列表可见 | note 被静默丢弃（F-126） | `web/src/features/system-settings/`（迁移后的规则 UI） | 浏览器：填 note 保存 → 刷新后仍在 |
| BR-119 | **审计仍为 record-only**：本需求不引入任何拦截/改写/中断行为，命中只写 `flags`/`hit_count`/`hit_severity` | 命中 high severity 规则 → 请求正常返回 200 | 命中后返回错误码或截断响应 | 全链路 | 构造必命中请求 → HTTP 200 且响应体完整 |
| BR-120 | **留存策略默认值保持保守**：本需求不放宽任何 kind 的默认 mode/limit（F-101 现值不变），新增 `tool_def` 取 `preview/1KB`；不新增留存策略配置 UI | 升级后 tool_result 仍 `drop`、user 仍 16KB full | 顺手把 tool_result 改成 full 留存，磁盘与隐私暴露面变化 | `audit/segment.go` | 单测断言 `defaultKindPolicy` 各 kind 值与 F-101 一致 + tool_def=preview/1024 |

### 2.2 UF 用户验收场景（索引）

| 场景 ID | Given | When | Then | 角色 | 验证方式 | Evidence |
|---|---|---|---|---|---|---|
| UF-101 | 管理员已登录，进入 `系统设置 > 安全 > 内容审计` | 新增 / 编辑 / 删除 / 启停一条审计规则（含 note） | 规则列表即时更新，watchlist version 递增，后续请求按新规则命中 | admin | browser + curl | EVD-101 |
| UF-102 | 管理员在内容审计配置区，模板包列表已加载 | 应用某个内置模板包，随后整包停用 / 移除 | 模板规则批量出现在规则列表（regex 默认停用并有说明），停用/移除只影响该包，手工规则不变 | admin | browser + curl | EVD-105 |
| UF-103 | 管理员在内容审计配置区 | 导出当前规则为 JSON，再导入一份 JSON（含一次非法内容） | 导出文件可下载；合法导入全部生效，非法导入整批拒绝并指出问题条目 | admin | browser + curl | EVD-107 |
| UF-104 | 管理员已登录，存在多条含命中的审计记录 | 打开 `使用日志 > 内容审计日志`，按 severity / 最小命中数 / 时间 / 用户 / 模型筛选，点开一行 | 列表按条件过滤，行展开显示 segments 与命中规则详情 | admin | browser | EVD-103 |
| UF-105 | 管理员在内容审计配置区，规则刚变更且存在历史记录 | 点击「重扫」并确认 | 进度可见，完成后命中数刷新，server log 有完成条目 | admin | browser + server log | EVD-104 |
| UF-106 | 普通用户已登录 | 直接访问 `/usage-logs/audit`；并调用 `/api/audit/logs`、`/api/audit/templates` | 页面跳 403；两个 API 均返回 403，响应体不含任何审计内容 | user | browser + curl | EVD-108 |
| UF-107 | relay 收到含 tool_calls / tool_result / tools 定义的请求（OpenAI / Claude / Gemini 三格式） | 请求正常完成 | `log_contents.segments` 含 `tool_call`（带 Text+Derived）、`tool_result`、`tool_def` 段；含 URL/关键词的 tool 内容产生 `flags` 命中 | 内部（2.3 豁免：后端采集链路，无用户交互界面） | curl + sqlite3 | EVD-102 |
| UF-108 | 同一带 tools 的请求分别以 stream=true / false 发送（OpenAI / Claude / Gemini） | 两次均成功 | 两次 `log_contents` 均含输出侧 assistant 段与 tool_call 段，覆盖面一致 | 内部（2.3 豁免：后端采集链路，无用户交互界面） | curl + sqlite3 | EVD-102 |
| UF-109 | `AuditContentTTLDays=30`，`log_contents` 存在 31 天前记录 | 清理任务执行 | 超龄记录被删除；TTL=0 时不删除任何记录 | 内部（2.3 豁免：后台系统任务，无用户交互界面） | sqlite3 + server log | EVD-109 |

> UF-107、UF-108、UF-109 为后端内部链路，无用户可见交互，2.3 节豁免流程脚本。2.3 节覆盖 UF-101 ~ UF-106。

### 2.3 核心业务流程（步骤级交互脚本）

#### UF-101: 管理员在系统设置内管理审计规则

**前置状态**：管理员已登录；`系统设置 > 安全` 页面已加载；「内容审计」section 可见；规则列表可能为空。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 侧边栏点「系统设置」→ 左侧 section 点「内容审计」 | section 高亮，URL 变为 `/system-settings/security/audit` | 拉取 `GET /api/audit/watchlist` + 现有 option 值 | 页面自上而下：采集开关区（总开关/单请求上限/保留天数）、审计规则表、模板包区、重扫入口 |
| 2 | 规则表点「新增规则」 | 弹出规则编辑对话框，kind 默认 keyword、severity 默认 medium、enabled 默认开 | — | 表单含 kind / pattern / severity / enabled / note 五项（BR-118） |
| 3 | 填 pattern 与 note，点「保存」 | 保存按钮进入 loading 并禁用，防重复提交 | `POST /api/audit/watchlist`，成功后 bump version | toast「保存成功」，对话框关闭，列表出现新行含 note 列 |
| 4 | 点某行「编辑」，改 severity 为 high，保存 | 同步 loading | `PUT /api/audit/watchlist/:id`，version++ | 该行 severity 徽章变为 high 配色 |
| 5 | 点某行 enabled 开关 | 开关立即切换为目标态并短暂禁用 | `PUT /api/audit/watchlist/:id` | 开关停留在新状态；失败则回滚并提示 |
| 6 | 点某行「删除」→ 确认 | 二次确认对话框；确认后行进入禁用态 | `DELETE /api/audit/watchlist/:id`，version++ | toast「删除成功」，该行消失 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| pattern 为空 | 提交时 pattern 空白 | 表单内联报错「Pattern required」，不发请求 | — | 补填后重新提交 |
| regex 超上限 | 新增第 9 条 enabled=true 的 regex 规则 | toast 显示后端返回的 regex 上限原因，对话框保持打开、表单数据保留 | 后端返回 `ErrRegexLimit`（F-115），不写库 | 改 enabled=false 保存，或先停用其他 regex |
| 非法 regex | pattern 无法编译 | toast 报错，规则不创建 | 后端校验 `regexp.Compile` 失败 | 修正 pattern 后重试 |
| 保存请求失败 | 网络/500 | toast「保存失败」，对话框保留输入 | 记录 server log | 直接重试 |

**界面状态机**：

```text
list-idle → dialog-open → submitting → success（列表刷新，dialog 关闭）
                              |
                              v
                            error（dialog 保留输入，可重试）
```

**入口接线清单**：

- 侧边栏「系统设置」→ `/system-settings/security` → 左侧 section 列表「内容审计」→ 规则表「新增规则」按钮 onClick
- 每行「编辑」/「删除」按钮 onClick、enabled Switch onCheckedChange

#### UF-102: 管理员应用与切换内置模板包

**前置状态**：管理员在「内容审计」section；模板包区已加载内置模板列表；规则表已有若干 `source=manual` 手工规则。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 滚动到「模板包」区 | 模板卡片列表渲染：名称、描述、规则条数、当前状态（未应用 / 已应用 N 条 / 已停用） | `GET /api/audit/templates` | 每张卡片含「应用」按钮；已应用的卡片显示「停用 / 启用 / 移除」 |
| 2 | 点某模板「应用」 | 按钮 loading 并禁用 | `POST /api/audit/templates/:id/apply`，幂等去重（BR-114），regex 条目 enabled=false（BR-116），version++ | toast「已应用 N 条规则」；含 regex 时追加说明「M 条正则规则默认未启用」；规则表出现带模板来源标记的行 |
| 3 | 再次点同一模板「应用」 | 同步 loading | 幂等：无新增 | toast 提示已应用条数不变，规则表行数不变 |
| 4 | 点该模板「停用」 | 卡片状态切换为已停用 | `POST /api/audit/templates/:id/disable`（批量 enabled=false，仅限该 template_id，BR-115），version++ | 该包规则 enabled 全部关闭；手工规则开关不变 |
| 5 | 点该模板「启用」 | 卡片状态切换为已应用 | 批量 enabled=true（regex 仍受上限约束） | 该包非 regex 规则重新启用 |
| 6 | 点该模板「移除」→ 确认 | 二次确认；确认后卡片回到未应用态 | 批量删除该 template_id 规则，version++ | 规则表该包行全部消失；手工规则保留 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| regex 名额不足 | 已有 8 条 enabled regex 时启用含 regex 的模板 | toast：非 regex 已启用 + regex 因上限未启用的说明；卡片仍为已应用 | 非 regex 照常处理，不整体失败（BR-116） | 停用其他 regex 后再启用 |
| 模板 id 不存在 | 手改 URL 或前后端版本不一致 | toast「模板不存在」 | 返回 404，不写库 | 刷新页面重取模板列表 |
| 应用中断网 | 请求失败 | toast「应用失败」，卡片回到操作前状态 | 事务未提交，无部分写入 | 重试；因幂等键存在不会产生重复 |

**界面状态机**：

```text
template-unapplied → applying → applied
        ^                           |  \
        |                    disable|   \remove
        |                           v    v
        └──────────────────────  disabled  → (removed → unapplied)
                                    |
                                  error（卡片回滚上一状态，可重试）
```

**入口接线清单**：

- `/system-settings/security/audit` → 「模板包」区 → 每张卡片「应用 / 启用 / 停用 / 移除」按钮 onClick
- 移除按钮 → 二次确认对话框 → 确认回调

#### UF-103: 管理员导入导出规则

**前置状态**：管理员在「内容审计」section；规则表至少一条规则。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点「导出规则」 | 按钮 loading | `GET /api/audit/watchlist/export` 返回 JSON（BR-117 结构） | 浏览器下载 JSON 文件，文件名含时间戳 |
| 2 | 点「导入规则」 | 打开文件选择对话框 | — | 可选择 `.json` 文件 |
| 3 | 选择合法 JSON，确认导入 | 导入按钮 loading，禁止重复提交 | `POST /api/audit/watchlist/import`，逐条校验后一次性写入，version++ | toast「已导入 N 条」，规则表刷新出现新规则 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| JSON 结构非法 | 缺字段 / 不是数组 / 非 JSON | toast 显示后端返回的具体原因，不写入任何规则 | 400，`SELECT count(*)` 不变（BR-117） | 修正文件后重新导入 |
| 含非法条目 | 某条 kind 或 severity 不在枚举内 | toast 指出出错条目位置，整批拒绝 | 400，无部分写入 | 修正该条后重新导入 |
| 超条数上限 | 导入条数超过接口上限 | toast 提示上限 | 400 | 拆分文件分批导入 |
| 空文件 | 选择 0 条规则的 JSON | toast「没有可导入的规则」 | 不发写请求或返回 0 条 | 换文件 |

**界面状态机**：

```text
idle → file-selected → importing → success（列表刷新）
                            |
                            v
                          error（列表不变，可重选文件）
```

**入口接线清单**：

- `/system-settings/security/audit` → 规则表工具栏「导出规则」按钮 onClick
- 同工具栏「导入规则」按钮 onClick → 隐藏 file input → onChange 回调

#### UF-104: 管理员查看与筛选内容审计日志

**前置状态**：管理员已登录；审计总开关曾开启且已产生若干 `log_contents` 记录，其中部分有命中。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 侧边栏「使用日志」→ tab 切到「内容审计日志」 | tab 高亮，URL 变 `/usage-logs/audit`，表格进入 loading 骨架 | `GET /api/audit/logs`（分页） | 表格列：时间、用户、模型、fidelity、命中数、最高 severity、request_id |
| 2 | 选择 severity = high | 筛选控件回显选中，表格 loading | 带 `severity=high` 重新请求 | 仅剩 high 命中行；分页总数更新 |
| 3 | 输入最小命中数 = 2，选时间范围，填用户 / 模型 | 各控件回显，表格 loading | 组合条件请求 | 表格按全部条件过滤 |
| 4 | 点某行 | 行展开 / 弹出详情 | `GET /api/log/content?request_id=` | 展示 segments（kind / mode / truncated / bytes / 正文预览 / derived）与命中规则列表 |
| 5 | 点「清空筛选」 | 控件复位 | 无参数重新请求 | 回到全量列表 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 空结果 | 筛选条件无匹配 | 表格显示空态文案与「清空筛选」按钮，非静默空白 | 返回 total=0 | 点清空筛选 |
| 审计从未开启 | `AuditEnabled=false` 且无历史记录 | 空态文案指向系统设置内容审计开关，并提供跳转 | 返回 total=0 | 点跳转去开启开关 |
| 详情不存在 | 记录已过 TTL 被清理（BR-112） | 详情区显示「无审计内容」，不报红 | `/api/log/content` 返回空 | 关闭详情继续浏览 |
| 列表请求失败 | 500 / 网络 | 表格错误态 + 「重试」按钮 | server log 记录 | 点重试 |

**界面状态机**：

```text
loading → list（有数据）
   |          |
   |          └── row-expanded → detail-loaded / detail-empty
   ├── empty（含筛选空态 / 未开启审计空态）
   └── error（可重试）
```

**入口接线清单**：

- 侧边栏「使用日志」→ `/usage-logs/common` → tab 列表「内容审计日志」→ `/usage-logs/audit`
- 路由 `beforeLoad` 中的 admin 守卫（BR-110）
- 筛选控件 onChange、行 onClick、空态跳转按钮 onClick、错误态重试按钮 onClick

#### UF-105: 管理员触发历史记录重扫

**前置状态**：管理员在「内容审计」section；规则刚变更；`log_contents` 存在 TTL 内的历史记录。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 点「重扫」 | 弹出确认对话框，说明只处理 TTL 内记录 | — | 确认 / 取消两个按钮 |
| 2 | 点确认 | 按钮 loading 并禁用，出现进度条 | `POST /api/audit/rescan` 启动后台任务 | 进度条显示 processed / total |
| 3 | 等待 | 进度条按轮询结果推进 | 轮询 `GET /api/audit/rescan/status` | 进度递增 |
| 4 | 完成 | 进度条消失，按钮恢复可点 | 状态变 done | toast「重扫完成」；审计日志列表命中数刷新后可见变化 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 无可重扫记录 | 无 TTL 内低版本记录 | toast「没有需要重扫的记录」，无进度条 | 返回 `no_op` | 无需操作 |
| 已有重扫在跑 | 重复点击或他人已触发 | toast「重扫进行中」，直接展示当前进度 | 返回进行中状态，不启新任务 | 等待完成 |
| 重扫出错 | 任务内 DB 错误 | 进度条转错误态 + toast | 状态 error + server log warn | 修复后重新触发 |

**界面状态机**：

```text
idle → confirm → running（轮询进度）→ done
                      |
                      v
                    error（可重新触发）
```

**入口接线清单**：

- `/system-settings/security/audit` → 「重扫」按钮 onClick → 确认对话框 → 轮询 hook 启动

#### UF-106: 普通用户被隔离在审计能力之外

**前置状态**：普通用户（role < admin）已登录。

**成功主路径**（对普通用户而言「成功」= 被正确拒绝）：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | 直接访问 `/usage-logs/audit` | 立即跳转，不闪现表格 | 路由 `beforeLoad` 角色判定失败（BR-110） | 403 页面 |
| 2 | 侧边栏查看 | — | — | 无任何审计配置入口；系统设置整体不可见 |
| 3 | 直接 curl `GET /api/audit/logs` | — | `middleware.AdminAuth()` 拦截 | 403，响应体不含审计数据 |
| 4 | 直接 curl `GET /api/audit/templates` | — | 同上 | 403 |

**失败分支**（即隔离被破坏的情形，必须验证不发生）：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 表格闪现 | 守卫在渲染后才生效 | 不允许：必须在 `beforeLoad` 阶段拦截 | — | 修实现 |
| 越权拿到数据 | 新增审计接口漏挂 AdminAuth | 不允许：所有新增审计路由必须在 admin 组内 | — | 修路由注册 |

**界面状态机**：

```text
navigate → guard-check → redirect(/403)
```

**入口接线清单**：

- `/usage-logs/$section` 路由 `beforeLoad` 中对 `section === 'audit'` 的 admin 判定
- 新增审计路由全部注册在 `auditRoute`（已 `middleware.AdminAuth()`，F-116）

### 2.4 INV 不变量

| 不变量 ID | 内容 | 关联 BR/UF | 验证方式 |
|---|---|---|---|
| INV-101 | v1 全部业务规则（`docs/content-audit/spec.md` BR-001~BR-017）继续成立，尤其：BR-001 request_id 1:1、BR-002 logs.other 只存指针、BR-003 普通用户不见 admin_info、BR-004 audit 包分层、BR-005 关闭开关零开销、BR-006 满队列 drop 不阻塞 relay | 全部 | v1 相关单测继续通过 + `make test` |
| INV-102 | relay 主链路不因本需求引入同步阻塞：新增采集逻辑仍在 `common.RelayCtxGo` 异步路径或纯内存构造中完成 | BR-103~BR-108 | code review + 并发请求 P99 无显著抬升 |
| INV-103 | `log_contents.segments` 序列化结构与 v1 完全一致（仅可能新增 `kind` 取值），老记录无需迁移 | BR-102、BR-105 | v1 老记录经新代码读取正常 + 序列化单测 |
| INV-104 | 审计保持 record-only，无任何请求被拦截、改写或中断 | BR-119 | 必命中请求返回 200 且响应完整 |
| INV-105 | 各 kind 默认留存策略不放宽（F-101 值不变） | BR-120 | `defaultKindPolicy` 断言单测 |
| INV-106 | `relaykit/` 模块保持独立可构建，不引入 audit 依赖 | BR-103、BR-107、BR-108 | `cd relaykit && GOWORK=off go build ./...` |
| INV-107 | 全部审计接口保持 admin-only；普通用户日志路径（`/api/log/self`）响应不含审计字段 | BR-110、UF-106 | 普通用户 curl 验证 |
| INV-108 | 三数据库兼容：rule 表加列与 `log_contents` 清理逻辑在 SQLite / MySQL / PostgreSQL 均可用，不使用单库特有语法 | BR-112、ASM-104 | AutoMigrate + GORM 方法审查 |
| INV-109 | `ProcessStreamResponse` 对外签名不变，`relay/channel/xai/text.go:63` 调用点无需修改（F-110、ASM-103） | BR-107 | `GOWORK=off go build ./...` + xai 调用点 diff 为空 |

### 2.5 EVD 证据清单

| 证据 ID | 类型 | 期望证据 | 保存位置 |
|---|---|---|---|
| EVD-101 | screenshot + api | 系统设置内规则 CRUD 全流程截图（含 note 字段）+ 四个 CRUD 接口 request/response | `evidence/UF-101/` |
| EVD-102 | test + log | 三格式 tool_call/tool_result/tool_def 采集单测输出 + curl 后 `sqlite3` 查询到的 segments/flags 摘录（含 stream 与非 stream 各一份） | `evidence/UF-107/`、`evidence/UF-108/` |
| EVD-103 | screenshot | 审计日志列表筛选前后截图、空态截图、行展开详情截图、console 无 error | `evidence/UF-104/` |
| EVD-104 | screenshot + log | 重扫确认框、进度条、完成 toast 截图 + server log 重扫完成条目 | `evidence/UF-105/` |
| EVD-105 | screenshot + api | 模板包应用/停用/启用/移除四态截图 + 幂等验证（两次 apply 后 count 不变）+ regex 超限提示截图 | `evidence/UF-102/` |
| EVD-106 | log | rule 表新增列在三库 AutoMigrate 的建列确认输出（无法起容器时记录 SQLite 实测 + 其余库的 GORM 语句审查结论） | `evidence/phase-3/` |
| EVD-107 | screenshot + api | 导出 JSON 文件内容摘录 + 合法导入成功截图 + 非法导入被整批拒绝截图与 count 不变证据 | `evidence/UF-103/` |
| EVD-108 | screenshot + api | 普通用户访问 `/usage-logs/audit` 的 403 截图 + `/api/audit/logs`、`/api/audit/templates` 的 403 响应 | `evidence/UF-106/` |
| EVD-109 | log | TTL 清理任务执行前后 `log_contents` 行数对比 + TTL=0 时不删除的对比 | `evidence/UF-109/` |
| EVD-110 | test | `make test` 全绿输出 + `cd relaykit && GOWORK=off go build ./...` BUILD_OK | `evidence/phase-6/` |
| EVD-111 | log | `grep -r "audit/watchlist" web/src` 归零输出；旧 i18n 键在 7 个 locale 文件的归零输出 | `evidence/phase-4/` |

### 2.6 角色与权限矩阵

| 角色 | 可见 | 可操作 | 禁止 | 失败提示 | 验证场景 |
|---|---|---|---|---|---|
| admin（role ≥ 10） | 系统设置内容审计区、内容审计日志 tab、审计内容详情 | 规则 CRUD、模板应用/切换/移除、导入导出、重扫、修改采集开关 | — | — | UF-101 ~ UF-105 |
| 普通用户（role < 10） | 自己的使用日志（不含 admin_info） | 无审计相关操作 | 全部 `/api/audit/*`、`/api/log/content`、`/usage-logs/audit` | 页面跳 /403；API 返回 403 | UF-106 |

### 2.7 负向 / 破坏性场景

| 场景 | Given | When | Then | Evidence |
|---|---|---|---|---|
| 权限不足 | 普通用户登录 | 访问审计页面与接口 | 全部 403，无数据泄露 | EVD-108 |
| 空数据 | 从未开启审计 | 打开内容审计日志 tab | 空态文案 + 指向系统设置开关的跳转，非空白页 | EVD-103 |
| 重复提交 | 网络慢时连点「应用模板」 | 多次请求到达 | 幂等，规则不重复（BR-114） | EVD-105 |
| 非法输入 | 导入含非法 kind 的 JSON | 提交导入 | 整批拒绝，无部分写入（BR-117） | EVD-107 |
| 上限边界 | 已有 8 条 enabled regex | 应用含 regex 的模板 | 非 regex 正常应用，regex 未启用且有明确说明（BR-116） | EVD-105 |
| 旧数据兼容 | 存在 v1 写入的 `log_contents` 记录 | 新代码读取并在列表/详情展示 | 正常解析展示，未知 kind 兜底显示，无报错（INV-103） | EVD-103 |
| 超大内容 | 单请求内容远超 `AuditPerRequestMaxBytes` | 请求正常完成 | 按降级顺序降级（BR-106），user 段最后被砍，扫描仍基于全文（BR-101） | EVD-102 |
| 依赖失败 | 审计 sink 落库报错 | relay 请求进行中 | relay 正常返回，仅 server log warn（INV-101 之 BR-015） | EVD-110 |

### 2.8 非目标

- **不做拦截/阻断**：审计保持 record-only，命中不影响请求（BR-119）。用户已明确「暂不拦截，后期考虑」。
- **不合并敏感词系统**：`setting/sensitive.go` + `service/sensitive.go` 的前置拦截保持原样，本次不与 watchlist 规则引擎合流。
- **不开放留存策略配置 UI**：默认保守值不变（BR-120），按需配置留待后续需求。
- **不修 v1 已知遗留**：`log_contents` 在 ClickHouse LOG_DB 下不建表且无 UI 提示、审计落库受 `LogConsumeEnabled` 连带控制、满队列 drop 计数无可视化、多节点并发重扫——本次不处理（TTL 清理除外，见 BR-112）。
- **不扩展采集范围到非文本 relay 路径**：image / task（视频、Suno、Midjourney）/ embedding / rerank / responses 路径的采集接线不在本次范围，仅 OpenAI / Claude / Gemini 文本 relay。
- **不改 `logs` 表结构**，不改 `GetAllLogs` 签名（ASM-106）。

---

## 3. 技术方案

### 3.0 事实校正与补充勘察（v0.2.0 新增）

> Stage 2 展开时对 1.3 节部分事实做了复核，以下为**修正与补充**。凡与 1.3 冲突，以本节为准。

| 事实 ID | 事实 | 来源命令 | 输出摘要 |
|---|---|---|---|
| F-131 | **F-112 路径有误更正**：`ClaudeResponseInfo` 别名实际在 `relay/channel/claude/relay-claude.go:43`（`= relayconvert.ClaudeResponseInfo`），`relay/channel/droid/` 目录**不存在**；上游别名链 `relaykit/relayconvert/response_compat.go:12` → `claudemessages.ClaudeResponseInfo` | `grep -rn "ClaudeResponseInfo" relay/channel/claude/relay-claude.go relaykit/relayconvert/response_compat.go`；`test -d relay/channel/droid` | claude/relay-claude.go:43；droid → NO |
| F-132 | Claude handler 挂点行号：`HandleStreamFinalResponse` L153、`ClaudeStreamHandler` L194、`ClaudeHandler` L268 | `grep -n "^func " relay/channel/claude/relay-claude.go` | 三处确认 |
| F-133 | **F-121 表述需细化**：`web/src/features/usage-logs/section-registry.tsx` 中 **全部四个** section（common/drawing/task/audit）均为 `build: () => null`，注释 `Content is rendered directly in the page component`——audit 并非特例；真正的占位是 `index.tsx:62 AuditSectionPlaceholder`，其跳转 `<Link to='/audit/watchlist'>` 在 L70，分支判断在 L179-180 | `sed -n '20,50p' web/src/features/usage-logs/section-registry.tsx`；`grep -n "audit" web/src/features/usage-logs/index.tsx` | audit entry titleKey=`Audit Logs`；index.tsx L57/62/70/143/179-180 |
| F-134 | **F-120 补充**：安全设置 audit section **已有真实 build**，渲染 `AuditSettingsSection`（import 在 section-registry.tsx:23，build 在 L46-48），组件文件 `web/src/features/system-settings/request-limits/audit-settings-section.tsx` 共 150 行——即「采集开关区」已存在，本需求是**在其下扩充规则/模板/重扫**，不是从零新建 | `grep -n "id: '\|AuditSettingsSection" web/src/features/system-settings/security/section-registry.tsx`；`wc -l .../audit-settings-section.tsx` | L23 import、L46 id、L48 build；150 行 |
| F-135 | 现有前端审计资产规模：`web/src/features/audit/api.ts` 58 行、`index.tsx` 429 行、`types.ts` 38 行（共 525 行），为 P4 迁移的搬运基数 | `wc -l web/src/features/audit/*.ts*` | 58 / 429 / 38 |
| F-136 | OnInput 接线现状：`controller/relay.go:131-133` 注入 `GetAuditSink()`；三格式 segment 构建调用点 L291（`BuildOpenAISegments(r.Messages, cfg)`）、L294（`BuildClaudeSegments(r, cfg)`）、L297（`BuildGeminiSegments(r, cfg)`）——**OpenAI 只传 Messages，故 `req.Tools` 在此处即被丢弃**（对应 F-106） | `grep -n "GetAuditSink\|Build.*Segments" controller/relay.go` | L131-133、L291/294/297 |
| F-137 | Gemini tools 已有解析器 `(*GeminiChatRequest) GetTools() []GeminiChatTool`（`relaykit/dto/gemini.go:128`），兼容 `[` 数组与 `{` 对象两种上游写法（L130/L136），tool_def 采集无需自行解析 RawMessage | `grep -n "Tools\|func (r \*GeminiChatRequest) GetTools" relaykit/dto/gemini.go` | L17 字段、L128 方法 |
| F-138 | TTL 清理可复用的系统任务范式：`model/system_task.go:19 SystemTaskTypeLogCleanup`；`service/system_task.go` 中 `logCleanupHandler.Type()` L80、`LogCleanupPayload` L90、`LogCleanupState` L95、`LogCleanupResult` L102、`StartLogCleanupTask` L168、`runLogCleanupTask` L338、批删调用 `model.DeleteOldLogBatch` L379 | `grep -n "LogCleanup" model/system_task.go service/system_task.go` | 全部命中行确认 |
| F-139 | 审计 option 三件套接线位置：seed `model/option.go:51-53`；sync switch `case "AuditEnabled"` L339（bool）、`case "AuditPerRequestMaxBytes"` L407、`case "AuditContentTTLDays"` L412（int，带 `>0` 守卫）；运行时变量 `common/constants.go:95-98` | `grep -n "Audit" model/option.go common/constants.go` | 行号确认 |

### 3.1 架构 Before / After

> v1 链路已建成（见 v1 spec §3.1），本节只画**增量差异**。粗体 = 本需求新增/修改。

```text
Before（v1 现状，三处缺陷）:
  relay 请求
    controller/relay.go:131 注入 ContentSink
      → OnInput
          BuildOpenAISegments(r.Messages, cfg)   ← ① req.Tools 丢弃（F-106/F-136）
            makeToolCallSegment      → 有 Text，无 deriveFacts → Domains 恒空（F-102）
            makeDropSegment          → 有 deriveFacts，Text 清空 → keyword/regex 无面（F-105）
          BuildClaudeSegments  → makeClaudeToolUseSegment    → 无 Text 无 derive（F-103）
          BuildGeminiSegments  → makeGeminiFunctionCallSegment → 无 Text 无 derive（F-103）
      → DoResponse
          openai/relay-openai.go:195-204  OnOutput（仅 assistant 全文）
            └ 流式：ProcessStreamResponse 把 tool name+args 拼进 responseTextBuilder（F-109）
            └ 非流式：只累加 choice.Message.StringContent()，tool_calls 丢失（F-108）
          claude/relay-claude.go   ← ② 无 OnOutput（F-111/F-131）
          gemini/relay-gemini.go   ← ② 无 OnOutput（F-111）
      → OnSettled → logs_content

    service/audit_watchlist.go:39 ScanSegments
      domain  ← seg.Derived.Domains
      keyword ← seg.Text != ""   (L86)   ← ③ 截断/丢弃后的残文，非全文（F-104）
      regex   ← seg.Text != ""   (L96)

After（本需求）:
  relay 请求
    controller/relay.go 注入 ContentSink（不变）
      → OnInput
          **BuildOpenAIInputSegments(req, cfg)**   ← 新入口，内部 = 消息段 + **tool_def 段**
            makeToolCallSegment      → Text（截断）+ **ScanText（全文）** + **deriveFacts(args)**
            makeDropSegment          → Text 仍空 + **ScanText（全文）** + deriveFacts
            **makeToolDefSegment**   → kind=tool_def, preview/1KB
          **BuildClaudeInputSegments(req, cfg)**   → makeClaudeToolUseSegment 同样三补齐
          **BuildGeminiInputSegments(req, cfg)**   → makeGeminiFunctionCallSegment 同样三补齐
      → DoResponse
          openai：**流式独立 tool 累加器**（不改 ProcessStreamResponse 签名，INV-109）
                  **非流式补采 tool_calls → 独立 tool_call 段**
          **claude：ClaudeStreamHandler / ClaudeHandler 接 OnOutput**
          **gemini：GeminiChatStreamHandler / GeminiChatHandler 接 OnOutput**
      → OnSettled → logs_content
          **flush 前清空所有 Segment.ScanText**（BR-101 落库面）

    ScanSegments
      domain  ← seg.Derived.Domains（不变）
      keyword ← **scanFace(seg) = ScanText 优先，回退 Text**
      regex   ← **scanFace(seg)**

  **新增旁路能力（与 relay 链路无关）**：
    model/audit_watchlist_rule.go  + Source / TemplateId 两列
    service/audit_template.go      内置模板包（Go 常量，只读）
    controller/audit_content.go    + 模板 apply/enable/disable/remove、import/export、审计日志列表
    service/system_task.go         + logContentCleanupHandler（复用 F-138 范式）
    前端                            /audit/watchlist 路由删除 → 配置并入系统设置；
                                   /usage-logs/audit 占位 → 真实列表
```

**留存面 / 扫描面双轨（BR-101 核心设计）**：

```text
                     ┌──────────────── Segment ────────────────┐
  原始文本 ──derive──→│ Derived  (URLs/Domains/Tools/ArgsKeys)  │──→ domain 档
      │              │                                         │
      ├──全文────────→│ ScanText  `json:"-"`  （不落库）        │──→ keyword / regex 档
      │              │                                         │
      └──按 policy───→│ Text      `json:"text,omitempty"`       │──→ 落库正文（保守留存）
                     └─────────────────────────────────────────┘
                                    ▲
                      sink.flush() 前置：ScanText = ""（INV-103 保证落库 JSON 结构不变）
```

**分层约束不变**（v1 BR-004 / INV-101）：`audit/` 仍只 import `common` + `relaykit/dto` + `relaykit/types`；`ScanText` 是纯字段，不引入新依赖。`service/audit_sink.go` 仍是唯一同时 import `audit` + `model` 的层。

### 3.2 模块改造

| 模块 | 职责 | 改造说明 | 关联 BR |
|---|---|---|---|
| `audit/types.go` | 快照与 Segment 类型 | `Segment` 增 `ScanText string \`json:"-"\``；`SegmentKind` 增 `KindToolDef` | BR-101, BR-102, BR-105 |
| `audit/segment.go` | 分段构建 | ①`defaultKindPolicy` 增 `tool_def=preview/1KB`；②`downgradeOrder` 插入 `tool_def`；③ 三个 tool_call builder 补 `Text`+`ScanText`+`deriveFacts`；④`makeDropSegment` 补 `ScanText`；⑤ 新增 `makeToolDefSegment` + 三格式 tool_def 提取；⑥ 新增 `BuildOpenAIInputSegments` / `BuildClaudeInputSegments` / `BuildGeminiInputSegments` 三入口；⑦ 新增 `BuildAssistantToolCallOutputSegments` | BR-101, BR-103~BR-106, BR-107, BR-120 |
| `audit/segment_test.go` / `segment_walkers_test.go` | 单测 | 补 ScanText 不落库、三格式 tool_call 命中、tool_def 降级序、`defaultKindPolicy` 值钉死 | BR-101~BR-106, BR-120 |
| `service/audit_watchlist.go` | 扫描 | `ScanSegments` keyword/regex 分支改用「ScanText 优先，回退 Text」 | BR-101, BR-104 |
| `service/audit_sink.go` | 落库 | `flush` 前清空全部 `Segment.ScanText` | BR-101, BR-102 |
| `controller/relay.go` | OnInput 接线 | L291/294/297 三处改调新的 `Build*InputSegments(req, cfg)` | BR-105, F-136 |
| `relay/channel/openai/relay-openai.go` | OpenAI 输出 | `OaiStreamHandler` 加独立 tool 累加器；`OpenaiHandler` 补采 `tool_calls` | BR-107, INV-109 |
| `relay/channel/claude/relay-claude.go` | Claude 输出 | `ClaudeStreamHandler`(L194) / `ClaudeHandler`(L268) 接 `OnOutput` | BR-108 |
| `relay/channel/gemini/relay-gemini.go` | Gemini 输出 | `GeminiChatStreamHandler`(L209) 回调内旁路累加 + `GeminiChatHandler`(L313) 接 `OnOutput` | BR-108 |
| `model/audit_watchlist_rule.go` | 规则表 | 加 `Source` / `TemplateId` 两列；新增按 template_id 的批量启停/删除、幂等 upsert、导入导出校验 | BR-114~BR-117, ASM-104 |
| `model/main.go` | 迁移 | `AuditWatchlistRule` AutoMigrate 自动 ADD COLUMN（三库兼容）；无需手写 DDL | INV-108 |
| `service/audit_template.go`（新） | 内置模板包 | Go 侧只读常量：`id`/`name`/`description`/`rules[]`；regex 条目 `Enabled=false` | BR-113, BR-116 |
| `model/log_content.go` | 审计内容 | 新增 `ListLogContents`（列表筛选）+ `CountLogContents` + `DeleteOldLogContentBatch` | BR-110, BR-112, ASM-106 |
| `service/system_task.go` | TTL 清理 | 新增 `logContentCleanupHandler`（照 F-138 范式）+ `StartLogContentCleanupTask` | BR-112 |
| `model/system_task.go` | 任务类型 | 新增 `SystemTaskTypeLogContentCleanup` | BR-112 |
| `controller/audit_content.go` | 审计 API | 新增 8 个 handler：模板 list/apply/enable/disable/remove、规则 export/import、审计日志列表 | BR-113~BR-117, BR-110 |
| `router/api-router.go` | 路由 | `auditRoute`(L283) 内新增 8 条；**不新增 route group**（沿用已有 AdminAuth） | BR-110, INV-107 |
| `web/src/features/audit/` | 旧规则页 | **整目录迁移**到 `web/src/features/system-settings/security/audit/`（525 行，F-135） | BR-109 |
| `web/src/routes/_authenticated/audit/` | 旧路由 | **整目录删除** | BR-109, ASM-107 |
| `web/src/hooks/use-sidebar-data.ts` | 侧边栏 | 删除 L161 `Audit Watchlist` 顶级项 | BR-109 |
| `web/src/features/system-settings/request-limits/audit-settings-section.tsx` | 采集开关区 | 在现有 150 行基础上扩充：规则表 + 模板包区 + 导入导出 + 重扫（F-134） | BR-109, BR-118, UF-101~UF-103, UF-105 |
| `web/src/features/usage-logs/index.tsx` | 审计日志 | 删除 `AuditSectionPlaceholder`(L62-74)，替换为真实列表组件；L143 pageMeta 分支保留 | BR-110 |
| `web/src/routes/_authenticated/usage-logs/$section.tsx` | 路由守卫 | `beforeLoad`(L53) 增 `section === 'audit'` 的 admin 判定 | BR-110, UF-106 |
| `web/src/i18n/locales/*.json`（7 文件） | 文案 | 按 BR-111 三分术语；删除旧键 | BR-111 |

### 3.3 三段式定位清单

> 行号是 hint，漂移时以 symbol + rg anchor 为准。「待创建」= 新文件/新符号。

| 文件 | 稳定定位 | 搜索定位（rg anchor） | 行号 hint | 备注 |
|---|---|---|---|---|
| `audit/types.go` | `type Segment struct` | `rg "type Segment struct" audit/types.go` | L73-83 | 加 `ScanText string \`json:"-"\`` |
| `audit/types.go` | `SegmentKind` const 组 | `rg "KindToolResult" audit/types.go` | L36-44 | 加 `KindToolDef = "tool_def"` |
| `audit/types.go` | `type DerivedFacts struct` | `rg "type DerivedFacts" audit/types.go` | L86-92 | 结构不变，仅填充率提升 |
| `audit/segment.go` | `defaultKindPolicy` | `rg "defaultKindPolicy" audit/segment.go` | L31-39 | 加 `KindToolDef: {ModePreview, DefaultToolDefPreviewBytes}` |
| `audit/segment.go` | 字节常量组 | `rg "DefaultToolCallDeriveBytes" audit/segment.go` | L17-24 | 加 `DefaultToolDefPreviewBytes = 1024` |
| `audit/segment.go` | `downgradeOrder` | `rg "downgradeOrder" audit/segment.go` | L43-49 | `tool_result, **tool_def**, tool_call, system, assistant, user`（BR-106） |
| `audit/segment.go` | `func BuildOpenAISegments` | `rg "func BuildOpenAISegments" audit/segment.go` | L54 | 保留（内部复用）；新增 `BuildOpenAIInputSegments` 包装 |
| `audit/segment.go` | `func makeDropSegment` | `rg "func makeDropSegment" audit/segment.go` | L144 | 补 `seg.ScanText = text`（BR-104） |
| `audit/segment.go` | `func makeToolCallSegment` | `rg "func makeToolCallSegment" audit/segment.go` | L161 | 补 deriveFacts 合并 + ScanText（BR-103） |
| `audit/segment.go` | `func BuildAssistantOutputSegment` | `rg "func BuildAssistantOutputSegment" audit/segment.go` | L219 | 复用于 Claude/Gemini OnOutput（BR-108） |
| `audit/segment.go` | `func BuildClaudeSegments` | `rg "func BuildClaudeSegments" audit/segment.go` | L254 | 新增 `BuildClaudeInputSegments` 包装 |
| `audit/segment.go` | `func makeClaudeToolUseSegment` | `rg "func makeClaudeToolUseSegment" audit/segment.go` | L309 | 补 Text+ScanText+deriveFacts（BR-103） |
| `audit/segment.go` | `func BuildGeminiSegments` | `rg "func BuildGeminiSegments" audit/segment.go` | L333 | 新增 `BuildGeminiInputSegments` 包装 |
| `audit/segment.go` | `func makeGeminiFunctionCallSegment` | `rg "func makeGeminiFunctionCallSegment" audit/segment.go` | L379 | 补 Text+ScanText+deriveFacts（BR-103） |
| `audit/segment.go` | `func applyBudget` | `rg "func applyBudget" audit/segment.go` | L410 | tool_def 自动纳入（读 downgradeOrder） |
| `audit/segment.go` | `func deriveFacts` | `rg "func deriveFacts" audit/segment.go` | L486 | 复用，不改 |
| `audit/segment.go` | `func makeToolDefSegment` | `rg "func makeToolDefSegment" audit/segment.go` | 待创建 | BR-105 |
| `service/audit_watchlist.go` | `func ScanSegments` | `rg "func ScanSegments" service/audit_watchlist.go` | L39 | keyword 分支 L86、regex 分支 L96 改扫描面 |
| `service/audit_sink.go` | `func (s *LogContentSink) flush` | `rg "func .*LogContentSink. flush" service/audit_sink.go` | L259 | 落库前清 ScanText（BR-101/BR-102） |
| `service/audit_sink.go` | `func GetAuditSink` | `rg "func GetAuditSink" service/audit_sink.go` | L81 | 不改 |
| `controller/relay.go` | `BuildOpenAISegments` 调用点 | `rg "Build.*Segments" controller/relay.go` | L291/294/297 | 改调 `Build*InputSegments(req, cfg)`（F-136） |
| `relay/channel/openai/relay-openai.go` | `func OaiStreamHandler` | `rg "func OaiStreamHandler" relay/channel/openai/relay-openai.go` | L105 | OnOutput 现址 L195-204 |
| `relay/channel/openai/relay-openai.go` | `func OpenaiHandler` | `rg "func OpenaiHandler" relay/channel/openai/relay-openai.go` | L234 | OnOutput 现址 L348-360，补 tool_calls |
| `relay/channel/openai/relay-openai.go` | `func collectStreamFunctionCallNames` | `rg "func collectStreamFunctionCallNames" relay/channel/openai/relay-openai.go` | L209 | **已有的按 (choice,toolIdx) 去重范式**，tool 累加器照此写 |
| `relay/channel/openai/helper.go` | `func ProcessStreamResponse` | `rg "func ProcessStreamResponse" relay/channel/openai/helper.go` | L93 | **签名不得改**（INV-109）；外部调用者 `relay/channel/xai/text.go:63` |
| `relay/channel/claude/relay-claude.go` | `type ClaudeResponseInfo` | `rg "type ClaudeResponseInfo" relay/channel/claude/relay-claude.go` | L43 | 别名，含 `ResponseText strings.Builder`（F-131） |
| `relay/channel/claude/relay-claude.go` | `func HandleStreamFinalResponse` | `rg "func HandleStreamFinalResponse" relay/channel/claude/relay-claude.go` | L153 | 流式 OnOutput 挂点参考 |
| `relay/channel/claude/relay-claude.go` | `func ClaudeStreamHandler` | `rg "func ClaudeStreamHandler" relay/channel/claude/relay-claude.go` | L194 | 流式 OnOutput（BR-108） |
| `relay/channel/claude/relay-claude.go` | `func ClaudeHandler` | `rg "func ClaudeHandler" relay/channel/claude/relay-claude.go` | L268 | 非流式 OnOutput（BR-108） |
| `relay/channel/gemini/relay-gemini.go` | `func geminiStreamHandler` | `rg "func geminiStreamHandler" relay/channel/gemini/relay-gemini.go` | L147 | 内部 `responseText` L151/L173，**不改签名**，在 L209 回调内旁路累加 |
| `relay/channel/gemini/relay-gemini.go` | `func GeminiChatStreamHandler` | `rg "func GeminiChatStreamHandler" relay/channel/gemini/relay-gemini.go` | L209 | 回调 L216；OnOutput 放在 L297 err 判定之后 |
| `relay/channel/gemini/relay-gemini.go` | `func GeminiChatHandler` | `rg "func GeminiChatHandler" relay/channel/gemini/relay-gemini.go` | L313 | OnOutput 放在 L387 `IOCopyBytesGracefully` 之后 |
| `relaykit/dto/openai_request.go` | `Tools []ToolCallRequest` | `rg "Tools \[\]ToolCallRequest" relaykit/dto/openai_request.go` | L55 | tool_def 来源（F-107） |
| `relaykit/dto/claude.go` | `type Tool struct` | `rg "^type Tool struct" relaykit/dto/claude.go` | L172 | tool_def 结构 |
| `relaykit/dto/claude.go` | `Tools any` + `GetTools()` | `rg "Tools .*any\|func .*GetTools" relaykit/dto/claude.go` | L221 / L402 | `any` 需先 GetTools（F-107） |
| `relaykit/dto/gemini.go` | `func (r *GeminiChatRequest) GetTools` | `rg "func .GeminiChatRequest. GetTools" relaykit/dto/gemini.go` | L128 | **已有解析器，直接用**（F-137） |
| `model/audit_watchlist_rule.go` | `type AuditWatchlistRule struct` | `rg "type AuditWatchlistRule struct" model/audit_watchlist_rule.go` | L25-34 | 加 `Source` / `TemplateId`（ASM-104） |
| `model/audit_watchlist_rule.go` | `MaxEnabledRegexRules` | `rg "MaxEnabledRegexRules" model/audit_watchlist_rule.go` | L18 | 值保持 8（ASM-105） |
| `model/audit_watchlist_rule.go` | `ErrRegexLimit` | `rg "ErrRegexLimit" model/audit_watchlist_rule.go` | L22 | 模板应用时不整体失败（BR-116） |
| `model/audit_watchlist_rule.go` | `func validateWatchlistRule` | `rg "func validateWatchlistRule" model/audit_watchlist_rule.go` | L122 | 导入校验复用（BR-117） |
| `model/audit_watchlist_rule.go` | `func countEnabledRegexRules` | `rg "func countEnabledRegexRules" model/audit_watchlist_rule.go` | L151 | 批量启用时的名额计算（BR-116） |
| `model/audit_watchlist_rule.go` | `func bumpWatchlistVersionTx` | `rg "func bumpWatchlistVersionTx" model/audit_watchlist_rule.go` | L162 | 模板/导入操作均须调用（BR-114） |
| `model/log_content.go` | `type LogContent struct` | `rg "type LogContent struct" model/log_content.go` | L10-25 | 结构不变；`HitSeverity` 已有 index（F-130） |
| `model/log_content.go` | `func ListLogContentsForRescan` | `rg "func ListLogContentsForRescan" model/log_content.go` | L58 | 新列表查询照此分页范式 |
| `model/log_content.go` | `func ListLogContents` | `rg "func ListLogContents\b" model/log_content.go` | 待创建 | ASM-106 筛选查询 |
| `model/log_content.go` | `func DeleteOldLogContentBatch` | `rg "func DeleteOldLogContentBatch" model/log_content.go` | 待创建 | BR-112 |
| `model/log.go` | `func DeleteOldLogBatch` | `rg "func DeleteOldLogBatch" model/log.go` | L703 | 批删范式参考（F-125） |
| `model/main.go` | `func migrateLOGDB` | `rg "func migrateLOGDB" model/main.go` | L414 | 已注册 LogContent（L422），本次不改 |
| `model/main.go` | `func seedWatchlistMeta` | `rg "func seedWatchlistMeta" model/main.go` | L405 | 不改 |
| `model/system_task.go` | `SystemTaskTypeLogCleanup` | `rg "SystemTaskTypeLogCleanup" model/system_task.go` | L19 | 加 `SystemTaskTypeLogContentCleanup` |
| `service/system_task.go` | `logCleanupHandler` 全套 | `rg "LogCleanup" service/system_task.go` | L80/90/95/102/168/338/379 | 照此范式复制（F-138） |
| `service/audit_template.go` | `BuiltinAuditTemplates` | `rg "BuiltinAuditTemplates" service/audit_template.go` | 待创建 | BR-113 只读 Go 常量 |
| `controller/audit_content.go` | 现有 handler 组 | `rg "^func " controller/audit_content.go` | L1-201 | 新增 8 handler 追加此文件 |
| `router/api-router.go` | `auditRoute` | `rg "auditRoute" router/api-router.go` | L283-292 | 组内追加 8 条（已 AdminAuth，INV-107） |
| `model/option.go` | Audit option seed / sync | `rg "Audit" model/option.go` | L51-53 / L339 / L407 / L412 | 本需求**不新增 option**（BR-120） |
| `common/constants.go` | `AuditContentTTLDays` | `rg "AuditContentTTLDays" common/constants.go` | L98 | BR-112 清理任务读此值 |
| `web/src/features/audit/index.tsx` | `RuleEditorDialog` | `rg "RuleEditorDialog" web/src/features/audit/index.tsx` | L311-429 | 迁移 + 补 note 字段（BR-118, F-126） |
| `web/src/features/audit/api.ts` | 审计 API 封装 | `rg "export" web/src/features/audit/api.ts` | 58 行全量 | 迁移 + 扩模板/导入导出/日志列表 |
| `web/src/features/system-settings/security/section-registry.tsx` | `id: 'audit'` | `rg "id: 'audit'" web/src/features/system-settings/security/section-registry.tsx` | L46-48 | build 已指向 `AuditSettingsSection`（F-134） |
| `web/src/features/system-settings/request-limits/audit-settings-section.tsx` | `AuditSettingsSection` | `rg "export function AuditSettingsSection\|export const AuditSettingsSection" .../audit-settings-section.tsx` | 150 行 | 在此扩充规则/模板/重扫（F-134） |
| `web/src/features/usage-logs/index.tsx` | `AuditSectionPlaceholder` | `rg "AuditSectionPlaceholder" web/src/features/usage-logs/index.tsx` | L62-74，用于 L179-180 | 删除；`<Link to='/audit/watchlist'>` 在 L70（F-133） |
| `web/src/features/usage-logs/section-registry.tsx` | `id: 'audit'` | `rg "id: 'audit'" web/src/features/usage-logs/section-registry.tsx` | L41 | `titleKey: 'Audit Logs'`；`build: () => null` 是全局约定，**不改**（F-133） |
| `web/src/routes/_authenticated/usage-logs/$section.tsx` | `beforeLoad` | `rg "beforeLoad" web/src/routes/_authenticated/usage-logs/\$section.tsx` | L53 | 加 admin 守卫（BR-110） |
| `web/src/routes/_authenticated/audit/watchlist/index.tsx` | `WatchlistPage` route | `rg "createFileRoute" web/src/routes/_authenticated/audit/watchlist/index.tsx` | 整文件 | **删除**（ASM-107） |
| `web/src/hooks/use-sidebar-data.ts` | `Audit Watchlist` | `rg "Audit Watchlist" web/src/hooks/use-sidebar-data.ts` | L161 | **删除该项**（BR-109） |
| `web/src/i18n/locales/*.json` | `Audit Content Monitoring` 等 | `rg "Audit Content Monitoring\|Audit Watchlist\|Manage watchlist" web/src/i18n/locales/` | zh.json L482/486/2595 | 7 文件同步改（BR-111, F-123/F-124） |

**定位清单统计**：待创建 6 条 / 总 62 条 ≈ **10%**，低于 30% 阈值 ✓

### 3.4 API / 数据 / 权限 / 路由影响

**新增 API 端点**（全部落在 `auditRoute`，已 `middleware.AdminAuth()`，F-116 / INV-107）：

| 方法 | 路径 | 说明 | 请求 | 响应 | 关联 |
|---|---|---|---|---|---|
| GET | `/api/audit/templates` | 列出内置模板包及其应用状态 | — | `[{id,name,description,rule_count,applied_count,enabled_count,status}]` | BR-113, UF-102 |
| POST | `/api/audit/templates/:id/apply` | 应用模板（幂等） | — | `{applied,skipped,regex_disabled,message}` | BR-114, BR-116 |
| POST | `/api/audit/templates/:id/enable` | 整包启用 | — | `{enabled,regex_skipped,message}` | BR-115, BR-116 |
| POST | `/api/audit/templates/:id/disable` | 整包停用 | — | `{disabled}` | BR-115 |
| DELETE | `/api/audit/templates/:id` | 整包移除 | — | `{removed}` | BR-115 |
| GET | `/api/audit/watchlist/export` | 导出规则 JSON | — | `{version,template_id,name,description,rules[]}` | BR-117, UF-103 |
| POST | `/api/audit/watchlist/import` | 导入规则 JSON | 同导出结构 | `{imported}` / 400 逐条错误 | BR-117, UF-103 |
| GET | `/api/audit/logs` | 审计日志列表（直查 `log_contents`） | `severity, min_hit, start_timestamp, end_timestamp, user_id, model_name, p, page_size` | `{items[],total}` | BR-110, ASM-106, UF-104 |

> **不新增 route group**：8 条全部追加进 `router/api-router.go:283` 已有的 `auditRoute`，权限自动继承 AdminAuth。
> **`DELETE /api/audit/templates/:id`** 选 DELETE 而非 `POST .../remove`，与既有 `DELETE /watchlist/:id`（L289）风格一致。

**数据表变更**：

| 表 | 变更 | DDL 方式 | 三库兼容性 |
|---|---|---|---|
| `audit_watchlist_rules` | 加列 `source varchar(16)`、`template_id varchar(64)`（含 index） | GORM `AutoMigrate` 自动 `ALTER TABLE ... ADD COLUMN` | SQLite/MySQL/PG 均支持 ADD COLUMN；**不使用 ALTER COLUMN**（INV-108） |
| `log_contents` | **无结构变更** | — | 列表查询走已有 `HitSeverity` / `CreatedAt` / `UserId` index（F-130） |
| `logs` | **无变更** | — | ASM-106：不改 `GetAllLogs` 12 参数签名 |

> `Source` 语义：`manual`（手工创建，含 v1 历史行——空值按 manual 解释）/ `template`（由模板应用生成）。
> **不给 `Source` 加 `gorm:"default:..."`**：AGENTS.md 明确禁止用 GORM default 表达业务规则默认值，改在 `CreateWatchlistRule` / 导入路径规范化（空 → `manual`）。
> 幂等键 `(template_id, kind, pattern)`（BR-114）由**代码查重**实现，不建唯一索引——三库对 NULL 参与唯一索引的语义不一致，且 `template_id` 对手工规则为空。

**Option 变更**：**无新增**。`AuditEnabled` / `AuditPerRequestMaxBytes` / `AuditContentTTLDays` 三项沿用（F-139）；`AuditContentTTLDays` 从「仅重扫截止时间」升级为「同时驱动清理任务」（BR-112）。留存策略不开放配置（BR-120）。

**系统任务**：新增 `SystemTaskTypeLogContentCleanup`，复用 `SystemTask` 表与调度框架（F-138），无表结构变更。

**权限与路由影响**：

| 类型 | 是否影响 | 说明 | 兼容策略 |
|---|---|---|---|
| 后端 API | 是 | +8 条，全在 AdminAuth 组内 | 普通用户 403（UF-106） |
| 前端路由 | 是 | **删除** `/audit/watchlist`；`/usage-logs/audit` 加 admin 守卫 | ASM-107：clean cutover，已收藏该 URL 者 404；跳转入口同步改指系统设置 |
| 侧边栏 | 是 | 删 Admin 组顶级 `Audit Watchlist` | 配置入口统一到系统设置（BR-109） |
| 数据 | 是 | 规则表 +2 列 | AutoMigrate 向前兼容；v1 历史规则 `source` 为空 → 按 manual 解释 |
| relay 性能 | 否 | ScanText 为内存字段，flush 前清空；tool_def 采集是纯内存构造 | INV-102 |
| 落库格式 | 否 | `ScanText` 标 `json:"-"`；仅新增 `kind` 取值 | INV-103：v1 老记录零迁移 |
| relaykit | 否 | 不引入 audit 依赖 | INV-106 |
| 拦截行为 | 否 | 仍为 record-only | INV-104 / BR-119 |


---

## 4. Phase 计划与任务详情

> 任务总数：32 条。Phase 依赖链见下图。

### Phase 依赖链

```
P0（基线校准）
  ↓
P1（后端采集修复：ScanText + tool 三补齐）
  ↓
P2（OpenAI 输出修复）+ P3（Claude/Gemini OnOutput）← 并行
  ↓
P4（规则扩展 + 模板 + TTL + 审计日志 API）← 可提前与 P1 并行
  ↓
P5（前端 IA 重构）
  ↓
P6（前端模板 + 日志 UI）
  ↓
P7（全量验收）
```

### Phase 0: 基线校准（P0）

> **你在哪里**：v1 已交付（40b459d5），v2 spec Stage 2 展开完毕。
> **做完之后**：测试基线确认、三大缺陷实证在手，可安心开始 P1。

#### Task 001: 验证基线 + 复现三大缺陷

- **关联**：BR-101～BR-120 / INV-101～INV-109 / EVD-110
- **前置任务**：无
- **风险等级**：P0

**为什么做**：确认 v1 测试基线干净；实证 F-102～F-111 描述的三大缺陷，为修复提供对照。

**涉及文件与定位**：
- `Makefile`：`make test`
- `relaykit/go.mod`：`cd relaykit && GOWORK=off go build ./...`

**具体操作**：
1. 运行 `make test`，确认全绿
2. 运行 `cd relaykit && GOWORK=off go build ./...`，确认 BUILD_OK
3. 实证缺陷：用 `curl` 发送含 `tools` 定义 + `tool_call` 的 OpenAI 请求，查 `sqlite3 one-api.db "SELECT segments FROM log_contents WHERE request_id='xxx'"`，确认：
   - tool_call segment 的 `derived.domains` 为空（F-102）
   - tool_result segment 无 `text` 字段（F-105）
   - `req.Tools` 未产生 tool_def segment（F-106）
4. curl 非流式含 tool_calls 的响应，确认输出侧 segments 只有 assistant，无 tool_call（F-108）
5. 将证据截图/日志存入 `docs/content-audit-v2/evidence/phase-0/`

**验证**：make test 全绿 + relaykit build OK + 三大缺陷实证文件齐全

**Evidence**：`evidence/phase-0/baseline.txt` + `defect-*.json`

---

### Phase 1: 后端采集修复（P1）

> **你在哪里**：P0 基线确认。
> **做完之后**：ScanText 双轨建成，tool_call/tool_result/tool_def 三补齐完成，make test 全绿。

#### Task 101: audit/types.go 加 ScanText + KindToolDef

- **关联**：BR-101, BR-102, BR-105 / INV-103
- **前置任务**：1
- **风险等级**：P0

**为什么做**：ScanText 是匹配面基础，KindToolDef 是新 kind。

**涉及文件与定位**：
- `audit/types.go`：`type Segment struct` L73

**具体操作**：
1. `Segment` struct 加字段 `ScanText string \`json:"-"\``（置于 `Text` 之后）
2. SegmentKind const 组加 `KindToolDef = "tool_def"`（置于 `KindToolResult` 之后）

**验证**：`GOWORK=off go build ./audit/...` 通过；写单测 `common.Marshal(Segment{ScanText:"x"})` 输出不含 `x`

**Evidence**：`evidence/phase-1/types-build.txt`

**注意事项**：`json:"-"` 确保落库 JSON 不含 ScanText（BR-102）

---

#### Task 102: audit/segment.go 常量 + defaultKindPolicy + downgradeOrder

- **关联**：BR-105, BR-106, BR-120
- **前置任务**：101
- **风险等级**：P1

**为什么做**：tool_def 的留存策略与降级序。

**涉及文件与定位**：
- `audit/segment.go`：字节常量组 L17、`defaultKindPolicy` L31、`downgradeOrder` L43

**具体操作**：
1. 字节常量组加 `DefaultToolDefPreviewBytes = 1024`
2. `defaultKindPolicy` map 加 `KindToolDef: {ModePreview, DefaultToolDefPreviewBytes}`
3. `downgradeOrder` 切片插入 `KindToolDef`，位置在 `KindToolResult` 之后、`KindToolCall` 之前（BR-106）

**验证**：单测断言 `defaultKindPolicy[KindToolDef].mode == ModePreview && limit == 1024`；断言 downgradeOrder 顺序

**Evidence**：`evidence/phase-1/policy-test.txt`

---

#### Task 103: makeDropSegment 补 ScanText

- **关联**：BR-104
- **前置任务**：101
- **风险等级**：P1

**为什么做**：tool_result（mode=drop）全文用于 keyword/regex 扫描。

**涉及文件与定位**：
- `audit/segment.go`：`func makeDropSegment` L144

**具体操作**：
1. 在 `seg := Segment{...}` 构造体内加 `ScanText: text`（text 是入参全文）

**验证**：单测 `makeDropSegment("tool_result", "含关键词全文", 0, "drop")` → `seg.Text == ""` 且 `seg.ScanText != ""`

**Evidence**：`evidence/phase-1/drop-scantext-test.txt`

---

#### Task 104: makeToolCallSegment 补 deriveFacts + ScanText

- **关联**：BR-103
- **前置任务**：101
- **风险等级**：P1

**为什么做**：OpenAI tool_call 的参数全文需 derive URLs/Domains + 填 ScanText。

**涉及文件与定位**：
- `audit/segment.go`：`func makeToolCallSegment` L161

**具体操作**：
1. 在 `derived := &DerivedFacts{Tools: []string{name}, ArgsKeys: argsKeys}` 之后，调 `derivedFromText := deriveFacts(call.Function.Arguments)`
2. 合并 URLs/Domains：`derived.URLs = append(derived.URLs, derivedFromText.URLs...); derived.Domains = append(derived.Domains, derivedFromText.Domains...)`
3. `seg` 构造体加 `ScanText: call.Function.Arguments`（全文，在截断判断之前）

**验证**：单测 tool_call 参数含 `https://evil.com` → `seg.Derived.Domains` 含 `evil.com`；`seg.ScanText` 为全参数

**Evidence**：`evidence/phase-1/toolcall-derive-test.txt`

---

#### Task 105: makeClaudeToolUseSegment + makeGeminiFunctionCallSegment 补齐

- **关联**：BR-103
- **前置任务**：101
- **风险等级**：P1

**为什么做**：Claude / Gemini tool_call 当前只有 Derived，无 Text 无 ScanText。

**涉及文件与定位**：
- `audit/segment.go`：`makeClaudeToolUseSegment` L309、`makeGeminiFunctionCallSegment` L379

**具体操作**：

**makeClaudeToolUseSegment**：
1. Marshal `mc.Input` → `argsJSON, _ := common.Marshal(mc.Input)`
2. `argsText := string(argsJSON)`
3. 调 `derivedFromText := deriveFacts(argsText)`，合并 URLs/Domains 进 `derived`
4. 按 tool_call policy 截断：`policy := defaultKindPolicy[KindToolCall]; text := argsText; scanText := argsText; if len(argsText) > policy.limit { text = truncateUTF8(argsText, policy.limit); truncated = true }`
5. 构造 seg 时填 `Text: text, ScanText: scanText, Truncated: truncated, Bytes: len(argsText)`

**makeGeminiFunctionCallSegment**：同样流程，入参 `fc.Arguments`

**验证**：单测 Claude/Gemini tool_call 含 URL + 关键词 → 三档命中

**Evidence**：`evidence/phase-1/claude-gemini-toolcall-test.txt`

---

#### Task 106: makeToolDefSegment + Build*InputSegments 三入口

- **关联**：BR-105, F-136
- **前置任务**：102
- **风险等级**：P1

**为什么做**：tool 定义采集（OpenAI/Claude/Gemini 三格式）+ 新入口函数。

**涉及文件与定位**：
- `audit/segment.go`：新增函数

**具体操作**：
1. 新增 `makeToolDefSegment(kind string, name string, description string, schemaJSON string, idx int) Segment`：
   - 拼接全文 `fullText := name + " " + description + " " + schemaJSON`
   - policy = tool_def (preview/1KB)
   - 截断、derive、填 Text/ScanText/Derived
2. 新增 `buildOpenAIToolDefSegments(tools []dto.ToolCallRequest) []Segment`：遍历 tools，每个调 makeToolDefSegment
3. 新增 `BuildOpenAIInputSegments(req *dto.GeneralOpenAIRequest, cfg SegmentConfig) []Segment`：
   - `msgSegs := BuildOpenAISegments(req.Messages, cfg)`
   - `toolSegs := buildOpenAIToolDefSegments(req.Tools)`
   - 合并 + `applyBudget`
4. 同理新增 `BuildClaudeInputSegments(req *dto.ClaudeRequest, cfg) []Segment`、`BuildGeminiInputSegments(req *dto.GeminiChatRequest, cfg) []Segment`，分别处理 Claude `GetTools()` 和 Gemini `GetTools()`（F-137）

**验证**：单测 req 带 3 个 tools → segments 含 3 条 kind=tool_def

**Evidence**：`evidence/phase-1/tooldef-test.txt`

---

#### Task 107: controller/relay.go 调用点更新

- **关联**：BR-105, F-136
- **前置任务**：106
- **风险等级**：P1

**为什么做**：OnInput 接线改调新入口，传入完整 request。

**涉及文件与定位**：
- `controller/relay.go`：L291 / L294 / L297

**具体操作**：
1. L291 `snap.Segments = audit.BuildOpenAISegments(r.Messages, cfg)` → `snap.Segments = audit.BuildOpenAIInputSegments(r, cfg)`
2. L294 `BuildClaudeSegments(r, cfg)` → `BuildClaudeInputSegments(r, cfg)`
3. L297 `BuildGeminiSegments(r, cfg)` → `BuildGeminiInputSegments(r, cfg)`

**验证**：curl 带 tools 的请求 → `segments[]` 含 tool_def

**Evidence**：`evidence/phase-1/oninput-tooldef.json`

---

#### Task 108: ScanSegments 改扫描面

- **关联**：BR-101, BR-104
- **前置任务**：101
- **风险等级**：P1

**为什么做**：keyword/regex 扫描改用 ScanText（全文）优先。

**涉及文件与定位**：
- `service/audit_watchlist.go`：`ScanSegments` L39，keyword 分支 L86、regex 分支 L96

**具体操作**：
1. 在 L74 `for si, seg := range segs {` 循环开头加辅助函数调用：
   ```go
   scanFace := func(s audit.Segment) string {
       if s.ScanText != "" { return s.ScanText }
       return s.Text
   }
   ```
2. L86 keyword 分支条件 `seg.Text != ""` → `scanFace(seg) != ""`；扫描入参 `seg.Text` → `scanFace(seg)`
3. L96 regex 分支同理

**验证**：单测 drop segment（Text 空，ScanText 有）→ keyword/regex 命中

**Evidence**：`evidence/phase-1/scanface-test.txt`

---

#### Task 109: audit_sink flush 前清 ScanText + 补单测

- **关联**：BR-101, BR-102 / INV-103
- **前置任务**：101; 103～108
- **风险等级**：P0

**为什么做**：落库前清空 ScanText，保证 JSON 结构不变；补齐全部 P1 单测。

**涉及文件与定位**：
- `service/audit_sink.go`：`func (s *LogContentSink) flush` L259
- `audit/segment_test.go` / `audit/segment_walkers_test.go`

**具体操作**：
1. 在 `flush` 函数内，`segments JSON marshal` 之前，遍历清空：
   ```go
   for i := range rec.segments {
       rec.segments[i].ScanText = ""
   }
   ```
2. 在 `audit/segment_test.go` 补测试：
   - `TestScanTextNotMarshal`：Segment{ScanText:"x"} marshal 后 JSON 不含 "x"
   - `TestDefaultKindPolicy`：断言 F-101 全部值 + tool_def=preview/1024
   - `TestDowngradeOrder`：断言 tool_result < tool_def < tool_call < system < assistant < user
   - `TestToolCallDerivesFacts`：T-104 验证
   - `TestClaudeGeminiToolCallComplete`：T-105 验证
   - `TestToolDefSegments`：T-106 验证
3. 在 `segment_walkers_test.go` 补 Claude/Gemini walker 测试

**验证**：`make test` 全绿；`cd audit && go test -v` 全 PASS

**Evidence**：`evidence/phase-1/test-output.txt`

---

### Phase 2: OpenAI 输出修复（P2）

> **你在哪里**：P1 完成，input 侧三补齐。
> **做完之后**：OpenAI 流式 + 非流式 output 侧 tool_calls 独立成段。

#### Task 201: audit.BuildOutputToolCallSegments

- **关联**：BR-107
- **前置任务**：106
- **风险等级**：P1

**为什么做**：辅助函数，解析响应 tool_calls JSON → segments。

**涉及文件与定位**：
- `audit/segment.go`：新增函数

**具体操作**：
1. 新增 `BuildOutputToolCallSegments(toolCallsJSON json.RawMessage, maxBytes int) []Segment`：
   - Unmarshal 为 `[]dto.ToolCallRequest`
   - 遍历，调 `makeToolCallSegment`（复用）
   - 返回 segments
2. 非流式 + 流式共用

**验证**：单测 response tool_calls JSON → segments

**Evidence**：`evidence/phase-2/output-toolcall-helper.txt`

---

#### Task 202: 非流式 OpenaiHandler 补采 tool_calls

- **关联**：BR-107, F-108
- **前置任务**：201
- **风险等级**：P1

**为什么做**：非流式响应 tool_calls 当前丢失。

**涉及文件与定位**：
- `relay/channel/openai/relay-openai.go`：`OpenaiHandler` L234，OnOutput 现址 L348-360

**具体操作**：
1. 在 L348-360 `if sink := info.ContentSink; sink != nil {` 块内：
2. 当前收集 assistant 全文后，追加：
   ```go
   var toolSegs []audit.Segment
   for _, choice := range simpleResponse.Choices {
       if len(choice.Message.ToolCalls) > 0 {
           toolSegs = append(toolSegs, audit.BuildOutputToolCallSegments(choice.Message.ToolCalls, common.AuditPerRequestMaxBytes)...)
       }
   }
   snap.Segments = append(snap.Segments, toolSegs...)
   ```

**验证**：curl 非流式 tool_calls 响应 → segments 含 tool_call kind

**Evidence**：`evidence/phase-2/nonstream-toolcall.json`

---

#### Task 203: 流式 OaiStreamHandler 独立 tool 累加器

- **关联**：BR-107, F-109 / INV-109
- **前置任务**：201
- **风险等级**：P2

**为什么做**：流式当前把 tool 数据混入 assistant 全文。

**涉及文件与定位**：
- `relay/channel/openai/relay-openai.go`：`OaiStreamHandler` L105

**具体操作**：
1. 在 L105 函数开头，声明局部 tool 累加器：
   ```go
   toolCallsByIdx := make(map[int]*dto.ToolCallRequest)
   ```
2. 在扫描 loop 内（`scanner.Scan()` 循环），解析每个 chunk，提取 `delta.ToolCalls`，按 `choice.Index * 1000 + tool.Index` 聚合 name + arguments
3. 参考 `collectStreamFunctionCallNames` 的去重范式（L209，F-132）
4. 在 L195-204 OnOutput 块内，追加：
   ```go
   var toolSegs []audit.Segment
   for _, tc := range toolCallsByIdx {
       toolSegs = append(toolSegs, makeToolCallSegment(*tc, 0))
   }
   snap.Segments = append(snap.Segments, toolSegs...)
   ```
5. **签名不改**（INV-109）

**验证**：curl 流式 tool_calls → segments 含独立 tool_call，assistant 全文不含 tool 数据

**Evidence**：`evidence/phase-2/stream-toolcall.json`

**注意事项**：`ProcessStreamResponse` 签名不变（INV-109），外部 xai 调用点无需改

---

### Phase 3: Claude / Gemini OnOutput（P3）

> **你在哪里**：P2 完成。
> **做完之后**：Claude / Gemini 流式 + 非流式 OnOutput 接线完成。

#### Task 301: Claude OnOutput 流式 + 非流式

- **关联**：BR-108, F-111, F-131, F-132
- **前置任务**：201
- **风险等级**：P1

**为什么做**：Claude 当前无 OnOutput。

**涉及文件与定位**：
- `relay/channel/droid/relay-claude.go`：`ClaudeStreamHandler` L194、`ClaudeHandler` L268

**具体操作**：

**ClaudeStreamHandler（流式）**：
1. 在函数末尾（当前 return usage 之前），加 OnOutput：
   ```go
   if sink := info.ContentSink; sink != nil {
       text := claudeInfo.ResponseText.String()
       if text != "" {
           seg := audit.BuildAssistantOutputSegment(text, common.AuditPerRequestMaxBytes)
           snap := audit.OutputSnapshot{RequestId: info.RequestId, Segments: []audit.Segment{seg}}
           common.RelayCtxGo(c, func() { sink.OnOutput(snap) })
       }
   }
   ```
2. `claudeInfo.ResponseText` 是 `strings.Builder`（F-131）

**ClaudeHandler（非流式）**：
1. 解析响应后（当前有 `claudeResponse` 变量），提取 `Content[].Text`
2. 同样构建 OutputSnapshot + OnOutput

**验证**：curl Claude 流式 + 非流式 → segments 含 assistant 输出

**Evidence**：`evidence/phase-3/claude-output.json`

---

#### Task 302: Gemini OnOutput 流式 + 非流式

- **关联**：BR-108, F-111, F-113
- **前置任务**：201
- **风险等级**：P1

**为什么做**：Gemini 当前无 OnOutput。

**涉及文件与定位**：
- `relay/channel/gemini/relay-gemini.go`：`GeminiChatStreamHandler` L209、`GeminiChatHandler` L313

**具体操作**：

**GeminiChatStreamHandler（流式）**：
1. 在 L216 callback 开头，声明局部累加器 `var auditText strings.Builder`
2. 在 callback 内每次处理 `geminiResponse.Candidates` 时，同步写入 `auditText`：
   ```go
   for _, candidate := range geminiResponse.Candidates {
       for _, part := range candidate.Content.Parts {
           if part.Text != "" {
               auditText.WriteString(part.Text)
           }
       }
   }
   ```
3. 在 L297 `if err != nil { return usage, err }` 之后，加 OnOutput（同 T-301 范式）

**GeminiChatHandler（非流式）**：
1. L361 已有 `fullTextResponse`，提取 text
2. 同样 OnOutput

**验证**：curl Gemini 流式 + 非流式 → segments 含 assistant 输出

**Evidence**：`evidence/phase-3/gemini-output.json`


---

### Phase 4: 规则扩展 + 模板 + TTL + 审计日志 API（P4）

> **你在哪里**：P3 完成（或与 P1 并行启动，P4 与 P1-P3 无代码依赖）。
> **做完之后**：规则表含 Source/TemplateId；内置模板可 apply/enable/disable/remove；导入导出可用；TTL 清理任务运行；GET /api/audit/logs 可查。

#### Task 401: AuditWatchlistRule 加 Source / TemplateId 列

- **关联**：BR-113, BR-114, BR-115 / INV-108 / ASM-104
- **前置任务**：1（基线）
- **风险等级**：P1

**为什么做**：区分手工规则与模板来源，支持按模板 id 批量操作。

**涉及文件与定位**：
- `model/audit_watchlist_rule.go`：`type AuditWatchlistRule struct` L25-34

**具体操作**：
1. struct 加两个字段（置于 `Note` 之后）：
   ```go
   Source     string `json:"source"     gorm:"type:varchar(16);index"`
   TemplateId string `json:"template_id" gorm:"type:varchar(64);index"`
   ```
2. `CreateWatchlistRule` / `UpdateWatchlistRule` 的 `Updates(map[...])` 补入 `source` / `template_id`
3. GORM AutoMigrate 自动 ADD COLUMN，**不写原始 DDL**（INV-108）
4. 历史规则 Source 空字符串 → 在展示层按 manual 解释，**不迁移旧数据**

**验证**：服务启动后 `sqlite3 one-api.db ".schema audit_watchlist_rules" | grep source` → 非空

**Evidence**：`evidence/phase-4/rule-schema.txt`

---

#### Task 402: service/audit_template.go 内置模板定义

- **关联**：BR-113 / UF-102
- **前置任务**：401
- **风险等级**：P1

**为什么做**：内置模板为 Go 侧只读常量，不写库、不可编辑（BR-113）。

**涉及文件与定位**：
- `service/audit_template.go`（待创建）

**具体操作**：
1. 新建文件，定义 `BuiltinTemplateRule` 与 `BuiltinTemplate` 结构体
2. `BuiltinAuditTemplates []BuiltinTemplate` 包含三个内置包：
   - **`basic-security`**（基础安全）：`keyword` 类规则若干（常见提示注入关键词）；无 regex
   - **`privacy-pii`**（隐私保护）：`keyword` 类 PII 词汇；`domain` 类常见数据中间商域名
   - **`api-key-leak`**（API 密钥检测）：`regex` 类 3 条（模式如 `sk-[A-Za-z0-9]{48}`、`AIza[0-9A-Za-z-_]{35}`），**Enabled 默认 false**（BR-116）
3. 每条规则含 `Kind` / `Pattern` / `Severity` / `Enabled`（regex 均 false）/ `Note`
4. `GetBuiltinTemplate(id string) (*BuiltinTemplate, bool)` 查找函数

**验证**：单测 `GetBuiltinTemplate("basic-security")` 返回非空；regex 规则 Enabled=false

**Evidence**：`evidence/phase-4/templates-list.json`

---

#### Task 403: 模板 API handlers（list / apply / enable / disable / remove）

- **关联**：BR-113～BR-116 / UF-102 / EVD-105
- **前置任务**：402; 401
- **风险等级**：P1

**为什么做**：五个端点驱动模板包全生命周期。

**涉及文件与定位**：
- `controller/audit_content.go`：现有 7 个 handler，追加此文件

**具体操作**：

**ListAuditTemplates**（GET /api/audit/templates）：
1. 遍历 `service.BuiltinAuditTemplates`，对每个模板查询 DB 已应用规则数（WHERE template_id=? AND source='template'）
2. 返回 `[{id, name, description, rule_count, applied_count, enabled_count, status}]`

**ApplyAuditTemplate**（POST /api/audit/templates/:id/apply）：
1. `service.GetBuiltinTemplate(id)` 找模板，404 则报错
2. 对每条规则：幂等键 `(template_id, kind, pattern)` 查 DB，已存在则跳过
3. regex 规则 Enabled=false（ASM-105）；非 regex 按模板 Enabled 字段
4. 批量 Create；version bump（BR-114）
5. 返回 `{applied, skipped, regex_disabled, message}`

**EnableAuditTemplate**（POST /api/audit/templates/:id/enable）：
1. 批量 UPDATE enabled=true WHERE template_id=? AND source='template' AND kind != 'regex'
2. regex 规则受 MaxEnabledRegexRules 约束：分批判断名额（BR-116）
3. version bump；返回 `{enabled, regex_skipped, message}`

**DisableAuditTemplate**（POST /api/audit/templates/:id/disable）：
1. UPDATE enabled=false WHERE template_id=? AND source='template'
2. version bump

**DeleteAuditTemplate**（DELETE /api/audit/templates/:id）：
1. 需二次确认（前端处理）；后端直接 DELETE WHERE template_id=? AND source='template'
2. version bump

**验证**：curl POST apply 两次 → DB count 不变（幂等，BR-114）

**Evidence**：`evidence/phase-4/template-apply-idempotent.json`

---

#### Task 404: 规则 export / import API handlers

- **关联**：BR-117 / UF-103 / EVD-107
- **前置任务**：401
- **风险等级**：P1

**为什么做**：导入导出规则 JSON（BR-117）。

**涉及文件与定位**：
- `controller/audit_content.go`：追加两个 handler

**具体操作**：

**ExportWatchlistRules**（GET /api/audit/watchlist/export）：
1. `model.ListWatchlistRules(nil, "")` 取全部规则
2. 构造导出结构 `{version, template_id:"custom-export", name:"", description:"", rules:[]}`
3. 响应头 `Content-Disposition: attachment; filename="audit-rules-<timestamp>.json"`

**ImportWatchlistRules**（POST /api/audit/watchlist/import）：
1. 解析 JSON body → `[]model.AuditWatchlistRule`（只读 Kind/Pattern/Severity/Note/Enabled）
2. 逐条校验 `validateWatchlistRule`（复用 L122）；非法条目收集错误 + **整批拒绝**（BR-117）
3. 超过上限（100 条）整批拒绝
4. 全部合法 → DB 批量 Create；Source='manual'；version bump

**验证**：导入含非法 kind 的 JSON → 400；`SELECT count(*)` 不变

**Evidence**：`evidence/phase-4/import-reject.json`

---

#### Task 405: model/log_content.go 补 ListLogContents + DeleteOldLogContentBatch

- **关联**：BR-110, BR-112 / ASM-106
- **前置任务**：1
- **风险等级**：P1

**为什么做**：审计日志列表查询 + TTL 批删所需 model 函数。

**涉及文件与定位**：
- `model/log_content.go`：追加两个函数

**具体操作**：

**ListLogContents**：
```go
type ListLogContentsParams struct {
    Severity     string
    MinHit       int
    StartTime    int64
    EndTime      int64
    UserId       int
    ModelName    string
    Page         int
    PageSize     int
}
func ListLogContents(p ListLogContentsParams) ([]*LogContent, int64, error)
```
1. GORM chain：WHERE 条件按参数附加（已有 index）
2. Count + Find（两次查询，先 count 再 find）

**DeleteOldLogContentBatch**（参考 `model/log.go:DeleteOldLogBatch` L703）：
```go
func DeleteOldLogContentBatch(ctx context.Context, cutoff int64, batchSize int) (int64, error)
```
1. `LOG_DB.Where("created_at < ?", cutoff).Limit(batchSize).Delete(&LogContent{})` 返回 RowsAffected

**验证**：单测 ListLogContents 多条件过滤；DeleteOldLogContentBatch cutoff 精确

**Evidence**：`evidence/phase-4/log-content-query.txt`

---

#### Task 406: TTL 清理系统任务

- **关联**：BR-112 / UF-109 / EVD-109
- **前置任务**：405
- **风险等级**：P1

**为什么做**：AuditContentTTLDays 当前无清理消费者（F-125）。

**涉及文件与定位**：
- `model/system_task.go`：`SystemTaskTypeLogCleanup` L19
- `service/system_task.go`：完整范式 L77-428（F-138）
- `controller/`：若有触发 API 入口

**具体操作**：
1. `model/system_task.go` 加 `SystemTaskTypeLogContentCleanup = "log_content_cleanup"`
2. `service/system_task.go` 照 `logCleanupHandler` 范式新增 `logContentCleanupHandler`：
   - `Payload`：`TargetTimestamp int64`（cutoff = now - TTLDays * 86400）、`BatchSize int`（默认 500）
   - `State`：`Processed int64`
   - `runLogContentCleanupTask`：循环调 `model.DeleteOldLogContentBatch`，间隔 100ms
   - `StartLogContentCleanupTask(ttlDays int)`：TTL=0 时立即返回 no-op
3. 触发点：系统任务调度逻辑中，在触发 `log_cleanup` 的同等时机（每日/定时）也触发 `log_content_cleanup`

**验证**：插入 createdAt < cutoff 的记录，运行任务，记录消失；TTL=0 时不删除

**Evidence**：`evidence/phase-4/ttl-cleanup.txt`（含前后 count 对比）

---

#### Task 407: GET /api/audit/logs handler + router 注册全部新路由

- **关联**：BR-110 / ASM-106 / UF-104 / EVD-103
- **前置任务**：405; 403; 404
- **风险等级**：P1

**为什么做**：审计日志列表 API（ASM-106）；同步注册 P4 全部新路由。

**涉及文件与定位**：
- `controller/audit_content.go`：新增 `GetAuditLogs`
- `router/api-router.go`：`auditRoute` L283-292

**具体操作**：

**GetAuditLogs handler**：
1. 从 query 解析 `ListLogContentsParams`（severity/min_hit/start_timestamp/end_timestamp/user_id/model_name/p/page_size）
2. 调 `model.ListLogContents(params)`
3. 返回 `{items:[...], total:N}`

**路由注册**（auditRoute 组内追加）：
```go
auditRoute.GET("/logs", controller.GetAuditLogs)
auditRoute.GET("/templates", controller.ListAuditTemplates)
auditRoute.POST("/templates/:id/apply", controller.ApplyAuditTemplate)
auditRoute.POST("/templates/:id/enable", controller.EnableAuditTemplate)
auditRoute.POST("/templates/:id/disable", controller.DisableAuditTemplate)
auditRoute.DELETE("/templates/:id", controller.DeleteAuditTemplate)
auditRoute.GET("/watchlist/export", controller.ExportWatchlistRules)
auditRoute.POST("/watchlist/import", controller.ImportWatchlistRules)
```

**验证**：curl GET /api/audit/logs → 200 + json；普通用户 curl → 403

**Evidence**：`evidence/phase-4/audit-logs-api.json`

---

### Phase 5: 前端 IA 重构（P5）

> **你在哪里**：P4 完成（或与 P4 并行）。
> **做完之后**：`/audit/watchlist` 路由删除归零；`/usage-logs/audit` 有 admin 守卫；配置迁入系统设置；i18n 术语三分。

#### Task 501: 删除 /audit/watchlist 路由 + 侧边栏项

- **关联**：BR-109 / ASM-107 / EVD-111
- **前置任务**：1
- **风险等级**：P0（clean cutover 不留 shim）

**为什么做**：彻底消除旧入口，grep 归零作为验收（BR-109）。

**涉及文件与定位**：
- `web/src/routes/_authenticated/audit/watchlist/index.tsx`：整文件删除
- `web/src/hooks/use-sidebar-data.ts`：`Audit Watchlist` L161

**具体操作**：
1. 删除 `web/src/routes/_authenticated/audit/watchlist/index.tsx`（若目录下无其他文件则删目录）
2. `use-sidebar-data.ts` L161 删除 `{ title: t('Audit Watchlist'), ... }` 对象整行（注意逗号）
3. 在 `usage-logs/index.tsx` 找到 AuditSectionPlaceholder 内的 `<Link to='/audit/watchlist'>` (L70)，暂改为 `<Link to='/system-settings/security?section=audit'>`（T-503 彻底删占位）
4. 全仓搜索剩余 `/audit/watchlist` 引用：`grep -r "audit/watchlist" web/src` → 应归零

**验证**：`grep -r "audit/watchlist" web/src` 输出为空

**Evidence**：`evidence/phase-5/audit-watchlist-grep-zero.txt`

---

#### Task 502: 迁移 features/audit/ + 补 note 字段

- **关联**：BR-109, BR-118 / F-135
- **前置任务**：501
- **风险等级**：P1

**为什么做**：将现有 525 行审计功能搬运到系统设置目录，并补 note 表单字段（BR-118, F-126）。

**涉及文件与定位**：
- `web/src/features/audit/`（api.ts 58 行 / index.tsx 429 行 / types.ts 38 行）
- 目标：`web/src/features/system-settings/security/audit/`

**具体操作**：
1. 新建 `web/src/features/system-settings/security/audit/` 目录
2. **移动**（不复制）三个文件：`api.ts` → `audit/api.ts`；`types.ts` → `audit/types.ts`；`index.tsx` → `audit/watchlist-panel.tsx`（改名，语义更准）
3. 全局替换 `from '@/features/audit'` → `from '@/features/system-settings/security/audit'`（LSP rename 优先）
4. 在 `watchlist-panel.tsx`（原 index.tsx）的 `RuleEditorDialog`（原 L311 的表单）补 Note 输入框：
   - 在 pattern/severity 字段之后加 `<FormField name="note" ...>` + `<Input placeholder={t('Note (optional)')} />`
5. `types.ts` 中 `AuditWatchlistRuleInput` 已含 note 字段（F-126），无需改类型

**验证**：bun run typecheck 通过；规则编辑弹窗可填 note，保存后列表可见

**Evidence**：`evidence/phase-5/note-field.png`

---

#### Task 503: /usage-logs/audit admin 守卫 + 删 AuditSectionPlaceholder

- **关联**：BR-110 / UF-106 / EVD-108
- **前置任务**：501
- **风险等级**：P0

**为什么做**：添加路由守卫（普通用户跳 403），删除占位 UI（F-121/F-133）。

**涉及文件与定位**：
- `web/src/routes/_authenticated/usage-logs/$section.tsx`：`beforeLoad` L53
- `web/src/features/usage-logs/index.tsx`：`AuditSectionPlaceholder` L62-74，分支 L179-180

**具体操作**：

**beforeLoad 守卫**：
1. 在 L53 `beforeLoad` 函数，`isUsageLogsSectionId` 检查之后加：
   ```ts
   if (params.section === 'audit' && !isAdmin(context)) {
     throw redirect({ to: '/403' })
   }
   ```
2. `isAdmin` 复用现有 RBAC 工具（参考 audit/watchlist/index.tsx 的 ROLE.ADMIN 模式）

**删除占位**：
1. 删除 `AuditSectionPlaceholder` 函数（L62-74）
2. L179-180 `activeCategory === 'audit' ? <AuditSectionPlaceholder />` → 替换为 `<AuditLogListPage />`（T-604 实现）
3. T-501 中已改的 Link 跳转保留或删除（占位完全删除后无意义）

**验证**：普通用户浏览器访问 `/usage-logs/audit` → 跳 /403（不闪现表格）

**Evidence**：`evidence/phase-5/audit-guard-403.png`

---

#### Task 504: i18n 7 文件更新（BR-111 术语三分）

- **关联**：BR-111 / F-123 / F-124 / EVD-111
- **前置任务**：502; 503
- **风险等级**：P1

**为什么做**：清理混淆命名，三分术语（F-123）。

**涉及文件与定位**：
- `web/src/i18n/locales/*.json`（7 文件，F-124）

**具体操作**：
1. 按 BR-111 重命名/新增键：
   - 删除旧键 `Audit Watchlist`（侧边栏用，已删入口）
   - 旧键 `Audit Content Monitoring`（系统设置区标题）→ **保留**（仍适用）
   - 新增键 `Content Audit Logs`（日志页 tab 标题，zh="内容审计日志"）
   - 新增键 `Audit Rules`（规则表标题，zh="审计规则"）
   - 新增键 `Content Audit`（系统设置 section 短名，zh="内容审计"）
   - 旧键 `Manage watchlist` → **不使用**（删）
   - 新增模板包 / 导入导出 / 相关 UI 文案
2. 在 zh 文件手工翻译，其余 6 个文件用 English key 作占位（bun run i18n:sync 会补）
3. 运行 `bun run i18n:sync` → 0 missing / 0 untranslated

**验证**：`grep -c "Audit Watchlist" web/src/i18n/locales/*.json` 全为 0；`bun run i18n:sync` 无 missing

**Evidence**：`evidence/phase-5/i18n-sync.txt`


---

### Phase 6: 前端模板 + 日志 UI（P6）

> **你在哪里**：P4 API 就绪，P5 文件结构就位。
> **做完之后**：系统设置内容审计区包含规则 CRUD + 模板包 + 导入导出 + 重扫；/usage-logs/audit 是真实筛选列表。

#### Task 601: AuditSettingsSection 扩充规则 CRUD（迁移后）

- **关联**：BR-109, BR-118 / UF-101 / EVD-101
- **前置任务**：502
- **风险等级**：P1

**为什么做**：规则表从独立页面移入系统设置 audit section（F-134 中 AuditSettingsSection 已存在，扩充）。

**涉及文件与定位**：
- `web/src/features/system-settings/request-limits/audit-settings-section.tsx`（150 行，F-134）
- `web/src/features/system-settings/security/audit/watchlist-panel.tsx`（迁移后文件）

**具体操作**：
1. 在 `AuditSettingsSection` 中，采集开关区之后引入 `<WatchlistPanel />`（T-502 迁移后的规则表组件）
2. `WatchlistPanel` 从 `web/src/features/system-settings/security/audit/watchlist-panel.tsx` 导入
3. 确认 note 字段（T-502）已在表单中：列表展示 note 列（原 index.tsx 无 note 列，补加）
4. 确认规则 CRUD 所有接口调用路径正确（迁移后 api.ts 路径）

**验证**：浏览器打开 `/system-settings/security`，点「内容审计」section → 可见规则列表；新增规则填 note → 保存后列表 note 列显示

**Evidence**：`evidence/phase-6/rule-crud-note.png`

---

#### Task 602: 模板包 UI

- **关联**：BR-113～BR-116 / UF-102 / EVD-105
- **前置任务**：601; 403（API 就绪）
- **风险等级**：P1

**为什么做**：模板包卡片列表，四态操作（未应用 / 已应用 / 已停用 → apply/enable/disable/remove）。

**涉及文件与定位**：
- `web/src/features/system-settings/security/audit/template-panel.tsx`（待创建）
- `web/src/features/system-settings/security/audit/api.ts`（补模板 API）

**具体操作**：
1. `api.ts` 新增：`listTemplates()` / `applyTemplate(id)` / `enableTemplate(id)` / `disableTemplate(id)` / `removeTemplate(id)`
2. 新建 `template-panel.tsx`：
   - `useQuery` 拉取模板列表（GET /api/audit/templates）
   - 每张卡片渲染：name、description、rule_count、status badge（未应用/已应用N条/已停用）
   - 按钮：已应用态 → 停用/启用/移除；未应用态 → 应用
   - 移除需二次确认弹窗
   - regex 超限时 toast 含说明（BR-116，解析 response.regex_disabled 字段）
3. 在 `AuditSettingsSection` 中，规则列表之后引入 `<TemplatePanel />`

**验证**：应用模板 → 规则表出现带模板来源行；停用 → 规则 enabled 全关闭；手工规则不受影响（BR-115）

**Evidence**：`evidence/phase-6/template-panel.png`

---

#### Task 603: 导入导出 UI + 重扫 UI 迁入

- **关联**：BR-117 / UF-103, UF-105 / EVD-104, EVD-107
- **前置任务**：601; 404（export/import API 就绪）
- **风险等级**：P1

**为什么做**：规则工具栏补导出/导入按钮；重扫从旧页面迁入。

**涉及文件与定位**：
- `web/src/features/system-settings/security/audit/watchlist-panel.tsx`
- `web/src/features/system-settings/security/audit/api.ts`

**具体操作**：
1. **导出**：规则表工具栏加「导出规则」按钮，onClick 调 GET /api/audit/watchlist/export，触发浏览器下载 JSON
2. **导入**：工具栏加「导入规则」按钮，隐藏 `<input type="file" accept=".json">` 触发文件选择；onChange 读文件内容 → POST /api/audit/watchlist/import；失败时 toast 显示具体错误位置
3. **重扫**：将 `WatchlistPage` 中原有的重扫触发 UI（确认弹窗 + 进度条）搬运至 `watchlist-panel.tsx`（不重写，直接挪）
4. api.ts 补 `exportRules(): Promise<Blob>` / `importRules(json: string)` / 重扫接口已有无需重复

**验证**：浏览器：导出 → JSON 文件可下载；导入含非法条目 → toast 报错且规则数不变；重扫点击 → 进度可见

**Evidence**：`evidence/phase-6/export-import.png` + `evidence/UF-103/` + `evidence/UF-105/`

---

#### Task 604: 审计日志列表页（/usage-logs/audit）

- **关联**：BR-110 / UF-104 / EVD-103
- **前置任务**：503; 407（GET /api/audit/logs 就绪）
- **风险等级**：P1

**为什么做**：`/usage-logs/audit` 从占位卡片变成真实筛选列表（BR-110）。

**涉及文件与定位**：
- `web/src/features/usage-logs/audit-log-list.tsx`（待创建）
- `web/src/features/usage-logs/index.tsx`：L179-180 替换引用

**具体操作**：
1. 新建 `audit-log-list.tsx`，参考 `common-logs-columns.tsx` 表格模式：
   - 筛选栏：severity 下拉、min_hit 数字输入、时间范围 picker、用户 ID、模型名
   - 表格列：时间、用户、模型、fidelity、命中数、最高 severity、request_id 尾段
   - 行点击展开 → 复用已有 `DetailsDialog`（details-dialog.tsx 已有审计 Tab）
   - 空态：若 `AuditEnabled=false` 则提示「审计未开启」并提供跳转到系统设置的链接
2. api.ts 加 `listAuditLogs(params)` 调 GET /api/audit/logs
3. `usage-logs/index.tsx` L179-180：`<AuditSectionPlaceholder />` → `<AuditLogListPage />`（T-503 已改占位符结构，此处接入实际组件）
4. `web/src/features/usage-logs/section-registry.tsx` 的 `titleKey` 从 `'Audit Logs'` 改为 `t('Content Audit Logs')`（BR-111）

**验证**：admin 打开 `/usage-logs/audit` → 列表加载；普通用户 → 403；severity 筛选生效

**Evidence**：`evidence/phase-6/audit-log-list.png` + `evidence/UF-104/`

---

#### Task 605: bun run i18n:sync 归零验收

- **关联**：BR-111 / EVD-111
- **前置任务**：504; 601～604
- **风险等级**：P1

**为什么做**：前端全量 i18n 完整性最终检查。

**涉及文件与定位**：
- `web/src/i18n/locales/*.json`（7 文件）

**具体操作**：
1. `cd web && bun run i18n:sync`
2. 补全所有 missing key 的各语言翻译（zh 优先手工翻译，其余用 key 原文占位后提交）
3. `grep -c "Audit Watchlist" web/src/i18n/locales/*.json` → 全 0（EVD-111）

**验证**：`bun run i18n:sync` 输出 0 missing / 0 untranslated；`bun run typecheck` exit 0

**Evidence**：`evidence/phase-6/i18n-sync-final.txt`

---

### Phase 7: 全量验收（P7）

> **你在哪里**：P1-P6 全部完成。
> **做完之后**：所有 EVD 证据齐全，tasks.csv 全部「已完成」。

#### Task 701: 全量验收

- **关联**：全部 BR/UF/INV/EVD
- **前置任务**：所有
- **风险等级**：P0（任意 FAIL 需回溯）

**为什么做**：最终交叉验证，确保没有遗漏的 callsite / 测试 / i18n 问题。

**具体操作**：
1. `make test` → 无 FAIL
2. `GOWORK=off go build ./...` → BUILD_OK
3. `cd relaykit && GOWORK=off go build ./...` → BUILD_OK（INV-106）
4. `cd web && bun run typecheck` → exit 0
5. `cd web && bun run build` → BUILD_OK
6. `bun run i18n:sync` → 0 missing / 0 untranslated
7. `grep -r "audit/watchlist" web/src` → 空输出（BR-109）
8. `grep -c` 旧 i18n 键在 7 文件均为 0（BR-111）
9. curl 必命中请求（含关键词 tool_result）→ HTTP 200 + 响应完整（INV-104）
10. 普通用户 curl GET /api/audit/logs → 403（INV-107）
11. 归档所有 EVD-101～EVD-111 证据文件

**验证**：上方 11 条全部通过

**Evidence**：`evidence/phase-7/final-validation.txt`（汇总所有命令输出）


---

## 5. 验收协议

### 5.1 Phase 验收条件（绿门）

每个 Phase 完成后必须通过对应绿门才能进入下一 Phase。

| Phase | 绿门条件 | 失败时处置 |
|---|---|---|
| P0 | `make test` 全绿；三大缺陷实证文件存在 | 修复遗留失败后重试 |
| P1 | `make test` 全绿；`cd relaykit && GOWORK=off go build ./...` BUILD_OK；audit 包单测新增覆盖 T-101～T-109 全部断言；curl tool_call 含 URL 请求 → segments 有 domains + tool_def | 回溯具体失败 task |
| P2 | curl 非流式 tool_calls 响应 → output segments 含独立 tool_call；curl 流式同结果 | 回溯 T-202/T-203 |
| P3 | curl Claude + Gemini 各一次（流式 + 非流式）→ segments 含 assistant 输出段 | 回溯 T-301/T-302 |
| P4 | rule 表 `.schema` 含 source/template_id；模板 apply 幂等验证；export/import CRUD；GET /api/audit/logs 返回正确；TTL 批删单测通过 | 回溯失败 task |
| P5 | `grep -r "audit/watchlist" web/src` 为空；普通用户 `/usage-logs/audit` → 403（不闪现）；`bun run typecheck` exit 0 | 回溯 T-501/T-503 |
| P6 | 浏览器：系统设置内容审计 CRUD + 模板 + 导入导出 + 重扫全可用；`/usage-logs/audit` 列表加载；`bun run build` BUILD_OK；`bun run i18n:sync` 0 missing | 回溯失败 task |
| P7 | Task 701 全部 11 条通过；EVD-101～EVD-111 证据文件齐全 | 回溯对应 Phase |

### 5.2 真实场景矩阵（UF-101 ～ UF-109）

> UF-107/108/109 为后端内部链路，验证方式为 curl + sqlite3，无浏览器截图。

| 场景 ID | 验证角色 | 验证方式 | 验收命令 / 操作 | 期望结果 | Evidence |
|---|---|---|---|---|---|
| UF-101 | admin | browser | 进入系统设置 > 安全 > 内容审计；新增/编辑（含 note）/删除/启停规则 | 操作成功；watchlist version 递增；规则按新配置命中 | `evidence/UF-101/` |
| UF-102 | admin | browser + curl | 应用内置模板 → 重复应用（幂等）→ 整包停用 → 整包启用 → 整包移除；含 regex 超限场景 | 各态截图齐全；手工规则 enabled 不变；regex 说明出现 | `evidence/UF-102/` |
| UF-103 | admin | browser + curl | 导出 → 下载 JSON；导入合法文件；导入含非法 kind 文件 | 合法导入 count+N；非法导入 400 且 count 不变 | `evidence/UF-103/` |
| UF-104 | admin | browser | 打开 /usage-logs/audit；筛选 severity=high + 时间范围；行展开详情 | 列表按条件过滤；展开显示 segments + flags；console 无 error | `evidence/UF-104/` |
| UF-105 | admin | browser + server log | 在系统设置重扫入口点击重扫 → 确认 → 进度可见 → 完成 | 进度条出现推进；完成 toast；server log 有完成条目 | `evidence/UF-105/` |
| UF-106 | 普通用户 | browser + curl | 访问 /usage-logs/audit；curl GET /api/audit/logs；curl GET /api/audit/templates | 页面跳 403；两个 API 返回 403；响应体无审计数据 | `evidence/UF-106/` |
| UF-107 | 内部（curl + sqlite3）| curl + sqlite3 | 发含 tools 定义 + tool_call/tool_result 的请求（OpenAI/Claude/Gemini 三格式，stream=false）| segments 含 tool_def / tool_call（Text+Derived）/ tool_result；含 URL/关键词 → flags 命中 | `evidence/UF-107/` |
| UF-108 | 内部（curl + sqlite3）| curl + sqlite3 | 同 UF-107 三格式各发 stream=true | segments 含 assistant 输出段 + tool_call 段；覆盖面与 stream=false 一致 | `evidence/UF-108/` |
| UF-109 | 内部（sqlite3 + server log）| sqlite3 | 插入 createdAt < cutoff 的 log_contents 记录；触发 TTL 清理任务；TTL=0 不清理 | 超龄记录消失；TTL=0 记录保留 | `evidence/UF-109/` |

### 5.3 tasks.csv 状态板

> 本节为执行者工具参考，不复制 BR/UF/EVD。

tasks.csv 与本文件同目录，字段：`task_id, phase, title, status, assignee, notes`。

- status 取值：`pending` / `in_progress` / `done` / `blocked`
- 任务 ID 与 §4 编号对齐（T-001 ～ T-701）
- 每 Phase 开始时将该 Phase 所有任务置 `in_progress`，完成后置 `done` + 填 evidence 路径


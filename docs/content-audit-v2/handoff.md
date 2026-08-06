# content-audit-v2 Handoff

本文件是可直接交给执行 Agent 的交付 Prompt。目标是在不破坏 v1 业务不变量的前提下，完成 v2 spec 定义的三项修复：工具调用采集补齐、信息架构重构、内置模板包。

> 使用方式：把本文件完整粘贴给执行 Agent，或让 Agent 开工前先读本文件。
> 本文件只做入口导航，不复制 spec 内容；所有规则、任务、验收细节以 `spec.md` 为准。

---

## 1. 目标（三行话）

1. **采集补齐**：tool_call / tool_result / tool_def 三种 segment 在 OpenAI / Claude / Gemini 三格式的输入侧和输出侧全部正确采集，keyword/regex 规则能扫描全文而非截断残文。
2. **IA 重构**：审计规则配置从独立侧边栏顶级页迁入系统设置；`/usage-logs/audit` 从占位变真实日志列表（含 admin 守卫）。
3. **模板包**：内置3套只读规则模板，支持应用/启停/移除及 JSON 导入导出；`log_contents` TTL 清理任务上线。

---

## 2. 资料清单

| 资料 | 路径 | 规模 | 用途 |
|---|---|---|---|
| **Spec（唯一事实源）** | `docs/content-audit-v2/spec.md` | 1689 行 | 业务合同、技术方案、任务详情、验收协议 |
| **Tasks CSV（状态板）** | `docs/content-audit-v2/tasks.csv` | 32 条 | 任务状态跟踪 |
| v1 spec（只读参考）| `docs/content-audit/spec.md` | 1665 行 | v1 已有约束（BR-001~BR-017），不可破坏 |
| v1 summary | `docs/content-audit/SUMMARY.md` | 118 行 | v1 交付快照，含已知遗留 |

> **证据目录** `docs/content-audit-v2/evidence/` 需在执行期自行创建（make dir），结构见 spec §2.5 EVD 清单。

---

## 3. 开工上下文

### 当前代码状态

- v1 已完整交付（commit `40b459d5`），代码干净，make test 全绿
- v2 spec Stage 2 展开完毕（`spec.md` v0.2.0），tasks.csv 全 pending

### 三大缺陷快速定位

| 缺陷 | 事实 ID | 涉及文件 | 核心症状 |
|---|---|---|---|
| tool 扫描面不足 | F-102/103/104/105 | `audit/segment.go`，`service/audit_watchlist.go` | tool_call/tool_result segment 无全文 → keyword/regex 漏检 |
| tool_def + 输出侧 tool_call 丢失 | F-106/107/108/109/111 | `audit/segment.go`，`relay/channel/openai/relay-openai.go`，`relay/channel/droid/relay-claude.go`，`relay/channel/gemini/relay-gemini.go` | req.Tools 无采集；非流式 tool_calls 丢失；Claude/Gemini 无 OnOutput |
| IA 混乱 | F-118/119/121/122/123 | `web/src/routes/`，`web/src/hooks/use-sidebar-data.ts`，`web/src/features/usage-logs/` | 配置挂侧边栏顶级；日志页占位无守卫 |

### 架构 Before / After（增量，与 v1 无关的行保持不动）

```
采集侧（input）修复：
  controller/relay.go L291/294/297
    Before: BuildOpenAISegments(r.Messages, cfg)        ← req.Tools 丢弃
    After:  BuildOpenAIInputSegments(req, cfg)           ← 含 tool_def 段

  audit/segment.go
    Before: makeToolCallSegment → 无 deriveFacts，无 ScanText
            makeDropSegment    → 无 ScanText
    After:  全部 + ScanText（json:"-"）+ deriveFacts 合并

  service/audit_watchlist.go ScanSegments:
    Before: keyword/regex 只扫 seg.Text（截断后）
    After:  优先扫 seg.ScanText（全文），回退 seg.Text

  service/audit_sink.go flush:
    After: 落库前清空 seg.ScanText（保证 JSON 结构不变，INV-103）

采集侧（output）修复：
  relay/channel/openai/relay-openai.go
    Before: OpenaiHandler OnOutput 只有 assistant 全文
    After:  补采 choice.Message.ToolCalls → 独立 tool_call 段
    Before: OaiStreamHandler OnOutput 把 tool 数据混入 responseTextBuilder
    After:  独立 toolCallsByIdx 累加，不改 ProcessStreamResponse 签名（INV-109）

  relay/channel/droid/relay-claude.go（L194/L268）
    After:  ClaudeStreamHandler + ClaudeHandler 接 OnOutput

  relay/channel/gemini/relay-gemini.go（L209/L313）
    After:  GeminiChatStreamHandler + GeminiChatHandler 接 OnOutput

新增旁路（与 relay 主链路无耦合）：
  model/audit_watchlist_rule.go  → +Source / TemplateId 列
  service/audit_template.go      → 内置模板包（只读 Go 常量）
  controller/audit_content.go    → +8 handler（模板/导入导出/审计日志列表）
  router/api-router.go           → auditRoute 追加 8 条
  model/log_content.go           → +ListLogContents / DeleteOldLogContentBatch
  service/system_task.go         → +logContentCleanupHandler（TTL 清理）
  model/system_task.go           → +SystemTaskTypeLogContentCleanup

前端 IA 重构：
  删除: web/src/routes/_authenticated/audit/watchlist/index.tsx
  删除: use-sidebar-data.ts L161 侧边栏 Audit Watchlist
  迁移: web/src/features/audit/ → system-settings/security/audit/
  扩充: audit-settings-section.tsx（规则表+模板包+导入导出+重扫）
  修复: usage-logs/$section.tsx beforeLoad → audit section admin 守卫
  替换: AuditSectionPlaceholder → AuditLogListPage
  新建: audit-log-list.tsx（GET /api/audit/logs，筛选列表）
```

### Phase 地图

```
P0（基线校准，1 task）
  ↓
P1（后端采集修复，9 tasks）← 核心，最多并行工作
  ↓
P2（OpenAI 输出修复，3 tasks）+ P3（Claude/Gemini OnOutput，2 tasks）← 可并行
  ↓
P4（规则+模板+TTL+审计日志 API，7 tasks）← 可与 P1 提前并行
  ↓
P5（前端 IA 重构，4 tasks）
  ↓
P6（前端模板+日志 UI，5 tasks）← 依赖 P4 API + P5 文件结构
  ↓
P7（全量验收，1 task）
```

---

## 4. 关键规则（Top 12，完整定义见 spec.md §2.1）

| 规则 | 一句话 |
|---|---|
| **BR-101** | ScanText `json:"-"`；`flush` 落库前必须清空 |
| **BR-102** | 落库 JSON 结构与 v1 完全一致，仅可能新增 kind 取值 |
| **BR-103** | tool_call 三格式均填 Text + ScanText + deriveFacts（含 Domains） |
| **BR-104** | tool_result ScanText 填全文，Text 仍空（drop policy） |
| **BR-105** | req.Tools 生成 kind=tool_def 段（preview/1KB） |
| **BR-106** | downgradeOrder：tool_result < **tool_def** < tool_call < system < assistant < user |
| **BR-107** | OpenAI 流式+非流式输出侧 tool_calls 独立成段 |
| **BR-108** | Claude / Gemini 输出侧接 OnOutput |
| **BR-109** | `/audit/watchlist` 路由与侧边栏项彻底删除，全仓无残留引用 |
| **BR-110** | `/usage-logs/audit` 是可筛选真实列表，beforeLoad 有 admin 守卫 |
| **BR-114** | 模板应用幂等，判重键 (template_id, kind, pattern) |
| **BR-116** | 模板内 regex 默认 enabled=false；超限不整体失败，返回说明 |
| **INV-101** | v1 所有 BR/UF/INV 不得破坏（尤其 BR-004 audit 包分层） |
| **INV-103** | v1 已落库 log_contents 无需迁移；新 ScanText 落库 JSON 不含此字段 |
| **INV-106** | `cd relaykit && GOWORK=off go build ./...` 始终通过 |
| **INV-109** | `ProcessStreamResponse`（helper.go L93）签名不改；xai 调用点不动 |

---

## 5. v2 特有硬约束（写代码前必读，已有真实撞坑记录）

1. **`relay/channel/droid/` 目录不存在**（F-131）：Claude handler 实际在工作区 `relay/channel/droid/`，但该目录无法被 `glob` / `read` 工具访问（可能是符号链接）。**必须用 `bash grep -n "func" relay/channel/droid/relay-claude.go`** 等 shell 命令读取；行号以 spec §3.3 所列为准：`ClaudeStreamHandler` L194、`ClaudeHandler` L268。

2. **`ProcessStreamResponse` 签名绝对不改**（INV-109）：该函数在 `relay/channel/xai/text.go:63` 有外部调用。流式 tool_calls 累加**必须在 `OaiStreamHandler` 局部完成**，不得修改 `ProcessStreamResponse`。

3. **ScanText 落库前清空不可遗漏**（BR-101/BR-102）：`service/audit_sink.go` 的 `flush` 函数在序列化 Segments 之前，必须遍历将所有 `seg.ScanText = ""`。漏掉会导致全文进 DB，违反 INV-103。

4. **`Source` 字段不加 `gorm:"default:..."`**（AGENTS.md 禁止）：业务默认值在 `CreateWatchlistRule` 代码里规范化（空 → `"manual"`），不依赖 GORM tag default。

5. **模板 regex 应用时 count 要排除 template 自身已有的 enabled regex**（BR-116）：`countEnabledRegexRules(excludeId)` 现只排除单条 id，批量应用时需自行累加本次要启用的 regex 数，与现有 enabled 数比较，不能直接调原函数。

6. **前端路由迁移不留 shim**（ASM-107）：直接删除 `/audit/watchlist` 路由文件，`/usage-logs/audit` 占位跳转链接同步改指向系统设置，全仓 grep 归零作为交付条件。

---

## 6. 开工前初始化

```bash
# 1. 确认工作区干净
git status --porcelain   # 期望空（或只有 docs/content-audit-v2/ 的未跟踪文件）

# 2. 基线验证
make test
GOWORK=off go build ./...
cd relaykit && GOWORK=off go build ./...

# 3. 复现三大缺陷（可选但推荐，为 P1 修复建立对照）
# 见 spec §4 Task 001 具体步骤

# 4. 打开 tasks.csv，从 T-001 开始
```

---

## 7. 核心执行循环

```
WHILE tasks.csv 存在 pending/in_progress 任务:
    1. 找前置任务全部 done 的下一条任务
    2. 读 spec.md §4 对应 Task 详情（#### Task NNN: 标题）
    3. 确认关联 BR/UF/INV；确认哪些行为不能变
    4. tasks.csv 状态更新为 in_progress
    5. 按 spec §3.3 三段式定位表校验文件位置（symbol + bash grep，行号是 hint）
    6. 执行具体操作步骤
    7. 运行验证命令，输出保存到 docs/content-audit-v2/evidence/<phase>/
    8. 通过 → tasks.csv 置 done；失败 → 排障（≤3 次）
    9. 仍失败 → 置 blocked:{原因}，继续不依赖该任务的后续任务
   10. Phase 末回归通过后，输出 summary 到 evidence/phase-N/summary.md
```

**不要中途询问"是否继续"。** 除非所有剩余任务都被阻塞，否则持续推进。

---

## 8. 禁止事项

- **不得**在 `audit/` 包内 import `model` 或 `relay/common`（BR-004，成环）
- **不得**向 `relaykit/` 添加任何 audit 依赖（INV-106）
- **不得**修改 `ProcessStreamResponse` 函数签名（INV-109）
- **不得**直接 `import "encoding/json"` 做 marshal/unmarshal（AGENTS.md：用 `common.Marshal/Unmarshal`）
- **不得**给 `AuditWatchlistRule.Source` 加 `gorm:"default:..."` tag（AGENTS.md 禁止）
- **不得**保留 `/audit/watchlist` 路由的任何 shim 或 redirect（ASM-107：clean cutover）
- **不得**只跑 make test 就宣称完成——完成标准是 spec §5.2 真实场景矩阵 9 行全部通过并有 Evidence

---

## 9. 排障顺序

1. **import cycle** → 检查 spec §3.1 分层图，确认是否 `audit/` 包直接 import 了 model
2. **ScanText 出现在 DB** → 检查 `audit_sink.go` flush 函数是否漏了清空循环
3. **tool_def 段不出现** → 检查 `controller/relay.go` L291 是否已换调 `BuildOpenAIInputSegments`
4. **tool_call 没有 Domains** → 检查 `makeToolCallSegment` / `makeClaudeToolUseSegment` / `makeGeminiFunctionCallSegment` 是否补了 deriveFacts + Domains 合并
5. **ProcessStreamResponse 报 xai 编译错误** → 说明改了签名，必须撤销，改用局部累加器
6. **Claude handler 文件找不到** → 用 `bash grep -rn "func ClaudeStreamHandler" relay/channel/` 定位
7. **模板 regex 超限整体失败** → 检查 apply 函数是否对非 regex 和 regex 分别处理（BR-116）
8. **前端 grep 未归零** → `grep -r "audit/watchlist" web/src` 定位残留引用
9. **make test FAIL** → `make test 2>&1 | grep FAIL` 精确定位，优先修再继续
10. 最多主动修复 3 次，仍失败则阻塞该任务继续其他

---

## 10. 完成标准与汇报

全部 32 个任务 done 后，运行终验序列：

```bash
# 后端
make test
GOWORK=off go build ./...
cd relaykit && GOWORK=off go build ./...

# 前端
cd web && bun run typecheck && bun run build
bun run i18n:sync   # 期望 0 missing / 0 untranslated

# 清洁性
grep -r "audit/watchlist" web/src   # 期望空输出
```

然后逐条执行 **spec §5.2 真实场景矩阵**（UF-101 ～ UF-109，9 行），截图/API 样例存入 `evidence/UF-1xx/`。

最后输出完成总结：

```markdown
## 完成总结
- 完成范围：P0～P7，32/32 任务
- 修改文件：（列出）
- 通过的 BR：BR-101～BR-120
- 通过的 UF：UF-101～UF-109（5.2 矩阵 9/9 行通过）
- 未破坏的不变量：INV-101～INV-109
- Evidence：docs/content-audit-v2/evidence/ 齐全
- 剩余风险：（无 / 具体描述）
```

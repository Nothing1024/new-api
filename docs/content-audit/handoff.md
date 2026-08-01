# content-audit Handoff

本文件是可直接交给执行 Agent（Codex / Claude / Generic）的交付 Prompt。目标不是"按文件改代码"，而是在不破坏业务不变量的前提下，完成 spec 定义的用户可见行为。

> 使用方式：把本文件完整粘贴给执行 Agent，或让 Agent 开工前先读本文件。
> 本文件只做入口导航，不复制 spec 内容；所有规则、任务、验收细节以 `spec.md` 为准。

---

## 1. 目标

为 QuantumNous/new-api（Go API 网关）添加内容监控 / API 输入输出安全审计能力：采集 relay 请求的输入输出，存入独立表 `logs_content`（LOG_DB），通过 `request_id` 与 `logs` 1:1 关联，管理员在现有 Usage Logs 详情弹窗查看审计内容，支持 watchlist 命中检测和版本驱动重扫。

---

## 2. 资料清单

| 资料 | 路径 | 状态 | 用途 |
|---|---|---|---|
| Spec（唯一事实源）| `spec.md` | 存在（1664 行）| 业务合同、技术方案、任务详情、验收协议 |
| Tasks CSV（状态板）| `tasks.csv` | 存在（43 条任务）| 任务状态跟踪 |
| Evidence 目录 | `evidence/` | 存在（含 README.md）| 证据归档 |

---

## 3. 开工上下文

### 架构 Before / After（简）

```
Before: relay → controller → GenRelayInfo → DoResponse → PostTextConsumeQuota → RecordConsumeLog → logs

After（新增 audit 钩子×3）:
  relay → controller → GenRelayInfo（注入 ContentSink）
       → [OnInput] audit sink (async)
       → DoResponse → [OnOutput] audit sink (async, Phase2)
       → PostTextConsumeQuota → [OnSettled] audit sink (async) → logs_content（新表）
                              → RecordConsumeLog → logs（Other.admin_info.audit=指针）
```

### Phase 地图

```
P0 勘察校准（3 tasks）
  └─ P1 OnInput+OnSettled 骨架（10 tasks）
       ├─ P2 Response 采集（5 tasks）
       │    └─ P3 多格式 Walker（4 tasks）
       │              └─ P4 Watchlist+重扫（8 tasks）
       │                           └─ P5 前端可视化+全套测试（13 tasks）
       └─ P4（P4 也直接依赖 P1）
```

### 关键规则（Top 10，全量见 spec.md 第 2 章）

- **BR-004**：`audit/` 包禁止 import `model`/`relay/common`（会成环，F-34 真实案例）
- **BR-005**：审计总开关关闭时 `RelayInfo.ContentSink == nil`，调用点做 nil 检查，零开销
- **BR-006**：sink channel 满时立即 drop，绝不阻塞 relay goroutine
- **BR-007**：任何将 drop/omit 的 segment，必须先提取 derived facts（urls/domains）再丢弃原文
- **BR-008**：默认 mode：system=preview/512B, user=full/16KB, assistant=full/16KB, tool_result=drop
- **BR-011**：watchlist 规则存主库独立表，增删改均 version++
- **BR-015**：audit sink 任何 error 必须用 `logger.LogWarn`（F-08：GORM LogLevel=Warn，CREATE TABLE 不打印）
- **INV-001**：relay 主链路 P99 不受审计影响（sink 全异步）
- **INV-004**：`cd relaykit && GOWORK=off go build ./...` 始终通过（relaykit 不引入 audit 依赖）
- **INV-005**：普通用户 GET /api/log/self 响应不含 `admin_info` 字段

### 3 个已知硬约束（实现期撞出，写代码前必读）

1. **audit → model 成环**（F-34）：`audit/` 只能 import `common`/`relaykit/dto`/`relaykit/types`；`service/audit_sink.go` 是唯一允许同时 import audit + model 的层
2. **GORM logger.Warn 不打印普通 SQL**（F-08）：不能用"日志里没有 CREATE TABLE"来判断 AutoMigrate 未执行；审计异常必须显式 `logger.LogWarn`
3. **`(m *Message) ParseContent()` 块注释陷阱**（F-22）：真实定义在 `relaykit/dto/openai_request.go:L543`，L733 在块注释内，grep 可能返回两处——以 L543 为准

---

## 4. 开工前初始化

1. 确认工作区干净：`git status --porcelain` → 空
2. 通读 `spec.md` 第 1、2 章（重点：2.3 节 7 条流程脚本）
3. 预读 `spec.md` 第 5.2 节真实场景测试矩阵——先知道完成标准，再开工
4. 打开 `tasks.csv`，找到 Task 1（P0 勘察校准，前置=无）
5. 运行基线：`make test`（全绿）+ `GOWORK=off go build ./...`（BUILD_OK）+ `cd relaykit && GOWORK=off go build ./...`（BUILD_OK）
6. 后端启动确认：`go run main.go &`（:3000），`curl -s http://localhost:3000/api/status` → JSON；测试完 kill 进程

---

## 5. 核心执行循环

```
WHILE 存在待开始或进行中的任务:
    1. 在 tasks.csv 找到下一条前置任务已完成的任务
    2. 读 spec.md 第 4 章对应 Task 详情（### Task N: 标题）
    3. 确认关联 BR/UF/INV/EVD；哪些行为不能变（INV）
    4. tasks.csv 状态更新为「进行中」
    5. 按三段式定位（spec.md §3.3）校验文件位置（symbol + rg anchor，行号只是 hint）
    6. 执行具体操作步骤
    7. 运行验证命令，输出保存到 evidence/
    8. 通过 → tasks.csv 状态「已完成」；失败 → 排障（最多主动修复 3 次）
    9. 仍失败 → 标记「已阻塞:{原因}」，继续不依赖该任务的后续任务
   10. Phase 最后一条任务（回归验证）通过后，输出 Phase summary 到 evidence/phase-N/summary.md
```

不要中途询问"是否继续"。除非所有剩余任务都被阻塞，否则持续推进。

---

## 6. 禁止事项

- **不得** import `model` 或 `relay/common` 在 `audit/` 包内（BR-004，成环）
- **不得** 向 `relaykit/` 添加任何 audit 依赖（BR-017，INV-004）
- **不得** 用阻塞 send 写 sink channel（BR-006，relay P99 影响）
- **不得** 依赖 GORM warn 日志判断 AutoMigrate 是否执行（F-08）
- **不得** 用 `ApiErrorStr`（不存在，F-12）；用 `ApiErrorMsg` 或 `ApiError`
- **不得** 直接 `import "encoding/json"`（AGENTS.md：用 `common.Marshal/Unmarshal`）
- **不得** 只实现组件/函数而不接线到真实入口——接线清单见 spec.md §2.3 各 UF 末尾
- **不得** 只跑单测就宣称完成——完成的唯一标准是 spec.md §5.2 真实场景全套测试

---

## 7. 排障顺序

1. 查 spec.md 第 4 章当前任务的**注意事项**
2. 查 spec.md 第 2 章关联 BR/UF/INV（尤其 INV-001/004/005）
3. import cycle → 检查分层约束（spec.md §3.1 分层图）
4. AutoMigrate 未生效 → 参考 Task 1 结论（F-35 疑点已闭环：需显式追加 `&LogContent{}`）
5. test 失败 → `make test` 完整输出定位，先修 FAIL 再继续
6. 最多主动修复 3 次，仍失败则阻塞并继续其他任务

---

## 8. 完成标准与汇报

所有任务「已完成」后：

1. 运行最终命令级验证（入场券）：
   ```bash
   make test
   GOWORK=off go build ./...
   cd relaykit && GOWORK=off go build ./...
   cd web && bun run typecheck && bun run build
   ```
2. **执行 spec.md §5.2 真实场景全套测试**：
   - 启动 `go run main.go`（:3000）+ `make dev-web`（:5173）
   - 按 §5.2 执行矩阵逐行回放（17 行，覆盖 UF-001～UF-007 主路径+失败分支）
   - 保存截图/console/API 样例到 evidence/UF-xxx/ 对应路径
3. 重跑校验脚本（证据闸门）：
   ```bash
   python3 /Users/nothing/.agents/skills/prd-workflow/scripts/validate_package.py docs/content-audit
   ```
   期望：0 FAIL（标记完成后脚本会检查 evidence 路径真实存在）
4. 对照 spec.md §2 章逐条核销 BR-001～BR-017 / UF-001～UF-009 / INV-001～INV-006
5. 对照 spec.md §5.4 专项检查清单自检（入口接线、交互反馈、隔离验证）
6. 输出最终总结：

```markdown
## 完成总结
- 完成范围：P0～P5，43/43 任务
- 修改文件：（列出）
- 通过的 BR/UF：BR-001～BR-017；UF-001～UF-007（5.2 矩阵 17/17 行通过）
- 未破坏的不变量：INV-001～INV-006
- Evidence：evidence/ 目录齐全（17 条截图 + API 样例 + 构建日志）
- 剩余风险：（无 / 具体描述）
```

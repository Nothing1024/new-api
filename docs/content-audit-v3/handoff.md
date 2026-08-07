# content-audit-v3 Handoff

本文件是可直接交给执行 Agent 的交付 Prompt。目标是在不破坏 v1/v2 业务不变量的前提下，把审计命中信息接入主用量日志视图。

> 使用方式：把本文件完整粘贴给执行 Agent，或让 Agent 开工前先读本文件。
> 本文件只做入口导航，所有规则、任务、验收细节以 `spec.md` 为准。

---

## 1. 目标（三行话）

1. **后端三列迁移**：`logs` 表增加 `AuditHitSeverity/AuditHitCount/AuditWLVersion` 三列；`UpdateLogAuditPointer` 改为 `UpdateLogAuditFields` 双写（JSON + 新列）；`GetAllLogs`/`GetUserLogs` 支持按 severity 过滤。
2. **列表增强**：主日志列表（`/usage-logs/common`）admin 可见 "Audit" 命中列；filter bar 新增 Audit Severity 下拉。
3. **详情分离**：details-dialog 为有命中的日志新增 Audit tab，billing 内容保留在 Billing tab。

---

## 2. 资料清单

| 资料 | 路径 | 规模 | 用途 |
|---|---|---|---|
| **Spec（唯一事实源）** | `spec.md` | 807 行 | 业务合同、技术方案、任务详情、验收协议 |
| **Tasks CSV（状态板）** | `tasks.csv` | 15 条 | 任务状态跟踪 |
| v2 spec（只读参考） | `../content-audit-v2/spec.md` | 1689 行 | v2 已有约束，不可破坏 |
| v1 spec（只读参考） | `../content-audit/spec.md` | 1665 行 | v1 已有约束，不可破坏 |

---

## 3. 开工上下文

### 当前代码状态

- v2 已完整交付，代码干净，`make test` 全绿
- v3 spec Stage 1+2 展开完毕，tasks.csv 全 pending

### 架构 Before / After（增量）

```
logs 表（model/log.go Log struct）:
  Before: Other string（JSON 含 admin_info.audit.hit_count）
  After:  + AuditHitSeverity varchar(8) index
          + AuditHitCount    int
          + AuditWLVersion   int

service/audit_sink.go:
  Before: updateAuditPointer(requestId, hitCount) → 只写 JSON
  After:  改调 UpdateLogAuditFields(requestId, hitCount, hitSeverity, wlVersion) → 双写

GET /api/log:
  Before: 12 params，无 audit 过滤
  After:  + audit_hit_severity query param（可选）

前端列表（common-logs-columns）:
  Before: buildDetailSegments 中文字 audit badge
  After:  + 独立 "Audit" 列（admin-only）

前端 filter bar（common-logs-filter-bar）:
  Before: 无 severity 筛选
  After:  + Audit Severity Select

前端 details-dialog:
  Before: AuditContentSection 内联 billing 流
  After:  Billing tab（默认）+ Audit tab（admin && hit_count>0）
```

### Phase 地图

```
P0（基线验证，2 tasks）
  ↓
P1（后端三列迁移 + sink 重写 + API 扩展，4 tasks）
  ↓
P2（前端 schema/类型 + 列表列 + 筛选条，4 tasks）
  ↓
P3（详情对话框 Tab 分离，2 tasks）
  ↓
P4（i18n + 真实场景验收，3 tasks）
```

---

## 4. 关键规则（Top 10，完整定义见 spec.md §2.1/§2.4）

| 规则 | 一句话 |
|---|---|
| **BR-301** | `Log` 加3列，GORM AutoMigrate，不用 bool `default:` tag |
| **BR-302** | `UpdateLogAuditFields` 双写：JSON（向后兼容）+ 新列 |
| **BR-303** | audit_sink 调用点改用 `UpdateLogAuditFields`，传 hitSeverity + wlVersion |
| **BR-304** | `GetAllLogs`/`GetUserLogs` 加 `auditHitSeverity` 参数；空值不过滤 |
| **BR-306** | Audit 列 admin-only；hit_count>0 显示 badge；否则显示 `-` |
| **BR-307** | filter bar Audit Severity Select → URL state `auditHitSeverity` |
| **BR-308** | Audit tab：admin && hit_count>0 时出现；Billing tab 内容不变 |
| **BR-309** | 所有新 UI 字符串通过 `t()`，7个 locale 全同步 |
| **INV-302** | `logs.other.admin_info.audit` JSON 路径不移除（向后兼容） |
| **INV-303** | `cd relaykit && GOWORK=off go build ./...` 始终通过 |

---

## 5. 开工前初始化

```bash
# 1. 确认工作区干净
git status --porcelain   # 期望空（或只有 docs/ 未跟踪文件）

# 2. 基线验证
make test
GOWORK=off go build ./...
cd relaykit && GOWORK=off go build ./...

# 3. 打开 tasks.csv，从 T-001 开始
```

---

## 6. 核心执行循环

```
WHILE tasks.csv 存在 pending 任务:
    1. 找前置任务全部 done 的下一条
    2. 读 spec.md §4 对应 Task 标题（### Task N:）
    3. 确认关联 BR/UF/INV
    4. tasks.csv 状态 → 进行中
    5. 按 spec §3.3 三段式定位表校验文件位置（symbol + rg anchor，行号是 hint）
    6. 执行操作步骤
    7. 运行验证命令，输出保存到 evidence/<phase>/
    8. 通过 → tasks.csv → 已完成；失败 → 排障（≤3 次）
    9. 仍失败 → 已阻塞:{原因}，继续不依赖该任务的后续
   10. Phase 末回归通过后输出 summary 到 evidence/phase-N/
```

**不要中途询问"是否继续"。** 除非所有剩余任务都被阻塞，否则持续推进。

---

## 7. 排障顺序

1. **AutoMigrate 失败** → 检查 `Log` struct 新列 gorm tag 是否有 bool `default:true`（禁止）
2. **UpdateLogAuditPointer 编译错误** → 确认已全仓替换为 `UpdateLogAuditFields`（`grep -rn UpdateLogAuditPointer --include=*.go .` → 0 命中）
3. **新列不写值** → 检查 `UpdateLogAuditFields` body 使用 `map` 而非 struct（struct Updates 跳过零值）
4. **GetAllLogs 过滤无效** → 检查 controller handler 是否解析 `audit_hit_severity` 并传入 model
5. **Audit 列非 admin 可见** → 检查 `useCommonLogsColumns(isAdmin)` 是否有 isAdmin guard
6. **URL state 无 auditHitSeverity** → 检查 `usageLogsSearchSchema` 是否已加字段
7. **Billing tab 内容丢失** → 确认 billing 内容移入 `<TabsContent value="billing">`，未被删除
8. **i18n missing** → `bun run i18n:sync` 列出 missing keys，逐条补全
9. `make test` FAIL → `make test 2>&1 | grep FAIL` 精确定位
10. 最多主动修复 3 次，仍失败则阻塞该任务继续其他

---

## 8. 禁止事项

- **不得**修改 `relaykit/` 包（INV-303）
- **不得**给 `Log` 新列加 bool `gorm:"default:true"` tag（AGENTS.md 禁止）
- **不得**在 `UpdateLogAuditFields` 中用 struct Updates（会跳过零值，必须用 map）
- **不得**移除 `logs.other.admin_info.audit` JSON 路径（INV-302）
- **不得**向普通用户暴露 audit 列/filter/tab（admin-only 硬约束）
- **不得**只跑 `make test` 就宣称完成——完成标准是 spec §5.2 真实场景矩阵7行全部通过并有 Evidence

---

## 9. 完成标准与汇报

全部 15 个任务 done 后，运行终验序列：

```bash
make test
GOWORK=off go build ./...
cd relaykit && GOWORK=off go build ./...
cd web && bun run typecheck && bun run build
bun run i18n:sync   # 期望 0 missing / 0 untranslated
```

然后逐条执行 **spec §5.2 真实场景矩阵**（7行），截图存入 `evidence/UF-301/`、`evidence/UF-302/`、`evidence/UF-303/`。

最后输出完成总结：

```markdown
## 完成总结
- 完成范围：P0～P4，15/15 任务
- 修改文件：（列出）
- 通过的 BR：BR-301～BR-309
- 通过的 UF：UF-301～UF-303（5.2 矩阵 7/7 行通过）
- 未破坏的不变量：INV-301～INV-305
- Evidence：docs/content-audit-v3/evidence/ 齐全
- 剩余风险：（无 / 具体描述）
```

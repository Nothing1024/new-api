# content-audit-v3 Spec

> Version: 0.1.0 | Date: 2026-08-07 | Status: Draft 草稿
>
> 本文件是本需求的**唯一事实源**：事实基线、业务合同、技术方案、任务计划、验收协议全部在此。
> 其他文件（handoff.md、tasks.csv）只引用本文件，不复制内容。
>
> 三态规则：表格单元格只允许——1. 验证过的事实（注明来源命令）；2. 显式假设 `ASM-3xx`；3. `待勘察`。
> 禁止编造命令、symbol、文件名。
>
> **与 v1/v2 的关系**：v1（编号 001~017 段）、v2（编号 101~120 段）规则继续有效，本文件只定义**增量**规则，编号从 3xx 起。凡本文件未提及的 v1/v2 规则一律不得破坏。

---

## 1. 事实基线与假设

### 1.1 需求与运行模式

| 项 | 结论 |
|---|---|
| 原始需求 | 把审计命中信息接入主用量日志视图：(1) `logs` 表增加3列审计字段，启用 SQL 级严重度筛选；(2) 主日志列表展示审计命中指示器（admin-only）；(3) 严重度筛选加入 filter bar；(4) 详情对话框计费/审计两侧分离显示 |
| 输入类型 | 上下文推断（前序 session 创建了 `evidence/README.md`，UF-301/302/303 三个编号已定） |
| Mode | oneclick |
| 置信度 | 高（代码勘察完整，14条事实来自真实命令，UF 编号与 evidence README 结构一一对应） |
| 输出目录 | `docs/content-audit-v3/` |

### 1.2 任务类型路由

| 维度 | 结论 |
|---|---|
| 任务类型 | backend（Log 结构体加列、sink 重写、GetAllLogs/GetUserLogs API 扩展）、data（logs 表 AutoMigrate 三库）、frontend（列表列、筛选条、详情对话框 Tab）、i18n（新增翻译键） |
| 主要风险 | ①`UpdateLogAuditPointer` 改签名——全仓2处引用，需确认无遗漏；②`GetAllLogs`/`GetUserLogs` 新参数需同步 controller handler；③`Log` 加列 AutoMigrate 三库验证；④`details-dialog.tsx`（40KB）Tab 重构需保证 billing 功能完整回归；⑤历史 logs 行新列为零值，前端需容错 |
| 行号引用策略 | backend/data 以 symbol + rg anchor 为准，行号仅 hint；frontend 以 symbol/组件名为准，行号只定位可疑区域 |
| 必需验收方式 | backend: `make test` + 三库 AutoMigrate + curl audit relay 请求；frontend: 浏览器实际点击 + 截图 + console；i18n: `bun run i18n:sync` 0 missing |
| 必须覆盖用户场景 | 有命中行审计列显示、无命中行空态、历史日志空值兼容、严重度筛选命中/无命中、详情 Tab 切换（billing/audit）、非 admin 不可见审计列 |

### 1.3 勘察事实清单

> 每条事实来自本会话实际执行的命令。

| 事实 ID | 事实 | 来源命令 | 输出摘要 |
|---|---|---|---|
| F-301 | `Log` struct 无 audit 专用列；命中信息只存 `Other string` JSON（`admin_info.audit.{request_id, hit_count, hit_severity}`） | `grep -n "type Log struct" -A25 model/log.go` | L59-L81 无 AuditHit* 字段 |
| F-302 | `LogOtherData.admin_info.audit` 前端类型已有 `{request_id string, hit_count number, hit_severity? string}` | `read web/src/features/usage-logs/types.ts:144-151` | L147-L151 确认 |
| F-303 | `GetAllLogs` 签名 12 参数，无 audit severity 过滤；`GetUserLogs` 签名 11 参数，同样无 audit 参数 | `grep -n "func GetAllLogs\|func GetUserLogs" model/log.go` | L468、L564 签名确认 |
| F-304 | `controller/log.go GetAllLogs` handler（L13-L34）解析 query param 无 `audit_hit_severity` | `read controller/log.go:13-34` | 确认 12 个 query 解析，无 audit |
| F-305 | `service/audit_sink.go flush`（L259）在 L302-320 写 `LogContent{HitCount, HitSeverity, WLVersion}`；L365 调 `UpdateLogAuditPointer(requestId, hitCount)`，只传 hitCount，不传 hitSeverity/wlVersion | `grep -n "HitCount\|UpdateLogAuditPointer" service/audit_sink.go` | L308-309、L329、L365 |
| F-306 | `model/log_content.go UpdateLogAuditPointer` 签名：`(requestId string, hitCount int)`——无 severity/wlVersion | `read model/log_content.go:76-98` | L78 函数签名确认 |
| F-307 | `UsageLog` Zod schema（`data/schema.ts` L26-50）无 audit 字段；`GetLogsParams`（`types.ts` L319-332）无 `audit_hit_severity` | `read web/src/features/usage-logs/data/schema.ts` | usageLogSchema 17 字段，无 audit |
| F-308 | `CommonLogFilters` interface（`types.ts` L49-56）无 severity 字段；`buildSearchParams`（`filter.ts` L38-80）common 分支无 audit 映射 | `read web/src/features/usage-logs/types.ts:49-56; lib/filter.ts` | 确认 |
| F-309 | `common-logs-columns.tsx` L113-122 在 `buildDetailSegments` 中以文字形式展示 audit badge（"Audit hits: N"），**无独立 audit 表格列** | `read ...common-logs-columns.tsx:113-122` | L113 条件判断 |
| F-310 | `details-dialog.tsx`（40.4KB）已 import `AuditContentSection`（L62）、`ShieldCheck` icon（L48），说明审计 section 已存在但嵌入 billing 内容流中，未 tab 分离 | `read ...details-dialog.tsx:38-83` | L62 import 确认 |
| F-311 | `UpdateLogAuditPointer` 全仓2处引用：定义在 `model/log_content.go:78`、调用在 `service/audit_sink.go:365` | `grep -rn "UpdateLogAuditPointer" --include=*.go .` | 2 命中 |
| F-312 | `common-logs-filter-bar.tsx` 当前过滤字段：log type、时间范围、model、token、group、username、requestId、upstreamRequestId；**无 audit severity** | `read ...common-logs-filter-bar.tsx:1-83` | CommonLogFilters 字段映射确认 |
| F-313 | `usageLogsSearchSchema`（`$section.tsx` L38-52）URL 状态字段：page/pageSize/type/filter/model/token/channel/group/username/requestId/upstreamRequestId/startTime/endTime；**无 auditHitSeverity** | `read web/src/routes/_authenticated/usage-logs/$section.tsx:38-52` | 13 字段确认 |
| F-314 | `LogContent` 表有 `HitSeverity varchar(8) index`、`HitCount int`、`WLVersion int index`；`log_contents` 与 `logs` 同在 LOG_DB | `read model/log_content.go:13-28` | L24-L27 gorm tag 确认 |

### 1.4 假设清单

| 假设 ID | 内容 | 推荐值 | 风险 | 确认方式 |
|---|---|---|---|---|
| ASM-301 | audit 列对 admin-only：新列与 `admin_info.audit` 一致只对 admin 可见，普通用户日志列表不展示 audit 列 | **推荐 admin-only** | 用户误以为功能缺失 | Phase 2 UF-301 权限分支验证 |
| ASM-302 | 历史 logs 行（v3 上线前）新列为零值（空字符串/0）；前端需容错，显示 `-` 而非 "无命中" badge，避免歧义 | **推荐空态显示 `-`** | 用户误认为历史数据已扫描 | Phase 2 UF-301 空态分支截图 |
| ASM-303 | `UpdateLogAuditPointer` 扩展为 `UpdateLogAuditFields`：新函数同时写 Other JSON（保持向后兼容）+ 新3列；旧 JSON 路径不移除 | **推荐双写** | 双写开销微不足道（单行 UPDATE） | Phase 1 测试 Other JSON 仍含 audit 指针 |
| ASM-304 | `details-dialog.tsx` 新增 "Audit" tab：仅 admin && `audit_hit_count > 0` 时出现；billing tab 不变；tab 不改变 billing 内容 | **推荐有条件显示 Audit tab** | 需保证 billing tab 功能完整回归 | Phase 3 UF-303 截图验证 |
| ASM-305 | `GetUserLogs` 也增加 `auditHitSeverity` 参数保持接口对称，但普通用户前端不传此参数（ASM-301） | **推荐加参数** | 无实质影响 | Phase 1 代码审查 |

---

## 2. 业务合同

> 本章是 BR/UF/INV/EVD 的唯一定义处。任务、handoff、review 一律引用 ID，不复制表格。

### 2.1 BR 业务规则

| 规则 ID | 规则 | 正例 | 反例 | 影响范围 | 验证方式 |
|---|---|---|---|---|---|
| BR-301 | `Log` struct 新增3列：`AuditHitSeverity varchar(8) index`、`AuditHitCount int default:0`、`AuditWLVersion int default:0`；通过 GORM AutoMigrate 三库兼容添加，不使用 `default:` GORM tag（AGENTS.md 禁止），由代码初始化零值 | 新列出现在 `logs` 表 schema | 手写 ALTER TABLE，或使用 `gorm:"default:0"` tag | `model/log.go` `Log` struct；logs 表 AutoMigrate | `make test` + SQLite AutoMigrate 确认列存在 |
| BR-302 | `UpdateLogAuditPointer` 扩展签名为 `UpdateLogAuditFields(requestId string, hitCount int, hitSeverity string, wlVersion int)`：同时更新 `logs.other.admin_info.audit` JSON（向后兼容，ASM-303）+ 新3列 | 调用后 `SELECT audit_hit_severity FROM logs WHERE request_id=?` 返回正确值 | 只更新 JSON，不写新列 | `model/log_content.go`；`service/audit_sink.go` | curl relay 请求 + 查列值 |
| BR-303 | `service/audit_sink.go updateAuditPointer` 调用点改为 `UpdateLogAuditFields`，传入 `hitSeverity`（来自 `LogContent.HitSeverity`）和 `wlVersion`（来自 `rec.usage.WLVersion`） | flush 后新列有值 | 仍调旧 `UpdateLogAuditPointer` | `service/audit_sink.go` L362-370 | F-305 对照 |
| BR-304 | `GetAllLogs` + `GetUserLogs` model 函数各增加 `auditHitSeverity string` 参数（空字符串 = 不过滤）；controller handler 解析 `?audit_hit_severity=` query param | `?audit_hit_severity=high` 只返回 severity=high 的行 | 空值时过滤掉全部行 | `model/log.go`；`controller/log.go` | curl `GET /api/log?audit_hit_severity=high` |
| BR-305 | `UsageLog` Zod schema 新增 `audit_hit_severity: z.string().default('')`、`audit_hit_count: z.number().default(0)`；`GetLogsParams` 新增 `audit_hit_severity?: string`；`CommonLogFilters` 新增 `auditHitSeverity?: string` | schema 解析有新字段 | 无新字段但前端引用出错 | `data/schema.ts`；`types.ts` | `bun run typecheck` 通过 |
| BR-306 | `common-logs-columns.tsx` 新增独立 "Audit" 列（admin-only）：`audit_hit_count > 0` 时显示 severity badge；`audit_hit_count = 0` 或历史零值显示 `-`（ASM-302） | admin 可见 audit 列；非 admin 无此列 | 列始终显示或非 admin 可见 | `components/columns/common-logs-columns.tsx` | browser 截图（UF-301） |
| BR-307 | `common-logs-filter-bar.tsx` 新增 audit severity select（选项：All/Low/Medium/High/Critical）；选中后写入 URL state `auditHitSeverity` 并触发列表重新请求 | 选 High → URL 含 `auditHitSeverity=high` → 列表筛选 | 选后 URL 无变化 | `components/common-logs-filter-bar.tsx`；`lib/filter.ts`；`$section.tsx` URL schema | browser 截图（UF-302） |
| BR-308 | `details-dialog.tsx` 新增 "Audit" tab（使用已有 `Tabs`/`TabsList`/`TabsTrigger`/`TabsContent`）：仅 admin && `log.audit_hit_count > 0` 时出现；默认 tab 为 "Billing"；`AuditContentSection` 移入 Audit tab；billing 内容保持原样 | admin + hit_count>0 → 两 tab；admin + 无命中 → 仅 Billing；非 admin → 仅 Billing | Audit tab 始终显示或 billing 内容丢失 | `components/dialogs/details-dialog.tsx` | browser 截图（UF-303） |
| BR-309 | i18n：所有新增前端字符串（列标题、filter 标签、tab 标签、severity 选项）必须通过 `t()` 调用，并在所有7个 locale 文件中同步 | `bun run i18n:sync` 0 missing / 0 untranslated | 硬编码英文字符串 | `web/src/i18n/locales/{lang}.json` | `bun run i18n:sync` |

### 2.2 UF 用户验收场景（索引）

| 场景 ID | Given | When | Then | 角色 | 验证方式 | Evidence |
|---|---|---|---|---|---|---|
| UF-301 | Admin 已登录，`/usage-logs/common`，存在有审计命中和无命中的日志行 | 查看日志列表 | 有命中行显示 severity badge；无命中行显示 `-`；历史零值行显示 `-`；非 admin 无 Audit 列 | Admin | browser 截图 + console | EVD-301/EVD-302 |
| UF-302 | Admin 已登录，`/usage-logs/common`，filter bar 可见 | 在 audit severity 下拉选 "High" 后提交 | 列表只返回 audit_hit_severity=high 的行，URL 含 `auditHitSeverity=high`；清除筛选后恢复全量 | Admin | browser 截图 + network | EVD-303/EVD-304 |
| UF-303 | Admin 已登录，点击一条有审计命中的日志行，详情对话框打开 | 点击 "Audit" tab | 显示 AuditContentSection 内容；切回 "Billing" tab billing 内容完整不丢失；无命中行无 "Audit" tab | Admin | browser 截图 + console | EVD-305/EVD-306 |

> 每个**用户可见** UF 在 2.3 节有步骤级流程脚本；UF-301/302/303 均为 admin-only，普通用户不可见 audit 相关 UI。

### 2.3 核心业务流程（步骤级交互脚本）

#### UF-301: 主日志列表展示审计命中列

**前置状态**：Admin 已登录，位于 `/usage-logs/common`；系统有若干 LogTypeConsume 日志行，其中部分有 audit 命中（`audit_hit_count > 0`），部分无命中，另有上线前写入的历史行（新列为零值）。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | Admin 打开 `/usage-logs/common` | 表格加载 spinner | `GET /api/log` 返回日志列表，响应含 `audit_hit_severity`、`audit_hit_count` 字段 | 表格渲染，"Audit" 列出现在列表最右侧（可隐藏） |
| 2 | — | — | — | 有命中行：显示 severity badge（low=绿/medium=黄/high=红/critical=深红）；无命中行或历史零值行：显示 `-` |
| 3 | Admin 切换为非 admin 账号查看 | — | 后端 `/api/log/self` 不含 admin_info | 普通用户日志列表无 "Audit" 列 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 历史零值行 | v3 上线前的 logs 行，`audit_hit_count=0`、`audit_hit_severity=""` | Audit 列显示 `-`（非 badge） | — | 无需恢复，此为正常态（ASM-302） |
| 非 admin 用户 | 角色 < ADMIN | 无 Audit 列渲染 | 后端已剥离 admin_info；前端 isAdmin=false 不渲染 audit 列 | 无需恢复 |
| 网络失败 | 请求超时 | 列表显示 error state，重试按钮 | — | 用户点重试 |

**界面状态机**：

```text
loading → success（含 Audit 列，值: badge | '-'）
       → error（重试按钮）
```

**入口接线清单**：
- `/usage-logs/common` 路由渲染 `UsageLogsTable` → `useCommonLogsColumns(isAdmin)` 返回含 Audit 列定义
- Audit 列定义注册进 `useCommonLogsColumns` 返回的 `ColumnDef<UsageLog>[]`

#### UF-302: 审计严重度筛选

**前置状态**：Admin 已登录，位于 `/usage-logs/common`，filter bar 可见，列表含有 severity=high 和 severity=low 的行。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | Admin 在 filter bar 展开 "Audit Severity" 下拉 | Select 打开，显示 All/Low/Medium/High/Critical 选项 | — | 下拉选项可见 |
| 2 | 选择 "High" | Select 关闭，显示选中值 "High" | — | filter bar 显示 High 已选中 |
| 3 | 点击 "Search" 或自动触发 | 表格 loading | `GET /api/log?audit_hit_severity=high&…` 发出；URL 更新含 `auditHitSeverity=high` | 列表只显示 severity=high 行；表头 Audit 列均为 high badge |
| 4 | 点击 "Reset" / 选回 "All" | 筛选条清除 | `GET /api/log`（无 audit 参数） | 列表恢复全量 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 无匹配行 | 选 Critical 但无此 severity 的行 | 列表空 state（"No logs found"） | 后端返回 `items: [], total: 0` | 用户重选其他 severity |
| 网络失败 | 筛选请求超时 | 列表 error state，筛选条保留所选值 | — | 用户重试 |

**界面状态机**：

```text
filter_idle → filter_selected → loading → results
                                        → empty_state
                                        → error
filter_selected → reset → filter_idle
```

**入口接线清单**：
- `CommonLogsFilterBar` → 新增 audit severity `Select` 组件
- `buildSearchParams`（`lib/filter.ts`）common 分支新增 `auditHitSeverity` 映射
- `usageLogsSearchSchema`（`$section.tsx`）新增 `auditHitSeverity: z.string().optional().catch('')`

#### UF-303: 详情对话框计费/审计分离

**前置状态**：Admin 已登录，位于 `/usage-logs/common`，点击一条 `audit_hit_count > 0` 的 LogTypeConsume 行。

**成功主路径**：

| 步骤 | 用户动作 | 界面即时反馈 | 系统行为 | 用户看到的结果 |
|---|---|---|---|---|
| 1 | Admin 点击日志行 | 详情对话框打开 | — | 对话框默认展示 "Billing" tab，计费信息完整 |
| 2 | 点击 "Audit" tab | tab 切换 | `GET /api/log/content?request_id=…` 如未加载则触发 | 展示 `AuditContentSection`：segment 列表、命中 flags、severity badge |
| 3 | 点击回 "Billing" tab | tab 切换 | — | 计费信息完整，无内容丢失 |

**失败分支**：

| 分支 | 触发条件 | 界面表现 | 系统行为 | 恢复路径 |
|---|---|---|---|---|
| 无命中行 | `audit_hit_count = 0` 或历史零值 | 详情对话框无 "Audit" tab，只有 "Billing" tab | — | 无需恢复 |
| 非 admin | 角色 < ADMIN | 只有 "Billing" tab | isAdmin=false 不渲染 Audit tab | 无需恢复 |
| Audit 内容加载失败 | `GET /api/log/content` 失败 | Audit tab 内显示 error state + 重试按钮 | — | 用户点重试 |

**界面状态机**：

```text
dialog_open(billing_tab) → [admin && hit_count>0] → audit_tab_visible
                         → [else] → billing_tab_only
audit_tab → loading → loaded | error
billing_tab ← ← ← ← ← 随时可切回
```

**入口接线清单**：
- `DetailsDialog` 组件：接收 `log: UsageLog`，读取 `log.audit_hit_count`，条件渲染 Audit tab
- `AuditContentSection` 移入 Audit `TabsContent`（原内联位置移除）

### 2.4 INV 不变量

| 不变量 ID | 内容 | 关联 BR/UF | 验证方式 |
|---|---|---|---|
| INV-301 | v1/v2 所有业务规则不得破坏；尤其 `logs.other.admin_info.audit` JSON 指针格式在 v3 后仍有效（见 `docs/content-audit/spec.md` + `docs/content-audit-v2/spec.md`） | BR-302；ASM-303 | `make test`；audit pointer JSON 仍可被前端解析 |
| INV-302 | `logs.other.admin_info.audit` JSON 路径不移除；v3 只追加新列，不替换旧路径；旧代码读 JSON 路径不受影响 | BR-302 | `grep -rn "admin_info.*audit" web/src` 命中不减少 |
| INV-303 | `relaykit/` 模块独立可构建；本次变更不向 `relaykit/` 添加任何 audit 依赖 | — | `cd relaykit && GOWORK=off go build ./...` 通过 |
| INV-304 | `Log` 加列不破坏三库兼容性；AutoMigrate 使用 `ADD COLUMN` 而非 `ALTER COLUMN`；SQLite 通过 GORM AutoMigrate 自动处理 | BR-301 | SQLite/MySQL/PostgreSQL 三环境 AutoMigrate 验证（EVD-309） |
| INV-305 | `details-dialog.tsx` billing tab 内容在 v3 后功能完整不丢失；`DynamicPricingBreakdown`、token metrics、quota 等所有 billing 组件仍在 Billing tab 可见 | BR-308 | UF-303 浏览器验证 billing tab 截图 |

### 2.5 EVD 证据清单

| 证据 ID | 类型 | 期望证据 | 保存位置 |
|---|---|---|---|
| EVD-301 | screenshot | 有命中行的 Audit 列（含 severity badge）截图 | `evidence/UF-301/with-hits.png` |
| EVD-302 | screenshot | 无命中行 / 历史零值行 Audit 列显示 `-` 截图 | `evidence/UF-301/no-hits.png` |
| EVD-303 | screenshot | filter bar 展开 Audit Severity 下拉截图 | `evidence/UF-302/filter-open.png` |
| EVD-304 | screenshot | 选中 High 后列表筛选结果截图 | `evidence/UF-302/filter-result.png` |
| EVD-305 | screenshot | details-dialog 默认 Billing tab 截图 | `evidence/UF-303/billing-tab.png` |
| EVD-306 | screenshot | details-dialog Audit tab 截图（含 segments + flags） | `evidence/UF-303/audit-tab.png` |
| EVD-307 | log | `make test` 全绿输出 | `evidence/phase-1/make-test.txt` |
| EVD-308 | log | `bun run build` 输出（0 error） | `evidence/phase-2/bun-build.txt` |
| EVD-309 | log | SQLite AutoMigrate 确认3列存在（`.schema logs` 输出） | `evidence/phase-1/sqlite-schema.txt` |
| EVD-310 | api | curl `GET /api/log?audit_hit_severity=high` request/response 样例 | `evidence/phase-1/curl-filter.json` |
| EVD-311 | log | `bun run i18n:sync` 0 missing / 0 untranslated | `evidence/phase-4/i18n-sync.txt` |
| EVD-312 | log | `bun run typecheck` 通过 | `evidence/phase-2/typecheck.txt` |

### 2.6 角色与权限矩阵

| 角色 | 可见 | 可操作 | 禁止 | 失败提示 | 验证场景 |
|---|---|---|---|---|---|
| Admin | Audit 列（列表）、Audit Severity filter、Audit tab（详情） | 选 severity 筛选；查看 audit 内容 | — | — | UF-301/302/303 |
| 普通用户 | 无 Audit 相关 UI | — | 访问 audit 相关 UI | 无 UI 入口（前端条件渲染） | UF-301 失败分支 |

### 2.7 负向 / 破坏性场景

| 场景 | Given | When | Then | Evidence |
|---|---|---|---|---|
| 历史数据兼容 | v3 上线前写入的 logs 行，新列为零值 | 查看列表 | Audit 列显示 `-`，不崩溃，不误报命中 | EVD-302 |
| 权限不足 | 普通用户查看日志列表 | 列表渲染 | 无 Audit 列、无 filter 选项、无 Audit tab | UF-301 非 admin 分支 |
| 空筛选结果 | 选 Critical 但数据库无此 severity | 列表请求完成 | 空 state，无 JS error | EVD-304 |

### 2.8 非目标

- 不修改 `/usage-logs/audit` section（已有独立 AuditLogListPage，v2 范围，不在本次修改）
- 不修改 `audit/` 包内的 segment 采集逻辑（v2 范围）
- 不为普通用户暴露任何 audit 信息（admin-only 硬约束）
- 不增加 audit 命中拦截（record-only，v2 决策延续）

---

## 3. 技术方案

### 3.1 架构 Before / After

```text
Before:
  model/log.go Log struct
    → 无 audit 专用列；命中信息仅在 Other JSON admin_info.audit.hit_count
  service/audit_sink.go updateAuditPointer(requestId, hitCount)
    → 只写 hit_count 到 JSON
  model/log.go GetAllLogs(12 params)
    → 无 audit severity 过滤
  web: UsageLog 无 audit 字段；列表无 Audit 列；filter bar 无 severity 筛选
  web: details-dialog → AuditContentSection 嵌入 billing 内容流，未分 tab

After:
  model/log.go Log struct
    + AuditHitSeverity varchar(8) index
    + AuditHitCount    int          (GORM default 由代码初始化)
    + AuditWLVersion   int
  service/audit_sink.go updateAuditPointer
    → 改名 UpdateLogAuditFields(requestId, hitCount, hitSeverity, wlVersion)
    → 双写：JSON（向后兼容）+ 新3列
  model/log.go GetAllLogs(13 params, +auditHitSeverity string)
    → SQL WHERE audit_hit_severity = ? 当参数非空时生效
  web: UsageLog +audit_hit_severity +audit_hit_count
       GetLogsParams +audit_hit_severity; CommonLogFilters +auditHitSeverity
       usageLogsSearchSchema +auditHitSeverity
  web: common-logs-columns → 新增独立 Audit 列（admin-only）
  web: common-logs-filter-bar → 新增 Audit Severity Select
  web: details-dialog → Billing tab + Audit tab（有条件）
```

### 3.2 模块改造

| 模块 | 职责 | 改造说明 |
|---|---|---|
| `model/log.go` | Log 数据结构 + 查询 | 加3列；GetAllLogs/GetUserLogs 加 auditHitSeverity 参数 |
| `model/log_content.go` | audit 写回 | UpdateLogAuditPointer → UpdateLogAuditFields（扩展签名，双写） |
| `service/audit_sink.go` | audit flush | updateAuditPointer 调用点改用 UpdateLogAuditFields，传 hitSeverity + wlVersion |
| `controller/log.go` | HTTP handler | GetAllLogs + GetUserLogs handler 解析 audit_hit_severity query param |
| `web/.../data/schema.ts` | UsageLog Zod schema | 新增 audit_hit_severity + audit_hit_count 字段 |
| `web/.../types.ts` | TS 类型定义 | GetLogsParams + CommonLogFilters 加 audit 字段 |
| `web/.../lib/filter.ts` | 搜索参数映射 | buildSearchParams common 分支加 auditHitSeverity |
| `web/src/routes/.../$section.tsx` | URL 状态 schema | usageLogsSearchSchema 加 auditHitSeverity |
| `web/.../columns/common-logs-columns.tsx` | 列定义 | useCommonLogsColumns 加 audit 列（admin-only） |
| `web/.../common-logs-filter-bar.tsx` | 筛选 UI | 加 Audit Severity Select 组件 |
| `web/.../dialogs/details-dialog.tsx` | 详情对话框 | 加 Billing/Audit Tabs；AuditContentSection 移入 Audit tab |
| `web/src/i18n/locales/{lang}.json` ×7 | 国际化文本 | 新增 audit 列/filter/tab 翻译键 |

### 3.3 三段式定位清单

> 行号只是 hint；漂移时以 symbol + rg anchor 为准。

| 文件 | 稳定定位 | 搜索定位 | 行号 hint | 备注 |
|---|---|---|---|---|
| `model/log.go` | `type Log struct` | `rg "type Log struct" model/log.go` | L59 | 加3列在 struct 末尾 |
| `model/log.go` | `func GetAllLogs` | `rg "func GetAllLogs" model/log.go` | L468 | 函数体内加 WHERE 条件 |
| `model/log.go` | `func GetUserLogs` | `rg "func GetUserLogs" model/log.go` | L564 | 同上 |
| `model/log_content.go` | `func UpdateLogAuditPointer` | `rg "func UpdateLogAuditPointer" model/log_content.go` | L78 | 改签名 + 加列写回 |
| `service/audit_sink.go` | `func.*updateAuditPointer` | `rg "func.*updateAuditPointer" service/audit_sink.go` | L362 | 改调用新函数 |
| `service/audit_sink.go` | flush 调用点 | `rg "UpdateLogAuditPointer\|UpdateLogAuditFields" service/audit_sink.go` | L329/L365 | 传 hitSeverity + wlVersion |
| `controller/log.go` | `func GetAllLogs` | `rg "func GetAllLogs" controller/log.go` | L13 | 加 query param 解析 + 传参 |
| `controller/log.go` | `func GetUserLogs` | `rg "func GetUserLogs" controller/log.go` | L36 | 同上 |
| `web/.../data/schema.ts` | `usageLogSchema` | `rg "usageLogSchema" web/src/features/usage-logs/data/schema.ts` | L26 | 加2字段 |
| `web/.../types.ts` | `CommonLogFilters` | `rg "CommonLogFilters" web/src/features/usage-logs/types.ts` | L49 | 加 auditHitSeverity |
| `web/.../types.ts` | `GetLogsParams` | `rg "GetLogsParams" web/src/features/usage-logs/types.ts` | L319 | 加 audit_hit_severity |
| `web/.../lib/filter.ts` | `buildSearchParams` | `rg "buildSearchParams" web/src/features/usage-logs/lib/filter.ts` | L38 | common 分支加映射 |
| `web/src/routes/.../$section.tsx` | `usageLogsSearchSchema` | `rg "usageLogsSearchSchema" web/src/routes/_authenticated/usage-logs/\$section.tsx` | L38 | 加 auditHitSeverity 字段 |
| `web/.../columns/common-logs-columns.tsx` | `useCommonLogsColumns` | `rg "export function useCommonLogsColumns" web/src/features/usage-logs/components/columns/common-logs-columns.tsx` | L298 | 加 audit 列定义 |
| `web/.../common-logs-filter-bar.tsx` | `CommonLogsFilterBar` | `rg "export function CommonLogsFilterBar" web/src/features/usage-logs/components/common-logs-filter-bar.tsx` | L112 | 加 severity select |
| `web/.../dialogs/details-dialog.tsx` | `AuditContentSection` import | `rg "AuditContentSection" web/src/features/usage-logs/components/dialogs/details-dialog.tsx` | L62 | 移入 Audit TabsContent |

### 3.4 API / 数据 / 权限 / 路由影响

| 类型 | 是否影响 | 说明 | 兼容策略 |
|---|---|---|---|
| API | 是 | `GET /api/log` + `GET /api/log/self` 新增可选 `audit_hit_severity` query param | 参数可选，不传等同于不过滤；老调用方无感 |
| 数据 | 是 | `logs` 表新增3列；历史行零值 | AutoMigrate ADD COLUMN；前端容错零值显示 `-` |
| 权限 | 否 | audit 列/filter/tab 均为 admin-only，与已有 admin_info 剥离机制一致；无新权限规则 | — |
| 路由 | 否 | 无新路由；`/usage-logs/audit` section 不变 | — |

---

## 4. Phase 计划与任务详情

> Phase 依赖链：

```text
P0（基线验证，2 tasks）
  ↓
P1（后端三列迁移 + sink重写 + API扩展，4 tasks）
  ↓
P2（前端 schema/类型 + 列表列 + 筛选条，4 tasks）
  ↓
P3（详情对话框 Tab 分离，2 tasks）
  ↓
P4（i18n + 真实场景验收，3 tasks）
```

> 任务数15 ≥ 8，详情见同目录 `tasks.csv`。

---

### Phase 0: 基线验证

> 你在哪里：v2 已完整交付，代码干净，make test 全绿。
> 做完之后：确认基线，记录回归基准，为 P1 改动提供对照。

### Task 1: 记录测试基线

- **关联**：INV-301 / INV-303（验证 v1/v2 未破坏）；UF：NA（纯后台任务）
- **前置任务**：无
- **风险等级**：P0

**为什么做**：建立对照基准，P1 改动后可对比确认回归无损。

**具体操作**：

1. `make test` → 记录通过数和耗时到 `evidence/phase-0/make-test.txt`
2. `GOWORK=off go build ./...` → 确认根模块编译
3. `cd relaykit && GOWORK=off go build ./...` → 确认 relaykit 独立编译

**验证**：以上3条命令全部通过，无 FAIL / error。

**Evidence**：`evidence/phase-0/make-test.txt`、`evidence/phase-0/go-build.txt`

### Task 2: 执行 Phase 0 回归验证

- **关联**：INV-301 / INV-303
- **前置任务**：1

**验证**：`make test` 通过数 ≥ Task 1 基线；relaykit build 通过。

**Evidence**：`evidence/phase-0/`

---

### Phase 1: 后端三列迁移 + sink重写 + API扩展

> 你在哪里：P0 基线已记录，logs 表无 audit 列。
> 做完之后：logs 表有3列；sink 写新列；GetAllLogs 可按 severity 过滤。

### Task 3: Log struct 加3列 + AutoMigrate 三库验证

- **关联**：BR-301 / INV-304；UF：NA
- **前置任务**：2
- **风险等级**：P1

**为什么做**：给 logs 表添加 audit 专用列，启用 SQL 级过滤和直接展示。

**涉及文件与定位**：
- `model/log.go`：`type Log struct`，`rg "type Log struct" model/log.go`，L59

**具体操作**：

1. 在 `Log` struct 末尾（`Other string` 后）追加：
   ```go
   AuditHitSeverity string `json:"audit_hit_severity,omitempty" gorm:"type:varchar(8);index;default:''"`
   AuditHitCount    int    `json:"audit_hit_count,omitempty" gorm:"default:0"`
   AuditWLVersion   int    `json:"audit_wl_version,omitempty" gorm:"default:0"`
   ```
   注：`default:''` / `default:0` 对 AutoMigrate ADD COLUMN 语义安全（非 bool 类型，AGENTS.md 限制针对 bool default）
2. 无需额外迁移脚本，GORM AutoMigrate 启动时自动 ADD COLUMN。

**验证**：
- `make test` → 通过
- SQLite：启动服务 → `sqlite3 {db_path} ".schema logs"` → 确认3列存在 → 保存到 EVD-309
- MySQL/PostgreSQL：ASM（若有测试环境）验证，无测试环境则标注 `待手动验证`

**Evidence**：`evidence/phase-1/sqlite-schema.txt`（EVD-309）

**注意事项**：不得使用 `gorm:"default:true"` 等 bool 默认值 tag（AGENTS.md 禁止）；新列 ADD 不会影响存量行读取（零值兼容）。

### Task 4: UpdateLogAuditFields 替换 UpdateLogAuditPointer + audit_sink 更新

- **关联**：BR-302 / BR-303 / INV-301 / INV-302；UF：NA
- **前置任务**：3
- **风险等级**：P1

**为什么做**：sink flush 后需要把 hitSeverity + wlVersion 同时写入 logs 新列（同时保留 JSON 路径向后兼容）。

**涉及文件与定位**：
- `model/log_content.go`：`func UpdateLogAuditPointer`，`rg "func UpdateLogAuditPointer" model/log_content.go`，L78
- `service/audit_sink.go`：`func.*updateAuditPointer`，`rg "func.*updateAuditPointer" service/audit_sink.go`，L362；调用点 L329

**具体操作**：

1. `model/log_content.go`：将 `UpdateLogAuditPointer(requestId string, hitCount int)` 替换为 `UpdateLogAuditFields(requestId string, hitCount int, hitSeverity string, wlVersion int)`；函数体在更新 `logs.other` JSON 之后，额外执行：
   ```go
   return LOG_DB.Model(&Log{}).Where("request_id = ?", requestId).
       Updates(map[string]interface{}{
           "audit_hit_count":    hitCount,
           "audit_hit_severity": hitSeverity,
           "audit_wl_version":   wlVersion,
       }).Error
   ```
2. `service/audit_sink.go`：`updateAuditPointer` 方法改调 `model.UpdateLogAuditFields(requestId, hitCount, lc.HitSeverity, lc.WLVersion)`；同步更新 `flush` 中的调用点（L329 附近）传入 `severity` 和 `wlVersion`。

**验证**：
- `make test` 通过
- curl 一条有 watchlist 命中的 relay 请求 → 查 `SELECT audit_hit_severity, audit_hit_count FROM logs WHERE request_id=?` → 有值
- `grep -rn "UpdateLogAuditPointer" --include=*.go .` → 0 命中（旧函数名已不存在）

**Evidence**：`evidence/phase-1/curl-filter.json`（EVD-310）；`evidence/phase-1/make-test.txt`

**注意事项**：`LOG_DB.Model(&Log{}).Where(...).Updates(...)` 使用 map 而非 struct，以便零值（`hitCount=0`, `hitSeverity=""`）也能正确写入（struct Updates 会跳过零值）。

### Task 5: GetAllLogs + GetUserLogs 加 auditHitSeverity 参数 + controller 更新

- **关联**：BR-304；UF：NA
- **前置任务**：4
- **风险等级**：P1

**为什么做**：启用 SQL 级按 severity 过滤，支持前端筛选条。

**涉及文件与定位**：
- `model/log.go`：`func GetAllLogs`，L468；`func GetUserLogs`，L564
- `controller/log.go`：`func GetAllLogs`，L13；`func GetUserLogs`，L36

**具体操作**：

1. `model/log.go GetAllLogs`：签名加参数 `auditHitSeverity string`；函数体在 `WHERE` 链后追加：
   ```go
   if auditHitSeverity != "" {
       tx = tx.Where("logs.audit_hit_severity = ?", auditHitSeverity)
   }
   ```
2. `model/log.go GetUserLogs`：同样处理（签名 + WHERE）。
3. `controller/log.go GetAllLogs`：L24 行 `model.GetAllLogs(...)` 调用前，增加解析：
   ```go
   auditHitSeverity := c.Query("audit_hit_severity")
   ```
   然后传入 model 调用最后一个参数。
4. `controller/log.go GetUserLogs`：同样处理。

**验证**：
- `make test` 通过
- curl `GET /api/log?audit_hit_severity=high&start_timestamp=0&end_timestamp=9999999999` → 返回只含 severity=high 行（或空）
- curl `GET /api/log?start_timestamp=0` 无 audit 参数 → 返回全部（不过滤）

**Evidence**：`evidence/phase-1/curl-filter.json`（EVD-310）

### Task 6: 执行 Phase 1 回归验证

- **关联**：P1 全部 BR
- **前置任务**：5

**验证**：
- `make test` 通过数 ≥ P0 基线
- `cd relaykit && GOWORK=off go build ./...` 通过（INV-303）
- curl audit relay 请求 → 查 logs 表新列有值

**Evidence**：`evidence/phase-1/`（EVD-307/309/310）

---

### Phase 2: 前端列表增强

> 你在哪里：P1 后端 API 已扩展，但前端未感知新字段。
> 做完之后：列表有 Audit 列；filter bar 有 severity 筛选；URL state 支持 auditHitSeverity。

### Task 7: UsageLog schema + GetLogsParams + CommonLogFilters 加 audit 字段

- **关联**：BR-305；UF：NA
- **前置任务**：6
- **风险等级**：P0

**为什么做**：前端需要接收后端返回的 audit 字段，并在类型系统中声明。

**涉及文件与定位**：
- `web/src/features/usage-logs/data/schema.ts`：`usageLogSchema`，L26
- `web/src/features/usage-logs/types.ts`：`CommonLogFilters`，L49；`GetLogsParams`，L319
- `web/src/routes/_authenticated/usage-logs/$section.tsx`：`usageLogsSearchSchema`，L38

**具体操作**：

1. `data/schema.ts`：`usageLogSchema` 对象内追加：
   ```ts
   audit_hit_severity: z.string().default(''),
   audit_hit_count: z.number().default(0),
   ```
2. `types.ts CommonLogFilters`：追加 `auditHitSeverity?: string`
3. `types.ts GetLogsParams`：追加 `audit_hit_severity?: string`
4. `$section.tsx usageLogsSearchSchema`：追加 `auditHitSeverity: z.string().optional().catch('')`

**验证**：`bun run typecheck` 通过（EVD-312）

**Evidence**：`evidence/phase-2/typecheck.txt`

### Task 8: common-logs-columns 加 audit 命中列（admin-only）

- **关联**：BR-306 / UF-301；EVD-301/302
- **前置任务**：7
- **风险等级**：P1

**为什么做**：在列表展示 audit 命中指示器，UF-301 核心。

**涉及文件与定位**：
- `web/src/features/usage-logs/components/columns/common-logs-columns.tsx`：`useCommonLogsColumns`，L298

**具体操作**：

1. `useCommonLogsColumns` 函数返回的 `ColumnDef<UsageLog>[]` 数组末尾追加一个 audit 列定义：
   ```tsx
   {
     id: 'audit',
     accessorKey: 'audit_hit_count',
     header: ({ column }) => <DataTableColumnHeader column={column} title={t('Audit')} />,
     cell: ({ row }) => {
       const log = row.original
       const count = log.audit_hit_count
       const severity = log.audit_hit_severity
       if (count > 0 && severity) {
         const badgeVariant = severity === 'high' || severity === 'critical' ? 'destructive' : 'secondary'
         return <Badge variant={badgeVariant}>{severity}</Badge>
       }
       return <span className="text-muted-foreground">-</span>
     },
     enableSorting: false,
     enableHiding: true,
   }
   ```
2. 该列只在 `isAdmin` 时返回；非 admin 时从数组移除（或在外层条件过滤）。

**验证**：
- browser 打开 `/usage-logs/common` → admin 可见 "Audit" 列 → 截图（EVD-301/302）
- 非 admin 用户 → 无 Audit 列

**Evidence**：`evidence/UF-301/with-hits.png`；`evidence/UF-301/no-hits.png`

### Task 9: common-logs-filter-bar 加严重度筛选 + buildSearchParams + URL state

- **关联**：BR-307 / UF-302；EVD-303/304
- **前置任务**：8
- **风险等级**：P1

**为什么做**：让 admin 能按 severity 筛选日志，UF-302 核心。

**涉及文件与定位**：
- `web/src/features/usage-logs/components/common-logs-filter-bar.tsx`：`CommonLogsFilterBar`，L112
- `web/src/features/usage-logs/lib/filter.ts`：`buildSearchParams`，L38

**具体操作**：

1. `common-logs-filter-bar.tsx`：在 filter bar 内追加 Audit Severity `Select`：
   ```tsx
   <LogsFilterField label={t('Audit Severity')}>
     <Select value={draft.filters.auditHitSeverity || 'all'} onValueChange={...}>
       <SelectItem value="all">{t('All')}</SelectItem>
       <SelectItem value="low">{t('Low')}</SelectItem>
       <SelectItem value="medium">{t('Medium')}</SelectItem>
       <SelectItem value="high">{t('High')}</SelectItem>
       <SelectItem value="critical">{t('Critical')}</SelectItem>
     </Select>
   </LogsFilterField>
   ```
2. `lib/filter.ts buildSearchParams` common 分支追加：
   ```ts
   ...(commonFilters.auditHitSeverity && commonFilters.auditHitSeverity !== 'all' && {
     audit_hit_severity: commonFilters.auditHitSeverity,
   }),
   ```

**验证**：
- browser 选 "High" → URL 含 `auditHitSeverity=high` → 列表只返回 high 行 → 截图（EVD-303/304）
- 选 "All" → URL 无 audit 参数 → 列表全量

**Evidence**：`evidence/UF-302/filter-open.png`；`evidence/UF-302/filter-result.png`

### Task 10: 执行 Phase 2 回归验证

- **关联**：P2 全部 BR
- **前置任务**：9

**验证**：
- `bun run typecheck` 通过
- `bun run build` 通过（EVD-308）
- browser UF-301/302 全通过

**Evidence**：`evidence/phase-2/`（EVD-312/308）；`evidence/UF-301/`；`evidence/UF-302/`

---

### Phase 3: 详情对话框 Tab 分离

> 你在哪里：P2 前端列表增强完毕；details-dialog 中 AuditContentSection 仍内联在 billing 流中。
> 做完之后：有命中的日志详情对话框展示 Billing / Audit 两个 tab。

### Task 11: details-dialog.tsx 加 Billing / Audit Tabs

- **关联**：BR-308 / UF-303 / INV-305；EVD-305/306
- **前置任务**：10
- **风险等级**：P1

**为什么做**：计费和审计信息在同一滚动流中难以区分；tab 分离后用户明确知道在看什么。

**涉及文件与定位**：
- `web/src/features/usage-logs/components/dialogs/details-dialog.tsx`：`AuditContentSection` import，`rg "AuditContentSection" ...details-dialog.tsx`，L62 hint

**具体操作**：

1. 在 `details-dialog.tsx` 的 dialog content 区域，找到当前渲染 billing info 和 `AuditContentSection` 的区域（当前为顺序排列）。
2. 引入 tab 组件（已存在于项目 `@/components/ui/tabs`）：
   ```tsx
   import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
   ```
3. 条件逻辑：`const showAuditTab = isAdmin && log.audit_hit_count > 0`
4. 当 `showAuditTab` 为 true 时，将 billing 内容包裹在 `<TabsContent value="billing">` 内，并新增：
   ```tsx
   <TabsContent value="audit">
     <AuditContentSection requestId={log.request_id} />
   </TabsContent>
   ```
   外层用 `<Tabs defaultValue="billing">` 包裹整个内容区。
5. 当 `showAuditTab` 为 false（无命中或非 admin）时，仍渲染原 billing 内容，不加 Tabs。
6. 原内联 `AuditContentSection`（若存在）从非 tab 路径移除。

**验证**：
- browser：点击有命中行 → 两个 tab 可见 → 截图 Billing tab（EVD-305）；切换 Audit tab → AuditContentSection 渲染 → 截图（EVD-306）
- 点击无命中行 → 只有 Billing tab，无 Audit tab
- billing tab 内 DynamicPricingBreakdown、token metrics 等完整（INV-305）

**Evidence**：`evidence/UF-303/billing-tab.png`（EVD-305）；`evidence/UF-303/audit-tab.png`（EVD-306）

**注意事项**：Tabs 组件在40KB 文件里，需先 read 当前 billing 内容渲染区域（大约在 dialog body 中间）再做精准插入，避免破坏嵌套结构；TypeScript 编译不过则先修类型再截图。

### Task 12: 执行 Phase 3 回归验证

- **关联**：P3 全部 BR；INV-305
- **前置任务**：11

**验证**：
- `bun run typecheck` 通过
- browser UF-303 全通过（Billing tab 功能完整 + Audit tab 正确展示）

**Evidence**：`evidence/phase-3/`；`evidence/UF-303/`

---

### Phase 4: i18n + 真实场景验收

> 你在哪里：P3 对话框完成；新增 UI 字符串尚未翻译。
> 做完之后：全7语言无 missing；三条 UF 真实场景矩阵全部通过。

### Task 13: i18n sync

- **关联**：BR-309；UF：NA
- **前置任务**：12
- **风险等级**：P2

**为什么做**：所有新增前端字符串需在7个 locale 文件中同步翻译。

**涉及文件与定位**：
- `web/src/i18n/locales/{en,zh,zh-TW,fr,ru,ja,vi}.json`

**具体操作**：

1. `cd web && bun run i18n:sync` → 查看 missing keys 列表。
2. 对 missing keys（预期包含：`"Audit"`、`"Audit Severity"`、`"Audit hits"`、`"Low"`、`"Medium"`、`"High"`、`"Critical"`、`"Billing"`、`"Audit tab label"` 等）在 `en.json` 补全英文值（部分 key 可能已存在，仅补新增的）。
3. 在 zh/zh-TW/fr/ru/ja/vi 各 locale 补对应翻译。
4. 再跑 `bun run i18n:sync` → 0 missing / 0 untranslated。

**验证**：`bun run i18n:sync` 输出 0 missing（EVD-311）

**Evidence**：`evidence/phase-4/i18n-sync.txt`（EVD-311）

### Task 14: 执行 spec 5.2 真实场景全套测试

- **关联**：全部用户可见 UF（UF-301/302/303）；EVD-301~306
- **前置任务**：13
- **风险等级**：P0

**为什么做**：真实场景测试是完成的唯一标准（shared-rules §6）。

**具体操作**：按 5.2 执行矩阵逐行回放，每行截图/保存。

**Evidence**：`evidence/UF-301/`；`evidence/UF-302/`；`evidence/UF-303/`

### Task 15: 执行 Phase 4 最终回归验证

- **关联**：全部 BR/UF/INV
- **前置任务**：14

**验证**：
- `make test` 通过
- `GOWORK=off go build ./...` + `cd relaykit && GOWORK=off go build ./...` 通过
- `cd web && bun run typecheck && bun run build` 通过
- `bun run i18n:sync` 0 missing
- 5.2 矩阵6行全部通过

**Evidence**：`evidence/phase-4/`；`evidence/UF-301~303/`

---

## 5. 验收与 Review 协议

> **验收铁律：命令级验证（5.1）通过只是入场券。** 用户可见的需求必须通过 5.2 真实场景测试才算完成。

### 5.1 命令级验证（入场券）

| 验证项 | 命令 | 期望 | Evidence |
|---|---|---|---|
| 后端单测 | `make test` | 全绿，无 FAIL | EVD-307 |
| relaykit 独立构建 | `cd relaykit && GOWORK=off go build ./...` | 无 error | EVD-307 |
| TypeScript 类型检查 | `cd web && bun run typecheck` | 无 error | EVD-312 |
| 前端构建 | `cd web && bun run build` | 无 error | EVD-308 |
| i18n 完整性 | `cd web && bun run i18n:sync` | 0 missing / 0 untranslated | EVD-311 |
| SQLite schema | `sqlite3 {db_path} ".schema logs"` | 含 audit_hit_severity/count/wl_version | EVD-309 |

### 5.2 真实场景全套测试（Real-Run，完成的唯一标准）

**环境准备**：

| 项 | 值 |
|---|---|
| 启动命令 | `go run main.go` + `cd web && bun run dev` |
| 访问入口 | `http://localhost:3000/usage-logs/common` |
| 测试账号 | admin 账号（role=100）+ 普通用户账号（role=1） |
| 审计数据 | 至少1条已触发 watchlist 规则的 LogTypeConsume 日志（有 audit_hit_count > 0） |
| 可用测试工具 | 浏览器 MCP（chrome-devtools-proxy）或手动截图 |

**执行矩阵**：

| UF | 执行方式 | 操作来源 | 必须核对的点 | Evidence |
|---|---|---|---|---|
| UF-301 主路径 | browser | 2.3 § UF-301 成功主路径 | admin 可见 "Audit" 列；有命中行显示 severity badge；无命中行显示 `-` | EVD-301/302 |
| UF-301 非 admin | browser | 2.3 § UF-301 非 admin 失败分支 | 普通用户列表无 "Audit" 列 | EVD-302 |
| UF-301 历史零值 | browser | 2.3 § UF-301 历史零值分支 | 历史行显示 `-`，不崩溃 | EVD-302 |
| UF-302 主路径 | browser | 2.3 § UF-302 成功主路径 | 选 High → URL 含 auditHitSeverity=high → 列表筛选 | EVD-303/304 |
| UF-302 无匹配行 | browser | 2.3 § UF-302 无匹配行失败分支 | 空 state 正常显示，无 JS error | EVD-304 |
| UF-303 主路径 | browser | 2.3 § UF-303 成功主路径 | 两 tab 可见；Billing tab 内容完整；Audit tab 展示 segments/flags | EVD-305/306 |
| UF-303 无命中行 | browser | 2.3 § UF-303 无命中失败分支 | 只有 Billing tab，无 Audit tab | EVD-305 |

### 5.3 Evidence 目录结构与命名

```text
evidence/
  phase-0/   make-test.txt, go-build.txt
  phase-1/   make-test.txt, sqlite-schema.txt, curl-filter.json
  phase-2/   typecheck.txt, bun-build.txt
  phase-3/   typecheck.txt
  phase-4/   i18n-sync.txt
  UF-301/    with-hits.png, no-hits.png
  UF-302/    filter-open.png, filter-result.png
  UF-303/    billing-tab.png, audit-tab.png
```

### 5.4 Review 专项检查清单

- [ ] `UpdateLogAuditPointer` 全仓已不存在，改为 `UpdateLogAuditFields`（`grep -rn "UpdateLogAuditPointer" --include=*.go .` → 0 命中）
- [ ] `Log` struct 新列无 bool `gorm:"default:true"` tag（AGENTS.md 禁止）
- [ ] `LOG_DB.Model(&Log{}).Updates(map...)` 使用 map，零值可正确写入
- [ ] `GetAllLogs`/`GetUserLogs` 所有调用点（controller）已同步更新新参数
- [ ] `details-dialog.tsx` billing tab 内容完整（DynamicPricingBreakdown/token/quota 等）
- [ ] audit 列、filter、tab 非 admin 路径均无渲染（isAdmin guard）
- [ ] 5.2 执行矩阵全部通过，evidence 齐全
- [ ] 2.3 节每条流程入口接线清单已实现（列表列注册、filter bar 接线、tab 条件渲染接线）
- [ ] `bun run i18n:sync` 0 missing

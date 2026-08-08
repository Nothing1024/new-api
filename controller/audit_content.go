package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/audit"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// logContentResponse 是 GET /api/log/content 的响应（segments/flags 已解析为 JSON）。
type logContentResponse struct {
	RequestId        string          `json:"request_id"`
	UserId           int             `json:"user_id"`
	ChannelId        int             `json:"channel_id"`
	CreatedAt        int64           `json:"created_at"`
	ModelName        string          `json:"model_name"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	Quota            int             `json:"quota"`
	Fidelity         string          `json:"fidelity"`
	Segments         []audit.Segment `json:"segments"`
	HitSeverity      string          `json:"hit_severity"`
	HitCount         int             `json:"hit_count"`
	Flags            []audit.HitFlag `json:"flags"`
	WLVersion        int             `json:"wl_version"`
}

// GetLogContent 查询单条审计内容（AdminAuth 前置，UF-002）。
func GetLogContent(c *gin.Context) {
	requestId := c.Query("request_id")
	if requestId == "" {
		common.ApiErrorMsg(c, "request_id is required")
		return
	}
	lc, err := model.GetLogContent(requestId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "audit content not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	resp := logContentResponse{
		RequestId:        lc.RequestId,
		UserId:           lc.UserId,
		ChannelId:        lc.ChannelId,
		CreatedAt:        lc.CreatedAt,
		ModelName:        lc.ModelName,
		PromptTokens:     lc.PromptTokens,
		CompletionTokens: lc.CompletionTokens,
		Quota:            lc.Quota,
		Fidelity:         lc.Fidelity,
		HitSeverity:      lc.HitSeverity,
		HitCount:         lc.HitCount,
		WLVersion:        lc.WLVersion,
	}
	if lc.Segments != "" {
		_ = common.UnmarshalJsonStr(lc.Segments, &resp.Segments)
	}
	if lc.Flags != "" {
		_ = common.UnmarshalJsonStr(lc.Flags, &resp.Flags)
	}
	common.ApiSuccess(c, resp)
}

type watchlistRuleRequest struct {
	Id       uint   `json:"id"`
	Kind     string `json:"kind"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"`
	Enabled  *bool  `json:"enabled"`
	Note     string `json:"note"`
}

// ListWatchlistRules 列出规则（UF-006）。
func ListWatchlistRules(c *gin.Context) {
	enabledStr := c.Query("enabled")
	var enabled *bool
	if enabledStr == "true" || enabledStr == "false" {
		v := enabledStr == "true"
		enabled = &v
	}
	kind := c.Query("kind")
	rules, err := model.ListWatchlistRules(enabled, kind)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rules)
}

// CreateWatchlistRule 新增规则（UF-006）。
func CreateWatchlistRule(c *gin.Context) {
	var req watchlistRuleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Kind == "" || req.Pattern == "" {
		common.ApiErrorMsg(c, "kind and pattern are required")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &model.AuditWatchlistRule{
		Kind:     req.Kind,
		Pattern:  req.Pattern,
		Severity: req.Severity,
		Enabled:  enabled,
		Note:     req.Note,
	}
	if err := model.CreateWatchlistRule(rule); err != nil {
		if errors.Is(err, model.ErrRegexLimit) {
			common.ApiErrorMsg(c, "regex rules limit reached (max 8 enabled)")
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": rule})
}

// UpdateWatchlistRule 更新规则（UF-006）。
func UpdateWatchlistRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	var req watchlistRuleRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &model.AuditWatchlistRule{
		Id:       uint(id),
		Kind:     req.Kind,
		Pattern:  req.Pattern,
		Severity: req.Severity,
		Enabled:  enabled,
		Note:     req.Note,
	}
	if err := model.UpdateWatchlistRule(rule); err != nil {
		if errors.Is(err, model.ErrRegexLimit) {
			common.ApiErrorMsg(c, "regex rules limit reached (max 8 enabled)")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rule)
}

// DeleteWatchlistRule 删除规则（UF-006）。
func DeleteWatchlistRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid id")
		return
	}
	if err := model.DeleteWatchlistRule(uint(id)); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(200, gin.H{"success": true, "message": ""})
}

// TriggerRescan 发起重扫（UF-007）。
func TriggerRescan(c *gin.Context) {
	wlVersion := model.GetWatchlistVersion()
	started, err := service.TriggerRescan(c, wlVersion)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !started {
		common.ApiErrorMsg(c, "rescan already running")
		return
	}
	common.ApiSuccess(c, gin.H{"wl_version": wlVersion})
}

// GetRescanStatus 查询重扫进度（UF-007）。
func GetRescanStatus(c *gin.Context) {
	var st service.RescanStatus
	_ = service.GetRescanStatus(c, &st)
	common.ApiSuccess(c, st)
}

// auditTemplateListItem is the response shape of GET /api/audit/templates.
type auditTemplateListItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	RuleCount     int    `json:"rule_count"`
	AppliedCount  int64  `json:"applied_count"`
	EnabledCount  int64  `json:"enabled_count"`
	Status        string `json:"status"` // unapplied / applied / disabled
}

// enablePtr builds a *bool filter for count queries.
func enablePtr(v bool) *bool { return &v }

// ListAuditTemplates 列出内置模板包及其应用状态（BR-113, UF-102）。
func ListAuditTemplates(c *gin.Context) {
	items := make([]auditTemplateListItem, 0, len(service.BuiltinAuditTemplates))
	for i := range service.BuiltinAuditTemplates {
		tpl := &service.BuiltinAuditTemplates[i]
		applied, err := model.CountWatchlistRulesByTemplate(tpl.ID, nil)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		enabled, err := model.CountWatchlistRulesByTemplate(tpl.ID, enablePtr(true))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		status := "unapplied"
		if enabled > 0 {
			status = "applied"
		} else if applied > 0 {
			status = "disabled"
		}
		items = append(items, auditTemplateListItem{
			ID:           tpl.ID,
			Name:         tpl.Name,
			Description:  tpl.Description,
			RuleCount:    len(tpl.Rules),
			AppliedCount: applied,
			EnabledCount: enabled,
			Status:       status,
		})
	}
	common.ApiSuccess(c, items)
}

// ApplyAuditTemplate 应用模板包（幂等，BR-114/BR-116）。regex 规则默认 disabled。
func ApplyAuditTemplate(c *gin.Context) {
	id := c.Param("id")
	tpl, ok := service.GetBuiltinTemplate(id)
	if !ok {
		common.ApiErrorMsg(c, "template not found")
		return
	}
	rules := make([]model.AuditWatchlistRule, 0, len(tpl.Rules))
	regexDisabled := 0
	for _, r := range tpl.Rules {
		enabled := r.Enabled
		if r.Kind == model.WatchlistKindRegex {
			// BR-116: 模板内 regex 默认停用，不受 MaxEnabledRegexRules 影响。
			enabled = false
			regexDisabled++
		}
		rules = append(rules, model.AuditWatchlistRule{
			Kind:      r.Kind,
			Pattern:   r.Pattern,
			Severity:  r.Severity,
			Enabled:   enabled,
			Note:      r.Note,
			TemplateId: id,
			Source:    "template",
		})
	}
	applied, skipped, err := model.ApplyTemplateRules(id, rules)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	msg := "applied"
	if regexDisabled > 0 {
		msg = splitMessageWithNote(applied, regexDisabled)
	}
	common.ApiSuccess(c, gin.H{
		"applied":        applied,
		"skipped":        skipped,
		"regex_disabled": regexDisabled,
		"message":        msg,
	})
}

// splitMessageWithNote builds a human-facing message for apply result.
func splitMessageWithNote(applied int, regexDisabled int) string {
	base := "applied " + strconv.Itoa(applied)
	if regexDisabled > 0 {
		base += "; " + strconv.Itoa(regexDisabled) + " regex rule(s) left disabled, enable manually"
	}
	return base
}

// EnableAuditTemplate 整包启用（仅非 regex 规则；regex 保持停用，BR-115/BR-116）。
func EnableAuditTemplate(c *gin.Context) {
	id := c.Param("id")
	if _, ok := service.GetBuiltinTemplate(id); !ok {
		common.ApiErrorMsg(c, "template not found")
		return
	}
	if _, err := model.EnableTemplateRules(id); err != nil {
		common.ApiError(c, err)
		return
	}
	enabledCount, err := model.CountWatchlistRulesByTemplate(id, enablePtr(true))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// regex rules stay disabled by design; report them as skipped.
	allRules, err := model.ListWatchlistRulesByTemplate(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	regexSkipped := 0
	for _, r := range allRules {
		if r.Kind == model.WatchlistKindRegex && !r.Enabled {
			regexSkipped++
		}
	}
	common.ApiSuccess(c, gin.H{
		"enabled":       enabledCount,
		"regex_skipped": regexSkipped,
		"message":       "template enabled (regex rules left disabled)",
	})
}

// DisableAuditTemplate 整包停用（BR-115）。
func DisableAuditTemplate(c *gin.Context) {
	id := c.Param("id")
	if _, ok := service.GetBuiltinTemplate(id); !ok {
		common.ApiErrorMsg(c, "template not found")
		return
	}
	n, err := model.DisableTemplateRules(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"disabled": n})
}

// DeleteAuditTemplate 整包移除（BR-115）。
func DeleteAuditTemplate(c *gin.Context) {
	id := c.Param("id")
	if _, ok := service.GetBuiltinTemplate(id); !ok {
		common.ApiErrorMsg(c, "template not found")
		return
	}
	n, err := model.DeleteTemplateRules(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"removed": n})
}

// watchlistExport is the export payload shape (BR-117).
type watchlistExport struct {
	Version    int                       `json:"version"`
	TemplateID string                    `json:"template_id"`
	Name       string                    `json:"name"`
	Description string                   `json:"description"`
	Rules      []watchlistExportRule     `json:"rules"`
}

type watchlistExportRule struct {
	Kind     string `json:"kind"`
	Pattern  string `json:"pattern"`
	Severity string `json:"severity"`
	Enabled  bool   `json:"enabled"`
	Note     string `json:"note"`
}

// ExportWatchlistRules 导出规则为 JSON。
func ExportWatchlistRules(c *gin.Context) {
	rules, err := model.ListWatchlistRules(nil, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	exp := watchlistExport{
		Version:    model.GetWatchlistVersion(),
		TemplateID: "custom-export",
		Rules:      make([]watchlistExportRule, 0, len(rules)),
	}
	for _, r := range rules {
		exp.Rules = append(exp.Rules, watchlistExportRule{
			Kind:     r.Kind,
			Pattern:  r.Pattern,
			Severity: r.Severity,
			Enabled:  r.Enabled,
			Note:     r.Note,
		})
	}
	body, err := common.Marshal(exp)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filename := "audit-rules-" + strconv.FormatInt(common.GetTimestamp(), 10) + ".json"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Content-Type", "application/json; charset=utf-8")
	_, _ = c.Writer.Write(body)
}

// watchlistImport is the accepted import payload (BR-117): the export shape or
// a bare array.
type watchlistImport struct {
	TemplateID string                `json:"template_id"`
	Name       string                `json:"name"`
	Description string               `json:"description"`
	Rules      []watchlistExportRule `json:"rules"`
}

const maxImportRules = 100

// ImportWatchlistRules 导入规则（ALL-or-nothing，BR-117）。
func ImportWatchlistRules(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var result []model.AuditWatchlistRule

	// Try structured export shape first; fall back to a bare array.
	var structured struct {
		TemplateID string                `json:"template_id"`
		Rules      []watchlistExportRule `json:"rules"`
	}
	if uerr := common.Unmarshal(body, &structured); uerr == nil && structured.Rules != nil {
		for _, r := range structured.Rules {
			result = append(result, toWatchlistRule(r))
		}
	} else {
		var arr []watchlistExportRule
		if aerr := common.Unmarshal(body, &arr); aerr != nil {
			common.ApiErrorMsg(c, "invalid JSON: expected export object or rule array")
			return
		}
		for _, r := range arr {
			result = append(result, toWatchlistRule(r))
		}
	}

	if len(result) == 0 {
		common.ApiErrorMsg(c, "no rules to import")
		return
	}
	if len(result) > maxImportRules {
		common.ApiErrorMsg(c, "too many rules: max 100 per import")
		return
	}

	// ALL-or-nothing validation (BR-117).
	for i := range result {
		if err := model.ValidateWatchlistRuleImport(&result[i]); err != nil {
			common.ApiErrorMsg(c, "rule "+strconv.Itoa(i)+" invalid: "+err.Error())
			return
		}
	}

	if err := model.CreateWatchlistRulesBatch(result); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"imported": len(result)})
}

func toWatchlistRule(r watchlistExportRule) model.AuditWatchlistRule {
	return model.AuditWatchlistRule{
		Kind:     r.Kind,
		Pattern:  r.Pattern,
		Severity: r.Severity,
		Enabled:  r.Enabled,
		Note:     r.Note,
	}
}

// GetAuditLogs 审计日志列表（BR-110, ASM-106, UF-104）。
func GetAuditLogs(c *gin.Context) {
	params := model.ListLogContentsParams{
		Severity:  c.Query("severity"),
		StartTime: queryInt64(c, "start_timestamp"),
		EndTime:   queryInt64(c, "end_timestamp"),
		ModelName: c.Query("model_name"),
		Page:      queryInt(c, "p"),
		PageSize:  queryInt(c, "page_size"),
	}
	params.UserId = queryInt(c, "user_id")
	params.MinHit = queryInt(c, "min_hit")

	items, total, err := model.ListLogContents(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items, "total": total})
}

func queryInt(c *gin.Context, key string) int {
	v, _ := strconv.Atoi(c.Query(key))
	return v
}

func queryInt64(c *gin.Context, key string) int64 {
	v, _ := strconv.ParseInt(c.Query(key), 10, 64)
	return v
}

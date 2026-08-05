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

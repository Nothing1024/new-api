package model

import (
	"context"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LogContent 存储审计内容（logs_content 表，LOG_DB）。
// 通过 request_id 与 logs 表 1:1 关联（BR-001）；正文只存此表，logs.other 只存指针（BR-002）。
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
	Segments         string `gorm:"type:text"` // JSON []audit.Segment
	HitSeverity      string `gorm:"type:varchar(8);index"`
	HitCount         int
	Flags            string `gorm:"type:text"` // JSON []audit.HitFlag
	WLVersion        int    `gorm:"index"`
}

// CreateLogContent 写入一条审计记录（BR-001：request_id 主键保证 1:1）。
// 使用 ON CONFLICT / ON DUPLICATE KEY UPDATE upsert，兼容 SQLite/MySQL/PostgreSQL（BR-016）。
func CreateLogContent(lc *LogContent) error {
	return LOG_DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(lc).Error
}

// GetLogContent 按 request_id 查询单条审计记录。
func GetLogContent(requestId string) (*LogContent, error) {
	var lc LogContent
	if err := LOG_DB.Where("request_id = ?", requestId).First(&lc).Error; err != nil {
		return nil, err
	}
	return &lc, nil
}

// UpdateLogContent 全量更新审计记录（主键 upsert 语义）。
func UpdateLogContent(lc *LogContent) error {
	return LOG_DB.Save(lc).Error
}

// UpdateLogContentFlags 更新命中信息（watchlist 扫描/重扫后调用）。
func UpdateLogContentFlags(requestId string, hitSeverity string, hitCount int, flags string, wlVersion int) error {
	return LOG_DB.Model(&LogContent{}).Where("request_id = ?", requestId).Updates(map[string]interface{}{
		"hit_severity": hitSeverity,
		"hit_count":    hitCount,
		"flags":        flags,
		"wl_version":   wlVersion,
	}).Error
}

// ListLogContentsForRescan 分批返回需要重扫的记录（BR-013：仅 TTL 内且 wl_version 落后）。
func ListLogContentsForRescan(wlVersion int, cutoff int64, limit int, offset int) ([]*LogContent, error) {
	var list []*LogContent
	err := LOG_DB.Where("wl_version < ? AND created_at > ?", wlVersion, cutoff).
		Order("created_at desc").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

// CountLogContentsForRescan 统计需要重扫的记录总数。
func CountLogContentsForRescan(wlVersion int, cutoff int64) (int64, error) {
	var count int64
	err := LOG_DB.Model(&LogContent{}).
		Where("wl_version < ? AND created_at > ?", wlVersion, cutoff).Count(&count).Error
	return count, err
}

// UpdateLogAuditFields 将审计命中信息双写：logs.other.admin_info.audit 指针（向后兼容 BR-002）
// + logs 新列 audit_hit_severity / audit_hit_count / audit_wl_version（BR-302）。
// 只存 {request_id, hit_count} 指针（约 40B），正文不进 logs.other。
func UpdateLogAuditFields(requestId string, hitCount int, hitSeverity string, wlVersion int) error {
	var log Log
	if err := LOG_DB.Where("request_id = ?", requestId).First(&log).Error; err != nil {
		return err
	}
	otherMap, err := common.StrToMap(log.Other)
	if err != nil {
		otherMap = map[string]interface{}{}
	}
	adminInfo, ok := otherMap["admin_info"].(map[string]interface{})
	if !ok || adminInfo == nil {
		adminInfo = map[string]interface{}{}
		otherMap["admin_info"] = adminInfo
	}
	adminInfo["audit"] = map[string]interface{}{
		"request_id": requestId,
		"hit_count":  hitCount,
	}
	newOther := common.MapToJsonStr(otherMap)
	if err := LOG_DB.Model(&Log{}).Where("request_id = ?", requestId).Update("other", newOther).Error; err != nil {
		return err
	}
	// 用 map 而非 struct Updates，确保零值（hitCount=0/hitSeverity=""）也能写入（BR-302）。
	return LOG_DB.Model(&Log{}).Where("request_id = ?", requestId).
		Updates(map[string]interface{}{
			"audit_hit_count":    hitCount,
			"audit_hit_severity": hitSeverity,
			"audit_wl_version":   wlVersion,
		}).Error
}

// ListLogContentsParams carries the audit-log list filters (BR-110, ASM-106).
type ListLogContentsParams struct {
	Severity  string
	MinHit    int
	StartTime int64
	EndTime   int64
	UserId    int
	ModelName string
	Page      int
	PageSize  int
}

// CountLogContents counts log_contents rows matching the given filters but
// ignores paging.
func CountLogContents(p ListLogContentsParams) (int64, error) {
	var count int64
	err := buildLogContentQuery(p).Count(&count).Error
	return count, err
}

// buildLogContentQuery applies the ListLogContentsParams filters to a
// *gorm.DB chain over LOG_DB. The where clauses reuse existing indexes
// (HitSeverity/CreatedAt/UserId, F-130).
func buildLogContentQuery(p ListLogContentsParams) *gorm.DB {
	query := LOG_DB.Model(&LogContent{})
	if p.Severity != "" {
		query = query.Where("hit_severity = ?", p.Severity)
	}
	if p.MinHit > 0 {
		query = query.Where("hit_count >= ?", p.MinHit)
	}
	if p.StartTime > 0 {
		query = query.Where("created_at >= ?", p.StartTime)
	}
	if p.EndTime > 0 {
		query = query.Where("created_at <= ?", p.EndTime)
	}
	if p.UserId > 0 {
		query = query.Where("user_id = ?", p.UserId)
	}
	if p.ModelName != "" {
		query = query.Where("model_name = ?", p.ModelName)
	}
	return query
}

// ListLogContents returns a page of audit log contents matching the filters,
// newest first, plus the total count (BR-110). It follows the
// ListLogContentsForRescan count-then-find pattern.
func ListLogContents(p ListLogContentsParams) ([]*LogContent, int64, error) {
	page := p.Page
	if page <= 0 {
		page = 1
	}
	pageSize := p.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var total int64
	countQuery := buildLogContentQuery(p)
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []*LogContent
	err := buildLogContentQuery(p).
		Order("created_at desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// CountOldLogContent counts log_contents rows older than the cutoff timestamp
// (BR-112). Mirrors CountOldLog in model/log.go.
func CountOldLogContent(ctx context.Context, cutoff int64) (int64, error) {
	var total int64
	if err := LOG_DB.WithContext(ctx).Model(&LogContent{}).Where("created_at < ?", cutoff).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// DeleteOldLogContentBatch deletes up to batchSize log_contents rows older than
// the cutoff timestamp and returns how many were removed (BR-112). It mirrors
// DeleteOldLogBatch in model/log.go.
func DeleteOldLogContentBatch(ctx context.Context, cutoff int64, batchSize int) (int64, error) {
	if batchSize <= 0 {
		batchSize = 500
	}
	if ctx != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}
	result := LOG_DB.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Limit(batchSize).
		Delete(&LogContent{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

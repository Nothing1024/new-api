package model

import (
	"github.com/QuantumNous/new-api/common"
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

// UpdateLogAuditPointer 将 logs.other.admin_info.audit 指针写入对应请求的日志行（BR-002）。
// 只存 {request_id, hit_count} 指针（约 40B），正文不进 logs.other。
func UpdateLogAuditPointer(requestId string, hitCount int) error {
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
	return LOG_DB.Model(&Log{}).Where("request_id = ?", requestId).Update("other", newOther).Error
}

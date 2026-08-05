package model

import (
	"errors"
	"regexp"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// watchlist 规则 kind 与约束
const (
	WatchlistKindDomain  = "domain"
	WatchlistKindKeyword = "keyword"
	WatchlistKindRegex   = "regex"

	// MaxEnabledRegexRules 同时启用的 regex 规则上限（BR-010）。
	MaxEnabledRegexRules = 8
)

// ErrRegexLimit 表示 regex 规则已达上限。
var ErrRegexLimit = errors.New("regex rules limit reached (max 8 enabled)")

// AuditWatchlistRule 是 watchlist 规则，存主库独立表（BR-011）。
type AuditWatchlistRule struct {
	Id        uint   `json:"id" gorm:"primaryKey"`
	Kind      string `json:"kind" gorm:"type:varchar(16);index"` // domain/keyword/regex
	Pattern   string `json:"pattern" gorm:"type:varchar(512)"`
	Severity  string `json:"severity" gorm:"type:varchar(8)"` // low/medium/high
	Enabled   bool   `json:"enabled" gorm:"index"`
	Note      string `json:"note" gorm:"type:varchar(512)"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// AuditWatchlistMeta 是 watchlist 版本元数据，单行 id=1（BR-011：增删改均 version++）。
type AuditWatchlistMeta struct {
	Id      uint `json:"id" gorm:"primaryKey"`
	Version int  `json:"version"`
}

// ListWatchlistRules 列出规则；enabled/kind 为可选过滤。
func ListWatchlistRules(enabled *bool, kind string) ([]AuditWatchlistRule, error) {
	query := DB.Model(&AuditWatchlistRule{})
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var rules []AuditWatchlistRule
	err := query.Order("id asc").Find(&rules).Error
	return rules, err
}

// GetWatchlistVersion 返回当前规则版本（无 meta 行时为 0）。
func GetWatchlistVersion() int {
	var meta AuditWatchlistMeta
	if err := DB.Where("id = 1").First(&meta).Error; err != nil {
		return 0
	}
	return meta.Version
}

// CreateWatchlistRule 创建规则并 version++（BR-011）；regex 超限返回 ErrRegexLimit（BR-010）。
func CreateWatchlistRule(rule *AuditWatchlistRule) error {
	if err := validateWatchlistRule(rule); err != nil {
		return err
	}
	now := common.GetTimestamp()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rule).Error; err != nil {
			return err
		}
		return bumpWatchlistVersionTx(tx)
	})
}

// UpdateWatchlistRule 更新规则并 version++（BR-011）。
func UpdateWatchlistRule(rule *AuditWatchlistRule) error {
	if err := validateWatchlistRule(rule); err != nil {
		return err
	}
	rule.UpdatedAt = common.GetTimestamp()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AuditWatchlistRule{}).Where("id = ?", rule.Id).
			Updates(map[string]interface{}{
				"kind":       rule.Kind,
				"pattern":    rule.Pattern,
				"severity":   rule.Severity,
				"enabled":    rule.Enabled,
				"note":       rule.Note,
				"updated_at": rule.UpdatedAt,
			}).Error; err != nil {
			return err
		}
		return bumpWatchlistVersionTx(tx)
	})
}

// DeleteWatchlistRule 删除规则并 version++（BR-011）。
func DeleteWatchlistRule(id uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&AuditWatchlistRule{}, id).Error; err != nil {
			return err
		}
		return bumpWatchlistVersionTx(tx)
	})
}

// GetWatchlistRule 按 id 查询单条规则。
func GetWatchlistRule(id uint) (*AuditWatchlistRule, error) {
	var rule AuditWatchlistRule
	if err := DB.Where("id = ?", id).First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func validateWatchlistRule(rule *AuditWatchlistRule) error {
	if rule.Pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	switch rule.Kind {
	case WatchlistKindDomain, WatchlistKindKeyword:
		// 无额外校验
	case WatchlistKindRegex:
		if rule.Enabled {
			count, err := countEnabledRegexRules(rule.Id)
			if err != nil {
				return err
			}
			if count >= MaxEnabledRegexRules {
				return ErrRegexLimit
			}
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return errors.New("invalid regex pattern: " + err.Error())
		}
	default:
		return errors.New("invalid kind: must be domain/keyword/regex")
	}
	if rule.Severity == "" {
		rule.Severity = "medium"
	}
	return nil
}

func countEnabledRegexRules(excludeId uint) (int64, error) {
	var count int64
	query := DB.Model(&AuditWatchlistRule{}).Where("kind = ? AND enabled = ?", WatchlistKindRegex, true)
	if excludeId > 0 {
		query = query.Where("id <> ?", excludeId)
	}
	err := query.Count(&count).Error
	return count, err
}

// bumpWatchlistVersionTx 在事务内将 meta 行版本 +1（BR-011）。
func bumpWatchlistVersionTx(tx *gorm.DB) error {
	var meta AuditWatchlistMeta
	if err := lockForUpdate(tx).Where("id = 1").First(&meta).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&AuditWatchlistMeta{Id: 1, Version: 1}).Error
		}
		return err
	}
	return tx.Model(&AuditWatchlistMeta{}).Where("id = 1").Update("version", meta.Version+1).Error
}

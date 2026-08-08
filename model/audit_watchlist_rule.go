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
	Enabled    bool   `json:"enabled" gorm:"index"`
	Note       string `json:"note" gorm:"type:varchar(512)"`
	Source     string `json:"source"      gorm:"type:varchar(16);index"`  // manual/template
	TemplateId string `json:"template_id" gorm:"type:varchar(64);index"` // applied builtin template id
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
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
	if rule.Source == "" {
		rule.Source = "manual"
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
				"kind":        rule.Kind,
				"pattern":     rule.Pattern,
				"severity":    rule.Severity,
				"enabled":     rule.Enabled,
				"note":        rule.Note,
				"source":      rule.Source,
				"template_id": rule.TemplateId,
				"updated_at":  rule.UpdatedAt,
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

// ValidateWatchlistRuleImport validates structural legality of an imported rule
// (kind/severity/pattern/regex-pattern) without enforcing the enabled-regex
// quota, which batch enable/apply handle separately (BR-116/BR-117).
func ValidateWatchlistRuleImport(rule *AuditWatchlistRule) error {
	if rule.Pattern == "" {
		return errors.New("pattern cannot be empty")
	}
	switch rule.Kind {
	case WatchlistKindDomain, WatchlistKindKeyword:
		// 无额外校验
	case WatchlistKindRegex:
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return errors.New("invalid regex pattern: " + err.Error())
		}
	default:
		return errors.New("invalid kind: must be domain/keyword/regex")
	}
	switch rule.Severity {
	case "", "low", "medium", "high":
	default:
		return errors.New("invalid severity: must be low/medium/high")
	}
	if rule.Severity == "" {
		rule.Severity = "medium"
	}
	return nil
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

// CountWatchlistRulesByTemplate returns the number of rules carrying the given
// template_id and source='template'. enabled filters the count when non-nil.
func CountWatchlistRulesByTemplate(templateID string, enabled *bool) (int64, error) {
	query := DB.Model(&AuditWatchlistRule{}).
		Where("template_id = ? AND source = ?", templateID, "template")
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

// ListWatchlistRulesByTemplate returns the rules of a given template_id whose
// source is 'template'.
func ListWatchlistRulesByTemplate(templateID string) ([]AuditWatchlistRule, error) {
	var rules []AuditWatchlistRule
	err := DB.Model(&AuditWatchlistRule{}).
		Where("template_id = ? AND source = ?", templateID, "template").
		Order("id asc").
		Find(&rules).Error
	return rules, err
}

// ApplyTemplateRules idempotently creates the given rules from a built-in
// template (BR-114). Dedup key is (template_id, kind, pattern). Rules already
// present are skipped. Returns applied/skipped counts.
func ApplyTemplateRules(templateID string, rules []AuditWatchlistRule) (applied int, skipped int, err error) {
	existing, err := ListWatchlistRulesByTemplate(templateID)
	if err != nil {
		return 0, 0, err
	}
	seen := make(map[string]bool, len(existing))
	for _, r := range existing {
		seen[templateRuleKey(templateID, r.Kind, r.Pattern)] = true
	}

	var toCreate []*AuditWatchlistRule
	now := common.GetTimestamp()
	toCreateSet := map[string]bool{}
	for _, r := range rules {
		key := templateRuleKey(templateID, r.Kind, r.Pattern)
		if seen[key] || toCreateSet[key] {
			skipped++
			continue
		}
		toCreateSet[key] = true
		rc := r
		rc.Source = "template"
		rc.TemplateId = templateID
		rc.CreatedAt = now
		rc.UpdatedAt = now
		toCreate = append(toCreate, &rc)
	}

	if len(toCreate) == 0 {
		return 0, skipped, nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toCreate).Error; err != nil {
			return err
		}
		return bumpWatchlistVersionTx(tx)
	})
	if err != nil {
		return 0, 0, err
	}
	return len(toCreate), skipped, nil
}

func templateRuleKey(templateID string, kind string, pattern string) string {
	return templateID + "|" + kind + "|" + pattern
}

// CreateWatchlistRulesBatch bulk-creates manual rules (import path, BR-117)
// and bumps the watchlist version once. Each rule's Source is normalized to
// "manual".
func CreateWatchlistRulesBatch(rules []AuditWatchlistRule) error {
	if len(rules) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	rows := make([]*AuditWatchlistRule, 0, len(rules))
	for i := range rules {
		r := rules[i]
		if r.Source == "" {
			r.Source = "manual"
		}
		r.CreatedAt = now
		r.UpdatedAt = now
		rows = append(rows, &r)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(rows).Error; err != nil {
			return err
		}
		return bumpWatchlistVersionTx(tx)
	})
}

// EnableTemplateRules bulk-enables the template's non-regex rules (BR-115).
// Returns the number of newly enabled rows.
func EnableTemplateRules(templateID string) (int64, error) {
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AuditWatchlistRule{}).
			Where("template_id = ? AND source = ? AND kind <> ? AND enabled = ?", templateID, "template", WatchlistKindRegex, false).
			Update("enabled", true)
		if res.Error != nil {
			return res.Error
		}
		affected = res.RowsAffected
		if affected == 0 {
			return nil
		}
		return bumpWatchlistVersionTx(tx)
	})
	return affected, err
}

// DisableTemplateRules bulk-disables every rule of the template (BR-115).
func DisableTemplateRules(templateID string) (int64, error) {
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AuditWatchlistRule{}).
			Where("template_id = ? AND source = ? AND enabled = ?", templateID, "template", true).
			Update("enabled", false)
		if res.Error != nil {
			return res.Error
		}
		affected = res.RowsAffected
		if affected == 0 {
			return nil
		}
		return bumpWatchlistVersionTx(tx)
	})
	return affected, err
}

// DeleteTemplateRules bulk-deletes every rule of the template (BR-115).
func DeleteTemplateRules(templateID string) (int64, error) {
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("template_id = ? AND source = ?", templateID, "template").
			Delete(&AuditWatchlistRule{})
		if res.Error != nil {
			return res.Error
		}
		affected = res.RowsAffected
		if affected == 0 {
			return nil
		}
		return bumpWatchlistVersionTx(tx)
	})
	return affected, err
}

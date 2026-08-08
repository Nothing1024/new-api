package service

import (
	"github.com/QuantumNous/new-api/model"
)

// BuiltinTemplateRule describes a single rule inside a built-in template. It
// mirrors the writable AuditWatchlistRule fields; regex rules always ship with
// Enabled=false (BR-116) and defer to the user to enable within the max limit.
type BuiltinTemplateRule struct {
	Kind     string
	Pattern  string
	Severity string
	Enabled  bool
	Note     string
}

// BuiltinTemplate is a read-only code asset (BR-113): users may apply/remove a
// template or edit generated rules, but cannot edit the built-in definition.
type BuiltinTemplate struct {
	ID          string
	Name        string
	Description string
	Rules       []BuiltinTemplateRule
}

// builtinRule is a shorthand constructor for keyword rules without regex.
func kw(pattern string, severity string, note string) BuiltinTemplateRule {
	return BuiltinTemplateRule{
		Kind:     model.WatchlistKindKeyword,
		Pattern:  pattern,
		Severity: severity,
		Enabled:  true,
		Note:     note,
	}
}

// domainRule builds a domain-kind rule.
func domain(pattern string, severity string, note string) BuiltinTemplateRule {
	return BuiltinTemplateRule{
		Kind:     model.WatchlistKindDomain,
		Pattern:  pattern,
		Severity: severity,
		Enabled:  true,
		Note:     note,
	}
}

// regexRule builds a regex-kind rule that ships disabled (BR-116). enable is
// reserved for future explicit overrides; built-ins currently always false.
func regexRule(pattern string, severity string, enable bool, note string) BuiltinTemplateRule {
	return BuiltinTemplateRule{
		Kind:     model.WatchlistKindRegex,
		Pattern:  pattern,
		Severity: severity,
		Enabled:  enable,
		Note:     note,
	}
}

// BuiltinAuditTemplates lists the built-in template packages. Order matters for
// stable frontend rendering.
var BuiltinAuditTemplates = []BuiltinTemplate{
	{
		ID:          "basic-security",
		Name:        "基础安全",
		Description: "覆盖常见提示注入与越权关键词的通用基础规则",
		Rules: []BuiltinTemplateRule{
			kw("ignore previous instructions", "high", "提示注入：要求忽略先前指令"),
			kw("disregard all previous", "high", "提示注入：要求无视先前指令"),
			kw("forget your instructions", "high", "提示注入：要求遗忘指令"),
			kw("system prompt", "medium", "探测系统提示词"),
			kw("reveal your system prompt", "high", "要求泄露系统提示词"),
			kw("jailbreak", "medium", "越狱关键词"),
			kw("developer mode", "medium", "开发者模式越狱"),
			kw("DAN", "medium", "DAN 越狱模式"),
			kw("do anything now", "medium", "越狱口令"),
		},
	},
	{
		ID:          "privacy-pii",
		Name:        "隐私保护",
		Description: "识别常见个人身份信息关键词与数据经纪商域名",
		Rules: []BuiltinTemplateRule{
			kw("social security number", "high", "美国社保号关键词"),
			kw("credit card number", "high", "信用卡号关键词"),
			kw("passport number", "high", "护照号关键词"),
			kw("date of birth", "medium", "出生日期关键词"),
			kw("phone number", "medium", "电话号码关键词"),
			kw("home address", "medium", "家庭住址关键词"),
			kw("email address", "medium", "邮箱地址关键词"),
			kw("bank account", "high", "银行账户关键词"),
			domain("spokeo.com", "high", "数据经纪商 Spokeo"),
			domain("peoplefinder.com", "high", "数据经纪商 PeopleFinder"),
			domain("whitepages.com", "medium", "数据聚合 Whitepages"),
			domain("fastpeoplesearch.com", "high", "数据经纪商 FastPeopleSearch"),
			domain("zabasearch.com", "medium", "公共记录聚合 Zabasearch"),
		},
	},
	{
		ID:          "api-key-leak",
		Name:        "API 密钥检测",
		Description: "正则匹配常见 API 密钥格式，防止密钥落入对话输出（默认全部停用，启用时受正则上限约束）",
		Rules: []BuiltinTemplateRule{
			regexRule(`sk-[A-Za-z0-9]{48}`, "high", false, "OpenAI 风格密钥"),
			regexRule(`AIza[0-9A-Za-z\-_]{35}`, "high", false, "Google API 密钥"),
			regexRule(`ghp_[A-Za-z0-9]{36}`, "high", false, "GitHub Personal Access Token"),
		},
	},
}

// GetBuiltinTemplate returns the built-in template with the given id and
// whether it exists.
func GetBuiltinTemplate(id string) (*BuiltinTemplate, bool) {
	for i := range BuiltinAuditTemplates {
		if BuiltinAuditTemplates[i].ID == id {
			return &BuiltinAuditTemplates[i], true
		}
	}
	return nil, false
}

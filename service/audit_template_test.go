package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuiltinTemplateReturnsExpected(t *testing.T) {
	tpl, ok := GetBuiltinTemplate("basic-security")
	require.True(t, ok)
	require.NotNil(t, tpl)
	assert.Equal(t, "basic-security", tpl.ID)
	require.NotEmpty(t, tpl.Rules)

	tpl, ok = GetBuiltinTemplate("privacy-pii")
	require.True(t, ok)
	require.NotNil(t, tpl)

	tpl, ok = GetBuiltinTemplate("api-key-leak")
	require.True(t, ok)
	require.NotNil(t, tpl)
}

func TestGetBuiltinTemplateMissing(t *testing.T) {
	tpl, ok := GetBuiltinTemplate("does-not-exist")
	assert.False(t, ok)
	assert.Nil(t, tpl)
}

// BR-116: 模板内 regex 规则启用态必须为 false。
func TestBuiltinTemplateRegexRulesDisabled(t *testing.T) {
	tpl, ok := GetBuiltinTemplate("api-key-leak")
	require.True(t, ok)
	regexCount := 0
	for _, r := range tpl.Rules {
		if r.Kind == model.WatchlistKindRegex {
			regexCount++
			assert.False(t, r.Enabled, "regex rule %q must ship disabled", r.Pattern)
		}
	}
	assert.Greater(t, regexCount, 0, "api-key-leak template should contain regex rules")
}

// BR-116 与非 regex 模板：basic-security 不应含 regex。
func TestBuiltinTemplateBasicSecurityNoRegex(t *testing.T) {
	tpl, ok := GetBuiltinTemplate("basic-security")
	require.True(t, ok)
	for _, r := range tpl.Rules {
		assert.NotEqual(t, model.WatchlistKindRegex, r.Kind)
	}
}

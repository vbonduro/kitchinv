package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vbonduro/kitchinv/internal/domain"
)

func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name    string
		rule    domain.OverrideRule
		input   string
		want    bool
	}{
		{
			name:  "exact match, case-sensitive, matches",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true},
			input: "Milk", want: true,
		},
		{
			name:  "exact match, case-sensitive, no match different case",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true},
			input: "milk", want: false,
		},
		{
			name:  "exact match, case-insensitive, matches",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true, MatchCaseInsensitive: true},
			input: "MILK", want: true,
		},
		{
			name:  "exact match, case-insensitive, no match on partial",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true, MatchCaseInsensitive: true},
			input: "Whole Milk", want: false,
		},
		{
			name:  "substring match, case-sensitive, matches",
			rule:  domain.OverrideRule{MatchPattern: "OJ", MatchSubstring: true},
			input: "Tropicana OJ 52oz", want: true,
		},
		{
			name:  "substring match, case-sensitive, no match different case",
			rule:  domain.OverrideRule{MatchPattern: "OJ", MatchSubstring: true},
			input: "Tropicana oj 52oz", want: false,
		},
		{
			name:  "substring match, case-insensitive, matches",
			rule:  domain.OverrideRule{MatchPattern: "oj", MatchSubstring: true, MatchCaseInsensitive: true},
			input: "Tropicana OJ 52oz", want: true,
		},
		{
			name:  "no flags set, never matches",
			rule:  domain.OverrideRule{MatchPattern: "Milk"},
			input: "Milk", want: false,
		},
		{
			name:  "both exact and substring, exact wins on full match",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true, MatchSubstring: true},
			input: "Milk", want: true,
		},
		{
			name:  "both exact and substring, substring matches partial",
			rule:  domain.OverrideRule{MatchPattern: "Milk", MatchExact: true, MatchSubstring: true},
			input: "Whole Milk", want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesRule(&tc.rule, tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestApplyOverrides_FirstRuleWins(t *testing.T) {
	rules := []*domain.OverrideRule{
		{MatchPattern: "Milk", Replacement: "First", MatchExact: true},
		{MatchPattern: "Milk", Replacement: "Second", MatchExact: true},
	}
	assert.Equal(t, "First", applyOverrides(rules, "Milk"))
}

func TestApplyOverrides_NoMatch_ReturnsOriginal(t *testing.T) {
	rules := []*domain.OverrideRule{
		{MatchPattern: "Butter", Replacement: "Butter", MatchExact: true},
	}
	assert.Equal(t, "Milk", applyOverrides(rules, "Milk"))
}

func TestApplyOverrides_EmptyReplacement(t *testing.T) {
	rules := []*domain.OverrideRule{
		{MatchPattern: "Milk", Replacement: "", MatchExact: true},
	}
	// empty replacement is valid (caller filters empty names)
	assert.Equal(t, "", applyOverrides(rules, "Milk"))
}

func TestApplyOverrides_EmptyRules(t *testing.T) {
	assert.Equal(t, "Milk", applyOverrides(nil, "Milk"))
}

package service

import (
	"strings"

	"github.com/vbonduro/kitchinv/internal/domain"
)

// applyOverrides returns the first replacement whose rule matches name,
// or name unchanged if no rule matches. Rules must arrive pre-sorted
// (ListForArea ordering: sort_order ASC, area-scoped before global).
func applyOverrides(rules []*domain.OverrideRule, name string) string {
	for _, r := range rules {
		if matchesRule(r, name) {
			return r.Replacement
		}
	}
	return name
}

// matchesRule returns true when name satisfies the rule's match flags.
// A rule with no flags set never matches.
func matchesRule(r *domain.OverrideRule, name string) bool {
	if r.MatchExact && !r.MatchCaseInsensitive && name == r.MatchPattern {
		return true
	}
	if r.MatchExact && r.MatchCaseInsensitive && strings.EqualFold(name, r.MatchPattern) {
		return true
	}
	if r.MatchSubstring && !r.MatchCaseInsensitive && strings.Contains(name, r.MatchPattern) {
		return true
	}
	if r.MatchSubstring && r.MatchCaseInsensitive &&
		strings.Contains(strings.ToLower(name), strings.ToLower(r.MatchPattern)) {
		return true
	}
	return false
}

package common

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	GitHubOAuthMinimumAgeYearsOptionKey = "GitHubOAuthMinimumAgeYears"
	DefaultGitHubOAuthMinimumAgeYears   = 1
	MaxGitHubOAuthMinimumAgeYears       = 100
)

// GitHubOAuthMinimumAgeYears is the minimum calendar age required when a new
// local user is created from a GitHub OAuth account. Zero disables the check.
var GitHubOAuthMinimumAgeYears = DefaultGitHubOAuthMinimumAgeYears

// ParseGitHubOAuthMinimumAgeYears validates the persisted option value and
// returns the configured number of calendar years. Whitespace around the
// integer is accepted because option values are stored as strings.
func ParseGitHubOAuthMinimumAgeYears(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("GitHub OAuth minimum account age must be an integer from 0 to %d", MaxGitHubOAuthMinimumAgeYears)
	}

	years, err := strconv.Atoi(trimmed)
	if err != nil || years < 0 || years > MaxGitHubOAuthMinimumAgeYears {
		return 0, fmt.Errorf("GitHub OAuth minimum account age must be an integer from 0 to %d", MaxGitHubOAuthMinimumAgeYears)
	}
	return years, nil
}

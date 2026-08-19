package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func preserveGitHubOAuthOptionRuntime(t *testing.T) {
	t.Helper()
	previousMap := common.OptionMap
	previousYears := common.GitHubOAuthMinimumAgeYears
	t.Cleanup(func() {
		common.OptionMap = previousMap
		common.GitHubOAuthMinimumAgeYears = previousYears
	})
}

func TestValidateGitHubOAuthMinimumAgeYearsOption(t *testing.T) {
	for _, value := range []string{"", "-1", "1.5", "101", "years"} {
		assert.Error(t, validateOptionValue(common.GitHubOAuthMinimumAgeYearsOptionKey, value))
	}
	for _, value := range []string{"0", "1", "100"} {
		require.NoError(t, validateOptionValue(common.GitHubOAuthMinimumAgeYearsOptionKey, value))
	}
}

func TestInitOptionMapPublishesGitHubOAuthMinimumAgeYears(t *testing.T) {
	tests := []struct {
		name      string
		persisted *string
		want      int
	}{
		{name: "default without persisted option", want: common.DefaultGitHubOAuthMinimumAgeYears},
		{name: "valid persisted option", persisted: func() *string { value := "3"; return &value }(), want: 3},
		{name: "invalid persisted option falls back", persisted: func() *string { value := "1.5"; return &value }(), want: common.DefaultGitHubOAuthMinimumAgeYears},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := useFrontendOptionMigrationDB(t)
			preserveGitHubOAuthOptionRuntime(t)
			common.OptionMap = map[string]string{}
			common.GitHubOAuthMinimumAgeYears = 42
			if test.persisted != nil {
				require.NoError(t, db.Create(&Option{
					Key:   common.GitHubOAuthMinimumAgeYearsOptionKey,
					Value: *test.persisted,
				}).Error)
			}

			InitOptionMap()

			assert.Equal(t, test.want, common.GitHubOAuthMinimumAgeYears)
			assert.Equal(t, common.Interface2String(test.want), common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])
		})
	}
}

func TestUpdateOptionPersistsGitHubOAuthMinimumAgeYears(t *testing.T) {
	db := useFrontendOptionMigrationDB(t)
	preserveGitHubOAuthOptionRuntime(t)
	common.OptionMap = map[string]string{}
	common.GitHubOAuthMinimumAgeYears = common.DefaultGitHubOAuthMinimumAgeYears

	require.NoError(t, UpdateOption(common.GitHubOAuthMinimumAgeYearsOptionKey, "3"))
	assert.Equal(t, 3, common.GitHubOAuthMinimumAgeYears)
	assert.Equal(t, "3", common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])
	assert.Equal(t, "3", requireOptionValue(t, db, common.GitHubOAuthMinimumAgeYearsOptionKey))

	require.Error(t, UpdateOption(common.GitHubOAuthMinimumAgeYearsOptionKey, "101"))
	assert.Equal(t, 3, common.GitHubOAuthMinimumAgeYears)
	assert.Equal(t, "3", common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])
	assert.Equal(t, "3", requireOptionValue(t, db, common.GitHubOAuthMinimumAgeYearsOptionKey))
}

func TestUpdateOptionMapGitHubOAuthMinimumAgeYearsFallsBackOnInvalidValue(t *testing.T) {
	preserveGitHubOAuthOptionRuntime(t)
	common.OptionMap = map[string]string{}
	common.GitHubOAuthMinimumAgeYears = 42

	err := updateOptionMap(common.GitHubOAuthMinimumAgeYearsOptionKey, "not-a-year")
	require.Error(t, err)
	assert.Equal(t, common.DefaultGitHubOAuthMinimumAgeYears, common.GitHubOAuthMinimumAgeYears)
	assert.Equal(t, "1", common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])

	require.NoError(t, updateOptionMap(common.GitHubOAuthMinimumAgeYearsOptionKey, " 3 "))
	assert.Equal(t, 3, common.GitHubOAuthMinimumAgeYears)
	assert.Equal(t, " 3 ", common.OptionMap[common.GitHubOAuthMinimumAgeYearsOptionKey])
}

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitHubOAuthMinimumAgeYears(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "default", value: "1", want: 1},
		{name: "disabled", value: "0", want: 0},
		{name: "trimmed", value: " 12 ", want: 12},
		{name: "maximum", value: "100", want: 100},
		{name: "negative", value: "-1", wantErr: true},
		{name: "fractional", value: "1.5", wantErr: true},
		{name: "overflow", value: "101", wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "invalid", value: "one", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseGitHubOAuthMinimumAgeYears(test.value)
			if test.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

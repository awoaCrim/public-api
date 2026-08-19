package oauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitHubCreatedAt(t *testing.T) {
	createdAt := parseGitHubCreatedAt([]byte(`"2020-02-29T12:34:56Z"`))
	require.NotNil(t, createdAt)
	assert.Equal(t, time.Date(2020, time.February, 29, 12, 34, 56, 0, time.UTC), *createdAt)
	assert.Nil(t, parseGitHubCreatedAt(nil))
	assert.Nil(t, parseGitHubCreatedAt([]byte("null")))
	assert.Nil(t, parseGitHubCreatedAt([]byte(`"not-a-timestamp"`)))
	assert.Nil(t, parseGitHubCreatedAt([]byte("123")))
}

func TestGitHubProviderGetUserInfoProjectsOptionalCreatedAt(t *testing.T) {
	tests := []struct {
		name          string
		createdAtJSON string
		wantCreatedAt *time.Time
	}{
		{
			name:          "valid timestamp",
			createdAtJSON: `,"created_at":"2020-02-29T12:34:56Z"`,
			wantCreatedAt: func() *time.Time {
				value := time.Date(2020, time.February, 29, 12, 34, 56, 0, time.UTC)
				return &value
			}(),
		},
		{name: "missing timestamp"},
		{name: "malformed timestamp", createdAtJSON: `,"created_at":"not-a-timestamp"`},
		{name: "wrong timestamp JSON type", createdAtJSON: `,"created_at":123`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				_, err := w.Write([]byte(`{"id":12345,"login":"octocat","name":"Octo Cat","email":"octo@example.com"` + test.createdAtJSON + `}`))
				assert.NoError(t, err)
			}))
			defer server.Close()

			provider := &GitHubProvider{
				userInfoURL: server.URL,
				httpClient:  server.Client(),
			}
			user, err := provider.GetUserInfo(t.Context(), &OAuthToken{AccessToken: "test-token"})

			require.NoError(t, err)
			require.NotNil(t, user)
			assert.Equal(t, "12345", user.ProviderUserID)
			assert.Equal(t, "octocat", user.Username)
			assert.Equal(t, "Octo Cat", user.DisplayName)
			assert.Equal(t, "octo@example.com", user.Email)
			assert.Equal(t, "octocat", user.Extra["legacy_id"])
			if test.wantCreatedAt == nil {
				assert.Nil(t, user.CreatedAt)
			} else {
				require.NotNil(t, user.CreatedAt)
				assert.Equal(t, *test.wantCreatedAt, *user.CreatedAt)
			}
		})
	}
}

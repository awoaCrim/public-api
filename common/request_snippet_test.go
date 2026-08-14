package common

import (
	"bytes"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRequestSnippetReadsOnlyPrefixAndPreservesStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(nil)
	payload := bytes.Repeat([]byte("a"), 4096)
	storage, err := CreateBodyStorage(payload)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, storage.Close()) })
	context.Set(KeyBodyStorage, storage)

	snippet := ExtractRequestSnippet(context)
	assert.Len(t, snippet, 2048)
	assert.Equal(t, string(payload[:2048]), snippet)

	body, err := storage.Bytes()
	require.NoError(t, err)
	assert.Equal(t, payload, body)
}

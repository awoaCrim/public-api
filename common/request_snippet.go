package common

import (
	"io"

	"github.com/gin-gonic/gin"
)

func ExtractRequestSnippet(c *gin.Context) string {
	if c == nil {
		return ""
	}
	storage, err := GetBodyStorage(c)
	if err != nil || storage.Size() == 0 {
		return ""
	}
	reader, err := storage.NewReader()
	if err != nil {
		return ""
	}
	defer reader.Close()
	const maxSnippetBytes = 2048
	body, err := io.ReadAll(io.LimitReader(reader, maxSnippetBytes))
	if err != nil {
		return ""
	}
	return string(body)
}

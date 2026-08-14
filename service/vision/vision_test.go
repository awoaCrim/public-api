package vision

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPNGDataURI renders a solid-color PNG and returns it as a data URI.
func testPNGDataURI(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeBase64Image(t *testing.T) {
	uri := testPNGDataURI(t, 8, 8, color.RGBA{R: 255, A: 255})
	img, err := DecodeBase64Image(uri)
	require.NoError(t, err)
	require.NotNil(t, img)

	// Non-data URI rejected.
	_, err = DecodeBase64Image("http://example.com/a.png")
	require.Error(t, err)

	// Missing comma rejected.
	_, err = DecodeBase64Image("data:image/png;base64")
	require.Error(t, err)

	// Oversized dimensions rejected without full decode.
	big := testPNGDataURI(t, 9000, 8, color.RGBA{B: 255, A: 255})
	_, err = DecodeBase64Image(big)
	require.Error(t, err)
}

func TestComputePhashAndHammingDistance(t *testing.T) {
	// Horizontal bars pattern (strong low-frequency structure).
	bars := func(offset int) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				if ((y + offset) / 8 % 2) == 0 {
					img.Set(x, y, color.RGBA{R: 255, A: 255})
				} else {
					img.Set(x, y, color.RGBA{B: 255, A: 255})
				}
			}
		}
		return img
	}
	// Deterministic pseudo-noise (no low-frequency structure).
	noise := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		state := uint32(0x12345678)
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				state = state*1664525 + 1013904223
				img.Set(x, y, color.RGBA{R: uint8(state >> 24), G: uint8(state >> 16), B: uint8(state >> 8), A: 255})
			}
		}
		return img
	}
	img1 := bars(0)
	img2 := bars(1) // one-pixel shift of the same pattern
	img3 := noise()

	h1, err := ComputePhash(img1)
	require.NoError(t, err)
	h2, err := ComputePhash(img2)
	require.NoError(t, err)
	h3, err := ComputePhash(img3)
	require.NoError(t, err)

	assert.Equal(t, 0, HammingDistance(h1, h1))
	assert.LessOrEqual(t, HammingDistance(h1, h2), 6)
	assert.Greater(t, HammingDistance(h1, h3), 6)
}

func TestExtractImagesOpenAIAndClaude(t *testing.T) {
	root := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://img.example/a.png"}},
					map[string]any{"type": "input_image", "image_url": map[string]any{"url": "data:image/png;base64,AAAA"}},
					map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": "https://img.example/b.png"}},
					map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "data:image/png;base64,BBBB"}},
				},
			},
		},
	}

	entries, err := ExtractImages(root)
	require.NoError(t, err)
	require.Len(t, entries, 4)
	assert.Equal(t, "https://img.example/a.png", entries[0].URL)
	assert.Equal(t, "data:image/png;base64,AAAA", entries[1].URL)
	assert.Equal(t, "https://img.example/b.png", entries[2].URL)
	assert.Equal(t, "data:image/png;base64,BBBB", entries[3].URL)
}

func TestExtractImagesRespectsCap(t *testing.T) {
	root := map[string]any{"messages": []any{}}
	var content []any
	for i := 0; i < MaxImagesPerRequest+1; i++ {
		content = append(content, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://img.example/a.png"}})
	}
	root["messages"] = []any{map[string]any{"role": "user", "content": content}}

	_, err := ExtractImages(root)
	require.Error(t, err)
}

func TestExtractImagesReplaceHooks(t *testing.T) {
	root := map[string]any{
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://img.example/a.png", "detail": "high"}},
					map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": "https://img.example/b.png"}},
				},
			},
		},
	}

	entries, err := ExtractImages(root)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// OpenAI part becomes a text part; image_url and detail are removed.
	entries[0].Replace("a cat")
	content := root["messages"].([]any)[0].(map[string]any)["content"].([]any)
	openaiPart := content[1].(map[string]any)
	assert.Equal(t, "text", openaiPart["type"])
	assert.Equal(t, "a cat", openaiPart["text"])
	assert.NotContains(t, openaiPart, "image_url")

	// Claude block is replaced wholesale by a text block.
	entries[1].Replace("a dog")
	claudeBlock := content[2].(map[string]any)
	assert.Equal(t, "text", claudeBlock["type"])
	assert.Equal(t, "a dog", claudeBlock["text"])
	assert.NotContains(t, claudeBlock, "source")

	out, err := common.Marshal(root)
	require.NoError(t, err)
	assert.Contains(t, string(out), "a cat")
	assert.Contains(t, string(out), "a dog")
	assert.NotContains(t, string(out), "img.example")
}

func TestClusterImages(t *testing.T) {
	// Two near-identical bar-pattern images and one checkerboard.
	toURI := func(img *image.RGBA) string {
		var buf bytes.Buffer
		require.NoError(t, png.Encode(&buf, img))
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	}
	bars := func(offset int) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				if ((y + offset) / 8 % 2) == 0 {
					img.Set(x, y, color.RGBA{R: 255, A: 255})
				} else {
					img.Set(x, y, color.RGBA{B: 255, A: 255})
				}
			}
		}
		return img
	}
	noise := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		state := uint32(0x12345678)
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				state = state*1664525 + 1013904223
				img.Set(x, y, color.RGBA{R: uint8(state >> 24), G: uint8(state >> 16), B: uint8(state >> 8), A: 255})
			}
		}
		return img
	}
	uriA := toURI(bars(0))
	uriB := toURI(bars(1))
	uriC := toURI(noise())

	entries := []ImageEntry{{URL: uriA}, {URL: uriB}, {URL: uriC}}
	groups := clusterImages(entries, 6)
	require.Len(t, groups, 2)

	// A and B must land in the same cluster.
	assert.Len(t, groups[0].entries, 2)
	assert.Len(t, groups[1].entries, 1)
}

func TestValidateImageURLRejectsPrivateTargets(t *testing.T) {
	require.Error(t, ValidateImageURL("http://127.0.0.1/x.png"))
	require.Error(t, ValidateImageURL("http://10.0.0.1/x.png"))
	require.Error(t, ValidateImageURL("http://169.254.169.254/latest/meta-data/"))
	require.Error(t, ValidateImageURL("file:///etc/passwd"))
}

func TestLookupCachedDescriptionIsolation(t *testing.T) {
	config := dto.UserVisionSetting{VisionModel: "m", PromptTemplate: "p"}
	// Distinct user ids must never share cached entries.
	_, found := LookupCachedDescription(1, "https://img.example/a.png", config, nil)
	assert.False(t, found)
}

func TestInterceptImagesFailsOpenOnInvalidImage(t *testing.T) {
	root := map[string]any{"model": "x"}
	entries := []ImageEntry{{URL: "http://127.0.0.1/x.png", Replace: func(string) {}}}
	err := InterceptImages(nil, root, entries, dto.UserVisionSetting{VisionModel: "m", PromptTemplate: "p"}, "req-1")
	require.Error(t, err)
}

func TestNewVisionSubContextInheritsParentCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	parent, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	parent.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)

	child := newVisionSubContext(parent, parent.Request.Context())
	cancel()

	require.ErrorIs(t, child.Request.Context().Err(), context.Canceled)
}

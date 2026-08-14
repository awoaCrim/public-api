package vision

import (
	"fmt"
	"image"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
)

// maxWalkDepth bounds recursive JSON traversal during image extraction.
const maxWalkDepth = 12

// ImageEntry is one extracted image reference with an in-place mutation hook
// that replaces the image entry with a text description.
type ImageEntry struct {
	URL string // HTTP(S) URL or data: URI
	// Replace replaces this image entry with a text description, mutating
	// the parsed request tree in place (format dependent).
	Replace func(desc string)
}

// ExtractImages walks the parsed request JSON and extracts bounded image
// entries together with their in-place replacement hooks. It handles both
// OpenAI-style parts (image_url / input_image) and Claude-style content
// blocks (image with url/base64 source).
func ExtractImages(root map[string]any) ([]ImageEntry, error) {
	var entries []ImageEntry
	if err := walkForImages(root, "", 0, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// walkForImages recursively inspects the parsed request tree. Images are
// recognized at the message-content level; arbitrary deep values are walked
// only to a bounded depth.
func walkForImages(value any, path string, depth int, entries *[]ImageEntry) error {
	if depth > maxWalkDepth {
		return nil
	}
	switch v := value.(type) {
	case map[string]any:
		// Claude-style content block or OpenAI-style part carrying an image.
		if part, kind, url, ok := extractImageFromPart(v); ok {
			if len(*entries) >= MaxImagesPerRequest {
				return fmt.Errorf("too many images in request (limit %d)", MaxImagesPerRequest)
			}
			e := ImageEntry{URL: url, Replace: buildReplacer(part, kind)}
			*entries = append(*entries, e)
		}
		// Descend only through known container keys to avoid walking raw
		// URLs/strings in unrelated fields.
		for _, key := range []string{"messages", "content", "input", "parts"} {
			if child, exists := v[key]; exists {
				if err := walkForImages(child, path+"."+key, depth+1, entries); err != nil {
					return err
				}
			}
		}
	case []any:
		for i, child := range v {
			if err := walkForImages(child, fmt.Sprintf("%s[%d]", path, i), depth+1, entries); err != nil {
				return err
			}
		}
	}
	return nil
}

// extractImageFromPart recognizes one image part/block. Returns the part
// map, the kind ("openai" | "claude") and the image URL/data URI.
func extractImageFromPart(part map[string]any) (map[string]any, string, string, bool) {
	partType, _ := part["type"].(string)
	switch partType {
	case "image_url", "input_image":
		inner, ok := part["image_url"].(map[string]any)
		if !ok {
			return nil, "", "", false
		}
		url, _ := inner["url"].(string)
		if url == "" {
			return nil, "", "", false
		}
		return part, "openai", url, true
	case "image":
		src, ok := part["source"].(map[string]any)
		if !ok {
			return nil, "", "", false
		}
		if url, _ := src["url"].(string); url != "" {
			return part, "claude", url, true
		}
		if data, _ := src["data"].(string); data != "" {
			return part, "claude", data, true
		}
	}
	return nil, "", "", false
}

// buildReplacer builds the in-place replacement hook for one image entry.
func buildReplacer(part map[string]any, kind string) func(string) {
	return func(desc string) {
		switch kind {
		case "openai":
			part["type"] = "text"
			part["text"] = desc
			delete(part, "image_url")
			delete(part, "input_image")
			delete(part, "index")
		case "claude":
			for key := range part {
				delete(part, key)
			}
			part["type"] = "text"
			part["text"] = desc
		}
	}
}

// InterceptImages clusters images by perceptual hash, resolves each cluster
// through cache or the vision sub-call, and replaces each image with its text
// description in place. Any failure is returned to the caller so the
// middleware can fail open with the untouched original body.
func InterceptImages(c *gin.Context, root map[string]any, entries []ImageEntry, config dto.UserVisionSetting, requestID string) error {
	if c == nil {
		return fmt.Errorf("nil request context")
	}
	userId := c.GetInt("id")
	groups := clusterImages(entries, config.PhashThreshold)
	for _, group := range groups {
		leader := group.entries[0]
		phash := group.hash
		desc, found := LookupCachedDescription(userId, leader.URL, config, phash)
		if !found {
			var err error
			desc, found, err = AnalyzeImage(c, c.Request.Context(), config, leader.URL, requestID, phash)
			if err != nil {
				return fmt.Errorf("vision analysis failed: %w", err)
			}
		}
		for _, entry := range group.entries {
			entry.Replace(desc)
		}
	}
	return nil
}

// imageGroup is one cluster of near-identical images sharing one
// description.
type imageGroup struct {
	hash    *uint64
	entries []ImageEntry
}

// clusterImages groups entries by perceptual hash distance (or exact URL
// hash when pHash is disabled). Order within groups follows request order,
// and group order follows the first appearance of its leader.
func clusterImages(entries []ImageEntry, phashThreshold int) []imageGroup {
	type resolved struct {
		entry ImageEntry
		hash  uint64
		ok    bool
	}
	resolvedEntries := make([]resolved, 0, len(entries))
	for _, e := range entries {
		if !isImageURL(e.URL) {
			continue
		}
		img, err := decodeImageForHash(e.URL)
		if err != nil {
			// A single undecodable image cannot be described; the middleware
			// will fail open.
			resolvedEntries = append(resolvedEntries, resolved{entry: e})
			continue
		}
		hash, err := ComputePhash(img)
		if err != nil {
			resolvedEntries = append(resolvedEntries, resolved{entry: e})
			continue
		}
		resolvedEntries = append(resolvedEntries, resolved{entry: e, hash: hash, ok: true})
	}

	var groups []imageGroup
	for _, re := range resolvedEntries {
		if !re.ok {
			groups = append(groups, imageGroup{entries: []ImageEntry{re.entry}})
			continue
		}
		matched := false
		for i := range groups {
			if groups[i].hash == nil {
				continue
			}
			if HammingDistance(*groups[i].hash, re.hash) <= phashThreshold {
				groups[i].entries = append(groups[i].entries, re.entry)
				matched = true
				break
			}
		}
		if !matched {
			hash := re.hash
			groups = append(groups, imageGroup{hash: &hash, entries: []ImageEntry{re.entry}})
		}
	}
	return groups
}

func isImageURL(url string) bool {
	return strings.HasPrefix(url, "data:") || strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

func decodeImageForHash(url string) (image.Image, error) {
	if strings.HasPrefix(url, "data:") {
		return DecodeBase64Image(url)
	}
	return DownloadImageForPhash(url)
}

package operation_setting

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// MaxPolicyTextRunes bounds the administrator-provided policy included in
// reviewer prompts and persisted task payloads.
const MaxPolicyTextRunes = 12000

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\([^\)]*\)`)
	markdownLinkPattern  = regexp.MustCompile(`\[([^\]]+)\]\([^\)]*\)`)
)

// NormalizePolicyText strips HTML/Markdown presentation wrappers and caps the
// length so policy readiness and reviewer payloads use the same text.
func NormalizePolicyText(raw string) string {
	text := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" {
		return ""
	}

	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		if plain, ok := extractHTMLText(text); ok {
			text = plain
		}
	}
	text = markdownImagePattern.ReplaceAllString(text, "$1")
	text = markdownLinkPattern.ReplaceAllString(text, "$1")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	lastBlank := true
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if !lastBlank {
				cleaned = append(cleaned, "")
				lastBlank = true
			}
			continue
		}
		cleaned = append(cleaned, line)
		lastBlank = false
	}
	text = strings.TrimSpace(strings.Join(cleaned, "\n"))

	runes := []rune(text)
	if len(runes) > MaxPolicyTextRunes {
		text = strings.TrimSpace(string(runes[:MaxPolicyTextRunes])) + "\n[policy text truncated]"
	}
	return text
}

func extractHTMLText(raw string) (string, bool) {
	doc, err := html.Parse(strings.NewReader("<div>" + raw + "</div>"))
	if err != nil {
		return "", false
	}
	var builder strings.Builder
	var walk func(*html.Node, bool)
	walk = func(node *html.Node, skipped bool) {
		if node.Type == html.ElementNode {
			switch strings.ToLower(node.Data) {
			case "script", "style", "noscript", "svg":
				skipped = true
			case "br", "p", "div", "section", "article", "header", "footer", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "tr":
				builder.WriteByte('\n')
			}
		}
		if node.Type == html.TextNode && !skipped {
			builder.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skipped)
		}
		if node.Type == html.ElementNode && !skipped {
			switch strings.ToLower(node.Data) {
			case "p", "div", "section", "article", "header", "footer", "li", "ul", "ol", "h1", "h2", "h3", "h4", "h5", "h6", "pre", "blockquote", "tr":
				builder.WriteByte('\n')
			}
		}
	}
	walk(doc, false)
	return builder.String(), true
}

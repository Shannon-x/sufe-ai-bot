package markdown

import (
	"regexp"
	"strings"

	"github.com/russross/blackfriday/v2"
)

// ToTelegramHTML converts markdown to Telegram-compatible HTML
func ToTelegramHTML(markdown string) string {
	if markdown == "" {
		return ""
	}

	// Convert markdown to HTML using blackfriday
	html := string(blackfriday.Run([]byte(markdown), blackfriday.WithExtensions(blackfriday.CommonExtensions)))

	// Clean up the HTML for Telegram
	html = cleanHTMLForTelegram(html)

	return html
}

// cleanHTMLForTelegram cleans HTML to be compatible with Telegram
func cleanHTMLForTelegram(html string) string {
	// Remove wrapping <p> tags
	html = regexp.MustCompile(`<p>(.*?)</p>`).ReplaceAllString(html, "$1\n")

	// Convert <strong> to <b>
	html = strings.ReplaceAll(html, "<strong>", "<b>")
	html = strings.ReplaceAll(html, "</strong>", "</b>")

	// Convert <em> to <i>
	html = strings.ReplaceAll(html, "<em>", "<i>")
	html = strings.ReplaceAll(html, "</em>", "</i>")

	// Handle code blocks
	html = regexp.MustCompile(`<pre><code(?: class="[^"]*")?>(.*?)</code></pre>`).ReplaceAllString(html, "<pre>$1</pre>")

	// Handle inline code
	html = regexp.MustCompile(`<code>(.*?)</code>`).ReplaceAllString(html, "<code>$1</code>")

	// Remove list tags but keep the content
	html = strings.ReplaceAll(html, "<ul>", "")
	html = strings.ReplaceAll(html, "</ul>", "")
	html = strings.ReplaceAll(html, "<ol>", "")
	html = strings.ReplaceAll(html, "</ol>", "")
	html = strings.ReplaceAll(html, "<li>", "• ")
	html = strings.ReplaceAll(html, "</li>", "\n")

	// Remove any other HTML tags that Telegram doesn't support
	supportedTags := []string{"b", "i", "u", "s", "code", "pre", "a", "br"}
	tagPattern := `</?([a-zA-Z]+)(?:\s[^>]*)?>` 
	
	html = regexp.MustCompile(tagPattern).ReplaceAllStringFunc(html, func(match string) string {
		// Extract tag name
		tagMatch := regexp.MustCompile(`</?([a-zA-Z]+)`).FindStringSubmatch(match)
		if len(tagMatch) > 1 {
			tagName := tagMatch[1]
			for _, supported := range supportedTags {
				if tagName == supported {
					return match
				}
			}
		}
		return ""
	})

	// Clean up extra newlines
	html = regexp.MustCompile(`\n{3,}`).ReplaceAllString(html, "\n\n")
	
	return strings.TrimSpace(html)
}
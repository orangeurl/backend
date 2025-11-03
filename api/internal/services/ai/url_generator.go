package ai

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// URLMetadata holds extracted information from a URL
type URLMetadata struct {
	Domain       string
	PathSegments []string
	PageTitle    string
	Keywords     []string
}

// GenerateSmartShortID creates a meaningful short ID from URL
// useAI: if true, uses Gemini Pro for intelligent generation (Premium feature)
// if false, uses local metadata extraction (Free/Pro feature)
func GenerateSmartShortID(targetURL string, useAI bool) (string, error) {
	// Parse URL and extract metadata
	metadata, err := extractMetadata(targetURL)
	if err != nil {
		log.Printf("[AI] Failed to extract metadata: %v", err)
		return "", err
	}

	// Try local generation first (always)
	localID := generateFromMetadata(metadata)

	// If AI is requested (Premium feature) and we have API key
	if useAI {
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Printf("[AI] GEMINI_API_KEY not set, falling back to local generation")
			return localID, nil
		}

		aiID, err := generateWithGemini(targetURL, metadata, apiKey)
		if err != nil {
			log.Printf("[AI] Gemini generation failed: %v, using local generation", err)
			return localID, nil
		}

		log.Printf("[AI] Generated with Gemini: %s (local was: %s)", aiID, localID)
		return aiID, nil
	}

	log.Printf("[AI] Generated locally: %s", localID)
	return localID, nil
}

// extractMetadata pulls information from the URL
func extractMetadata(targetURL string) (*URLMetadata, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	metadata := &URLMetadata{
		Domain:       parsed.Host,
		PathSegments: strings.Split(strings.Trim(parsed.Path, "/"), "/"),
	}

	// Try to fetch page title (with timeout) - optional enhancement
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "OrangeURL-Bot/1.0")

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			metadata.PageTitle = extractTitleFromHTML(resp.Body)
		}
	}

	// Extract keywords from path
	metadata.Keywords = extractKeywords(metadata.PathSegments, metadata.PageTitle)

	return metadata, nil
}

// extractTitleFromHTML extracts <title> tag from HTML
func extractTitleFromHTML(body io.Reader) string {
	// Read first 5KB only (title is usually in head)
	buf := make([]byte, 5120)
	n, _ := body.Read(buf)
	html := string(buf[:n])

	// Extract title using regex
	re := regexp.MustCompile(`<title[^>]*>(.*?)</title>`)
	matches := re.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// generateFromMetadata creates short ID using local logic (FREE)
func generateFromMetadata(metadata *URLMetadata) string {
	// Strategy 1: Use keywords if available
	if len(metadata.Keywords) > 0 {
		// Take first 2-3 keywords, max 10 chars total
		words := metadata.Keywords
		if len(words) > 3 {
			words = words[:3]
		}

		result := strings.Join(words, "")
		result = toPascalCase(result)

		// Ensure length is between 6-10 characters
		if len(result) > 10 {
			result = result[:10]
		}
		if len(result) >= 6 {
			return result
		}
	}

	// Strategy 2: Use last path segment
	if len(metadata.PathSegments) > 0 && metadata.PathSegments[0] != "" {
		last := metadata.PathSegments[len(metadata.PathSegments)-1]
		last = cleanString(last)
		if len(last) > 3 {
			cleaned := toPascalCase(last)
			if len(cleaned) > 10 {
				cleaned = cleaned[:10]
			}
			if len(cleaned) >= 6 {
				return cleaned
			}
		}
	}

	// Strategy 3: Use domain name
	domain := strings.TrimPrefix(metadata.Domain, "www.")
	domain = strings.Split(domain, ".")[0]
	domain = cleanString(domain)
	result := toPascalCase(domain)
	if len(result) < 6 {
		result = result + "Link"
	}
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

// generateWithGemini uses Google Gemini Pro for intelligent generation (PREMIUM)
func generateWithGemini(targetURL string, metadata *URLMetadata, apiKey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	// Use Gemini 1.5 Flash (faster and cheaper than Pro)
	model := client.GenerativeModel("gemini-1.5-flash")

	// Configure model
	model.SetTemperature(0.7)
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(50)

	// Build context-rich prompt
	contextInfo := ""
	if len(metadata.Keywords) > 0 {
		contextInfo = fmt.Sprintf("\nKeywords found: %s", strings.Join(metadata.Keywords, ", "))
	}
	if metadata.PageTitle != "" {
		contextInfo += fmt.Sprintf("\nPage title: %s", metadata.PageTitle)
	}

	prompt := fmt.Sprintf(`Generate a short, memorable URL slug for this URL: %s%s

Requirements:
- 6-10 characters long
- Use PascalCase (e.g., CoolStuff, BlogPost, ShopNow)
- Must be pronounceable and memorable
- Based on the URL content/path/meaning
- Return ONLY the slug, no explanation or punctuation

Examples:
- https://blog.com/how-to-cook-pasta → CookPasta
- https://shop.com/products/red-shoes → RedShoes
- https://github.com/user/awesome-project → AwesomeProj

URL slug:`, targetURL, contextInfo)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("Gemini API error: %w", err)
	}

	// Extract text from response
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty response from Gemini")
	}

	// Get the generated text
	generatedText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	shortID := cleanAIResponse(generatedText)

	// Validate result
	if len(shortID) < 4 || len(shortID) > 12 {
		return "", fmt.Errorf("invalid length from Gemini: %d chars", len(shortID))
	}

	// Ensure it's alphanumeric
	if !isAlphanumeric(shortID) {
		return "", fmt.Errorf("non-alphanumeric response from Gemini: %s", shortID)
	}

	return shortID, nil
}

// extractKeywords pulls meaningful words from path and title
func extractKeywords(segments []string, title string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
	}

	keywords := []string{}

	// Extract from path segments
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		words := strings.FieldsFunc(seg, func(r rune) bool {
			return r == '-' || r == '_' || r == '.'
		})
		for _, word := range words {
			word = strings.ToLower(cleanString(word))
			if len(word) > 2 && !stopWords[word] {
				keywords = append(keywords, word)
			}
		}
	}

	// Extract from title (if available)
	if title != "" {
		words := strings.Fields(title)
		for _, word := range words {
			word = strings.ToLower(cleanString(word))
			if len(word) > 3 && !stopWords[word] && len(keywords) < 5 {
				keywords = append(keywords, word)
			}
		}
	}

	return keywords
}

// Helper functions

func toPascalCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '.'
	})
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, "")
}

func cleanString(s string) string {
	// Remove all non-alphanumeric characters
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	return reg.ReplaceAllString(s, "")
}

func cleanAIResponse(s string) string {
	// Remove any markdown, quotes, or extra text
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'")
	s = strings.Split(s, "\n")[0] // Take first line only
	return cleanString(s)
}

func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

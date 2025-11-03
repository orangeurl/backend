package ai

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

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

// generateFromMetadata creates short ID using local logic with intelligent abbreviation
func generateFromMetadata(metadata *URLMetadata) string {
	// Strategy 1: Intelligent abbreviation from page title (best quality)
	if metadata.PageTitle != "" {
		abbreviated := abbreviateText(metadata.PageTitle, metadata.Domain)
		if len(abbreviated) >= 3 && len(abbreviated) <= 6 {
			return abbreviated
		}
	}

	// Strategy 2: Abbreviate from keywords
	if len(metadata.Keywords) > 0 {
		// Take first 1-2 keywords and abbreviate intelligently
		words := metadata.Keywords
		if len(words) > 2 {
			words = words[:2]
		}

		result := abbreviateKeywords(words)
		if len(result) >= 3 && len(result) <= 6 {
			return result
		}
	}

	// Strategy 3: Use path segment with abbreviation
	if len(metadata.PathSegments) > 0 {
		for i := len(metadata.PathSegments) - 1; i >= 0; i-- {
			segment := cleanString(metadata.PathSegments[i])
			if len(segment) > 2 {
				abbreviated := abbreviateText(segment, "")
				if len(abbreviated) >= 3 && len(abbreviated) <= 6 {
					return abbreviated
				}
			}
		}
	}

	// Strategy 4: Fallback - use domain initials + random suffix
	domain := strings.TrimPrefix(metadata.Domain, "www.")
	domain = strings.Split(domain, ".")[0]
	initials := getInitials(domain)
	if len(initials) < 3 {
		initials = initials + randomSuffix(3-len(initials))
	}
	return initials
}

// abbreviateText intelligently abbreviates text by removing vowels or taking initials
func abbreviateText(text string, excludeDomain string) string {
	text = cleanString(text)
	text = strings.ToLower(text)

	// Remove domain name from title if present (e.g., "Title - YouTube" → "Title")
	if excludeDomain != "" {
		domainName := strings.Split(strings.TrimPrefix(excludeDomain, "www."), ".")[0]
		text = strings.ReplaceAll(text, strings.ToLower(domainName), "")
		text = strings.TrimSpace(strings.Trim(text, "-|–—"))
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	// For single word: remove vowels intelligently
	if len(words) == 1 {
		return removeVowelsKeepStructure(words[0], 6)
	}

	// For multiple words: use initials + first letters
	if len(words) >= 2 {
		// Strategy A: First letters of first two words + number if present
		result := ""
		for i := 0; i < len(words) && len(result) < 6; i++ {
			word := cleanString(words[i])
			if word == "" {
				continue
			}
			// If word is a number, add it
			if isNumeric(word) {
				result += word
			} else {
				// Take first 2-3 letters
				takeLen := 2
				if i == 0 {
					takeLen = 3
				}
				if len(word) < takeLen {
					takeLen = len(word)
				}
				result += word[:takeLen]
			}
		}
		if len(result) >= 3 && len(result) <= 6 {
			return toPascalCase(result)
		}
	}

	// Fallback: take first 6 chars and remove vowels
	combined := strings.Join(words, "")
	return removeVowelsKeepStructure(combined, 6)
}

// abbreviateKeywords creates short form from keywords
func abbreviateKeywords(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}

	// Take first letters of each keyword
	result := ""
	for _, word := range keywords {
		if word == "" {
			continue
		}
		result += string(word[0])
		if len(word) > 1 {
			result += string(word[1])
		}
		if len(result) >= 6 {
			break
		}
	}

	return toPascalCase(result[:min(len(result), 6)])
}

// removeVowelsKeepStructure removes vowels but keeps consonants and numbers
func removeVowelsKeepStructure(text string, maxLen int) string {
	vowels := "aeiouAEIOU"
	result := ""

	// First pass: keep all consonants and numbers
	for _, ch := range text {
		if !strings.ContainsRune(vowels, ch) && (unicode.IsLetter(ch) || unicode.IsDigit(ch)) {
			result += string(ch)
			if len(result) >= maxLen {
				break
			}
		}
	}

	// If too short, add some vowels back
	if len(result) < 3 {
		for _, ch := range text {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				if !strings.ContainsRune(result, ch) {
					result += string(ch)
					if len(result) >= maxLen {
						break
					}
				}
			}
		}
	}

	return toPascalCase(result[:min(len(result), maxLen)])
}

// getInitials extracts initials from text
func getInitials(text string) string {
	words := strings.FieldsFunc(cleanString(text), func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	result := ""
	for _, word := range words {
		if len(word) > 0 {
			result += string(unicode.ToUpper(rune(word[0])))
			if len(result) >= 3 {
				break
			}
		}
	}
	return result
}

// randomSuffix generates random alphanumeric suffix
func randomSuffix(length int) string {
	chars := "0123456789abcdefghijklmnopqrstuvwxyz"
	result := ""
	for i := 0; i < length; i++ {
		result += string(chars[randInt(len(chars))])
	}
	return result
}

// randInt returns random int from 0 to max-1
func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// min returns minimum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// isNumeric checks if string is a number
func isNumeric(s string) bool {
	for _, ch := range s {
		if !unicode.IsDigit(ch) {
			return false
		}
	}
	return len(s) > 0
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

	// Configure model for creativity and variety
	model.SetTemperature(0.9) // Higher temperature for more variety
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(20) // Short outputs only

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
- 4-6 characters MAXIMUM (strictly enforce this)
- Mix of lowercase and uppercase letters (creative casing)
- Must be UNIQUE and creative each time
- Based on the page title and URL content, NOT the domain name
- Abbreviate intelligently (e.g., "JavaScript Tutorial" → "JsTut" or "JScript")
- Return ONLY the slug, no explanation or punctuation

Examples:
- Blog post about cooking pasta (title: "How to Cook Perfect Pasta") → "PstaCk" or "CookIt"
- YouTube video "10 JavaScript Tips" → "Js10" or "JsTips"
- GitHub project "awesome-react-hooks" → "RctHks" or "AwsHks"
- Google Search Console → "GscDash" or "SrchCn"

Be creative and focus on the CONTENT, not the domain. Make it SHORT (max 6 chars).

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
	if len(shortID) < 3 || len(shortID) > 6 {
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

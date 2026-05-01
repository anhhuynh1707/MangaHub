package data

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"mangahub/pkg/models"
)

// Scraper handles web scraping from educational practice sites.
type Scraper struct {
	Client *http.Client
}

// NewScraper creates a new scraper with a configured HTTP client.
func NewScraper() *Scraper {
	return &Scraper{
		Client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ScrapeQuotes scrapes quotes from quotes.toscrape.com.
// This is an educational practice site specifically designed for scraping exercises.
// It returns up to `pages` pages of quotes (10 quotes per page).
func (s *Scraper) ScrapeQuotes(pages int) ([]models.ScrapedQuote, error) {
	if pages < 1 {
		pages = 1
	}
	if pages > 10 {
		pages = 10
	}

	var allQuotes []models.ScrapedQuote

	for page := 1; page <= pages; page++ {
		url := fmt.Sprintf("http://quotes.toscrape.com/page/%d/", page)
		log.Printf("Scraping quotes from: %s", url)

		resp, err := s.Client.Get(url)
		if err != nil {
			log.Printf("Failed to fetch page %d: %v", page, err)
			break
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Failed to read page %d body: %v", page, err)
			break
		}

		quotes := parseQuotesHTML(string(body))
		if len(quotes) == 0 {
			log.Printf("No more quotes found on page %d, stopping", page)
			break
		}

		allQuotes = append(allQuotes, quotes...)
		log.Printf("Scraped %d quotes from page %d", len(quotes), page)

		// Rate limiting: be polite to the practice server
		if page < pages {
			time.Sleep(200 * time.Millisecond)
		}
	}

	log.Printf("✅ Total scraped: %d quotes from %d pages", len(allQuotes), pages)
	return allQuotes, nil
}

// parseQuotesHTML extracts quotes from the HTML of quotes.toscrape.com.
// Uses simple string parsing instead of a full HTML parser to keep dependencies minimal.
func parseQuotesHTML(html string) []models.ScrapedQuote {
	var quotes []models.ScrapedQuote

	// Split by quote blocks
	blocks := strings.Split(html, `class="quote"`)
	if len(blocks) < 2 {
		return nil
	}

	for _, block := range blocks[1:] { // Skip first split (before first quote)
		quote := models.ScrapedQuote{}

		// Extract quote text: between <span class="text" itemprop="text"> and </span>
		textStart := strings.Index(block, `itemprop="text">`)
		if textStart == -1 {
			continue
		}
		textStart += len(`itemprop="text">`)
		textEnd := strings.Index(block[textStart:], `</span>`)
		if textEnd == -1 {
			continue
		}
		rawText := block[textStart : textStart+textEnd]
		// Remove unicode quotes (â\x80\x9c and â\x80\x9d) and HTML entities
		rawText = strings.ReplaceAll(rawText, "\u201c", "")
		rawText = strings.ReplaceAll(rawText, "\u201d", "")
		rawText = strings.TrimSpace(rawText)
		quote.Text = rawText

		// Extract author: <small class="author" itemprop="author">Name</small>
		authorStart := strings.Index(block, `itemprop="author">`)
		if authorStart != -1 {
			authorStart += len(`itemprop="author">`)
			authorEnd := strings.Index(block[authorStart:], `</small>`)
			if authorEnd != -1 {
				quote.Author = strings.TrimSpace(block[authorStart : authorStart+authorEnd])
			}
		}

		// Extract tags: <a class="tag" href="...">tagname</a>
		var tags []string
		remaining := block
		for {
			tagStart := strings.Index(remaining, `class="tag"`)
			if tagStart == -1 {
				break
			}
			remaining = remaining[tagStart:]
			linkStart := strings.Index(remaining, `>`)
			if linkStart == -1 {
				break
			}
			linkEnd := strings.Index(remaining[linkStart:], `</a>`)
			if linkEnd == -1 {
				break
			}
			tag := strings.TrimSpace(remaining[linkStart+1 : linkStart+linkEnd])
			if tag != "" {
				tags = append(tags, tag)
			}
			remaining = remaining[linkStart+linkEnd+4:]
		}
		quote.Tags = tags

		if quote.Text != "" && quote.Author != "" {
			quotes = append(quotes, quote)
		}
	}

	return quotes
}

// TestHTTPBin tests various HTTP methods against httpbin.org.
// This is an educational exercise demonstrating HTTP request/response handling.
func (s *Scraper) TestHTTPBin() (map[string]interface{}, error) {
	results := make(map[string]interface{})

	// Test 1: GET request
	log.Println("HTTPBin: Testing GET request...")
	getResp, err := s.Client.Get("http://httpbin.org/get")
	if err != nil {
		return nil, fmt.Errorf("GET test failed: %w", err)
	}
	var getData map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getData)
	getResp.Body.Close()
	results["get"] = map[string]interface{}{
		"status":  getResp.StatusCode,
		"url":     getData["url"],
		"headers": getData["headers"],
	}

	// Test 2: POST request with JSON body
	log.Println("HTTPBin: Testing POST request...")
	postBody := strings.NewReader(`{"test":"mangahub","message":"hello from scraper"}`)
	postResp, err := s.Client.Post("http://httpbin.org/post", "application/json", postBody)
	if err != nil {
		return nil, fmt.Errorf("POST test failed: %w", err)
	}
	var postData map[string]interface{}
	json.NewDecoder(postResp.Body).Decode(&postData)
	postResp.Body.Close()
	results["post"] = map[string]interface{}{
		"status": postResp.StatusCode,
		"data":   postData["data"],
		"json":   postData["json"],
	}

	// Test 3: Custom headers
	log.Println("HTTPBin: Testing custom headers...")
	req, _ := http.NewRequest("GET", "http://httpbin.org/headers", nil)
	req.Header.Set("X-MangaHub", "scraper-test")
	req.Header.Set("X-Request-ID", fmt.Sprintf("mh-%d", time.Now().Unix()))
	headerResp, err := s.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("headers test failed: %w", err)
	}
	var headerData map[string]interface{}
	json.NewDecoder(headerResp.Body).Decode(&headerData)
	headerResp.Body.Close()
	results["headers"] = map[string]interface{}{
		"status":  headerResp.StatusCode,
		"headers": headerData["headers"],
	}

	// Test 4: Status codes
	log.Println("HTTPBin: Testing status codes...")
	statusCodes := []int{200, 201, 404, 500}
	statusResults := make(map[string]int)
	for _, code := range statusCodes {
		resp, err := s.Client.Get(fmt.Sprintf("http://httpbin.org/status/%d", code))
		if err != nil {
			statusResults[fmt.Sprintf("%d", code)] = -1
			continue
		}
		statusResults[fmt.Sprintf("%d", code)] = resp.StatusCode
		resp.Body.Close()
	}
	results["status_codes"] = statusResults

	// Test 5: User-Agent echo
	log.Println("HTTPBin: Testing user-agent...")
	uaResp, err := s.Client.Get("http://httpbin.org/user-agent")
	if err != nil {
		return nil, fmt.Errorf("user-agent test failed: %w", err)
	}
	var uaData map[string]interface{}
	json.NewDecoder(uaResp.Body).Decode(&uaData)
	uaResp.Body.Close()
	results["user_agent"] = uaData

	log.Println("✅ All HTTPBin tests completed")
	return results, nil
}

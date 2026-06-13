// Package dlmf is the library behind the dlmf command line:
// the HTTP client, request shaping, and the typed data models for
// the NIST Digital Library of Mathematical Functions.
//
// The Client sets a real User-Agent, paces requests to stay polite,
// and retries transient failures (429 and 5xx). Endpoint methods are
// built on top of Get and parse the DLMF HTML with stdlib strings only.
package dlmf

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to DLMF servers.
const DefaultUserAgent = "dlmf-cli/dev (+https://github.com/tamnd/dlmf-cli)"

// Config holds all tunable parameters for Client.
type Config struct {
	// BaseURL is the root of the DLMF site, typically "https://dlmf.nist.gov".
	BaseURL   string
	Rate      time.Duration
	Retries   int
	UserAgent string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://dlmf.nist.gov",
		Rate:      200 * time.Millisecond,
		Retries:   5,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to the DLMF over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Chapter is one of the 36 DLMF chapters.
type Chapter struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// SearchResult is one hit from the DLMF search engine.
type SearchResult struct {
	Rank    int    `json:"rank"`
	Title   string `json:"title"`
	Section string `json:"section"`
	URL     string `json:"url"`
}

// Chapters fetches the DLMF home page and returns all chapters.
// limit <= 0 means all 36.
func (c *Client) Chapters(ctx context.Context, limit int) ([]Chapter, error) {
	body, err := c.Get(ctx, c.cfg.BaseURL+"/")
	if err != nil {
		return nil, fmt.Errorf("chapters: %w", err)
	}
	chapters := parseChapters(string(body), c.cfg.BaseURL)
	if limit > 0 && limit < len(chapters) {
		chapters = chapters[:limit]
	}
	return chapters, nil
}

// Search queries the DLMF search engine and returns matching sections.
// limit <= 0 means return whatever the first page has (up to 10 by default).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	u := c.cfg.BaseURL + "/search/search?q=" + url.QueryEscape(query)
	body, err := c.Get(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	results := parseSearch(string(body), c.cfg.BaseURL)
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results, nil
}

// parseChapters extracts Chapter records from the DLMF home page HTML.
// It reads <link rel="chapter" href="./N" title="Chapter N Title"> elements,
// which are present in the <head> and are the most reliable source.
func parseChapters(html, baseURL string) []Chapter {
	var chapters []Chapter
	rest := html
	for {
		idx := strings.Index(rest, `<link rel="chapter"`)
		if idx == -1 {
			break
		}
		rest = rest[idx:]
		end := strings.Index(rest, ">")
		if end == -1 {
			break
		}
		tag := rest[:end+1]
		rest = rest[end+1:]

		href := attrVal(tag, "href")
		title := attrVal(tag, "title")
		if href == "" || title == "" {
			continue
		}

		// href is like "./1" or "./10"
		href = strings.TrimPrefix(href, "./")

		// title is like "Chapter 5 Gamma Function"
		// strip the leading "Chapter N " prefix
		num := 0
		shortTitle := title
		if strings.HasPrefix(title, "Chapter ") {
			after := strings.TrimPrefix(title, "Chapter ")
			// find the space after the number
			sp := strings.IndexByte(after, ' ')
			if sp != -1 {
				fmt.Sscanf(after[:sp], "%d", &num)
				shortTitle = strings.TrimSpace(after[sp+1:])
			}
		}

		chURL := baseURL + "/" + href
		chapters = append(chapters, Chapter{
			Number: num,
			Title:  shortTitle,
			URL:    chURL,
		})
	}
	return chapters
}

// parseSearch extracts SearchResult records from a DLMF search result page.
// Results appear as:
//
//	<h5 class="ltx_hittag">N: <a href="../10.1" title="" class="ltx_ref">
//	  <span class="ltx_tag ltx_tag_ref">10.1 </span>Special Notation</a>
func parseSearch(html, baseURL string) []SearchResult {
	var results []SearchResult
	rest := html
	rank := 0
	for {
		idx := strings.Index(rest, `class="ltx_hittag"`)
		if idx == -1 {
			break
		}
		rest = rest[idx:]
		// find the <a href inside this h5
		aIdx := strings.Index(rest, "<a href=")
		if aIdx == -1 {
			break
		}
		// find end of <a ...> opening tag
		aEnd := strings.Index(rest[aIdx:], ">")
		if aEnd == -1 {
			break
		}
		aTag := rest[aIdx : aIdx+aEnd+1]
		href := attrVal(aTag, "href")

		// now find the closing </a> to extract inner text as title
		afterA := rest[aIdx+aEnd+1:]
		closeA := strings.Index(afterA, "</a>")
		if closeA == -1 {
			rest = rest[aIdx+1:]
			continue
		}
		inner := afterA[:closeA]
		title := stripTags(inner)
		title = strings.TrimSpace(title)

		// href is like "../10.1" — strip leading "../"
		href = strings.TrimPrefix(href, "../")
		section := href

		// full URL
		fullURL := baseURL + "/" + href

		rank++
		results = append(results, SearchResult{
			Rank:    rank,
			Title:   title,
			Section: section,
			URL:     fullURL,
		})

		rest = rest[aIdx+1:]
	}
	return results
}

// attrVal extracts the value of attribute name from a single HTML tag string.
// It handles both single- and double-quoted values.
func attrVal(tag, name string) string {
	needle := name + `="`
	idx := strings.Index(tag, needle)
	if idx == -1 {
		// try single quote
		needle = name + `='`
		idx = strings.Index(tag, needle)
		if idx == -1 {
			return ""
		}
		rest := tag[idx+len(needle):]
		end := strings.IndexByte(rest, '\'')
		if end == -1 {
			return ""
		}
		return rest[:end]
	}
	rest := tag[idx+len(needle):]
	end := strings.IndexByte(rest, '"')
	if end == -1 {
		return ""
	}
	return rest[:end]
}

// stripTags removes all HTML tags from s and collapses whitespace.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '<':
			inTag = true
		case s[i] == '>':
			inTag = false
		case !inTag:
			b.WriteByte(s[i])
		}
	}
	// collapse runs of whitespace
	out := b.String()
	var res strings.Builder
	prevSpace := false
	for _, r := range out {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				res.WriteRune(' ')
			}
			prevSpace = true
		} else {
			res.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(res.String())
}

// Get fetches url and returns the response body. It paces and retries
// according to the client configuration.
func (c *Client) Get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

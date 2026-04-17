package showtimes

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"golang.org/x/net/html"
)

// Movie represents a movie with its showtimes at various theaters.
type Movie struct {
	Title    string
	Link     string
	Theaters []TheaterShowings
}

// TheaterShowings represents a single theater's showtimes for a movie.
type TheaterShowings struct {
	Name     string
	Address  string
	Distance string
	Showings []Showing
}

// Showing represents a showtime entry (may have a format like IMAX, Dolby, etc).
type Showing struct {
	Time   string
	Format string // e.g. "Standard", "IMAX", "Dolby Cinema"
}

// GoogleClient scrapes Google Search for movie showtimes using a
// TLS-fingerprinted HTTP client (same approach as the traveler tool).
type GoogleClient struct {
	httpClient tls_client.HttpClient
	mu         sync.Mutex
	lastReq    time.Time
}

const maxRPS = 5

// NewGoogleClient creates a new Google scraping client with Chrome TLS fingerprint.
func NewGoogleClient() (*GoogleClient, error) {
	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(profiles.Chrome_131),
		tls_client.WithCookieJar(jar),
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create TLS client: %w", err)
	}

	return &GoogleClient{
		httpClient: client,
	}, nil
}

// Search queries Google for movie showtimes near the given zip code.
func (c *GoogleClient) Search(zip, date string) ([]Movie, error) {
	c.rateLimit()

	// Build the Google search URL.
	query := fmt.Sprintf("showtimes near %s", zip)
	params := url.Values{}
	params.Set("q", query)
	params.Set("hl", "en")
	params.Set("gl", "us")

	searchURL := "https://www.google.com/search?" + params.Encode()

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers to mimic a real Chrome browser.
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseGoogleHTML(body)
}

// rateLimit ensures we don't exceed maxRPS requests per second.
func (c *GoogleClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	minInterval := time.Second / time.Duration(maxRPS)
	elapsed := time.Since(c.lastReq)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	c.lastReq = time.Now()
}

// parseGoogleHTML extracts movie showtime data from Google search results HTML.
// Google embeds showtime data in the server-rendered HTML within structured
// div elements. This parser walks the HTML tree looking for the showtime
// result blocks.
func parseGoogleHTML(body []byte) ([]Movie, error) {
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	// Strategy 1: Look for structured showtime data in the HTML.
	movies := extractShowtimesFromHTML(doc)
	if len(movies) > 0 {
		return movies, nil
	}

	// Strategy 2: Try to extract from script tags (Google sometimes embeds
	// structured data as JSON in script elements).
	movies = extractShowtimesFromScripts(doc)
	if len(movies) > 0 {
		return movies, nil
	}

	// Strategy 3: Fallback — extract any recognizable movie/theater text
	// patterns from the raw HTML text content.
	movies = extractShowtimesFromText(string(body))
	if len(movies) > 0 {
		return movies, nil
	}

	return nil, fmt.Errorf("could not parse showtime data from Google response (Google may require JavaScript rendering). Response size: %d bytes", len(body))
}

// extractShowtimesFromHTML walks the DOM looking for showtime result blocks.
// Google uses various div structures; we look for common patterns:
// - data-attrid="kc:/film/film:showtimes" or similar
// - Divs with class patterns containing "movie" or "showtime"
// - Theater name + address + time patterns
func extractShowtimesFromHTML(doc *html.Node) []Movie {
	var movies []Movie
	movieMap := make(map[string]*Movie)
	var movieOrder []string

	// Collect all text nodes and their context to find showtime patterns.
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		// Look for data-attrid attributes related to showtimes.
		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "data-attrid" && strings.Contains(attr.Val, "showtime") {
					// Found a showtime block — extract its content.
					blockMovies := extractMoviesFromBlock(n)
					for _, m := range blockMovies {
						if _, exists := movieMap[m.Title]; !exists {
							movieMap[m.Title] = &m
							movieOrder = append(movieOrder, m.Title)
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// If data-attrid approach didn't work, try class-based extraction.
	if len(movieOrder) == 0 {
		movies = extractByClassPatterns(doc)
		if len(movies) > 0 {
			return movies
		}
	}

	for _, name := range movieOrder {
		movies = append(movies, *movieMap[name])
	}
	return movies
}

// extractMoviesFromBlock extracts movie information from a showtime block node.
func extractMoviesFromBlock(block *html.Node) []Movie {
	var movies []Movie
	texts := collectTexts(block)

	// Try to identify movie titles, theater names, and times from text content.
	var currentMovie *Movie
	var currentTheater *TheaterShowings

	for _, t := range texts {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		// Time pattern: matches "1:30pm", "10:00 AM", etc.
		if isShowtime(t) {
			if currentTheater != nil {
				currentTheater.Showings = append(currentTheater.Showings, Showing{
					Time:   t,
					Format: "Standard",
				})
			}
			continue
		}

		// If it looks like an address (contains digits and common address words).
		if isAddress(t) {
			if currentTheater != nil {
				currentTheater.Address = t
			}
			continue
		}

		// If it looks like a distance ("2.5 mi" or "5 miles").
		if isDistance(t) {
			if currentTheater != nil {
				currentTheater.Distance = t
			}
			continue
		}
	}

	if currentMovie != nil {
		if currentTheater != nil {
			currentMovie.Theaters = append(currentMovie.Theaters, *currentTheater)
		}
		movies = append(movies, *currentMovie)
	}

	return movies
}

// extractByClassPatterns looks for showtime-related elements by class name patterns.
func extractByClassPatterns(doc *html.Node) []Movie {
	var movies []Movie
	movieMap := make(map[string]*Movie)
	var movieOrder []string

	// Look for common Google result card patterns.
	var findCards func(*html.Node)
	findCards = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			classes := getAttr(n, "class")
			// Google uses various class names for showtime cards.
			if containsAny(classes, "MiPcId", "lr_c_fce", "tb_c", "s6JM6d") {
				title, theaters := parseShowtimeCard(n)
				if title != "" && len(theaters) > 0 {
					if _, exists := movieMap[title]; !exists {
						m := &Movie{Title: title, Theaters: theaters}
						movieMap[title] = m
						movieOrder = append(movieOrder, title)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findCards(c)
		}
	}
	findCards(doc)

	for _, name := range movieOrder {
		movies = append(movies, *movieMap[name])
	}
	return movies
}

// parseShowtimeCard parses a single showtime card element.
func parseShowtimeCard(node *html.Node) (string, []TheaterShowings) {
	texts := collectTexts(node)
	if len(texts) == 0 {
		return "", nil
	}

	var title string
	var theaters []TheaterShowings
	var currentTheater *TheaterShowings

	for _, t := range texts {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}

		if isShowtime(t) {
			if currentTheater != nil {
				currentTheater.Showings = append(currentTheater.Showings, Showing{
					Time:   t,
					Format: "Standard",
				})
			}
		} else if title == "" && !isAddress(t) && !isDistance(t) && len(t) > 2 {
			title = t
		} else if !isAddress(t) && !isDistance(t) && !isShowtime(t) && len(t) > 3 {
			// Could be a theater name.
			if currentTheater != nil && len(currentTheater.Showings) > 0 {
				theaters = append(theaters, *currentTheater)
			}
			ct := TheaterShowings{Name: t}
			currentTheater = &ct
		} else if isAddress(t) && currentTheater != nil {
			currentTheater.Address = t
		} else if isDistance(t) && currentTheater != nil {
			currentTheater.Distance = t
		}
	}

	if currentTheater != nil && len(currentTheater.Showings) > 0 {
		theaters = append(theaters, *currentTheater)
	}

	return title, theaters
}

// extractShowtimesFromScripts looks for JSON data embedded in script tags.
func extractShowtimesFromScripts(doc *html.Node) []Movie {
	// Google sometimes embeds structured data in script tags.
	// Look for JSON-LD or other embedded data.
	var scripts []string
	var findScripts func(*html.Node)
	findScripts = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.TextNode {
					scripts = append(scripts, c.Data)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findScripts(c)
		}
	}
	findScripts(doc)

	// Look for showtime-related data in scripts.
	for _, script := range scripts {
		if strings.Contains(script, "showtime") || strings.Contains(script, "theater") || strings.Contains(script, "cinema") {
			movies := parseShowtimeScript(script)
			if len(movies) > 0 {
				return movies
			}
		}
	}

	return nil
}

// parseShowtimeScript attempts to extract showtime data from a script tag's content.
func parseShowtimeScript(script string) []Movie {
	// Look for arrays of showtime data in the script content.
	// Google's internal data format often uses nested arrays.
	var movies []Movie

	// Extract movie titles — look for patterns like ["Movie Title",...]
	// This is a best-effort heuristic parser.
	movieTitleRe := regexp.MustCompile(`"([A-Z][^"]{2,50})"`)
	timeRe := regexp.MustCompile(`"(\d{1,2}:\d{2}\s*[AaPp][Mm])"`)

	titles := movieTitleRe.FindAllStringSubmatch(script, -1)
	times := timeRe.FindAllStringSubmatch(script, -1)

	if len(titles) > 0 && len(times) > 0 {
		// Group times with the nearest preceding title.
		seen := make(map[string]bool)
		for _, t := range titles {
			name := t[1]
			if seen[name] || len(name) < 3 {
				continue
			}
			seen[name] = true
			movies = append(movies, Movie{Title: name})
		}

		// Attach times to the first movie as a fallback.
		if len(movies) > 0 {
			theater := TheaterShowings{Name: "Nearby Theater"}
			for _, t := range times {
				theater.Showings = append(theater.Showings, Showing{
					Time:   t[1],
					Format: "Standard",
				})
			}
			movies[0].Theaters = append(movies[0].Theaters, theater)
		}
	}

	return movies
}

// extractShowtimesFromText uses regex patterns on the raw HTML to find
// showtime-like data that may be embedded in attributes or text.
func extractShowtimesFromText(body string) []Movie {
	var movies []Movie

	// Look for patterns that Google uses in aria-labels and text.
	// e.g., aria-label="Showtimes for Movie Title at Theater Name"
	ariaRe := regexp.MustCompile(`aria-label="[Ss]howtimes?\s+(?:for\s+)?(.+?)\s+at\s+(.+?)"`)
	matches := ariaRe.FindAllStringSubmatch(body, -1)

	movieMap := make(map[string]*Movie)
	var movieOrder []string

	for _, m := range matches {
		movieTitle := html.UnescapeString(m[1])
		theaterName := html.UnescapeString(m[2])

		movie, exists := movieMap[movieTitle]
		if !exists {
			movie = &Movie{Title: movieTitle}
			movieMap[movieTitle] = movie
			movieOrder = append(movieOrder, movieTitle)
		}

		// Check if this theater already exists for this movie.
		found := false
		for _, t := range movie.Theaters {
			if t.Name == theaterName {
				found = true
				break
			}
		}
		if !found {
			movie.Theaters = append(movie.Theaters, TheaterShowings{
				Name: theaterName,
			})
		}
	}

	// Also try to find time strings near theater/movie mentions.
	timeRe := regexp.MustCompile(`(\d{1,2}:\d{2}\s*[AaPp][Mm])`)
	allTimes := timeRe.FindAllString(body, -1)

	// Distribute times to theaters if we found some.
	if len(movieOrder) > 0 && len(allTimes) > 0 {
		// Simple heuristic: distribute times across theaters.
		for _, name := range movieOrder {
			movie := movieMap[name]
			for i := range movie.Theaters {
				if len(movie.Theaters[i].Showings) == 0 && len(allTimes) > 0 {
					// Take some times for this theater.
					take := min(5, len(allTimes))
					for _, t := range allTimes[:take] {
						movie.Theaters[i].Showings = append(movie.Theaters[i].Showings, Showing{
							Time:   t,
							Format: "Standard",
						})
					}
					allTimes = allTimes[take:]
				}
			}
		}
	}

	for _, name := range movieOrder {
		movies = append(movies, *movieMap[name])
	}
	return movies
}

// Helper functions.

var (
	timePattern     = regexp.MustCompile(`(?i)^\d{1,2}:\d{2}\s*[ap]\.?m\.?$`)
	addressPattern  = regexp.MustCompile(`\d+\s+\w+\s+(St|Ave|Blvd|Dr|Rd|Way|Ln|Ct|Pl|Pkwy|Hwy)`)
	distancePattern = regexp.MustCompile(`(?i)^\d+\.?\d*\s*(mi|miles?|km)$`)
)

func isShowtime(s string) bool {
	return timePattern.MatchString(strings.TrimSpace(s))
}

func isAddress(s string) bool {
	return addressPattern.MatchString(s)
}

func isDistance(s string) bool {
	return distancePattern.MatchString(strings.TrimSpace(s))
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func collectTexts(n *html.Node) []string {
	var texts []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t != "" {
				texts = append(texts, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return texts
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// FormatShowings returns a compact string of times grouped by format.
func FormatShowings(showings []Showing) string {
	byFormat := make(map[string][]string)
	var formatOrder []string
	for _, s := range showings {
		if _, exists := byFormat[s.Format]; !exists {
			formatOrder = append(formatOrder, s.Format)
		}
		byFormat[s.Format] = append(byFormat[s.Format], s.Time)
	}

	var parts []string
	for _, f := range formatOrder {
		times := strings.Join(byFormat[f], ", ")
		if f == "Standard" || f == "" {
			parts = append(parts, times)
		} else {
			parts = append(parts, fmt.Sprintf("[%s] %s", f, times))
		}
	}
	return strings.Join(parts, "  |  ")
}

package showtimes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
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

// GoogleClient uses a headless browser to render Google showtime results.
type GoogleClient struct{}

// NewGoogleClient creates a new client.
func NewGoogleClient() (*GoogleClient, error) {
	return &GoogleClient{}, nil
}

// Search queries Google for movie showtimes near the given zip code.
// It launches a headless Chrome browser via Rod to render the JavaScript-heavy
// Google search results page, then extracts showtime data from the live DOM.
func (c *GoogleClient) Search(zip, date string) ([]Movie, error) {
	// Launch headless Chrome with stealth settings to avoid bot detection.
	l := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("disable-blink-features", "AutomationControlled").
		MustLaunch()

	browser := rod.New().ControlURL(l).MustConnect()
	defer browser.MustClose()

	// Use stealth page to minimize bot detection.
	page := stealth.MustPage(browser)

	// Visit Google homepage first to establish a session/cookies.
	if err := page.Navigate("https://www.google.com"); err != nil {
		return nil, fmt.Errorf("navigate to google.com: %w", err)
	}
	page.MustWaitStable()
	time.Sleep(2 * time.Second)

	// Accept cookie consent if present.
	page.MustEval(`() => {
		const btns = document.querySelectorAll('button');
		for (const btn of btns) {
			if (btn.textContent.includes('Accept all') || btn.textContent.includes('I agree')) {
				btn.click();
				return true;
			}
		}
		return false;
	}`)
	time.Sleep(500 * time.Millisecond)

	// Navigate to showtime search results.
	query := fmt.Sprintf("showtimes near %s", zip)
	url := fmt.Sprintf("https://www.google.com/search?q=%s&hl=en&gl=us",
		strings.ReplaceAll(query, " ", "+"))

	if err := page.Navigate(url); err != nil {
		return nil, fmt.Errorf("navigate to search: %w", err)
	}
	page.MustWaitStable()
	time.Sleep(4 * time.Second)

	// Check for CAPTCHA / block page.
	finalURL := page.MustEval(`() => window.location.href`).Str()
	if strings.Contains(finalURL, "/sorry/") {
		return nil, fmt.Errorf("Google blocked the request (CAPTCHA). This IP may be rate-limited. Try again in a few minutes")
	}

	htmlSize := page.MustEval(`() => document.documentElement.outerHTML.length`).Int()
	if htmlSize < 50000 {
		return nil, fmt.Errorf("received unexpectedly small response (%d bytes), possibly blocked by Google", htmlSize)
	}

	// Extract showtime data from the rendered DOM using JavaScript.
	// This script walks the DOM and extracts structured showtime data.
	result := page.MustEval(`() => {
		const movies = [];
		const text = document.body.innerText;
		const lines = text.split('\n').map(l => l.trim()).filter(l => l);

		// Strategy 1: Look for structured showtime blocks in Google's result format.
		// Google typically renders showtimes in a card/widget format with:
		// - Movie title headings
		// - Theater names with addresses
		// - Time buttons/links

		// Find all time-like patterns and their surrounding context.
		const timeRe = /\d{1,2}:\d{2}\s*[AaPp]\.?[Mm]\.?/g;

		// First, try to find the showtime widget container.
		// Google puts showtime results in specific containers.
		const allDivs = document.querySelectorAll('div');
		let showtimeContainer = null;

		for (const div of allDivs) {
			const text = div.innerText || '';
			const times = text.match(timeRe);
			if (times && times.length >= 3) {
				// Found a div with multiple showtimes.
				// Check if it's a reasonable size (not the whole page).
				if (text.length > 100 && text.length < 20000) {
					showtimeContainer = div;
					break;
				}
			}
		}

		if (!showtimeContainer) {
			// Fallback: try to parse from body text.
			return { error: "no showtime container found", textSample: text.substring(0, 2000) };
		}

		// Parse the showtime container.
		// The structure varies, but generally follows this pattern:
		// [Movie Title]
		//   [Rating info, runtime]
		//   [Theater Name] [Distance]
		//   [Address]
		//   [Time1] [Time2] [Time3]
		//   [Another Theater Name]
		//   [Times...]

		const containerText = showtimeContainer.innerText;
		const containerLines = containerText.split('\n').map(l => l.trim()).filter(l => l);

		let currentMovie = null;
		let currentTheater = null;
		const movieMap = new Map();
		const movieOrder = [];

		const isTime = (s) => /^\d{1,2}:\d{2}\s*[AaPp]\.?[Mm]\.?$/.test(s.trim());
		const isTimeLine = (s) => {
			// A line is a "time line" if it contains multiple times or is just a single time.
			const times = s.match(timeRe);
			return times && times.length >= 1;
		};
		const isAddress = (s) => /\d+\s+\w+\s+(St|Ave|Blvd|Dr|Rd|Way|Ln|Ct|Pl|Pkwy|Hwy|Street|Avenue|Boulevard|Drive|Road)/i.test(s);
		const isDistance = (s) => /^\d+\.?\d*\s*(mi|miles?|km)\s*$/i.test(s.trim());
		const isRating = (s) => /^(G|PG|PG-13|R|NC-17|NR|Not Rated)\s*$/i.test(s.trim()) || /^\d+h\s*\d*m?$/.test(s.trim()) || /^\d+\s*hr\s*\d*\s*min/.test(s.trim());
		const isMovieGenre = (s) => /^(Action|Adventure|Animation|Comedy|Crime|Documentary|Drama|Family|Fantasy|Horror|Music|Mystery|Romance|Sci-Fi|Thriller|War|Western)\s*$/i.test(s.trim());
		const isNavOrUI = (s) => /^(All|Images|Videos|Maps|News|Shopping|More|Tools|Settings|Sign in|Search|About|Help|Privacy|Terms)$/i.test(s.trim());

		// More nuanced parsing: look for elements that are links (likely movie titles or theater names)
		const links = showtimeContainer.querySelectorAll('a');
		const linkTexts = new Set();
		for (const link of links) {
			const text = link.innerText.trim();
			const href = link.href || '';
			if (text && text.length > 2 && text.length < 80) {
				linkTexts.add(text);
			}
		}

		// Try a simpler approach: just extract all times and group them with nearby headings.
		for (let i = 0; i < containerLines.length; i++) {
			const line = containerLines[i];

			// Skip empty, nav, and UI lines.
			if (isNavOrUI(line) || line.length <= 1) continue;

			// If this line contains times, add them to current theater.
			if (isTimeLine(line)) {
				const times = line.match(timeRe);
				if (times && currentTheater) {
					for (const t of times) {
						currentTheater.showings.push({ time: t.trim(), format: "Standard" });
					}
				}
				continue;
			}

			// Skip known patterns.
			if (isDistance(line) || isRating(line) || isMovieGenre(line)) {
				if (isDistance(line) && currentTheater) {
					currentTheater.distance = line.trim();
				}
				continue;
			}

			if (isAddress(line)) {
				if (currentTheater) {
					currentTheater.address = line.trim();
				}
				continue;
			}

			// Potential movie or theater name.
			// Heuristic: if the next few lines contain times, this is a theater name.
			// If the line after this is another potential name followed by times, this is a movie title.
			let nextTimeLineIdx = -1;
			for (let j = i + 1; j < Math.min(i + 5, containerLines.length); j++) {
				if (isTimeLine(containerLines[j])) {
					nextTimeLineIdx = j;
					break;
				}
			}

			if (nextTimeLineIdx > 0) {
				// This line is likely a theater name (times follow closely).
				// But first, check if this is actually a movie title with theaters below.
				// A movie title usually appears with rating/runtime info nearby.

				// Simple heuristic: if the line is in linkTexts and seems like a proper name,
				// treat it based on whether there's already a current movie.
				if (!currentMovie) {
					// First potential heading - treat as movie.
					currentMovie = { title: line, theaters: [] };
					movieMap.set(line, currentMovie);
					movieOrder.push(line);
					currentTheater = { name: "Unknown Theater", address: "", distance: "", showings: [] };
					currentMovie.theaters.push(currentTheater);
				} else {
					// Already have a movie - this could be a theater name.
					if (currentTheater && currentTheater.showings.length > 0) {
						// Previous theater had times, start a new theater.
						currentTheater = { name: line, address: "", distance: "", showings: [] };
						currentMovie.theaters.push(currentTheater);
					} else if (currentTheater && currentTheater.name === "Unknown Theater") {
						currentTheater.name = line;
					} else {
						// Could be a new movie or theater.
						// If this looks more like a movie title (longer, mixed case), treat as movie.
						if (line.length > 15 || /[A-Z].*[a-z].*[A-Z]/.test(line)) {
							// Possibly a new movie title
							if (currentTheater && currentTheater.showings.length === 0) {
								// Remove empty theater.
								currentMovie.theaters = currentMovie.theaters.filter(t => t.showings.length > 0);
							}
							currentMovie = { title: line, theaters: [] };
							movieMap.set(line, currentMovie);
							movieOrder.push(line);
							currentTheater = null;
						} else {
							currentTheater = { name: line, address: "", distance: "", showings: [] };
							currentMovie.theaters.push(currentTheater);
						}
					}
				}
			} else if (currentMovie && !isTimeLine(line) && line.length > 3) {
				// No times nearby - could be a new movie title.
				// Check if the previous movie has any theater data.
				if (currentMovie.theaters.some(t => t.showings.length > 0)) {
					// Previous movie had data, this might be a new movie.
					currentMovie = { title: line, theaters: [] };
					movieMap.set(line, currentMovie);
					movieOrder.push(line);
					currentTheater = null;
				}
			}
		}

		// Build result.
		const result = [];
		for (const title of movieOrder) {
			const m = movieMap.get(title);
			const theaters = m.theaters.filter(t => t.showings.length > 0);
			if (theaters.length > 0) {
				result.push({ title: m.title, theaters: theaters });
			}
		}

		return {
			movies: result,
			containerTextLength: containerText.length,
			lineCount: containerLines.length,
		};
	}`)

	// Parse the JS result.
	if errMsg := result.Get("error").Str(); errMsg != "" {
		textSample := result.Get("textSample").Str()
		return nil, fmt.Errorf("%s. Page text sample: %s", errMsg, truncate(textSample, 500))
	}

	moviesJSON := result.Get("movies").JSON("", "")
	if moviesJSON == "" || moviesJSON == "null" {
		return nil, fmt.Errorf("no movies extracted from DOM")
	}

	var rawMovies []struct {
		Title    string `json:"title"`
		Theaters []struct {
			Name     string `json:"name"`
			Address  string `json:"address"`
			Distance string `json:"distance"`
			Showings []struct {
				Time   string `json:"time"`
				Format string `json:"format"`
			} `json:"showings"`
		} `json:"theaters"`
	}

	if err := json.Unmarshal([]byte(moviesJSON), &rawMovies); err != nil {
		return nil, fmt.Errorf("parse extracted data: %w", err)
	}

	var movies []Movie
	for _, rm := range rawMovies {
		m := Movie{Title: rm.Title}
		for _, rt := range rm.Theaters {
			t := TheaterShowings{
				Name:     rt.Name,
				Address:  rt.Address,
				Distance: rt.Distance,
			}
			for _, rs := range rt.Showings {
				t.Showings = append(t.Showings, Showing{
					Time:   rs.Time,
					Format: rs.Format,
				})
			}
			m.Theaters = append(m.Theaters, t)
		}
		movies = append(movies, m)
	}

	return movies, nil
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

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

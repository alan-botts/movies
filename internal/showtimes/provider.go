package showtimes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

// SerpApiProvider searches for showtimes using SerpApi's Google search.
type SerpApiProvider struct {
	apiKey string
}

// NewSerpApiProvider creates a new SerpApi-backed showtime provider.
// Reads the API key from SERPAPI_API_KEY environment variable.
func NewSerpApiProvider() (*SerpApiProvider, error) {
	key := os.Getenv("SERPAPI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("SERPAPI_API_KEY environment variable is not set.\n\nTo get an API key:\n  1. Sign up at https://serpapi.com (free tier: 250 searches/month)\n  2. Copy your API key from the dashboard\n  3. Export it: export SERPAPI_API_KEY=your_key_here")
	}
	return &SerpApiProvider{apiKey: key}, nil
}

// Search queries SerpApi for movie showtimes near the given zip code.
func (p *SerpApiProvider) Search(zip, date string) ([]Movie, error) {
	// Build the SerpApi request.
	params := url.Values{}
	params.Set("engine", "google")
	params.Set("q", fmt.Sprintf("showtimes near %s", zip))
	params.Set("api_key", p.apiKey)
	// gl=us ensures US-centric results
	params.Set("gl", "us")
	params.Set("hl", "en")

	apiURL := "https://serpapi.com/search.json?" + params.Encode()

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseResponse(body)
}

// serpApiResponse represents the top-level SerpApi JSON response.
type serpApiResponse struct {
	Showtimes []serpApiShowtimeDay `json:"showtimes"`
}

type serpApiShowtimeDay struct {
	Day      string           `json:"day"`
	Date     string           `json:"date"`
	Movies   []serpApiMovie   `json:"movies"`
	Theaters []serpApiTheater `json:"theaters"`
}

type serpApiMovie struct {
	Name     string                `json:"name"`
	Link     string                `json:"link"`
	Theaters []serpApiMovieTheater `json:"theaters"`
}

type serpApiMovieTheater struct {
	Name     string           `json:"name"`
	Link     string           `json:"link"`
	Address  string           `json:"address"`
	Distance string           `json:"distance"`
	Showing  []serpApiShowing `json:"showing"`
}

type serpApiTheater struct {
	Name     string                `json:"name"`
	Link     string                `json:"link"`
	Address  string                `json:"address"`
	Distance string                `json:"distance"`
	Movies   []serpApiTheaterMovie `json:"movies"`
}

type serpApiTheaterMovie struct {
	Name    string           `json:"name"`
	Link    string           `json:"link"`
	Showing []serpApiShowing `json:"showing"`
}

type serpApiShowing struct {
	Time []string `json:"time"`
	Type string   `json:"type"`
}

func parseResponse(body []byte) ([]Movie, error) {
	var resp serpApiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(resp.Showtimes) == 0 {
		// Try alternate format: sometimes SerpApi nests differently.
		return parseAlternateResponse(body)
	}

	movies := make(map[string]*Movie)
	var movieOrder []string

	for _, day := range resp.Showtimes {
		// Format 1: movies grouped at top level with theaters nested.
		for _, m := range day.Movies {
			movie, exists := movies[m.Name]
			if !exists {
				movie = &Movie{
					Title: m.Name,
					Link:  m.Link,
				}
				movies[m.Name] = movie
				movieOrder = append(movieOrder, m.Name)
			}

			for _, t := range m.Theaters {
				ts := TheaterShowings{
					Name:     t.Name,
					Address:  t.Address,
					Distance: t.Distance,
				}
				for _, s := range t.Showing {
					format := s.Type
					if format == "" {
						format = "Standard"
					}
					for _, timeStr := range s.Time {
						ts.Showings = append(ts.Showings, Showing{
							Time:   timeStr,
							Format: format,
						})
					}
				}
				movie.Theaters = append(movie.Theaters, ts)
			}
		}

		// Format 2: theaters grouped at top level with movies nested.
		for _, t := range day.Theaters {
			for _, m := range t.Movies {
				movie, exists := movies[m.Name]
				if !exists {
					movie = &Movie{
						Title: m.Name,
						Link:  m.Link,
					}
					movies[m.Name] = movie
					movieOrder = append(movieOrder, m.Name)
				}

				ts := TheaterShowings{
					Name:     t.Name,
					Address:  t.Address,
					Distance: t.Distance,
				}
				for _, s := range m.Showing {
					format := s.Type
					if format == "" {
						format = "Standard"
					}
					for _, timeStr := range s.Time {
						ts.Showings = append(ts.Showings, Showing{
							Time:   timeStr,
							Format: format,
						})
					}
				}
				movie.Theaters = append(movie.Theaters, ts)
			}
		}
	}

	// Convert map to ordered slice.
	result := make([]Movie, 0, len(movieOrder))
	seen := make(map[string]bool)
	for _, name := range movieOrder {
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, *movies[name])
	}
	return result, nil
}

// parseAlternateResponse handles cases where SerpApi returns the data
// in a different top-level structure.
func parseAlternateResponse(body []byte) ([]Movie, error) {
	// Try parsing as a flat structure with "showtimes_results".
	var alt struct {
		ShowtimesResults []struct {
			Day      string `json:"day"`
			Date     string `json:"date"`
			Theaters []struct {
				Name     string `json:"name"`
				Address  string `json:"address"`
				Distance string `json:"distance"`
				Movies   []struct {
					Name    string `json:"name"`
					Link    string `json:"link"`
					Showing []struct {
						Time []string `json:"time"`
						Type string   `json:"type"`
					} `json:"showing"`
				} `json:"movies"`
			} `json:"theaters"`
			Movies []struct {
				Name     string `json:"name"`
				Link     string `json:"link"`
				Theaters []struct {
					Name     string `json:"name"`
					Address  string `json:"address"`
					Distance string `json:"distance"`
					Showing  []struct {
						Time []string `json:"time"`
						Type string   `json:"type"`
					} `json:"showing"`
				} `json:"theaters"`
			} `json:"movies"`
		} `json:"showtimes_results"`
	}

	if err := json.Unmarshal(body, &alt); err != nil {
		// If we still can't parse, return what we know.
		return nil, fmt.Errorf("unrecognized API response format. Raw response (first 500 chars): %s",
			truncate(string(body), 500))
	}

	if len(alt.ShowtimesResults) == 0 {
		// Check if there's any useful data at all.
		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err == nil {
			// Look for known keys
			for _, key := range []string{"showtimes", "showtimes_results", "knowledge_graph"} {
				if _, ok := raw[key]; ok {
					return nil, fmt.Errorf("found '%s' key but could not parse its structure", key)
				}
			}
		}
		return nil, nil
	}

	movies := make(map[string]*Movie)
	var movieOrder []string

	for _, day := range alt.ShowtimesResults {
		for _, t := range day.Theaters {
			for _, m := range t.Movies {
				movie, exists := movies[m.Name]
				if !exists {
					movie = &Movie{
						Title: m.Name,
						Link:  m.Link,
					}
					movies[m.Name] = movie
					movieOrder = append(movieOrder, m.Name)
				}
				ts := TheaterShowings{
					Name:     t.Name,
					Address:  t.Address,
					Distance: t.Distance,
				}
				for _, s := range m.Showing {
					format := s.Type
					if format == "" {
						format = "Standard"
					}
					for _, timeStr := range s.Time {
						ts.Showings = append(ts.Showings, Showing{Time: timeStr, Format: format})
					}
				}
				movie.Theaters = append(movie.Theaters, ts)
			}
		}

		for _, m := range day.Movies {
			movie, exists := movies[m.Name]
			if !exists {
				movie = &Movie{
					Title: m.Name,
					Link:  m.Link,
				}
				movies[m.Name] = movie
				movieOrder = append(movieOrder, m.Name)
			}
			for _, t := range m.Theaters {
				ts := TheaterShowings{
					Name:     t.Name,
					Address:  t.Address,
					Distance: t.Distance,
				}
				for _, s := range t.Showing {
					format := s.Type
					if format == "" {
						format = "Standard"
					}
					for _, timeStr := range s.Time {
						ts.Showings = append(ts.Showings, Showing{Time: timeStr, Format: format})
					}
				}
				movie.Theaters = append(movie.Theaters, ts)
			}
		}
	}

	result := make([]Movie, 0, len(movieOrder))
	seen := make(map[string]bool)
	for _, name := range movieOrder {
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, *movies[name])
	}
	return result, nil
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

# movies

A Go CLI tool that searches for movie showtimes near a US zip code.

## Data Source

Uses a headless Chrome browser (via [Rod](https://go-rod.github.io/)) to render
Google Search showtime results. Google's showtime data requires JavaScript
rendering, so a headless browser is necessary. No API keys required.

**Note:** Rod auto-downloads Chromium (~170MB) on first run. Subsequent runs
use the cached browser.

## Build

```bash
go build -o movie-cli .
```

## Usage

```bash
# Search showtimes near a zip code (defaults to today, 20mi radius)
./movie-cli showtimes 94703

# Specify date and radius
./movie-cli showtimes 94703 --radius 20 --date 2026-04-17

# Plain text output (for piping / agent use)
./movie-cli showtimes 94703 --headless
```

## Output

Movies are grouped by title. Under each movie, theaters are listed with:
- Theater name and distance from zip
- Theater address
- Showtimes grouped by format (Standard, IMAX, Dolby Cinema, etc.)

## How It Works

The tool uses Rod (`github.com/go-rod/rod`) to launch a headless Chromium
browser with stealth settings (`github.com/go-rod/stealth`) to minimize bot
detection. It navigates to Google Search, waits for JavaScript to render the
showtime widget, then extracts structured data from the live DOM using
JavaScript evaluation.

Key implementation details:
- **Stealth mode**: Uses `go-rod/stealth` to avoid Google's bot detection
  (patches `navigator.webdriver`, plugins, languages, etc.)
- **Session establishment**: Visits `google.com` first to get cookies before
  searching
- **DOM extraction**: Runs JavaScript in the browser context to walk the
  rendered DOM and extract movie titles, theater names, addresses, and
  showtimes
- **CAPTCHA handling**: Detects Google CAPTCHA pages and returns a clear error
  message suggesting retry

## Known Limitations

- Google may rate-limit or CAPTCHA requests from server/datacenter IPs. The
  tool works best from residential IPs or when not making frequent requests.
- The DOM parser uses heuristics to extract showtime data from Google's
  rendered HTML, which may break if Google changes their page structure.

## Project Structure

```
movies/
├── cmd/
│   └── movies/
│       ├── root.go         # Root cobra command
│       └── showtimes.go    # Showtimes subcommand
├── internal/
│   ├── showtimes/
│   │   └── provider.go     # Google scraper with headless Chrome (Rod)
│   └── display/
│       └── display.go      # Output formatting
├── main.go                 # Entry point
├── go.mod
└── README.md
```

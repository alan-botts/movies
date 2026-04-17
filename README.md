# movies

A Go CLI tool that searches for movie showtimes near a US zip code.

## Data Source

Scrapes Google Search directly for showtime results, using a TLS-fingerprinted
HTTP client (same approach as the [traveler](../traveler/) tool for Google
Flights). No API keys required.

## Build

```bash
go build -o movies .
```

## Usage

```bash
# Search showtimes near a zip code (defaults to today, 20mi radius)
./movies showtimes 94703

# Specify date and radius
./movies showtimes 94703 --radius 20 --date 2026-04-17

# Plain text output (for piping / agent use)
./movies showtimes 94703 --headless
```

## Output

Movies are grouped by title. Under each movie, theaters are listed with:
- Theater name and distance from zip
- Theater address
- Showtimes grouped by format (Standard, IMAX, Dolby Cinema, etc.)

## How It Works

The tool uses `github.com/bogdanfinn/tls-client` to create an HTTP client with
a Chrome TLS fingerprint, making requests indistinguishable from a real browser.
It then parses the server-rendered HTML from Google Search to extract movie
titles, theater names, addresses, and showtimes.

Multiple parsing strategies are tried in order:
1. DOM-based extraction using `data-attrid` and class patterns
2. Script tag JSON extraction for embedded structured data
3. Regex-based fallback for aria-labels and text patterns

## Project Structure

```
movies/
├── cmd/
│   └── movies/
│       ├── root.go         # Root cobra command
│       └── showtimes.go    # Showtimes subcommand
├── internal/
│   ├── showtimes/
│   │   └── provider.go     # Google scraper with TLS fingerprinting
│   └── display/
│       └── display.go      # Output formatting
├── main.go                 # Entry point
├── go.mod
└── README.md
```
